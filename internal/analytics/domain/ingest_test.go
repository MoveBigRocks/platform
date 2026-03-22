package analyticsdomain

import (
	"testing"
	"time"
)

func TestClassifySource(t *testing.T) {
	if got := ClassifySource("newsletter", "https://google.com", "example.com"); got != "newsletter" {
		t.Fatalf("expected utm source to win, got %q", got)
	}
	if got := ClassifySource("", "https://www.google.com/search?q=test", "example.com"); got != "Google" {
		t.Fatalf("expected known source, got %q", got)
	}
	if got := ClassifySource("", "https://app.example.com/dashboard", "example.com"); got != "" {
		t.Fatalf("expected internal navigation to be blank, got %q", got)
	}
}

func TestCountryFromLanguage(t *testing.T) {
	if got := CountryFromLanguage("en-US,en;q=0.9"); got != "US" {
		t.Fatalf("expected US, got %q", got)
	}
	if got := CountryFromLanguage("en"); got != "" {
		t.Fatalf("expected empty country, got %q", got)
	}
}

func TestParseTrackedURL(t *testing.T) {
	path, source, medium, campaign := ParseTrackedURL("https://example.com/pricing?utm_source=ads&utm_medium=cpc&utm_campaign=spring")
	if path != "/pricing" || source != "ads" || medium != "cpc" || campaign != "spring" {
		t.Fatalf("unexpected parsed values: %q %q %q %q", path, source, medium, campaign)
	}
}

func TestSessionLifecycleHelpers(t *testing.T) {
	now := time.Now().UTC()
	session := NewSessionFromIngest(SessionParams{
		PropertyID: "prop_1",
		VisitorID:  42,
		Pathname:   "/",
		StartedAt:  now,
		EventName:  "pageview",
	})

	if session.Pageviews != 1 || session.IsBounce != 1 {
		t.Fatalf("expected initial pageview session, got pageviews=%d bounce=%d", session.Pageviews, session.IsBounce)
	}

	session.RecordActivity("pageview", "/pricing", now.Add(2*time.Minute))
	if session.ExitPage != "/pricing" {
		t.Fatalf("expected exit page to update, got %q", session.ExitPage)
	}
	if session.Pageviews != 2 || session.IsBounce != 0 {
		t.Fatalf("expected follow-up pageview to clear bounce, got pageviews=%d bounce=%d", session.Pageviews, session.IsBounce)
	}
	if session.Duration <= 0 {
		t.Fatalf("expected session duration to advance, got %d", session.Duration)
	}
}
