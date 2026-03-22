package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runAdminForms(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printFormsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr forms list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		status := fs.String("status", "", "Form status filter")
		limit := fs.Int("limit", 50, "Maximum number of forms to return")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue, err := requireWorkspaceID(*workspaceID, stored)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if *limit <= 0 {
			fmt.Fprintln(stderr, "--limit must be greater than 0")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "forms"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		filter := map[string]any{
			"workspaceID": workspaceValue,
			"first":       *limit,
		}
		if value := strings.TrimSpace(*status); value != "" {
			filter["status"] = value
		}

		var payload struct {
			AdminForms struct {
				Edges []struct {
					Node formOutput `json:"node"`
				} `json:"edges"`
				TotalCount int `json:"totalCount"`
			} `json:"adminForms"`
		}
		err = client.Query(ctx, `
			query CLIAdminForms($filter: AdminFormFilter) {
			  adminForms(filter: $filter) {
			    totalCount
			    edges {
			      node {
			        `+formSelection+`
			      }
			    }
			  }
			}
		`, map[string]any{"filter": filter}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		forms := make([]formOutput, 0, len(payload.AdminForms.Edges))
		for _, edge := range payload.AdminForms.Edges {
			forms = append(forms, edge.Node)
		}
		if *jsonOutput {
			return writeJSON(stdout, map[string]any{
				"totalCount": payload.AdminForms.TotalCount,
				"forms":      forms,
			}, stderr)
		}
		if len(forms) == 0 {
			fmt.Fprintln(stdout, "no forms found")
			return 0
		}
		for _, form := range forms {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", form.ID, form.Status, form.Slug, form.Name)
		}
		return 0
	case "create":
		fs := flag.NewFlagSet("mbr forms create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		name := fs.String("name", "", "Form name")
		slug := fs.String("slug", "", "Form slug")
		description := fs.String("description", "", "Form description")
		status := fs.String("status", "active", "Form status")
		definitionFile := fs.String("definition-file", "", "Path to form definition JSON file")
		definitionJSON := fs.String("definition-json", "", "Inline form definition JSON object")
		submissionMessage := fs.String("submission-message", "", "Optional submission confirmation message")
		redirectURL := fs.String("redirect-url", "", "Optional redirect URL")
		publicFlag := newOptionalBoolFlag()
		requiresCaptchaFlag := newOptionalBoolFlag()
		collectEmailFlag := newOptionalBoolFlag()
		autoCreateCaseFlag := newOptionalBoolFlag()
		fs.Var(publicFlag, "public", "Expose the form publicly")
		fs.Var(requiresCaptchaFlag, "requires-captcha", "Require CAPTCHA on public submission")
		fs.Var(collectEmailFlag, "collect-email", "Collect submitter email")
		fs.Var(autoCreateCaseFlag, "auto-create-case", "Automatically create a case on submission")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue, err := requireWorkspaceID(*workspaceID, stored)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if strings.TrimSpace(*name) == "" {
			fmt.Fprintln(stderr, "--name is required")
			return 2
		}
		if strings.TrimSpace(*slug) == "" {
			fmt.Fprintln(stderr, "--slug is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "forms"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		schemaData, err := readFormDefinitionInput(*definitionFile, *definitionJSON, *name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		input := map[string]any{
			"workspaceID": workspaceValue,
			"name":        *name,
			"slug":        *slug,
			"status":      strings.ToLower(strings.TrimSpace(*status)),
			"schemaData":  schemaData,
		}
		if value := strings.TrimSpace(*description); value != "" {
			input["description"] = value
		}
		if value := strings.TrimSpace(*submissionMessage); value != "" {
			input["submissionMessage"] = value
		}
		if value := strings.TrimSpace(*redirectURL); value != "" {
			input["redirectURL"] = value
		}
		if publicFlag.set {
			input["isPublic"] = publicFlag.value
		}
		if requiresCaptchaFlag.set {
			input["requiresCaptcha"] = requiresCaptchaFlag.value
		}
		if collectEmailFlag.set {
			input["collectEmail"] = collectEmailFlag.value
		}
		if autoCreateCaseFlag.set {
			input["autoCreateCase"] = autoCreateCaseFlag.value
		}

		var payload struct {
			AdminCreateForm formOutput `json:"adminCreateForm"`
		}
		err = client.Query(ctx, `
			mutation CLICreateForm($input: CreateFormInput!) {
			  adminCreateForm(input: $input) {
			    `+formSelection+`
			  }
			}
		`, map[string]any{"input": input}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.AdminCreateForm, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", payload.AdminCreateForm.ID, payload.AdminCreateForm.Status, payload.AdminCreateForm.Slug, payload.AdminCreateForm.Name)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown forms command %q\n\n", args[0])
		printFormsUsage(stderr)
		return 2
	}
}
