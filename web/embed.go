package web

import "embed"

// Templates contains embedded HTML templates for the admin panel and public forms.
//
//go:embed admin-panel/templates/*.html admin-panel/templates/partials/*.html
var Templates embed.FS

// Static contains all static assets (CSS, JS, images).
//
//go:embed static
var Static embed.FS
