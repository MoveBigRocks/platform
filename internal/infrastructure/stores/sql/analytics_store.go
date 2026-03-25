package sql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
)

const webAnalyticsExtensionSlug = "web-analytics"

// AnalyticsStore implements analytics CRUD operations against the
// ext_demandops_web_analytics schema in PostgreSQL.
type AnalyticsStore struct {
	db     *sqlx.DB
	schema string
}

// NewAnalyticsStore creates a new analytics store with the given analytics database connection.
func NewAnalyticsStore(adb *AnalyticsDB) *AnalyticsStore {
	return &AnalyticsStore{db: adb.Sqlx(), schema: adb.Schema()}
}

func (s *AnalyticsStore) query(query string) string {
	query = strings.ReplaceAll(query, "${SCHEMA_NAME}", s.schema)
	return s.db.Rebind(query)
}

func (s *AnalyticsStore) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.query(query), args...)
}

func (s *AnalyticsStore) selectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return s.db.SelectContext(ctx, dest, s.query(query), args...)
}

func (s *AnalyticsStore) queryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return s.db.QueryRowxContext(ctx, s.query(query), args...)
}

func (s *AnalyticsStore) begin(ctx context.Context) (*sqlx.Tx, error) {
	return s.db.BeginTxx(ctx, nil)
}

func (s *AnalyticsStore) lookupInstallIDForWorkspace(ctx context.Context, workspaceID string) (string, error) {
	var installID string
	err := s.queryRowxContext(ctx,
		`SELECT id
		 FROM core_platform.installed_extensions
		 WHERE workspace_id = ? AND slug = ? AND deleted_at IS NULL
		 ORDER BY activated_at DESC NULLS LAST, installed_at DESC
		 LIMIT 1`,
		workspaceID, webAnalyticsExtensionSlug).Scan(&installID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("web analytics extension is not installed for workspace %s", workspaceID)
		}
		return "", err
	}
	return installID, nil
}

// --- Internal row types for DB scanning ---

