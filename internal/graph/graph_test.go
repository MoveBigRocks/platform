package graph_test

import (
	"testing"

	"github.com/graph-gophers/graphql-go"

	automationresolvers "github.com/movebigrocks/platform/internal/automation/resolvers"
	"github.com/movebigrocks/platform/internal/graph"
	"github.com/movebigrocks/platform/internal/graph/model"
	"github.com/movebigrocks/platform/internal/graph/shared"
	schema "github.com/movebigrocks/platform/internal/graphql/schema"
	observabilityresolvers "github.com/movebigrocks/platform/internal/observability/resolvers"
	platformresolvers "github.com/movebigrocks/platform/internal/platform/resolvers"
	serviceresolvers "github.com/movebigrocks/platform/internal/service/resolvers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
)

func TestRootResolverCreation(t *testing.T) {
	// Test that we can create a root resolver
	cfg := graph.Config{
		Service: &serviceresolvers.Config{
			CaseService: nil, // Would need actual service in real test
			UserService: nil,
		},
	}

	resolver := graph.NewRootResolver(cfg)
	if resolver == nil {
		t.Fatal("expected resolver to be created")
	}
}

func TestModelTypes(t *testing.T) {
	// Test that model types work
	user := &model.User{
		ID:    "user123",
		Email: "test@example.com",
		Name:  "Test User",
	}

	if user.ID != "user123" {
		t.Errorf("expected ID user123, got %s", user.ID)
	}
}

func TestSharedScalars(t *testing.T) {
	// Test DateTime scalar
	dt := shared.DateTime{}
	if err := dt.UnmarshalGraphQL("2025-01-01T00:00:00Z"); err != nil {
		t.Errorf("failed to unmarshal DateTime: %v", err)
	}

	// Test JSON scalar
	j := shared.JSON{}
	if err := j.UnmarshalGraphQL(map[string]interface{}{"key": "value"}); err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}
}

// Compile-time check that service resolver type exists
func TestServiceResolverType(t *testing.T) {
	var resolver *serviceresolvers.Resolver
	_ = resolver
}

// This ensures the types are importable and correct
func TestServiceResolverConfig(t *testing.T) {
	cfg := serviceresolvers.Config{
		CaseService: (*serviceapp.CaseService)(nil),
	}
	_ = cfg
}

// TestGraphQLSchemaValidation validates that the GraphQL schema and resolvers match.
// This test catches mismatches between schema fields and resolver methods at test time
// rather than runtime (when MustParseSchema would panic during server startup).
func TestGraphQLSchemaValidation(t *testing.T) {
	schemaString := schema.SchemaString

	// Create resolver with all domain resolvers configured (nil services are OK for validation)
	cfg := graph.Config{
		Service: &serviceresolvers.Config{
			CaseService: nil, // nil services are OK - we're just validating schema
		},
		Observability: &observabilityresolvers.Config{
			IssueService:   nil,
			ProjectService: nil,
		},
		Platform: &platformresolvers.Config{
			UserService:      nil,
			WorkspaceService: nil,
			AgentService:     nil,
			ExtensionService: nil,
		},
		Automation: &automationresolvers.Config{
			RuleService:      nil,
			FormService:      nil,
			WorkspaceService: nil,
		},
	}

	resolver := graph.NewRootResolver(cfg)

	// Use defer/recover to catch panics from MustParseSchema
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GraphQL schema validation failed - resolver/schema mismatch:\n%v\n\n"+
				"This error means a resolver method doesn't match the GraphQL schema.\n"+
				"Common causes:\n"+
				"- Return type mismatch (e.g., []Type vs []*Type for slice fields)\n"+
				"- Missing method on resolver (check spelling and signature)\n"+
				"- Nullable vs non-nullable type mismatch (use *Type for nullable)\n"+
				"- Input type field not a pointer for nullable GraphQL field", r)
		}
	}()

	// This will panic if there's a mismatch between schema and resolvers
	_, err := graphql.ParseSchema(
		schemaString,
		resolver,
		graphql.UseFieldResolvers(),
		graphql.UseStringDescriptions(),
	)
	if err != nil {
		t.Fatalf("GraphQL schema parsing failed: %v", err)
	}
}
