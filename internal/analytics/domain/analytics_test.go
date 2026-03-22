package analyticsdomain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProperty(t *testing.T) {
	prop, err := NewProperty("ws-123", "example.com", "UTC")
	require.NoError(t, err)
	assert.Empty(t, prop.ID)
	assert.Equal(t, "ws-123", prop.WorkspaceID)
	assert.Equal(t, "example.com", prop.Domain)
	assert.Equal(t, "UTC", prop.Timezone)
	assert.Equal(t, "active", prop.Status)
	assert.Nil(t, prop.VerifiedAt)
}

func TestNewProperty_DefaultTimezone(t *testing.T) {
	prop, err := NewProperty("ws-123", "example.com", "")
	require.NoError(t, err)
	assert.Equal(t, "UTC", prop.Timezone)
}

func TestNewProperty_Validation(t *testing.T) {
	_, err := NewProperty("ws-123", "", "UTC")
	assert.Error(t, err)

	_, err = NewProperty("", "example.com", "UTC")
	assert.Error(t, err)
}

func TestProperty_MarkVerified(t *testing.T) {
	prop, _ := NewProperty("ws-123", "example.com", "UTC")
	assert.False(t, prop.IsVerified())
	prop.MarkVerified()
	assert.True(t, prop.IsVerified())
	// Second call should not change verified_at
	firstVerified := *prop.VerifiedAt
	prop.MarkVerified()
	assert.Equal(t, firstVerified, *prop.VerifiedAt)
}

func TestProperty_SnippetHTML(t *testing.T) {
	prop, _ := NewProperty("ws-123", "example.com", "UTC")
	snippet := prop.SnippetHTML("https://api.movebigrocks.com")
	assert.Contains(t, snippet, `data-domain="example.com"`)
	assert.Contains(t, snippet, `src="https://api.movebigrocks.com/js/analytics.js"`)
}

func TestNewGoal_Event(t *testing.T) {
	goal, err := NewGoal("prop-1", "event", "signup", "")
	require.NoError(t, err)
	assert.Equal(t, "event", goal.GoalType)
	assert.Equal(t, "signup", goal.EventName)
	assert.Equal(t, "signup", goal.DisplayName())
}

func TestNewGoal_Page(t *testing.T) {
	goal, err := NewGoal("prop-1", "page", "", "/thank-you")
	require.NoError(t, err)
	assert.Equal(t, "page", goal.GoalType)
	assert.Equal(t, "/thank-you", goal.PagePath)
	assert.Equal(t, "Visit /thank-you", goal.DisplayName())
}

func TestNewGoal_Validation(t *testing.T) {
	_, err := NewGoal("", "event", "signup", "")
	assert.Error(t, err)

	_, err = NewGoal("prop-1", "event", "", "")
	assert.Error(t, err)

	_, err = NewGoal("prop-1", "page", "", "")
	assert.Error(t, err)

	_, err = NewGoal("prop-1", "invalid", "", "")
	assert.Error(t, err)
}

func TestValidateGoalCount(t *testing.T) {
	assert.NoError(t, ValidateGoalCount(0))
	assert.NoError(t, ValidateGoalCount(19))
	assert.Error(t, ValidateGoalCount(20))
	assert.Error(t, ValidateGoalCount(25))
}

func TestHostnameRule_MatchesHostname(t *testing.T) {
	tests := []struct {
		pattern  string
		hostname string
		matches  bool
	}{
		{"example.com", "example.com", true},
		{"example.com", "other.com", false},
		{"example.com", "EXAMPLE.COM", true},
		{"*.example.com", "blog.example.com", true},
		{"*.example.com", "example.com", false},
		{"*.example.com", "deep.sub.example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.hostname, func(t *testing.T) {
			rule := &HostnameRule{Pattern: tt.pattern}
			assert.Equal(t, tt.matches, rule.MatchesHostname(tt.hostname))
		})
	}
}

func TestMatchesAnyHostnameRule(t *testing.T) {
	// Empty rules accept all
	assert.True(t, MatchesAnyHostnameRule(nil, "anything.com"))

	rules := []*HostnameRule{
		{Pattern: "example.com"},
		{Pattern: "*.example.com"},
	}
	assert.True(t, MatchesAnyHostnameRule(rules, "example.com"))
	assert.True(t, MatchesAnyHostnameRule(rules, "blog.example.com"))
	assert.False(t, MatchesAnyHostnameRule(rules, "other.com"))
}

func TestValidateHostnameRuleCount(t *testing.T) {
	assert.NoError(t, ValidateHostnameRuleCount(0))
	assert.NoError(t, ValidateHostnameRuleCount(9))
	assert.Error(t, ValidateHostnameRuleCount(10))
}

func TestGenerateVisitorID(t *testing.T) {
	salt := make([]byte, 16)
	for i := range salt {
		salt[i] = byte(i)
	}

	// Same inputs → same output
	id1 := GenerateVisitorID(salt, "Mozilla/5.0", "1.2.3.4", "example.com")
	id2 := GenerateVisitorID(salt, "Mozilla/5.0", "1.2.3.4", "example.com")
	assert.Equal(t, id1, id2)
	assert.NotZero(t, id1)

	// Different IP → different output
	id3 := GenerateVisitorID(salt, "Mozilla/5.0", "5.6.7.8", "example.com")
	assert.NotEqual(t, id1, id3)

	// Different domain → different output
	id4 := GenerateVisitorID(salt, "Mozilla/5.0", "1.2.3.4", "other.com")
	assert.NotEqual(t, id1, id4)

	// Different salt → different output
	salt2 := make([]byte, 16)
	for i := range salt2 {
		salt2[i] = byte(i + 100)
	}
	id5 := GenerateVisitorID(salt2, "Mozilla/5.0", "1.2.3.4", "example.com")
	assert.NotEqual(t, id1, id5)

	// Short salt → zero
	assert.Equal(t, int64(0), GenerateVisitorID([]byte("short"), "ua", "ip", "d"))
}

func TestNewSalt(t *testing.T) {
	salt, err := NewSalt()
	require.NoError(t, err)
	assert.Len(t, salt.Salt, 16)
	assert.False(t, salt.CreatedAt.IsZero())

	// Two salts should be different
	salt2, err := NewSalt()
	require.NoError(t, err)
	assert.NotEqual(t, salt.Salt, salt2.Salt)
}