type propertyRow struct {
	ID          string     `db:"id"`
	WorkspaceID string     `db:"workspace_id"`
	Domain      string     `db:"domain"`
	Timezone    string     `db:"timezone"`
	Status      string     `db:"status"`
	VerifiedAt  *time.Time `db:"verified_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (r *propertyRow) toDomain() *analyticsdomain.Property {
	return &analyticsdomain.Property{
		ID: r.ID, WorkspaceID: r.WorkspaceID, Domain: r.Domain,
		Timezone: r.Timezone, Status: r.Status, VerifiedAt: r.VerifiedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type goalRow struct {
	ID         string    `db:"id"`
	PropertyID string    `db:"property_id"`
	GoalType   string    `db:"goal_type"`
	EventName  string    `db:"event_name"`
	PagePath   string    `db:"page_path"`
	CreatedAt  time.Time `db:"created_at"`
}

func (r *goalRow) toDomain() *analyticsdomain.Goal {
	return &analyticsdomain.Goal{
		ID: r.ID, PropertyID: r.PropertyID, GoalType: r.GoalType,
		EventName: r.EventName, PagePath: r.PagePath, CreatedAt: r.CreatedAt,
	}
}

type saltRow struct {
	ID        int       `db:"id"`
	Salt      []byte    `db:"salt"`
	CreatedAt time.Time `db:"created_at"`
}

func (r *saltRow) toDomain() *analyticsdomain.Salt {
	return &analyticsdomain.Salt{ID: r.ID, Salt: r.Salt, CreatedAt: r.CreatedAt}
}

type sessionRow struct {
	SessionID      int64     `db:"session_id"`
	PropertyID     string    `db:"property_id"`
	VisitorID      int64     `db:"visitor_id"`
	EntryPage      string    `db:"entry_page"`
	ExitPage       string    `db:"exit_page"`
	ReferrerSource string    `db:"referrer_source"`
	UTMSource      string    `db:"utm_source"`
	UTMMedium      string    `db:"utm_medium"`
	UTMCampaign    string    `db:"utm_campaign"`
	CountryCode    string    `db:"country_code"`
	Region         string    `db:"region"`
	City           string    `db:"city"`
	Browser        string    `db:"browser"`
	OS             string    `db:"os"`
	DeviceType     string    `db:"device_type"`
	StartedAt      time.Time `db:"started_at"`
	LastActivity   time.Time `db:"last_activity"`
	Duration       int       `db:"duration"`
	Pageviews      int       `db:"pageviews"`
	IsBounce       bool      `db:"is_bounce"`
}

func (r *sessionRow) toDomain() *analyticsdomain.Session {
	return &analyticsdomain.Session{
		SessionID: r.SessionID, PropertyID: r.PropertyID, VisitorID: r.VisitorID,
		EntryPage: r.EntryPage, ExitPage: r.ExitPage,
		ReferrerSource: r.ReferrerSource, UTMSource: r.UTMSource,
		UTMMedium: r.UTMMedium, UTMCampaign: r.UTMCampaign,
		CountryCode: r.CountryCode, Region: r.Region, City: r.City,
		Browser: r.Browser, OS: r.OS, DeviceType: r.DeviceType,
		StartedAt: r.StartedAt, LastActivity: r.LastActivity,
		Duration: r.Duration, Pageviews: r.Pageviews, IsBounce: boolToInt(r.IsBounce),
	}
}

type hostnameRuleRow struct {
	ID         string    `db:"id"`
	PropertyID string    `db:"property_id"`
	Pattern    string    `db:"pattern"`
	CreatedAt  time.Time `db:"created_at"`
}

func (r *hostnameRuleRow) toDomain() *analyticsdomain.HostnameRule {
	return &analyticsdomain.HostnameRule{
		ID: r.ID, PropertyID: r.PropertyID, Pattern: r.Pattern, CreatedAt: r.CreatedAt,
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// --- Properties ---

func (s *AnalyticsStore) CreateProperty(ctx context.Context, p *analyticsdomain.Property) error {
	installID, err := s.lookupInstallIDForWorkspace(ctx, p.WorkspaceID)
	if err != nil {
		return err
	}
	normalizePersistedUUID(&p.ID)
	err = s.queryRowxContext(ctx,
		`INSERT INTO ${SCHEMA_NAME}.properties (
			id, workspace_id, extension_install_id, domain, timezone, status, verified_at, created_at, updated_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`,
		p.ID, p.WorkspaceID, installID, p.Domain, p.Timezone, p.Status, p.VerifiedAt, p.CreatedAt, p.UpdatedAt).Scan(&p.ID)
	return err
}

func (s *AnalyticsStore) GetProperty(ctx context.Context, propertyID string) (*analyticsdomain.Property, error) {
	var row propertyRow
	err := s.queryRowxContext(ctx,
		`SELECT id, workspace_id, domain, timezone, status, verified_at, created_at, updated_at
		 FROM ${SCHEMA_NAME}.properties WHERE id = ?`,
		propertyID).StructScan(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("property not found")
		}
		return nil, err
	}
	return row.toDomain(), nil
}

func (s *AnalyticsStore) GetPropertyByDomain(ctx context.Context, domain string) (*analyticsdomain.Property, error) {
	var row propertyRow
	err := s.queryRowxContext(ctx,
		`SELECT id, workspace_id, domain, timezone, status, verified_at, created_at, updated_at
		 FROM ${SCHEMA_NAME}.properties WHERE LOWER(domain) = LOWER(?)`,
		domain).StructScan(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("property not found")
		}
		return nil, err
	}
	return row.toDomain(), nil
}

func (s *AnalyticsStore) ListPropertiesByWorkspace(ctx context.Context, workspaceID string) ([]*analyticsdomain.Property, error) {
	var rows []propertyRow
	err := s.selectContext(ctx, &rows,
		`SELECT id, workspace_id, domain, timezone, status, verified_at, created_at, updated_at
		 FROM ${SCHEMA_NAME}.properties WHERE workspace_id = ? ORDER BY created_at DESC`,
		workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]*analyticsdomain.Property, len(rows))
	for i := range rows {
		result[i] = rows[i].toDomain()
	}
	return result, nil
}

func (s *AnalyticsStore) ListAllProperties(ctx context.Context) ([]*analyticsdomain.Property, error) {
	var rows []propertyRow
	err := s.selectContext(ctx, &rows,
		`SELECT id, workspace_id, domain, timezone, status, verified_at, created_at, updated_at
		 FROM ${SCHEMA_NAME}.properties ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	result := make([]*analyticsdomain.Property, len(rows))
	for i := range rows {
		result[i] = rows[i].toDomain()
	}
	return result, nil
}

func (s *AnalyticsStore) UpdateProperty(ctx context.Context, p *analyticsdomain.Property) error {
	p.UpdatedAt = time.Now().UTC()
	_, err := s.execContext(ctx,
		`UPDATE ${SCHEMA_NAME}.properties
		 SET domain = ?, timezone = ?, status = ?, verified_at = ?, updated_at = ? WHERE id = ?`,
		p.Domain, p.Timezone, p.Status, p.VerifiedAt, p.UpdatedAt, p.ID)
	return err
}

func (s *AnalyticsStore) DeleteProperty(ctx context.Context, propertyID string) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	deletes := []string{
		s.query(`DELETE FROM ${SCHEMA_NAME}.events WHERE property_id = ?`),
		s.query(`DELETE FROM ${SCHEMA_NAME}.sessions WHERE property_id = ?`),
		s.query(`DELETE FROM ${SCHEMA_NAME}.goals WHERE property_id = ?`),
		s.query(`DELETE FROM ${SCHEMA_NAME}.hostname_rules WHERE property_id = ?`),
		s.query(`DELETE FROM ${SCHEMA_NAME}.properties WHERE id = ?`),
	}
	for _, q := range deletes {
		if _, err := tx.ExecContext(ctx, q, propertyID); err != nil {
			return fmt.Errorf("delete property: %w", err)
		}
	}

	return tx.Commit()
}

