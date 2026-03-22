package graph

import (
	"html/template"
	"net/http"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/movebigrocks/platform/internal/graphql/schema"
)

// MustParseSchema parses the GraphQL schema and panics on error.
// This should only be called during application startup.
// Returns an http.Handler ready to use with gin.WrapH or directly as ServeHTTP.
func MustParseSchema(resolver *RootResolver) http.Handler {
	s := graphql.MustParseSchema(
		schema.SchemaString,
		resolver,
		graphql.UseFieldResolvers(),
		graphql.UseStringDescriptions(),
	)
	return &relay.Handler{Schema: s}
}

// NewPlaygroundHandler creates a GraphQL Playground handler
func NewPlaygroundHandler(endpoint string) http.Handler {
	return &playgroundHandler{endpoint: endpoint}
}

type playgroundHandler struct {
	endpoint string
}

func (h *playgroundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := playgroundTemplate.Execute(w, map[string]string{
		"endpoint": h.endpoint,
	}); err != nil {
		http.Error(w, "Failed to render playground", http.StatusInternalServerError)
	}
}

var playgroundTemplate = template.Must(template.New("playground").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset=utf-8/>
  <meta name="viewport" content="user-scalable=no, initial-scale=1.0, minimum-scale=1.0, maximum-scale=1.0">
  <title>GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react@1.7.26/build/static/css/index.css"/>
  <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react@1.7.26/build/favicon.png"/>
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react@1.7.26/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root"></div>
  <script>
    window.addEventListener('load', function() {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '{{.endpoint}}'
      })
    })
  </script>
</body>
</html>
`))
