package platformhandlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAdminBundlePageDataScopesStandaloneDocument(t *testing.T) {
	document, err := parseAdminExtensionDocument([]byte(`<!doctype html>
<html lang="en">
<head>
  <title>ATS Dashboard</title>
  <style>
    body { margin: 0; background: #f8fafc; }
    .card, main > section { color: #111827; }
    @media (min-width: 768px) {
      body { padding: 2rem; }
    }
  </style>
</head>
<body>
  <main>
    <section class="card">Hiring overview</section>
  </main>
</body>
</html>`))
	require.NoError(t, err)
	require.Equal(t, "ATS Dashboard", document.Title)
	require.Contains(t, string(document.BodyHTML), "Hiring overview")
	require.Contains(t, string(document.HeadHTML), ".extension-bundle-root")
	require.Contains(t, string(document.HeadHTML), ".extension-bundle-root .card")
	require.NotContains(t, string(document.HeadHTML), "body {")
}
