package analyticsservices

import (
	"context"
	"sync"
	"time"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
	"github.com/movebigrocks/platform/internal/shared/geoip"
	"github.com/movebigrocks/platform/pkg/logger"
)

// IngestService handles event ingestion, visitor ID computation, and session management.
type IngestService struct {
	store  IngestStore
	geoIP  geoip.Service
	logger *logger.Logger

	// Cached property map (refreshed every 60s)
	propertyMu     sync.RWMutex
	propertyCache  map[string]*analyticsdomain.Property       // domain → property
	hostRuleCache  map[string][]*analyticsdomain.HostnameRule // property_id → rules
	cacheRefreshed time.Time

	// Salt cache
	saltMu        sync.RWMutex
	saltCache     []*analyticsdomain.Salt
	saltRefreshed time.Time
}

// NewIngestService creates a new ingest service.
func NewIngestService(store IngestStore, geo geoip.Service, log *logger.Logger) *IngestService {
	if log == nil {
		log = logger.NewNop()
	}
	if geo == nil {
		geo = geoip.NewNoopService()
	}
	return &IngestService{
		store:         store,
		geoIP:         geo,
		logger:        log,
		propertyCache: make(map[string]*analyticsdomain.Property),
		hostRuleCache: make(map[string][]*analyticsdomain.HostnameRule),
	}
}

// IngestRequest represents a parsed ingest request with server-side context.
type IngestRequest struct {
	EventName  string
	URL        string
	Domain     string
	Referrer   string
	UserAgent  string
	RemoteIP   string
	AcceptLang string
}

// Ingest processes an analytics event from the tracking script.
func (s *IngestService) Ingest(ctx context.Context, req *IngestRequest) error {
	// Lookup property by domain (cached)
	prop := s.getCachedProperty(ctx, req.Domain)
	if prop == nil {
		return nil // Unknown domain — silently drop
	}

	if prop.IsPaused() {
		return nil // Paused — silently drop
	}

	// Check hostname shield
	rules := s.getCachedHostnameRules(ctx, prop.ID)
	if len(rules) > 0 {
		hostname := extractHostname(req.URL)
		if !analyticsdomain.MatchesAnyHostnameRule(rules, hostname) {
			return nil // Hostname mismatch — silently drop
		}
	}

	// Get salts for visitor ID computation
	salts := s.getCachedSalts(ctx)
	if len(salts) == 0 {
		s.logger.Warn("No salts available for visitor ID generation")
		return nil
	}

	// Compute visitor ID with current salt
	currentVisitorID := analyticsdomain.GenerateVisitorID(salts[0].Salt, req.UserAgent, req.RemoteIP, req.Domain)

	// Compute visitor IDs for both salts (for session continuity across rotation)
	visitorIDs := []int64{currentVisitorID}
	if len(salts) > 1 {
		prevVisitorID := analyticsdomain.GenerateVisitorID(salts[1].Salt, req.UserAgent, req.RemoteIP, req.Domain)
		if prevVisitorID != currentVisitorID {
			visitorIDs = append(visitorIDs, prevVisitorID)
		}
	}

	// Parse URL
	pathname, utmSource, utmMedium, utmCampaign := parseURL(req.URL)

	// Classify referrer
	referrerSource := ClassifySource(utmSource, req.Referrer, req.Domain)

	// Internal nav produces empty source — use Direct only if no referrer at all
	if referrerSource == "" && req.Referrer != "" {
		referrerSource = "" // Internal nav stays empty
	} else if referrerSource == "" {
		referrerSource = "Direct"
	}

	// Resolve location from IP via GeoIP database
	loc := s.geoIP.Lookup(req.RemoteIP)
	countryCode := loc.CountryCode
	region := loc.Region
	city := loc.City

	// Fallback to Accept-Language if GeoIP returns no country
	if countryCode == "" {
		countryCode = CountryFromLanguage(req.AcceptLang)
	}

	// Parse User-Agent
	browser, os, deviceType := ParseUA(req.UserAgent)

	now := time.Now().UTC()

	// UPSERT session
	sessionCutoff := now.Add(-30 * time.Minute)
	existingSession, err := s.store.FindRecentSession(ctx, prop.ID, visitorIDs, sessionCutoff)
	if err != nil {
		s.logger.Warn("Failed to find session", "error", err)
	}

	if existingSession != nil {
		existingSession.RecordActivity(req.EventName, pathname, now)
		if err := s.store.UpdateSession(ctx, existingSession); err != nil {
			s.logger.Warn("Failed to update session", "error", err)
		}
	} else {
		newSession := analyticsdomain.NewSessionFromIngest(analyticsdomain.SessionParams{
			PropertyID:     prop.ID,
			VisitorID:      currentVisitorID,
			Pathname:       pathname,
			ReferrerSource: referrerSource,
			UTMSource:      utmSource,
			UTMMedium:      utmMedium,
			UTMCampaign:    utmCampaign,
			CountryCode:    countryCode,
			Region:         region,
			City:           city,
			Browser:        browser,
			OS:             os,
			DeviceType:     deviceType,
			StartedAt:      now,
			EventName:      req.EventName,
		})
		if err := s.store.InsertSession(ctx, newSession); err != nil {
			s.logger.Warn("Failed to insert session", "error", err)
		}
	}

	// INSERT event
	event := &analyticsdomain.AnalyticsEvent{
		PropertyID:     prop.ID,
		VisitorID:      currentVisitorID,
		Name:           req.EventName,
		Pathname:       pathname,
		ReferrerSource: referrerSource,
		UTMSource:      utmSource,
		UTMMedium:      utmMedium,
		UTMCampaign:    utmCampaign,
		CountryCode:    countryCode,
		Region:         region,
		City:           city,
		Browser:        browser,
		OS:             os,
		DeviceType:     deviceType,
		Timestamp:      now,
	}
	if err := s.store.InsertEvent(ctx, event); err != nil {
		s.logger.Warn("Failed to insert event", "error", err)
		return err
	}

	// Mark property verified on first event
	if !prop.IsVerified() {
		prop.MarkVerified()
		if err := s.store.UpdateProperty(ctx, prop); err != nil {
			s.logger.Warn("Failed to mark property verified", "error", err)
		}
	}

	return nil
}