// --- Events ---

func (s *AnalyticsStore) InsertEvent(ctx context.Context, e *analyticsdomain.AnalyticsEvent) error {
	result, err := s.execContext(ctx,
		`INSERT INTO ${SCHEMA_NAME}.events (
			workspace_id, extension_install_id, property_id, visitor_id, name, pathname,
			referrer_source, utm_source, utm_medium, utm_campaign, country_code,
			region, city, browser, os, device_type, timestamp
		)
		SELECT
			p.workspace_id,
			p.extension_install_id,
			p.id,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
		FROM ${SCHEMA_NAME}.properties p
		WHERE p.id = ?`,
		e.VisitorID, e.Name, e.Pathname, e.ReferrerSource,
		e.UTMSource, e.UTMMedium, e.UTMCampaign, e.CountryCode, e.Region, e.City,
		e.Browser, e.OS, e.DeviceType, e.Timestamp, e.PropertyID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return fmt.Errorf("property not found")
	}
	return err
}

func (s *AnalyticsStore) HasEventsForProperty(ctx context.Context, propertyID string) (bool, error) {
	var count int
	err := s.queryRowxContext(ctx,
		`SELECT COUNT(*) FROM ${SCHEMA_NAME}.events WHERE property_id = ? LIMIT 1`, propertyID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *AnalyticsStore) DeleteEventsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.events WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *AnalyticsStore) DeleteEventsByProperty(ctx context.Context, propertyID string) error {
	_, err := s.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.events WHERE property_id = ?`, propertyID)
	return err
}

// --- Sessions ---

func (s *AnalyticsStore) FindRecentSession(ctx context.Context, propertyID string, visitorIDs []int64, cutoff time.Time) (*analyticsdomain.Session, error) {
	if len(visitorIDs) == 0 {
		return nil, fmt.Errorf("no visitor IDs provided")
	}

	query := `SELECT session_id, property_id, visitor_id, entry_page, exit_page,
		referrer_source, utm_source, utm_medium, utm_campaign,
		country_code, region, city, browser, os, device_type,
		started_at, last_activity, duration, pageviews, is_bounce
		FROM ${SCHEMA_NAME}.sessions
		WHERE property_id = ? AND visitor_id IN (?, ?) AND last_activity > ?
		ORDER BY last_activity DESC LIMIT 1`

	var v1, v2 int64
	v1 = visitorIDs[0]
	if len(visitorIDs) > 1 {
		v2 = visitorIDs[1]
	} else {
		v2 = v1
	}

	var row sessionRow
	err := s.queryRowxContext(ctx, query, propertyID, v1, v2, cutoff).StructScan(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return row.toDomain(), nil
}

func (s *AnalyticsStore) InsertSession(ctx context.Context, sess *analyticsdomain.Session) error {
	result, err := s.execContext(ctx,
		`INSERT INTO ${SCHEMA_NAME}.sessions (
			session_id, workspace_id, extension_install_id, property_id, visitor_id, entry_page, exit_page,
			referrer_source, utm_source, utm_medium, utm_campaign,
			country_code, region, city, browser, os, device_type,
			started_at, last_activity, duration, pageviews, is_bounce
		)
		SELECT
			?,
			p.workspace_id,
			p.extension_install_id,
			p.id,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
		FROM ${SCHEMA_NAME}.properties p
		WHERE p.id = ?`,
		sess.SessionID, sess.VisitorID, sess.EntryPage, sess.ExitPage,
		sess.ReferrerSource, sess.UTMSource, sess.UTMMedium, sess.UTMCampaign,
		sess.CountryCode, sess.Region, sess.City, sess.Browser, sess.OS, sess.DeviceType,
		sess.StartedAt, sess.LastActivity, sess.Duration, sess.Pageviews, sess.IsBounce != 0, sess.PropertyID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return fmt.Errorf("property not found")
	}
	return err
}

func (s *AnalyticsStore) UpdateSession(ctx context.Context, sess *analyticsdomain.Session) error {
	_, err := s.execContext(ctx,
		`UPDATE ${SCHEMA_NAME}.sessions SET exit_page = ?, last_activity = ?, duration = ?, pageviews = ?, is_bounce = ?
		 WHERE session_id = ? AND property_id = ?`,
		sess.ExitPage, sess.LastActivity, sess.Duration, sess.Pageviews, sess.IsBounce != 0,
		sess.SessionID, sess.PropertyID)
	return err
}

func (s *AnalyticsStore) DeleteSessionsByProperty(ctx context.Context, propertyID string) error {
	_, err := s.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.sessions WHERE property_id = ?`, propertyID)
	return err
}

