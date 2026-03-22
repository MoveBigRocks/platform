package analyticsservices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifySource_UTMPriority(t *testing.T) {
	source := ClassifySource("newsletter", "https://google.com/search?q=test", "example.com")
	assert.Equal(t, "newsletter", source)
}

func TestClassifySource_KnownReferrer(t *testing.T) {
	tests := []struct {
		referrer string
		expected string
	}{
		{"https://www.google.com/search?q=test", "Google"},
		{"https://www.bing.com/search?q=test", "Bing"},
		{"https://duckduckgo.com/?q=test", "DuckDuckGo"},
		{"https://t.co/abc123", "Twitter"},
		{"https://www.reddit.com/r/golang", "Reddit"},
		{"https://github.com/user/repo", "GitHub"},
		{"https://news.ycombinator.com/item?id=123", "Hacker News"},
		{"https://l.facebook.com/foo", "Facebook"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			source := ClassifySource("", tt.referrer, "example.com")
			assert.Equal(t, tt.expected, source)
		})
	}
}

func TestClassifySource_UnknownReferrer(t *testing.T) {
	source := ClassifySource("", "https://www.somesite.com/page", "example.com")
	assert.Equal(t, "somesite.com", source)
}

func TestClassifySource_Direct(t *testing.T) {
	source := ClassifySource("", "", "example.com")
	assert.Equal(t, "Direct", source)
}

func TestClassifySource_InternalNavigation(t *testing.T) {
	source := ClassifySource("", "https://example.com/about", "example.com")
	assert.Equal(t, "", source)
}

func TestClassifySource_SubdomainInternal(t *testing.T) {
	source := ClassifySource("", "https://blog.example.com/post", "example.com")
	assert.Equal(t, "", source)
}

func TestCountryFromLanguage(t *testing.T) {
	tests := []struct {
		acceptLang string
		expected   string
	}{
		{"en-US,en;q=0.9", "US"},
		{"nl-NL,nl;q=0.9", "NL"},
		{"de-DE", "DE"},
		{"fr-FR,fr;q=0.9,en-US;q=0.8", "FR"},
		{"en", ""},
		{"", ""},
		{"ja-JP;q=0.9", "JP"},
	}

	for _, tt := range tests {
		t.Run(tt.acceptLang, func(t *testing.T) {
			result := CountryFromLanguage(tt.acceptLang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseUA(t *testing.T) {
	tests := []struct {
		name       string
		ua         string
		wantDevice string
	}{
		{
			name:       "empty",
			ua:         "",
			wantDevice: "",
		},
		{
			name:       "desktop Chrome",
			ua:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			wantDevice: "Desktop",
		},
		{
			name:       "mobile Safari",
			ua:         "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantDevice: "Mobile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browser, os, device := ParseUA(tt.ua)
			if tt.ua == "" {
				assert.Empty(t, browser)
				assert.Empty(t, os)
				assert.Empty(t, device)
			} else {
				assert.NotEmpty(t, browser)
				assert.NotEmpty(t, os)
				assert.Equal(t, tt.wantDevice, device)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		rawURL      string
		pathname    string
		utmSource   string
		utmMedium   string
		utmCampaign string
	}{
		{
			rawURL:   "https://example.com/blog/post-1",
			pathname: "/blog/post-1",
		},
		{
			rawURL:   "https://example.com",
			pathname: "/",
		},
		{
			rawURL:      "https://example.com/page?utm_source=twitter&utm_medium=social&utm_campaign=launch",
			pathname:    "/page",
			utmSource:   "twitter",
			utmMedium:   "social",
			utmCampaign: "launch",
		},
		{
			rawURL:   "invalid-url",
			pathname: "invalid-url", // url.Parse treats this as a relative path
		},
	}

	for _, tt := range tests {
		t.Run(tt.rawURL, func(t *testing.T) {
			pathname, utmSource, utmMedium, utmCampaign := parseURL(tt.rawURL)
			assert.Equal(t, tt.pathname, pathname)
			assert.Equal(t, tt.utmSource, utmSource)
			assert.Equal(t, tt.utmMedium, utmMedium)
			assert.Equal(t, tt.utmCampaign, utmCampaign)
		})
	}
}

func TestExtractHostname(t *testing.T) {
	assert.Equal(t, "example.com", extractHostname("https://example.com/page"))
	assert.Equal(t, "sub.example.com", extractHostname("https://sub.example.com/page"))
	assert.Equal(t, "", extractHostname("invalid"))
}

func TestPeriodRange(t *testing.T) {
	tests := []string{"TODAY", "YESTERDAY", "LAST_7_DAYS", "LAST_28_DAYS", "THIS_MONTH", "LAST_MONTH", "LAST_6_MONTHS", "LAST_12_MONTHS"}

	for _, period := range tests {
		t.Run(period, func(t *testing.T) {
			from, to, err := PeriodRange(period, "UTC", nil, nil)
			assert.NoError(t, err)
			assert.True(t, from.Before(to), "from should be before to")
		})
	}
}

func TestPeriodRange_Unknown(t *testing.T) {
	_, _, err := PeriodRange("INVALID", "UTC", nil, nil)
	assert.Error(t, err)
}

func TestPreviousPeriodRange(t *testing.T) {
	from, to, _ := PeriodRange("LAST_7_DAYS", "UTC", nil, nil)
	prevFrom, prevTo := PreviousPeriodRange(from, to)

	assert.Equal(t, to.Sub(from), prevTo.Sub(prevFrom), "previous period should have same duration")
	assert.Equal(t, from, prevTo, "previous period should end at current period start")
}
