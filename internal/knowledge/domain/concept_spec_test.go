package knowledgedomain

import "testing"

func TestConceptSpecValidate(t *testing.T) {
	t.Parallel()

	spec := NewConceptSpec("ws_123", "team_123", "marketing/campaign-brief", "1", "Campaign Brief")
	spec.InstanceKind = KnowledgeResourceKindContext
	spec.Status = ConceptSpecStatusActive

	if err := spec.Validate(); err != nil {
		t.Fatalf("expected valid concept spec, got %v", err)
	}

	spec.Key = "Bad Key"
	if err := spec.Validate(); err == nil {
		t.Fatalf("expected invalid key to fail validation")
	}
}

func TestLookupBuiltInConceptSpec(t *testing.T) {
	t.Parallel()

	spec, ok := LookupBuiltInConceptSpec("core/rfc", "1")
	if !ok {
		t.Fatalf("expected built-in concept spec")
	}
	if spec.InstanceKind != KnowledgeResourceKindDecision {
		t.Fatalf("expected decision instance kind, got %q", spec.InstanceKind)
	}

	goal, ok := LookupBuiltInConceptSpec("core/goal", "1")
	if !ok {
		t.Fatalf("expected built-in strategic context concept spec")
	}
	if goal.InstanceKind != KnowledgeResourceKindContext {
		t.Fatalf("expected context instance kind for strategic concept, got %q", goal.InstanceKind)
	}
}

func TestDefaultConceptSpecForKind(t *testing.T) {
	t.Parallel()

	key, version := DefaultConceptSpecForKind(KnowledgeResourceKindTemplate)
	if key != "core/template" || version != "1" {
		t.Fatalf("unexpected default concept spec: %s@%s", key, version)
	}
}
