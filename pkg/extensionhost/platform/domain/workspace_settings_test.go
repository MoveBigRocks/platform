package platformdomain

import (
	"testing"
	"time"

	shared "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

func TestWorkspaceSettingsBusinessDayAndAccessors(t *testing.T) {
	ws := NewWorkspaceSettings("ws_1")
	ws.SetBusinessHours("Monday", BusinessHours{IsBusinessDay: false})
	ws.AddHoliday(Holiday{Name: "Liberation Day", Date: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC), IsRecurring: true})

	if ws.IsBusinessDay(time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("expected recurring holiday to be non-business day")
	}
	if ws.IsBusinessDay(time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("expected configured Monday hours to override default business day")
	}
	if !ws.IsBusinessDay(time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("expected Tuesday to default to business day")
	}

	ws.EmailFromName = "Support"
	ws.EmailFromAddress = "support@example.com"
	ws.EmailReplyToAddress = "reply@example.com"
	ws.EmailSignature = "Regards"
	ws.EmailFooter = "Footer"
	ws.AutoResponseEnabled = true
	ws.AutoResponseTemplate = "tpl"
	email := ws.Email()
	if email.FromName != "Support" || email.AutoResponseTpl != "tpl" || !email.AutoResponse {
		t.Fatalf("unexpected email config: %#v", email)
	}

	ws.PasswordMinLength = 14
	ws.PasswordRequireSpecial = true
	ws.PasswordRequireNumbers = true
	ws.PasswordRequireUppercase = true
	ws.SessionTimeoutMinutes = 45
	ws.TwoFactorRequired = true
	ws.IPWhitelist = []string{"127.0.0.1"}
	ws.IPBlacklist = []string{"10.0.0.1"}
	security := ws.Security()
	if security.PasswordMinLength != 14 || !security.TwoFactorRequired || len(security.IPWhitelist) != 1 || len(security.IPBlacklist) != 1 {
		t.Fatalf("unexpected security config: %#v", security)
	}
}

func TestWorkspaceSettingsUpdateSettingBranches(t *testing.T) {
	ws := NewWorkspaceSettings("ws_1")

	ws.UpdateSetting("timezone", shared.StringValue("Europe/Amsterdam"), "user_1")
	ws.UpdateSetting("language", shared.StringValue("nl"), "user_1")
	ws.UpdateSetting("theme", shared.StringValue("midnight"), "user_1")
	ws.UpdateSetting("auto_assign_cases", shared.BoolValue(true), "user_1")
	ws.UpdateSetting("unknown_key", shared.StringValue("ignored"), "user_1")

	if ws.Timezone != "Europe/Amsterdam" || ws.Language != "nl" || ws.Theme != "midnight" || !ws.AutoAssignCases {
		t.Fatalf("expected update setting branches to apply, got %#v", ws)
	}
	if ws.UpdatedByID != "user_1" {
		t.Fatalf("expected updater id to be recorded, got %#v", ws)
	}
}