func (s *AnalyticsStore) DeleteSessionsOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.sessions WHERE last_activity < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GenerateSessionID creates a random int64 for session identification.
// Uses int64 (reinterpreted from uint64) in a SQLite-friendly form.
func GenerateSessionID() int64 {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return int64(binary.LittleEndian.Uint64(buf[:]))
}

// --- Goals ---

func (s *AnalyticsStore) CreateGoal(ctx context.Context, g *analyticsdomain.Goal) error {
	normalizePersistedUUID(&g.ID)
	err := s.queryRowxContext(ctx,
		`INSERT INTO ${SCHEMA_NAME}.goals (
			id, workspace_id, extension_install_id, property_id, goal_type, event_name, page_path, created_at
		)
		SELECT COALESCE(NULLIF(?, '')::uuid, uuidv7()), p.workspace_id, p.extension_install_id, p.id, ?, ?, ?, ?
		FROM ${SCHEMA_NAME}.properties p
		WHERE p.id = ?
		RETURNING id`,
		g.ID, g.GoalType, g.EventName, g.PagePath, g.CreatedAt, g.PropertyID).Scan(&g.ID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("property not found")
	}
	return err
}

func (s *AnalyticsStore) GetGoal(ctx context.Context, goalID string) (*analyticsdomain.Goal, error) {
	var row goalRow
	err := s.queryRowxContext(ctx,
		`SELECT id, property_id, goal_type, event_name, page_path, created_at
		 FROM ${SCHEMA_NAME}.goals WHERE id = ?`,
		goalID).StructScan(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("goal not found")
		}
		return nil, err
	}
	return row.toDomain(), nil
}

