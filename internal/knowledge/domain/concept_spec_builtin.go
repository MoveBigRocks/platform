package knowledgedomain

import shareddomain "github.com/movebigrocks/platform/internal/shared/domain"

var builtInConceptSpecs = []*ConceptSpec{
	newBuiltInConcept("core/policy", "1", "Policy", "Canonical policy and governing guidance.", KnowledgeResourceKindPolicy, []string{"summary", "policy", "exceptions"}),
	newBuiltInConcept("core/guide", "1", "Guide", "Step-by-step operational guidance.", KnowledgeResourceKindGuide, []string{"summary", "steps", "references"}),
	newBuiltInConcept("core/skill", "1", "Skill", "Reusable agent or operator capability guidance.", KnowledgeResourceKindSkill, []string{"summary", "inputs", "procedure", "examples"}),
	newBuiltInConcept("core/context", "1", "Context", "Background context, briefs, and reference material.", KnowledgeResourceKindContext, []string{"summary", "background", "facts"}),
	newBuiltInConcept("core/constraint", "1", "Constraint", "Guardrails, requirements, and limits.", KnowledgeResourceKindConstraint, []string{"summary", "constraint", "applies_when"}),
	newBuiltInConcept("core/best-practice", "1", "Best Practice", "Preferred conventions and recommended ways of working.", KnowledgeResourceKindBestPractice, []string{"summary", "recommendation", "rationale"}),
	newBuiltInConcept("core/template", "1", "Template", "Reusable template or boilerplate artifact.", KnowledgeResourceKindTemplate, []string{"summary", "template", "usage"}),
	newBuiltInConcept("core/checklist", "1", "Checklist", "Checklist with ordered operational steps.", KnowledgeResourceKindChecklist, []string{"summary", "checklist", "completion"}),
	newBuiltInConcept("core/rfc", "1", "RFC", "Request for comments for materially important decisions.", KnowledgeResourceKindDecision, []string{"summary", "context", "proposal", "tradeoffs", "rollout"}),
	newBuiltInConcept("core/adr", "1", "ADR", "Architecture decision record for accepted decisions.", KnowledgeResourceKindDecision, []string{"summary", "decision", "context", "consequences"}),
	newBuiltInConcept("core/idea", "1", "Idea", "Early-stage idea or proposal under discussion.", KnowledgeResourceKindIdea, []string{"summary", "idea", "open_questions"}),
}

func BuiltInConceptSpecs() []*ConceptSpec {
	result := make([]*ConceptSpec, 0, len(builtInConceptSpecs))
	for _, spec := range builtInConceptSpecs {
		if spec == nil {
			continue
		}
		copy := *spec
		result = append(result, &copy)
	}
	return result
}

func LookupBuiltInConceptSpec(key, version string) (*ConceptSpec, bool) {
	normalizedKey := NormalizeConceptSpecKey(key)
	normalizedVersion := NormalizeConceptSpecVersion(version)
	for _, spec := range builtInConceptSpecs {
		if spec == nil {
			continue
		}
		if spec.Key == normalizedKey && spec.Version == normalizedVersion {
			copy := *spec
			return &copy, true
		}
	}
	return nil, false
}

func newBuiltInConcept(key, version, name, description string, kind KnowledgeResourceKind, sections []string) *ConceptSpec {
	spec := NewBuiltInConceptSpec(key, version, name, description, kind)
	spec.MetadataSchema = shareddomain.NewTypedSchema()
	spec.SectionsSchema = shareddomain.TypedSchemaFromMap(map[string]any{
		"required": sections,
	})
	spec.WorkflowSchema = shareddomain.TypedSchemaFromMap(map[string]any{
		"states": []string{"draft", "in_review", "approved", "archived"},
	})
	switch key {
	case "core/rfc":
		spec.AgentGuidanceMD = "Agents should summarize tradeoffs, call out risks, and request human review before approval."
	case "core/idea":
		spec.AgentGuidanceMD = "Agents should treat ideas as speculative and low-trust unless promoted."
	default:
		spec.AgentGuidanceMD = "Agents should follow the declared section structure and metadata schema."
	}
	return spec
}
