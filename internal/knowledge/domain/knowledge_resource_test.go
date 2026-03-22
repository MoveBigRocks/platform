package knowledgedomain

import "testing"

func TestKnowledgeResourceValidate(t *testing.T) {
	t.Parallel()

	resource := NewKnowledgeResource("ws_123", "team_123", "refund-policy", "Refund Policy")
	resource.Status = KnowledgeResourceStatusActive

	if err := resource.Validate(); err != nil {
		t.Fatalf("expected valid knowledge resource, got %v", err)
	}

	resource.Slug = "bad slug"
	if err := resource.Validate(); err == nil {
		t.Fatalf("expected invalid slug to fail validation")
	}
}

func TestNormalizeKnowledgeSlug(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"Refund Policy":           "refund-policy",
		"customer_support_guide":  "customer-support-guide",
		"  Release   Checklist  ": "release-checklist",
	}

	for input, want := range tests {
		if got := NormalizeKnowledgeSlug("", input); got != want {
			t.Fatalf("NormalizeKnowledgeSlug(%q) = %q, want %q", input, got, want)
		}
	}

	if got := NormalizeKnowledgeSlug("", ""); got != "knowledge" {
		t.Fatalf("expected fallback slug, got %q", got)
	}
}

func TestParseKnowledgeResourceKind(t *testing.T) {
	t.Parallel()

	tests := map[string]KnowledgeResourceKind{
		"policy":         KnowledgeResourceKindPolicy,
		"runbook":        KnowledgeResourceKindGuide,
		"prompts":        KnowledgeResourceKindSkill,
		"context":        KnowledgeResourceKindContext,
		"constraints":    KnowledgeResourceKindConstraint,
		"best practices": KnowledgeResourceKindBestPractice,
		"templates":      KnowledgeResourceKindTemplate,
		"checklists":     KnowledgeResourceKindChecklist,
		"rfc":            KnowledgeResourceKindDecision,
		"adr":            KnowledgeResourceKindDecision,
		"ideas":          KnowledgeResourceKindIdea,
	}

	for input, want := range tests {
		got, ok := ParseKnowledgeResourceKind(input)
		if !ok {
			t.Fatalf("ParseKnowledgeResourceKind(%q) reported invalid", input)
		}
		if got != want {
			t.Fatalf("ParseKnowledgeResourceKind(%q) = %q, want %q", input, got, want)
		}
	}

	if _, ok := ParseKnowledgeResourceKind("made-up"); ok {
		t.Fatalf("expected made-up kind to be invalid")
	}
}