func (s *AnalyticsStore) ListGoalsByProperty(ctx context.Context, propertyID string) ([]*analyticsdomain.Goal, error) {
	var rows []goalRow
	err := s.selectContext(ctx, &rows,
		`SELECT id, property_id, goal_type, event_name, page_path, created_at
		 FROM ${SCHEMA_NAME}.goals WHERE property_id = ? ORDER BY created_at`,
		propertyID)
	if err != nil {
		return nil, err
	}
	result := make([]*analyticsdomain.Goal, len(rows))
	for i := range rows {
		result[i] = rows[i].toDomain()
	}
	return result, nil
}

func (s *AnalyticsStore) CountGoalsByProperty(ctx context.Context, propertyID string) (int, error) {
	var count int
	err := s.queryRowxContext(ctx, `SELECT COUNT(*) FROM ${SCHEMA_NAME}.goals WHERE property_id = ?`, propertyID).Scan(&count)
	return count, err
}

func (s *AnalyticsStore) DeleteGoal(ctx context.Context, goalID string) error {
	_, err := s.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.goals WHERE id = ?`, goalID)
	return err
}

// --- Hostname Rules ---

func (s *AnalyticsStore) CreateHostnameRule(ctx context.Context, r *analyticsdomain.HostnameRule) error {
	normalizePersistedUUID(&r.ID)
	err := s.queryRowxContext(ctx,
		`INSERT INTO ${SCHEMA_NAME}.hostname_rules (
			id, workspace_id, extension_install_id, property_id, pattern, created_at
		)
		SELECT COALESCE(NULLIF(?, '')::uuid, uuidv7()), p.workspace_id, p.extension_install_id, p.id, ?, ?
		FROM ${SCHEMA_NAME}.properties p
		WHERE p.id = ?
		RETURNING id`,
		r.ID, r.Pattern, r.CreatedAt, r.PropertyID).Scan(&r.ID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("property not found")
	}
	return err
}

func (s *AnalyticsStore) ListHostnameRulesByProperty(ctx context.Context, propertyID string) ([]*analyticsdomain.HostnameRule, error) {
	var rows []hostnameRuleRow
	err := s.selectContext(ctx, &rows,
		`SELECT id, property_id, pattern, created_at
		 FROM ${SCHEMA_NAME}.hostname_rules WHERE property_id = ? ORDER BY created_at`,
		propertyID)
	if err != nil {
		return nil, err
	}
	result := make([]*analyticsdomain.HostnameRule, len(rows))
	for i := range rows {
		result[i] = rows[i].toDomain()
	}
	return result, nil
}

func (s *AnalyticsStore) CountHostnameRulesByProperty(ctx context.Context, propertyID string) (int, error) {
	var count int
	err := s.queryRowxContext(ctx, `SELECT COUNT(*) FROM ${SCHEMA_NAME}.hostname_rules WHERE property_id = ?`, propertyID).Scan(&count)
	return count, err
}

func (s *AnalyticsStore) GetHostnameRule(ctx context.Context, ruleID string) (*analyticsdomain.HostnameRule, error) {
	var row hostnameRuleRow
	err := s.queryRowxContext(ctx,
		`SELECT id, property_id, pattern, created_at FROM ${SCHEMA_NAME}.hostname_rules WHERE id = ?`, ruleID).StructScan(&row)
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (s *AnalyticsStore) DeleteHostnameRule(ctx context.Context, ruleID string) error {
	_, err := s.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.hostname_rules WHERE id = ?`, ruleID)
	return err
}

// --- Salts ---

func (s *AnalyticsStore) InsertSalt(ctx context.Context, salt *analyticsdomain.Salt) error {
	_, err := s.execContext(ctx,
		`INSERT INTO ${SCHEMA_NAME}.salts (salt, created_at) VALUES (?, ?)`,
		salt.Salt, salt.CreatedAt)
	return err
}

