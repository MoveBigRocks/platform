package shareddomain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestValueJSONRoundTrip(t *testing.T) {
	original := StringsValue([]string{"alpha", "beta"})

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}

	var decoded Value
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}

	if !original.Equals(decoded) {
		t.Fatalf("expected round trip equality, got %#v", decoded)
	}
}

func TestTypedContextJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ctx := NewTypedContext().
		WithCaseID("case_1").
		WithWorkspaceID("ws_1").
		WithUserID("user_1").
		WithEventType("case.created").
		WithSource("automation")
	ctx.Timestamp = now
	ctx.SetExtra("reason", StringValue("policy"))

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal typed context: %v", err)
	}

	var decoded TypedContext
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal typed context: %v", err)
	}

	if decoded.CaseID != ctx.CaseID || decoded.WorkspaceID != ctx.WorkspaceID || decoded.UserID != ctx.UserID {
		t.Fatalf("unexpected decoded identity fields: %#v", decoded)
	}
	if decoded.EventType != ctx.EventType || decoded.Source != ctx.Source {
		t.Fatalf("unexpected decoded event fields: %#v", decoded)
	}
	if !decoded.Timestamp.Equal(now) {
		t.Fatalf("expected timestamp %v, got %v", now, decoded.Timestamp)
	}
	value, ok := decoded.GetExtra("reason")
	if !ok || value.AsString() != "policy" {
		t.Fatalf("expected extra metadata to round trip, got %#v", decoded.Extra)
	}
}
