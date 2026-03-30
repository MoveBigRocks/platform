package servicehandlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	automationservices "github.com/movebigrocks/platform/pkg/extensionhost/automation/services"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/middleware"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// templateFuncs provides helper functions for templates
var templateFuncs = template.FuncMap{
	"json": func(v interface{}) template.JS {
		b, err := json.Marshal(v)
		if err != nil {
			return template.JS("{}")
		}
		return template.JS(b)
	},
}

const publicFormCryptoParam = "crypto_id"

// FormPublicHandler handles public form access and submissions
type FormPublicHandler struct {
	formService *automationservices.FormService
	logger      *logger.Logger
}

// NewFormPublicHandler creates a new public form handler
func NewFormPublicHandler(formService *automationservices.FormService) *FormPublicHandler {
	return &FormPublicHandler{
		formService: formService,
		logger:      logger.New().WithField("handler", "form-public"),
	}
}

// RenderPublicForm renders a public form by CryptoID
func (h *FormPublicHandler) RenderPublicForm(c *gin.Context) {
	identifier := c.Param(publicFormCryptoParam)

	form, err := h.formService.GetFormByCryptoID(c.Request.Context(), identifier)
	if err != nil {
		c.HTML(http.StatusNotFound, "form_public_error.html", gin.H{
			"Error": "Form not found",
		})
		return
	}

	// Check if form is public or embeddable
	if !form.IsPublic && !form.AllowEmbed {
		c.HTML(http.StatusForbidden, "form_public_error.html", gin.H{
			"Error": "This form is not publicly accessible",
		})
		return
	}

	// Check form status
	if form.Status != servicedomain.FormStatusActive {
		c.HTML(http.StatusGone, "form_public_error.html", gin.H{
			"Error": "This form is no longer accepting submissions",
		})
		return
	}

	// Check embedding domain if embedded
	referer := c.GetHeader("Referer")
	if referer != "" && len(form.EmbedDomains) > 0 {
		allowed := false
		for _, domain := range form.EmbedDomains {
			if matchDomain(referer, domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.HTML(http.StatusForbidden, "form_public_error.html", gin.H{
				"Error": "This form cannot be embedded on this domain",
			})
			return
		}
	}

	// Parse fields for rendering
	fields := extractFormFields(form)

	c.HTML(http.StatusOK, "form_public.html", gin.H{
		"Form":             form,
		"Fields":           fields,
		"HasMultipleSteps": form.HasWorkflow && len(form.WorkflowStates) > 1,
		"WorkflowStates":   form.WorkflowStates,
	})
}

// SubmitPublicForm handles public form submissions
func (h *FormPublicHandler) SubmitPublicForm(c *gin.Context) {
	identifier := c.Param(publicFormCryptoParam)

	// Rate limiting: limit submissions per IP address
	// Allow 10 submissions per minute per IP, block for 5 minutes if exceeded
	clientIP := c.ClientIP()

	allowed, retryAfter, err := h.formService.CheckSubmissionRateLimit(
		c.Request.Context(),
		clientIP,
		10,            // max 10 submissions per window
		time.Minute,   // 1 minute window
		5*time.Minute, // block for 5 minutes if exceeded
	)
	if err != nil {
		// Fail-closed: reject if rate limiter errors to prevent abuse during outages
		h.logger.WithError(err).Error("Rate limit check failed, rejecting request for safety")
		middleware.RespondWithError(c, http.StatusServiceUnavailable, "Service temporarily unavailable. Please try again.")
		return
	}
	if !allowed {
		c.Header("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
		middleware.RespondWithError(c, http.StatusTooManyRequests, "Too many submissions. Please try again later.")
		return
	}

	// Find form by CryptoID
	form, err := h.formService.GetFormByCryptoID(c.Request.Context(), identifier)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Form not found")
		return
	}

	// Validate form is accepting submissions
	if !form.IsPublic && !form.AllowEmbed {
		middleware.RespondWithError(c, http.StatusForbidden, "Form is not publicly accessible")
		return
	}

	if form.Status != servicedomain.FormStatusActive {
		middleware.RespondWithError(c, http.StatusGone, "Form is not accepting submissions")
		return
	}

	// Parse submission data
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid submission data")
		return
	}

	// Extract submitter info
	submitterEmail := ""
	submitterName := ""
	if email, ok := data["email"].(string); ok {
		submitterEmail = email
	}
	if name, ok := data["name"].(string); ok {
		submitterName = name
	}

	// Create submission
	submission := &servicedomain.PublicFormSubmission{
		WorkspaceID:    form.WorkspaceID,
		FormID:         form.ID,
		Data:           shareddomain.MetadataFromMap(data),
		SubmitterEmail: submitterEmail,
		SubmitterName:  submitterName,
		SubmitterIP:    c.ClientIP(),
		UserAgent:      c.GetHeader("User-Agent"),
		Referrer:       c.GetHeader("Referer"),
		Status:         servicedomain.SubmissionStatusPending,
		IsValid:        true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	event := contracts.NewFormSubmittedEvent(
		form.ID,
		form.Slug,
		submission.ID,
		form.WorkspaceID,
		submitterEmail,
		submitterName,
		data,
	)

	// Save submission with tenant context for RLS and persist the form event
	// in the same transaction to keep downstream automation durable.
	if err := h.formService.CreatePublicSubmission(c.Request.Context(), form.WorkspaceID, submission, &event); err != nil {
		h.logger.WithError(err).Error("Failed to save form submission", "form_id", form.ID)
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to save submission")
		return
	}

	// Return success response
	response := gin.H{
		"success":       true,
		"submission_id": submission.ID,
	}

	if form.SubmissionMessage != "" {
		response["message"] = form.SubmissionMessage
	} else {
		response["message"] = "Thank you for your submission!"
	}

	if form.RedirectURL != "" {
		response["redirect_url"] = form.RedirectURL
	}

	c.JSON(http.StatusOK, response)
}

// GetEmbedScript returns the JavaScript embed snippet
func (h *FormPublicHandler) GetEmbedScript(c *gin.Context) {
	identifier := c.Param(publicFormCryptoParam)

	form, err := h.formService.GetFormByCryptoID(c.Request.Context(), identifier)
	if err != nil {
		c.String(http.StatusNotFound, "// Form not found")
		return
	}

	if !form.AllowEmbed && !form.IsPublic {
		c.String(http.StatusForbidden, "// Form embedding not allowed")
		return
	}

	// Get base URL
	baseURL := "https://" + c.Request.Host

	// Generate embed script
	script := fmt.Sprintf(`(function() {
  var containerId = 'mbr-form-%s';
  var container = document.getElementById(containerId);
  if (!container) {
    console.error('Move Big Rocks Forms: Container #' + containerId + ' not found');
    return;
  }

  var iframe = document.createElement('iframe');
  iframe.src = '%s/forms/public/%s?embed=true';
  iframe.style.width = '100%%';
  iframe.style.border = 'none';
  iframe.style.minHeight = '400px';
  iframe.setAttribute('allowfullscreen', 'true');

  // Auto-resize iframe based on content
  var iframeOrigin = new URL(iframe.src).origin;
  window.addEventListener('message', function(e) {
    if (e.origin !== iframeOrigin) {
      return;
    }
    if (e.data && e.data.type === 'mbr-form-resize' && e.data.formId === '%s') {
      iframe.style.height = e.data.height + 'px';
    }
    if (e.data && e.data.type === 'mbr-form-submitted' && e.data.formId === '%s') {
      if (e.data.redirectUrl) {
        try {
          var redirectUrl = new URL(e.data.redirectUrl, iframeOrigin);
          if (redirectUrl.origin !== iframeOrigin) {
            return;
          }
          window.location.href = redirectUrl.href;
        } catch (err) {
          console.error('Move Big Rocks Forms: Invalid redirect URL');
        }
      }
    }
  });

  container.appendChild(iframe);
})();`, form.Slug, baseURL, identifier, form.ID, form.ID)

	c.Header("Content-Type", "application/javascript")
	c.String(http.StatusOK, script)
}

// Helper functions

// matchDomain checks if a referer URL matches the allowed domain pattern.
// Patterns can be:
//   - Exact domain: "example.com" matches only example.com
//   - Wildcard subdomain: "*.example.com" matches sub.example.com but NOT example.com
//
// The matching is performed against the parsed URL host only, not the full URL,
// preventing path injection attacks (e.g., "evil.com/example.com" won't match "example.com").
func matchDomain(referer, pattern string) bool {
	// Parse the referer URL to extract the host
	parsed, err := url.Parse(referer)
	if err != nil {
		return false
	}

	// Get the hostname (without port)
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}

	// Normalize pattern to lowercase for case-insensitive matching
	pattern = strings.ToLower(pattern)

	// Handle wildcard patterns like "*.example.com"
	if strings.HasPrefix(pattern, "*.") {
		baseDomain := pattern[2:] // "example.com" from "*.example.com"

		// The host must be a subdomain of baseDomain
		// e.g., "sub.example.com" should match "*.example.com"
		// but "example.com" should NOT match "*.example.com"
		// and "malicious-example.com" should NOT match "*.example.com"

		if !strings.HasSuffix(host, "."+baseDomain) {
			return false
		}

		// Verify it's actually a subdomain (not empty prefix)
		// "sub.example.com" -> prefix is "sub"
		prefix := strings.TrimSuffix(host, "."+baseDomain)
		return prefix != ""
	}

	// Exact domain match only
	return host == pattern
}

