package analyticshandlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBotUA(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		expected bool
	}{
		{"Googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true},
		{"normal Chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0", false},
		{"curl", "curl/7.88.1", true},
		{"wget", "Wget/1.21.4", true},
		{"python-requests", "python-requests/2.31.0", true},
		{"go-http-client", "Go-http-client/1.1", true},
		{"headless Chrome", "HeadlessChrome/120.0.0.0", true},
		{"Ahrefs", "AhrefsBot/7.0", true},
		{"Semrush", "SemrushBot/7~bl", true},
		{"empty", "", false},
		{"normal Safari", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/605.1.15", false},
		{"Slackbot", "Slackbot-LinkExpanding 1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsBotUA(tt.ua))
		})
	}
}

func TestIsReferrerSpam(t *testing.T) {
	tests := []struct {
		name     string
		referrer string
		expected bool
	}{
		{"semalt", "https://semalt.com/project/foo", true},
		{"normal", "https://google.com/search?q=test", false},
		{"darodar", "https://darodar.com/page", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsReferrerSpam(tt.referrer))
		})
	}
}

func TestCleanIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3.4", "1.2.3.4"},
		{"1.2.3.4:8080", "1.2.3.4"},
		{"[::1]:8080", "::1"},
		{"  1.2.3.4  ", "1.2.3.4"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, cleanIP(tt.input))
		})
	}
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abcdef", 3))
	assert.Equal(t, "ab", truncate("ab", 3))
	assert.Equal(t, "", truncate("", 3))

	// UTF-8 safety: don't split multi-byte characters
	// "hello🌍" = 5 bytes + 4 bytes = 9 bytes
	assert.Equal(t, "hello", truncate("hello🌍", 7))   // cuts before emoji rather than through it
	assert.Equal(t, "hello🌍", truncate("hello🌍", 9))  // exact fit
	assert.Equal(t, "hello🌍", truncate("hello🌍", 20)) // no truncation needed
}

func TestStripControlChars(t *testing.T) {
	assert.Equal(t, "hello world", stripControlChars("hello\x00 \x01world"))
	assert.Equal(t, "normal text", stripControlChars("normal text"))
}