func (s *AnalyticsStore) GetCurrentSalts(ctx context.Context) ([]*analyticsdomain.Salt, error) {
	var rows []saltRow
	err := s.selectContext(ctx, &rows,
		`SELECT id, salt, created_at FROM ${SCHEMA_NAME}.salts ORDER BY created_at DESC LIMIT 2`)
	if err != nil {
		return nil, err
	}
	result := make([]*analyticsdomain.Salt, len(rows))
	for i := range rows {
		result[i] = rows[i].toDomain()
	}
	return result, nil
}

func (s *AnalyticsStore) DeleteSaltsOlderThan(ctx context.Context, cutoff time.Time) error {
	_, err := s.execContext(ctx, `DELETE FROM ${SCHEMA_NAME}.salts WHERE created_at < ?`, cutoff)
	return err
}

// --- Query methods for dashboard ---

type metricsResultRow struct {
	UniqueVisitors   int     `db:"unique_visitors"`
	TotalVisits      int     `db:"total_visits"`
	TotalPageviews   int     `db:"total_pageviews"`
	ViewsPerVisit    float64 `db:"views_per_visit"`
	BounceRate       float64 `db:"bounce_rate"`
	AvgVisitDuration int     `db:"avg_visit_duration"`
}

func (r *metricsResultRow) toDomain() *analyticsdomain.Metrics {
	return &analyticsdomain.Metrics{
		UniqueVisitors:   r.UniqueVisitors,
		TotalVisits:      r.TotalVisits,
		TotalPageviews:   r.TotalPageviews,
		ViewsPerVisit:    r.ViewsPerVisit,
		BounceRate:       r.BounceRate,
		AvgVisitDuration: r.AvgVisitDuration,
	}
}

func (s *AnalyticsStore) GetMetrics(ctx context.Context, propertyID string, from, to time.Time) (*analyticsdomain.Metrics, error) {
	var result metricsResultRow

	err := s.queryRowxContext(ctx,
		`SELECT
			COALESCE(COUNT(DISTINCT visitor_id), 0) as unique_visitors,
			COALESCE(SUM(CASE WHEN name = 'pageview' THEN 1 ELSE 0 END), 0) as total_pageviews
		 FROM ${SCHEMA_NAME}.events
		 WHERE property_id = ? AND timestamp >= ? AND timestamp < ?`,
		propertyID, from, to).StructScan(&result)
	if err != nil {
		return nil, err
	}

	var sessMetrics struct {
		TotalVisits int     `db:"total_visits"`
		AvgBounce   float64 `db:"avg_bounce"`
		AvgDuration float64 `db:"avg_duration"`
	}
	err = s.queryRowxContext(ctx,
		`SELECT
			COALESCE(COUNT(*), 0) as total_visits,
			COALESCE(AVG(CASE WHEN is_bounce THEN 1.0 ELSE 0.0 END), 0) as avg_bounce,
			COALESCE(AVG(duration), 0) as avg_duration
		 FROM ${SCHEMA_NAME}.sessions
		 WHERE property_id = ? AND started_at >= ? AND started_at < ?`,
		propertyID, from, to).StructScan(&sessMetrics)
	if err != nil {
		return nil, err
	}

	result.TotalVisits = sessMetrics.TotalVisits
	result.BounceRate = sessMetrics.AvgBounce
	result.AvgVisitDuration = int(sessMetrics.AvgDuration)

	if result.TotalVisits > 0 {
		result.ViewsPerVisit = float64(result.TotalPageviews) / float64(result.TotalVisits)
	}

	return result.toDomain(), nil
}

type timeSeriesRow struct {
	Date      string `db:"date"`
	Visitors  int    `db:"visitors"`
	Pageviews int    `db:"pageviews"`
}

func (r *timeSeriesRow) toDomain() *analyticsdomain.TimeSeriesPoint {
	return &analyticsdomain.TimeSeriesPoint{
		Date:      r.Date,
		Visitors:  r.Visitors,
		Pageviews: r.Pageviews,
	}
}

