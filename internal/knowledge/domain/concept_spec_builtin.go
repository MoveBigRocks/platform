package knowledgedomain

import shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"

var builtInConceptSpecs = []*ConceptSpec{
	newBuiltInConcept("core/purpose", "1", "Purpose", "Why the team or workspace exists beyond immediate delivery work.", KnowledgeResourceKindContext, []string{"summary", "purpose", "beneficiaries"}),
	newBuiltInConcept("core/vision", "1", "Vision", "Future-state narrative that explains where the organization is heading.", KnowledgeResourceKindContext, []string{"summary", "vision", "time_horizon"}),
	newBuiltInConcept("core/mission", "1", "Mission", "Durable operating mandate for how the organization pursues its purpose.", KnowledgeResourceKindContext, []string{"summary", "mission", "operating_model"}),
	newBuiltInConcept("core/goal", "1", "Goal", "Concrete outcome the team intends to reach.", KnowledgeResourceKindContext, []string{"summary", "goal", "success_signals"}),
	newBuiltInConcept("core/strategy", "1", "Strategy", "Coherent plan for how a goal will be reached.", KnowledgeResourceKindContext, []string{"summary", "strategy", "tradeoffs", "proof"}),
	newBuiltInConcept("core/bet", "1", "Bet", "Focused strategic wager with explicit upside, downside, and decision points.", KnowledgeResourceKindContext, []string{"summary", "bet", "upside", "downside", "decision_points"}),
	newBuiltInConcept("core/okr", "1", "OKR", "Objective with measurable key results.", KnowledgeResourceKindContext, []string{"summary", "objective", "key_results"}),
	newBuiltInConcept("core/kpi", "1", "KPI", "Key performance indicator with target and cadence.", KnowledgeResourceKindContext, []string{"summary", "metric", "target", "cadence"}),
	newBuiltInConcept("core/milestone-goal", "1", "Milestone Goal", "Delivery-scoped goal that operationalizes the strategic stack.", KnowledgeResourceKindContext, []string{"summary", "goal", "scope_boundary", "success_signals"}),
	newBuiltInConcept("core/workstream", "1", "Workstream", "Coordinated slice of work tied to a milestone or broader goal.", KnowledgeResourceKindContext, []string{"summary", "workstream", "dependencies", "proof"}),
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