// RefreshCaches forces a refresh of the property and salt caches.
func (s *IngestService) RefreshCaches(ctx context.Context) {
	s.refreshPropertyCache(ctx)
	s.refreshSaltCache(ctx)
}

func (s *IngestService) getCachedProperty(ctx context.Context, domain string) *analyticsdomain.Property {
	s.propertyMu.RLock()
	if time.Since(s.cacheRefreshed) < 60*time.Second {
		prop := s.propertyCache[domain]
		s.propertyMu.RUnlock()
		return prop
	}
	s.propertyMu.RUnlock()

	s.refreshPropertyCache(ctx)

	s.propertyMu.RLock()
	defer s.propertyMu.RUnlock()
	return s.propertyCache[domain]
}

func (s *IngestService) getCachedHostnameRules(ctx context.Context, propertyID string) []*analyticsdomain.HostnameRule {
	s.propertyMu.RLock()
	rules := s.hostRuleCache[propertyID]
	s.propertyMu.RUnlock()
	return rules
}

func (s *IngestService) getCachedSalts(ctx context.Context) []*analyticsdomain.Salt {
	s.saltMu.RLock()
	if time.Since(s.saltRefreshed) < 60*time.Second && len(s.saltCache) > 0 {
		salts := s.saltCache
		s.saltMu.RUnlock()
		return salts
	}
	s.saltMu.RUnlock()

	s.refreshSaltCache(ctx)

	s.saltMu.RLock()
	defer s.saltMu.RUnlock()
	return s.saltCache
}

func (s *IngestService) refreshPropertyCache(ctx context.Context) {
	props, err := s.store.ListAllProperties(ctx)
	if err != nil {
		s.logger.Warn("Failed to refresh property cache", "error", err)
		return
	}

	cache := make(map[string]*analyticsdomain.Property, len(props))
	for _, p := range props {
		cache[p.Domain] = p
	}

	// Refresh hostname rules for all properties
	ruleCache := make(map[string][]*analyticsdomain.HostnameRule)
	for _, p := range props {
		rules, err := s.store.ListHostnameRulesByProperty(ctx, p.ID)
		if err != nil {
			s.logger.Warn("Failed to load hostname rules", "property", p.ID, "error", err)
			continue
		}
		if len(rules) > 0 {
			ruleCache[p.ID] = rules
		}
	}

	s.propertyMu.Lock()
	s.propertyCache = cache
	s.hostRuleCache = ruleCache
	s.cacheRefreshed = time.Now()
	s.propertyMu.Unlock()
}

func (s *IngestService) refreshSaltCache(ctx context.Context) {
	salts, err := s.store.GetCurrentSalts(ctx)
	if err != nil {
		s.logger.Warn("Failed to refresh salt cache", "error", err)
		return
	}

	s.saltMu.Lock()
	s.saltCache = salts
	s.saltRefreshed = time.Now()
	s.saltMu.Unlock()
}

// parseURL extracts pathname and UTM parameters from a URL string.
func parseURL(rawURL string) (pathname, utmSource, utmMedium, utmCampaign string) {
	return analyticsdomain.ParseTrackedURL(rawURL)
}

// extractHostname extracts the hostname from a URL string.
func extractHostname(rawURL string) string {
	return analyticsdomain.ExtractTrackedHostname(rawURL)
}