func (s *AnalyticsStore) GetTimeSeries(ctx context.Context, propertyID string, from, to time.Time, interval string) ([]*analyticsdomain.TimeSeriesPoint, error) {
	var bucketExpr string
	switch interval {
	case "hour":
		bucketExpr = `TO_CHAR(date_trunc('hour', timestamp AT TIME ZONE 'UTC'), 'YYYY-MM-DD"T"HH24:00:00"Z"')`
	default:
		bucketExpr = `TO_CHAR(date_trunc('day', timestamp AT TIME ZONE 'UTC'), 'YYYY-MM-DD')`
	}

	var rows []*timeSeriesRow
	err := s.selectContext(ctx, &rows,
		fmt.Sprintf(`SELECT
			%s as date,
			COUNT(DISTINCT visitor_id) as visitors,
			SUM(CASE WHEN name = 'pageview' THEN 1 ELSE 0 END) as pageviews
		 FROM ${SCHEMA_NAME}.events
		 WHERE property_id = ? AND timestamp >= ? AND timestamp < ?
		 GROUP BY date
		 ORDER BY date`, bucketExpr),
		propertyID, from, to)
	if err != nil {
		return nil, err
	}

	result := make([]*analyticsdomain.TimeSeriesPoint, len(rows))
	for i := range rows {
		result[i] = rows[i].toDomain()
	}
	return result, nil
}

type breakdownRow struct {
	Name      string `db:"name"`
	Visitors  int    `db:"visitors"`
	Pageviews *int   `db:"pageviews"`
}

func (r *breakdownRow) toDomain() *analyticsdomain.BreakdownRow {
	return &analyticsdomain.BreakdownRow{
		Name:      r.Name,
		Visitors:  r.Visitors,
		Pageviews: r.Pageviews,
	}
}

func (s *AnalyticsStore) GetBreakdown(ctx context.Context, propertyID string, from, to time.Time, dimension string, limit int) ([]*analyticsdomain.BreakdownRow, error) {
	var column string
	switch dimension {
	case "PAGE":
		column = "pathname"
	case "SOURCE":
		column = "referrer_source"
	case "COUNTRY":
		column = "country_code"
	case "REGION":
		column = "region"
	case "CITY":
		column = "city"
	case "BROWSER":
		column = "browser"
	case "OS":
		column = "os"
	case "DEVICE":
		column = "device_type"
	default:
		return nil, fmt.Errorf("unknown dimension: %s", dimension)
	}

	includePageviews := dimension == "PAGE"

	var query string
	if includePageviews {
		query = fmt.Sprintf(
			`SELECT %s as name,
				COUNT(DISTINCT visitor_id) as visitors,
				SUM(CASE WHEN name = 'pageview' THEN 1 ELSE 0 END) as pageviews
			 FROM ${SCHEMA_NAME}.events
			 WHERE property_id = ? AND timestamp >= ? AND timestamp < ? AND %s != ''
			 GROUP BY %s
			 ORDER BY visitors DESC
			 LIMIT ?`, column, column, column)
	} else {
		query = fmt.Sprintf(
			`SELECT %s as name,
				COUNT(DISTINCT visitor_id) as visitors,
				NULL as pageviews
			 FROM ${SCHEMA_NAME}.events
			 WHERE property_id = ? AND timestamp >= ? AND timestamp < ? AND %s != ''
			 GROUP BY %s
			 ORDER BY visitors DESC
			 LIMIT ?`, column, column, column)
	}

	var rows []*breakdownRow
	err := s.selectContext(ctx, &rows, query, propertyID, from, to, limit)
	if err != nil {
		return nil, err
	}

	result := make([]*analyticsdomain.BreakdownRow, len(rows))
	for i := range rows {
		result[i] = rows[i].toDomain()
	}
	return result, nil
}

