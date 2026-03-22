package platformhandlers

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/movebigrocks/platform/web"
)

const (
	adminTemplateMainPattern    = "admin-panel/templates/*.html"
	adminTemplatePartialPattern = "admin-panel/templates/partials/*.html"
)

var adminTemplatePatterns = []string{
	adminTemplateMainPattern,
	adminTemplatePartialPattern,
}

// AdminTemplateFuncMap returns the shared template helpers used by admin HTML rendering.
func AdminTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		// div divides a by b (returns 0 if b is 0 to avoid panic)
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		// substr returns a substring from start to end with bounds checking
		"substr": func(s string, start, end int) string {
			if start < 0 {
				start = 0
			}
			if start >= len(s) {
				return ""
			}
			if end > len(s) {
				end = len(s)
			}
			if end < start {
				return ""
			}
			return s[start:end]
		},
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(b)
		},
		// formatDate handles both time.Time and string (ISO format) types.
		"formatDate": func(v interface{}, layout string) string {
			switch t := v.(type) {
			case time.Time:
				if t.IsZero() {
					return ""
				}
				return t.Format(layout)
			case *time.Time:
				if t == nil || t.IsZero() {
					return ""
				}
				return t.Format(layout)
			case string:
				if t == "" {
					return ""
				}
				parsed, err := time.Parse(time.RFC3339, t)
				if err != nil {
					parsed, err = time.Parse("2006-01-02T15:04:05", t)
					if err != nil {
						return t
					}
				}
				return parsed.Format(layout)
			default:
				return ""
			}
		},
	}
}

// ParseAdminTemplates loads the same embedded template set used in production.
func ParseAdminTemplates() (*template.Template, error) {
	return template.New("").Funcs(AdminTemplateFuncMap()).ParseFS(web.Templates, adminTemplatePatterns...)
}

// AdminTemplateFiles lists embedded admin templates by type.
func AdminTemplateFiles() ([]string, []string, error) {
	mainTemplates, err := fs.Glob(web.Templates, adminTemplateMainPattern)
	if err != nil {
		return nil, nil, err
	}
	partialTemplates, err := fs.Glob(web.Templates, adminTemplatePartialPattern)
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(mainTemplates)
	sort.Strings(partialTemplates)
	return mainTemplates, partialTemplates, nil
}

// ParseAdminTemplateSet parses a specific embedded admin template subset.
func ParseAdminTemplateSet(paths ...string) (*template.Template, error) {
	return template.New("").Funcs(AdminTemplateFuncMap()).ParseFS(web.Templates, paths...)
}

// ParseAdminTemplateWithPartials parses one embedded template with all shared partials.
func ParseAdminTemplateWithPartials(templatePath string) (*template.Template, error) {
	if strings.Contains(templatePath, "/partials/") {
		return ParseAdminTemplateSet(templatePath)
	}

	_, partialTemplates, err := AdminTemplateFiles()
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, 1+len(partialTemplates))
	paths = append(paths, templatePath)
	paths = append(paths, partialTemplates...)
	return ParseAdminTemplateSet(paths...)
}
