package geoip

import (
	"fmt"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

// maxmindRecord maps the GeoLite2-City MMDB structure.
type maxmindRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

type maxmindService struct {
	db *maxminddb.Reader
}

// NewMaxMindService opens a GeoLite2 MMDB file and returns a Service.
func NewMaxMindService(dbPath string) (Service, error) {
	db, err := maxminddb.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open geoip database: %w", err)
	}
	return &maxmindService{db: db}, nil
}

func (s *maxmindService) Lookup(ip string) *GeoLocation {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return &GeoLocation{}
	}

	var record maxmindRecord
	if err := s.db.Lookup(parsed, &record); err != nil {
		return &GeoLocation{}
	}

	loc := &GeoLocation{
		CountryCode: record.Country.ISOCode,
		Country:     record.Country.Names["en"],
		City:        record.City.Names["en"],
	}

	if len(record.Subdivisions) > 0 {
		loc.Region = record.Subdivisions[0].Names["en"]
	}

	return loc
}

func (s *maxmindService) Close() error {
	return s.db.Close()
}
