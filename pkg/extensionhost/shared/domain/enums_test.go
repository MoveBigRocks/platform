package shareddomain

import "testing"

func TestTypedEnumsValidityAndStringers(t *testing.T) {
	tests := []struct {
		name     string
		valid    bool
		stringer string
	}{
		{name: string(CommTypeEmail), valid: CommTypeEmail.IsValid(), stringer: CommTypeEmail.String()},
		{name: string(DirectionInbound), valid: DirectionInbound.IsValid(), stringer: DirectionInbound.String()},
		{name: string(TriggerTypeCaseUpdated), valid: TriggerTypeCaseUpdated.IsValid(), stringer: TriggerTypeCaseUpdated.String()},
		{name: string(ConditionTypeTime), valid: ConditionTypeTime.IsValid(), stringer: ConditionTypeTime.String()},
		{name: string(OpExists), valid: OpExists.IsValid(), stringer: OpExists.String()},
		{name: string(ActionTypeSetCustomField), valid: ActionTypeSetCustomField.IsValid(), stringer: ActionTypeSetCustomField.String()},
		{name: string(LogicalOr), valid: LogicalOr.IsValid(), stringer: LogicalOr.String()},
		{name: string(SourceTypeWebhook), valid: SourceTypeWebhook.IsValid(), stringer: SourceTypeWebhook.String()},
	}

	for _, tt := range tests {
		if !tt.valid {
			t.Fatalf("expected %q to be valid", tt.name)
		}
		if tt.stringer != tt.name {
			t.Fatalf("expected stringer for %q to return %q, got %q", tt.name, tt.name, tt.stringer)
		}
	}

	if CommunicationType("fax").IsValid() {
		t.Fatal("expected invalid communication type")
	}
	if Direction("sideways").IsValid() {
		t.Fatal("expected invalid direction")
	}
	if TriggerType("cron").IsValid() {
		t.Fatal("expected invalid trigger type")
	}
	if RuleConditionType("issue").IsValid() {
		t.Fatal("expected invalid condition type")
	}
	if Operator("between").IsValid() {
		t.Fatal("expected invalid operator")
	}
	if RuleActionType("archive_case").IsValid() {
		t.Fatal("expected invalid action type")
	}
	if LogicalOperator("xor").IsValid() {
		t.Fatal("expected invalid logical operator")
	}
	if SourceType("clipboard").IsValid() {
		t.Fatal("expected invalid source type")
	}
}
