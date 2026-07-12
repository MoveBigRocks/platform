package graph

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMustParseSchemaRejectsExcessiveDepth(t *testing.T) {
	t.Parallel()

	server := MustParseSchema(NewRootResolver(Config{}), Limits{MaxDepth: 2})
	req := httptest.NewRequest(
		http.MethodPost,
		"/graphql",
		strings.NewReader(`{"query":"{ __schema { types { name } } }"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "exceeds max depth 2") {
		t.Fatalf("expected max-depth rejection, got status %d body %s", w.Code, w.Body.String())
	}
}

func TestMustParseSchemaRejectsOversizedQuery(t *testing.T) {
	t.Parallel()

	server := MustParseSchema(NewRootResolver(Config{}), Limits{MaxQueryBytes: 8})
	req := httptest.NewRequest(
		http.MethodPost,
		"/graphql",
		strings.NewReader(`{"query":"{ __typename }"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if !strings.Contains(strings.ToLower(w.Body.String()), "query length") {
		t.Fatalf("expected query-length rejection, got status %d body %s", w.Code, w.Body.String())
	}
}

func TestGinHandlerEnforcesRequestTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	started := make(chan struct{})
	server := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	})
	router := gin.New()
	router.POST("/graphql", GinHandler(server, 20*time.Millisecond))

	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	w := httptest.NewRecorder()
	startedAt := time.Now()
	router.ServeHTTP(w, req)

	<-started
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected timeout status %d, got %d body %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("request timeout was not enforced promptly: %s", elapsed)
	}
	if !strings.Contains(w.Body.String(), "GraphQL request timed out") {
		t.Fatalf("unexpected timeout response %s", w.Body.String())
	}
}