func (s *AnalyticsStore) GetGoalResults(ctx context.Context, propertyID string, from, to time.Time) ([]*analyticsdomain.GoalResult, error) {
	var totalVisitors int
	err := s.queryRowxContext(ctx,
		`SELECT COUNT(DISTINCT visitor_id) FROM ${SCHEMA_NAME}.events WHERE property_id = ? AND timestamp >= ? AND timestamp < ?`,
		propertyID, from, to).Scan(&totalVisitors)
	if err != nil {
		return nil, err
	}

	goals, err := s.ListGoalsByProperty(ctx, propertyID)
	if err != nil {
		return nil, err
	}

	var results []*analyticsdomain.GoalResult
	for _, goal := range goals {
		var uniques, total int
		switch goal.GoalType {
		case "event":
			err = s.queryRowxContext(ctx,
				`SELECT COUNT(DISTINCT visitor_id), COUNT(*) FROM ${SCHEMA_NAME}.events
				 WHERE property_id = ? AND timestamp >= ? AND timestamp < ? AND name = ?`,
				propertyID, from, to, goal.EventName).Scan(&uniques, &total)
		case "page":
			err = s.queryRowxContext(ctx,
				`SELECT COUNT(DISTINCT visitor_id), COUNT(*) FROM ${SCHEMA_NAME}.events
				 WHERE property_id = ? AND timestamp >= ? AND timestamp < ? AND name = 'pageview' AND pathname = ?`,
				propertyID, from, to, goal.PagePath).Scan(&uniques, &total)
		}
		if err != nil {
			return nil, err
		}

		var cr float64
		if totalVisitors > 0 {
			cr = float64(uniques) / float64(totalVisitors)
		}

		results = append(results, &analyticsdomain.GoalResult{
			GoalID:         goal.ID,
			Uniques:        uniques,
			Total:          total,
			ConversionRate: cr,
		})
	}

	return results, nil
}

func (s *AnalyticsStore) GetCurrentVisitors(ctx context.Context, propertyID string) (int, error) {
	var count int
	cutoff := time.Now().UTC().Add(-5 * time.Minute)
	err := s.queryRowxContext(ctx,
		`SELECT COUNT(DISTINCT visitor_id) FROM ${SCHEMA_NAME}.events WHERE property_id = ? AND timestamp > ?`,
		propertyID, cutoff).Scan(&count)
	return count, err
}

func (s *AnalyticsStore) GetVisitorsLast24h(ctx context.Context, propertyID string) (int, error) {
	var count int
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	err := s.queryRowxContext(ctx,
		`SELECT COUNT(DISTINCT visitor_id) FROM ${SCHEMA_NAME}.events WHERE property_id = ? AND timestamp > ?`,
		propertyID, cutoff).Scan(&count)
	return count, err
}

func (s *AnalyticsStore) ResetPropertyStats(ctx context.Context, propertyID string) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, s.query(`DELETE FROM ${SCHEMA_NAME}.events WHERE property_id = ?`), propertyID); err != nil {
		return fmt.Errorf("reset events: %w", err)
	}
	if _, err := tx.ExecContext(ctx, s.query(`DELETE FROM ${SCHEMA_NAME}.sessions WHERE property_id = ?`), propertyID); err != nil {
		return fmt.Errorf("reset sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, s.query(`UPDATE ${SCHEMA_NAME}.properties SET updated_at = ? WHERE id = ?`),
		time.Now().UTC(), propertyID); err != nil {
		return fmt.Errorf("reset property updated_at: %w", err)
	}

	return tx.Commit()
}

// DeleteExpiredData removes events and sessions older than the retention period (12 months).
// This should be called periodically (e.g., weekly) to keep the database size manageable.
func (s *AnalyticsStore) DeleteExpiredData(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().AddDate(-1, 0, 0) // 12 months ago

	tx, err := s.begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	eventsResult, err := tx.ExecContext(ctx, s.query(`DELETE FROM ${SCHEMA_NAME}.events WHERE timestamp < ?`), cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete expired events: %w", err)
	}
	eventsDeleted, _ := eventsResult.RowsAffected()

	sessionsResult, err := tx.ExecContext(ctx, s.query(`DELETE FROM ${SCHEMA_NAME}.sessions WHERE started_at < ?`), cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	sessionsDeleted, _ := sessionsResult.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return eventsDeleted + sessionsDeleted, nil
}