func extractFormFields(form *servicedomain.FormSchema) []map[string]interface{} {
	fields := []map[string]interface{}{}

	schema, err := form.ParseSchemaData()
	if err != nil || schema.Properties == nil {
		return fields
	}

	for name, prop := range schema.Properties {
		field := map[string]interface{}{
			"name":  name,
			"type":  prop.Type,
			"label": prop.Title,
		}

		// Check required
		for _, req := range schema.Required {
			if req == name {
				field["required"] = true
				break
			}
		}

		// Add description
		if prop.Description != "" {
			field["description"] = prop.Description
		}

		// Add enum options
		if len(prop.Enum) > 0 {
			field["options"] = prop.Enum
		}

		// Add validation
		if prop.MinLength > 0 {
			field["minLength"] = prop.MinLength
		}
		if prop.MaxLength > 0 {
			field["maxLength"] = prop.MaxLength
		}
		if prop.Pattern != "" {
			field["pattern"] = prop.Pattern
		}

		// Get UI schema hints
		if !form.UISchema.IsEmpty() {
			if uiFieldRaw, ok := form.UISchema.Get(name); ok {
				if uiField, ok := uiFieldRaw.(map[string]interface{}); ok {
					if widget, ok := uiField["ui:widget"].(string); ok {
						field["widget"] = widget
					}
					if placeholder, ok := uiField["ui:placeholder"].(string); ok {
						field["placeholder"] = placeholder
					}
					if help, ok := uiField["ui:help"].(string); ok {
						field["help"] = help
					}
				}
			}
		}

		// Get conditions from validation rules
		if !form.ValidationRules.IsEmpty() {
			if fieldRulesRaw, ok := form.ValidationRules.Get(name); ok {
				if fieldRules, ok := fieldRulesRaw.(map[string]interface{}); ok {
					if conditions, ok := fieldRules["conditions"].(map[string]interface{}); ok {
						field["conditions"] = conditions
					}
				}
			}
		}

		fields = append(fields, field)
	}

	return fields
}

