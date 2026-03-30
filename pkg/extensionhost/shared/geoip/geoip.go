package geoip

// GeoLocation holds the resolved location for an IP address.
type GeoLocation struct {
	CountryCode string // ISO 3166-1 alpha-2, e.g. "NL"
	Country     string // Full name, e.g. "Netherlands"
	Region      string // State/province, e.g. "North Holland"
	City        string // e.g. "Amsterdam"
}

// Service resolves IP addresses to geographic locations.
type Service interface {
	Lookup(ip string) *GeoLocation
	Close() error
}

// noopService returns empty results when no GeoIP database is configured.
type noopService struct{}

func NewNoopService() Service {
	return &noopService{}
}

func (s *noopService) Lookup(ip string) *GeoLocation {
	return &GeoLocation{}
}

func (s *noopService) Close() error {
	return nil
}