// FormAPITokenMiddleware returns a middleware that validates API tokens for programmatic form submissions
func (h *FormPublicHandler) FormAPITokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// Extract Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate token
		apiToken, err := h.formService.GetFormAPIToken(c.Request.Context(), token)
		if err != nil || apiToken == nil {
			middleware.RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Invalid API token")
			return
		}

		// Check if token is active
		if !apiToken.IsActive {
			middleware.RespondWithErrorAndAbort(c, http.StatusUnauthorized, "API token is inactive")
			return
		}

		// Check expiry
		if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now()) {
			middleware.RespondWithErrorAndAbort(c, http.StatusUnauthorized, "API token has expired")
			return
		}

		// Check host whitelist
		if len(apiToken.AllowedHosts) > 0 {
			clientIP := c.ClientIP()
			allowed := false
			for _, host := range apiToken.AllowedHosts {
				if host == clientIP || host == "*" {
					allowed = true
					break
				}
			}
			if !allowed {
				middleware.RespondWithErrorAndAbort(c, http.StatusForbidden, "Host not allowed")
				return
			}
		}

		// Set form ID in context
		c.Set("form_api_token", apiToken)
		c.Set("form_id", apiToken.FormID)

		c.Next()
	}
}

// PublicFormTemplate is the HTML template for public forms
var PublicFormTemplate = template.Must(template.New("form_public").Funcs(templateFuncs).Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Form.Name}}</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.5; color: #1a1a1a; background: #f5f5f5; }
        .form-container { max-width: 640px; margin: 2rem auto; padding: 2rem; background: white; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        .form-title { font-size: 1.5rem; font-weight: 600; margin-bottom: 0.5rem; }
        .form-description { color: #666; margin-bottom: 1.5rem; }
        .form-field { margin-bottom: 1.25rem; }
        .form-label { display: block; font-weight: 500; margin-bottom: 0.375rem; }
        .form-label .required { color: #dc2626; margin-left: 0.25rem; }
        .form-input { width: 100%; padding: 0.625rem 0.75rem; border: 1px solid #d1d5db; border-radius: 6px; font-size: 1rem; transition: border-color 0.15s; }
        .form-input:focus { outline: none; border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1); }
        .form-textarea { min-height: 120px; resize: vertical; }
        .form-select { appearance: none; background: white url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%23666' d='M6 8L1 3h10z'/%3E%3C/svg%3E") no-repeat right 0.75rem center; }
        .form-help { font-size: 0.875rem; color: #666; margin-top: 0.25rem; }
        .form-error { font-size: 0.875rem; color: #dc2626; margin-top: 0.25rem; }
        .form-submit { display: block; width: 100%; padding: 0.75rem 1.5rem; background: #3b82f6; color: white; border: none; border-radius: 6px; font-size: 1rem; font-weight: 500; cursor: pointer; transition: background 0.15s; }
        .form-submit:hover { background: #2563eb; }
        .form-submit:disabled { background: #9ca3af; cursor: not-allowed; }
        .form-success { text-align: center; padding: 2rem; }
        .form-success-icon { font-size: 3rem; color: #10b981; margin-bottom: 1rem; }
        .form-success-message { font-size: 1.125rem; color: #374151; }
        .form-steps { display: flex; gap: 0.5rem; margin-bottom: 1.5rem; }
        .form-step { flex: 1; padding: 0.5rem; text-align: center; font-size: 0.875rem; background: #e5e7eb; border-radius: 4px; }
        .form-step.active { background: #3b82f6; color: white; }
        .form-step.completed { background: #10b981; color: white; }
        .field-hidden { display: none; }
        .checkbox-group label, .radio-group label { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.5rem; cursor: pointer; }
        .checkbox-group input, .radio-group input { width: auto; }
    </style>
</head>
<body>
    <div class="form-container">
        <h1 class="form-title">{{.Form.Name}}</h1>
        {{if .Form.Description}}<p class="form-description">{{.Form.Description}}</p>{{end}}

        <form id="mbr-form" novalidate>
            {{range .Fields}}
            <div class="form-field" data-field="{{.name}}" {{if .conditions}}data-conditions='{{json .conditions}}'{{end}}>
                <label class="form-label">
                    {{.label}}
                    {{if .required}}<span class="required">*</span>{{end}}
                </label>

                {{if eq .widget "textarea"}}
                <textarea name="{{.name}}" class="form-input form-textarea" {{if .required}}required{{end}} {{if .placeholder}}placeholder="{{.placeholder}}"{{end}} {{if .minLength}}minlength="{{.minLength}}"{{end}} {{if .maxLength}}maxlength="{{.maxLength}}"{{end}}></textarea>
                {{else if eq .widget "select"}}
                <select name="{{.name}}" class="form-input form-select" {{if .required}}required{{end}}>
                    <option value="">Select...</option>
                    {{range .options}}<option value="{{.}}">{{.}}</option>{{end}}
                </select>
                {{else if eq .widget "radio"}}
                <div class="radio-group">
                    {{range $i, $opt := .options}}
                    <label><input type="radio" name="{{$.name}}" value="{{$opt}}" {{if eq $i 0}}{{if $.required}}required{{end}}{{end}}> {{$opt}}</label>
                    {{end}}
                </div>
                {{else if eq .widget "checkbox"}}
                <label class="checkbox-label"><input type="checkbox" name="{{.name}}" value="true"> {{.label}}</label>
                {{else if eq .type "boolean"}}
                <label class="checkbox-label"><input type="checkbox" name="{{.name}}" value="true"> Yes</label>
                {{else}}
                <input type="{{if eq .widget "email"}}email{{else if eq .widget "date"}}date{{else if eq .type "number"}}number{{else}}text{{end}}" name="{{.name}}" class="form-input" {{if .required}}required{{end}} {{if .placeholder}}placeholder="{{.placeholder}}"{{end}} {{if .pattern}}pattern="{{.pattern}}"{{end}} {{if .minLength}}minlength="{{.minLength}}"{{end}} {{if .maxLength}}maxlength="{{.maxLength}}"{{end}}>
                {{end}}

                {{if .help}}<p class="form-help">{{.help}}</p>{{end}}
                <p class="form-error" style="display:none;"></p>
            </div>
            {{end}}

            <button type="submit" class="form-submit">Submit</button>
        </form>

        <div id="form-success" class="form-success" style="display:none;">
            <div class="form-success-icon">✓</div>
            <p class="form-success-message">{{if .Form.SubmissionMessage}}{{.Form.SubmissionMessage}}{{else}}Thank you for your submission!{{end}}</p>
        </div>
    </div>

    <script>
    (function() {
        var form = document.getElementById('mbr-form');
        var successDiv = document.getElementById('form-success');
        var isEmbedded = window.location.search.indexOf('embed=true') > -1;

        // Condition evaluation
        function evaluateConditions() {
            document.querySelectorAll('[data-conditions]').forEach(function(field) {
                var conditions = JSON.parse(field.dataset.conditions || '{}');
                var formData = new FormData(form);
                var data = {};
                formData.forEach(function(v, k) { data[k] = v; });

                var visible = true;

                if (conditions.showWhen) {
                    visible = evaluateConditionGroup(conditions.showWhen, data, conditions.logic || 'and');
                }
                if (conditions.hideWhen && visible) {
                    visible = !evaluateConditionGroup(conditions.hideWhen, data, conditions.logic || 'and');
                }

                field.classList.toggle('field-hidden', !visible);

                // Handle requiredWhen
                var input = field.querySelector('input, select, textarea');
                if (input && conditions.requiredWhen) {
                    var required = evaluateConditionGroup(conditions.requiredWhen, data, conditions.logic || 'and');
                    input.required = required;
                }
            });
        }

        function evaluateConditionGroup(conditions, data, logic) {
            if (!conditions || conditions.length === 0) return true;

            var results = conditions.map(function(c) {
                return evaluateCondition(c, data);
            });

            if (logic === 'or') {
                return results.some(function(r) { return r; });
            }
            return results.every(function(r) { return r; });
        }

        function evaluateCondition(condition, data) {
            var value = data[condition.field];
            var target = condition.value;

            switch (condition.operator) {
                case 'equals': return value === target;
                case 'not_equals': return value !== target;
                case 'contains': return (value || '').indexOf(target) > -1;
                case 'not_contains': return (value || '').indexOf(target) === -1;
                case 'in': return Array.isArray(target) && target.indexOf(value) > -1;
                case 'not_in': return Array.isArray(target) && target.indexOf(value) === -1;
                case 'empty': return !value || value === '';
                case 'not_empty': return value && value !== '';
                case 'gt': return parseFloat(value) > parseFloat(target);
                case 'lt': return parseFloat(value) < parseFloat(target);
                case 'gte': return parseFloat(value) >= parseFloat(target);
                case 'lte': return parseFloat(value) <= parseFloat(target);
                default: return true;
            }
        }

        // Listen for changes
        form.addEventListener('change', evaluateConditions);
        form.addEventListener('input', evaluateConditions);
        evaluateConditions();

        // Handle submission
        form.addEventListener('submit', function(e) {
            e.preventDefault();

            var submitBtn = form.querySelector('.form-submit');
            submitBtn.disabled = true;
            submitBtn.textContent = 'Submitting...';

            var formData = new FormData(form);
            var data = {};
            formData.forEach(function(value, key) {
                data[key] = value;
            });

            fetch(window.location.pathname + '/submit', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            })
            .then(function(r) { return r.json(); })
            .then(function(result) {
                if (result.success) {
                    form.style.display = 'none';
                    successDiv.style.display = 'block';

                    if (isEmbedded) {
                        window.parent.postMessage({
                            type: 'mbr-form-submitted',
                            formId: '{{.Form.ID}}',
                            redirectUrl: result.redirect_url
                        }, '*');
                    } else if (result.redirect_url) {
                        window.location.href = result.redirect_url;
                    }
                } else {
                    alert(result.error || 'Submission failed');
                    submitBtn.disabled = false;
                    submitBtn.textContent = 'Submit';
                }
            })
            .catch(function(err) {
                alert('Network error: ' + err.message);
                submitBtn.disabled = false;
                submitBtn.textContent = 'Submit';
            });
        });

        // Report height for iframe resize
        if (isEmbedded) {
            function reportHeight() {
                window.parent.postMessage({
                    type: 'mbr-form-resize',
                    formId: '{{.Form.ID}}',
                    height: document.body.scrollHeight
                }, '*');
            }
            reportHeight();
            window.addEventListener('resize', reportHeight);
            new MutationObserver(reportHeight).observe(document.body, { childList: true, subtree: true });
        }
    })();
    </script>
</body>
</html>
`))
