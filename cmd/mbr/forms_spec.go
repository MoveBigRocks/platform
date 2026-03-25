package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

const formSpecSelection = `
id
workspaceID
name
slug
publicKey
descriptionMarkdown
fieldSpec
evidenceRequirements
inferenceRules
approvalPolicy
submissionPolicy
destinationPolicy
supportedChannels
isPublic
status
metadata
createdByID
createdAt
updatedAt
deletedAt
`

const formSubmissionSelection = `
id
workspaceID
formSpecID
conversationSessionID
caseID
contactID
status
channel
submitterEmail
submitterName
completionToken
collectedFields
missingFields
evidence
validationErrors
metadata
submittedAt
createdAt
updatedAt
`

type formSpecOutput struct {
	ID                   string           `json:"id"`
	WorkspaceID          string           `json:"workspaceID"`
	Name                 string           `json:"name"`
	Slug                 string           `json:"slug"`
	PublicKey            *string          `json:"publicKey,omitempty"`
	DescriptionMarkdown  *string          `json:"descriptionMarkdown,omitempty"`
	FieldSpec            map[string]any   `json:"fieldSpec"`
	EvidenceRequirements []map[string]any `json:"evidenceRequirements"`
	InferenceRules       []map[string]any `json:"inferenceRules"`
	ApprovalPolicy       map[string]any   `json:"approvalPolicy"`
	SubmissionPolicy     map[string]any   `json:"submissionPolicy"`
	DestinationPolicy    map[string]any   `json:"destinationPolicy"`
	SupportedChannels    []string         `json:"supportedChannels"`
	IsPublic             bool             `json:"isPublic"`
	Status               string           `json:"status"`
	Metadata             map[string]any   `json:"metadata"`
	CreatedByID          *string          `json:"createdByID,omitempty"`
	CreatedAt            string           `json:"createdAt"`
	UpdatedAt            string           `json:"updatedAt"`
	DeletedAt            *string          `json:"deletedAt,omitempty"`
}

type formSubmissionOutput struct {
	ID                    string           `json:"id"`
	WorkspaceID           string           `json:"workspaceID"`
	FormSpecID            string           `json:"formSpecID"`
	ConversationSessionID *string          `json:"conversationSessionID,omitempty"`
	CaseID                *string          `json:"caseID,omitempty"`
	ContactID             *string          `json:"contactID,omitempty"`
	Status                string           `json:"status"`
	Channel               string           `json:"channel"`
	SubmitterEmail        *string          `json:"submitterEmail,omitempty"`
	SubmitterName         *string          `json:"submitterName,omitempty"`
	CompletionToken       *string          `json:"completionToken,omitempty"`
	CollectedFields       map[string]any   `json:"collectedFields"`
	MissingFields         map[string]any   `json:"missingFields"`
	Evidence              []map[string]any `json:"evidence"`
	ValidationErrors      []string         `json:"validationErrors"`
	Metadata              map[string]any   `json:"metadata"`
	SubmittedAt           *string          `json:"submittedAt,omitempty"`
	CreatedAt             string           `json:"createdAt"`
	UpdatedAt             string           `json:"updatedAt"`
}

func runForms(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printFormsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "specs":
		switch args[1] {
		case "list":
			fs := flag.NewFlagSet("mbr form specs list", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			workspaceID := fs.String("workspace", "", "Workspace ID")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			if err := fs.Parse(args[2:]); err != nil {
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

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)

			var payload struct {
				FormSpecs []formSpecOutput `json:"formSpecs"`
			}
			err = client.Query(ctx, `
				query CLIFormSpecs($workspaceID: ID!) {
				  formSpecs(workspaceID: $workspaceID) {
				    `+formSpecSelection+`
				  }
				}
			`, map[string]any{"workspaceID": workspaceValue}, &payload)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, payload.FormSpecs, stderr)
			}
			if len(payload.FormSpecs) == 0 {
				fmt.Fprintln(stdout, "no form specs found")
				return 0
			}
			for _, spec := range payload.FormSpecs {
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", spec.ID, spec.Slug, spec.Status, spec.Name)
			}
			return 0
		case "create":
			fs := flag.NewFlagSet("mbr form specs create", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			workspaceID := fs.String("workspace", "", "Workspace ID")
			inputFile := fs.String("input-file", "", "Path to form spec input JSON")
			inputJSON := fs.String("input-json", "", "Inline form spec input JSON")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			if err := fs.Parse(args[2:]); err != nil {
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
			input, err := readRequiredJSONObjectFlagInput(*inputFile, *inputJSON, "form spec input")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			input["workspaceID"] = workspaceValue

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)
			spec, err := runFormSpecCreate(ctx, client, input)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, spec, stderr)
			}
			printFormSpec(stdout, spec)
			return 0
		case "show":
			fs := flag.NewFlagSet("mbr form specs show", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			flagArgs, positionals := splitSinglePositionalArgs(args[2:], map[string]bool{
				"--url":       true,
				"--api-url":   true,
				"--token":     true,
				"--workspace": true,
				"--json":      false,
			})
			if err := fs.Parse(flagArgs); err != nil {
				return 2
			}
			if len(positionals) != 1 {
				fmt.Fprintln(stderr, "form spec identifier is required")
				return 2
			}
			identifier := strings.TrimSpace(positionals[0])
			if identifier == "" {
				fmt.Fprintln(stderr, "form spec identifier is required")
				return 2
			}
			stored, err := cliapi.LoadStoredConfig()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)

			spec, err := runFormSpecShow(ctx, client, identifier, workspaceValue)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, spec, stderr)
			}

			fmt.Fprintf(stdout, "id:\t%s\n", spec.ID)
			fmt.Fprintf(stdout, "workspace:\t%s\n", spec.WorkspaceID)
			fmt.Fprintf(stdout, "slug:\t%s\n", spec.Slug)
			fmt.Fprintf(stdout, "name:\t%s\n", spec.Name)
			fmt.Fprintf(stdout, "status:\t%s\n", spec.Status)
			fmt.Fprintf(stdout, "public:\t%t\n", spec.IsPublic)
			if spec.PublicKey != nil {
				fmt.Fprintf(stdout, "publicKey:\t%s\n", *spec.PublicKey)
			}
			if spec.DescriptionMarkdown != nil {
				fmt.Fprintf(stdout, "description:\t%s\n", *spec.DescriptionMarkdown)
			}
			if len(spec.SupportedChannels) > 0 {
				fmt.Fprintf(stdout, "channels:\t%s\n", strings.Join(spec.SupportedChannels, ","))
			}
			fmt.Fprintf(stdout, "createdAt:\t%s\n", spec.CreatedAt)
			fmt.Fprintf(stdout, "updatedAt:\t%s\n", spec.UpdatedAt)
			return 0
		case "update":
			fs := flag.NewFlagSet("mbr form specs update", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
			inputFile := fs.String("input-file", "", "Path to form spec input JSON")
			inputJSON := fs.String("input-json", "", "Inline form spec input JSON")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			flagArgs, positionals := splitSinglePositionalArgs(args[2:], map[string]bool{
				"--url":        true,
				"--api-url":    true,
				"--token":      true,
				"--workspace":  true,
				"--input-file": true,
				"--input-json": true,
				"--json":       false,
			})
			if err := fs.Parse(flagArgs); err != nil {
				return 2
			}
			if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
				fmt.Fprintln(stderr, "form spec identifier is required")
				return 2
			}
			input, err := readRequiredJSONObjectFlagInput(*inputFile, *inputJSON, "form spec input")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			stored, err := cliapi.LoadStoredConfig()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)
			spec, err := runFormSpecShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			updated, err := runFormSpecUpdate(ctx, client, spec.ID, input)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, updated, stderr)
			}
			printFormSpec(stdout, updated)
			return 0
		default:
			fmt.Fprintf(stderr, "unknown form specs command %q\n\n", args[1])
			printFormsUsage(stderr)
			return 2
		}
	case "submissions":
		switch args[1] {
		case "list":
			fs := flag.NewFlagSet("mbr form submissions list", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			workspaceID := fs.String("workspace", "", "Workspace ID")
			formSpecID := fs.String("spec", "", "Form spec ID")
			status := fs.String("status", "", "Submission status filter")
			limit := fs.Int("limit", 20, "Maximum number of submissions to return")
			offset := fs.Int("offset", 0, "Number of submissions to skip")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			if err := fs.Parse(args[2:]); err != nil {
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
			if *offset < 0 {
				fmt.Fprintln(stderr, "--offset must be 0 or greater")
				return 2
			}

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)

			filter := map[string]any{
				"limit":  *limit,
				"offset": *offset,
			}
			if value := strings.TrimSpace(*formSpecID); value != "" {
				filter["formSpecID"] = value
			}
			if value := strings.TrimSpace(*status); value != "" {
				filter["status"] = strings.ToLower(value)
			}

			var payload struct {
				FormSubmissions []formSubmissionOutput `json:"formSubmissions"`
			}
			err = client.Query(ctx, `
				query CLIFormSubmissions($workspaceID: ID!, $filter: FormSubmissionFilter) {
				  formSubmissions(workspaceID: $workspaceID, filter: $filter) {
				    `+formSubmissionSelection+`
				  }
				}
			`, map[string]any{
				"workspaceID": workspaceValue,
				"filter":      filter,
			}, &payload)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, payload.FormSubmissions, stderr)
			}
			if len(payload.FormSubmissions) == 0 {
				fmt.Fprintln(stdout, "no form submissions found")
				return 0
			}
			for _, submission := range payload.FormSubmissions {
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", submission.ID, submission.FormSpecID, submission.Status, coalesce(submission.SubmitterEmail, ""))
			}
			return 0
		case "create":
			fs := flag.NewFlagSet("mbr form submissions create", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			workspaceID := fs.String("workspace", "", "Workspace ID for slug lookup")
			inputFile := fs.String("input-file", "", "Path to form submission input JSON")
			inputJSON := fs.String("input-json", "", "Inline form submission input JSON")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			flagArgs, positionals := splitSinglePositionalArgs(args[2:], map[string]bool{
				"--url":        true,
				"--api-url":    true,
				"--token":      true,
				"--workspace":  true,
				"--input-file": true,
				"--input-json": true,
				"--json":       false,
			})
			if err := fs.Parse(flagArgs); err != nil {
				return 2
			}
			if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
				fmt.Fprintln(stderr, "form spec identifier is required")
				return 2
			}
			input, err := readRequiredJSONObjectFlagInput(*inputFile, *inputJSON, "form submission input")
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			stored, err := cliapi.LoadStoredConfig()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			workspaceValue := resolveStoredWorkspaceID(*workspaceID, stored)

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)
			spec, err := runFormSpecShow(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			input["formSpecID"] = spec.ID
			submission, err := runFormSubmissionCreate(ctx, client, input)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, submission, stderr)
			}
			printFormSubmission(stdout, submission)
			return 0
		case "show":
			fs := flag.NewFlagSet("mbr form submissions show", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			flagArgs, positionals := splitSinglePositionalArgs(args[2:], map[string]bool{
				"--url":     true,
				"--api-url": true,
				"--token":   true,
				"--json":    false,
			})
			if err := fs.Parse(flagArgs); err != nil {
				return 2
			}
			if len(positionals) != 1 {
				fmt.Fprintln(stderr, "form submission identifier is required")
				return 2
			}
			identifier := strings.TrimSpace(positionals[0])
			if identifier == "" {
				fmt.Fprintln(stderr, "form submission identifier is required")
				return 2
			}

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)

			submission, err := runFormSubmissionShow(ctx, client, identifier)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, submission, stderr)
			}

			fmt.Fprintf(stdout, "id:\t%s\n", submission.ID)
			fmt.Fprintf(stdout, "workspace:\t%s\n", submission.WorkspaceID)
			fmt.Fprintf(stdout, "formSpecID:\t%s\n", submission.FormSpecID)
			fmt.Fprintf(stdout, "status:\t%s\n", submission.Status)
			fmt.Fprintf(stdout, "channel:\t%s\n", submission.Channel)
			if submission.SubmitterEmail != nil {
				fmt.Fprintf(stdout, "submitterEmail:\t%s\n", *submission.SubmitterEmail)
			}
			if submission.SubmitterName != nil {
				fmt.Fprintf(stdout, "submitterName:\t%s\n", *submission.SubmitterName)
			}
			if submission.CompletionToken != nil {
				fmt.Fprintf(stdout, "completionToken:\t%s\n", *submission.CompletionToken)
			}
			if submission.SubmittedAt != nil {
				fmt.Fprintf(stdout, "submittedAt:\t%s\n", *submission.SubmittedAt)
			}
			fmt.Fprintf(stdout, "createdAt:\t%s\n", submission.CreatedAt)
			fmt.Fprintf(stdout, "updatedAt:\t%s\n", submission.UpdatedAt)
			if len(submission.ValidationErrors) > 0 {
				fmt.Fprintf(stdout, "validationErrors:\t%s\n", strings.Join(submission.ValidationErrors, " | "))
			}
			return 0
		default:
			fmt.Fprintf(stderr, "unknown form submissions command %q\n\n", args[1])
			printFormsUsage(stderr)
			return 2
		}
	default:
		fmt.Fprintf(stderr, "unknown forms command %q\n\n", args[0])
		printFormsUsage(stderr)
		return 2
	}
}

func runFormSpecShow(ctx context.Context, client *cliapi.Client, identifier, workspaceID string) (formSpecOutput, error) {
	if strings.TrimSpace(workspaceID) != "" {
		spec, err := runFormSpecShowBySlug(ctx, client, workspaceID, identifier)
		if err != nil {
			return formSpecOutput{}, err
		}
		if spec != nil {
			return *spec, nil
		}
	}

	var payload struct {
		FormSpec *formSpecOutput `json:"formSpec"`
	}
	err := client.Query(ctx, `
		query CLIFormSpec($id: ID!) {
		  formSpec(id: $id) {
		    `+formSpecSelection+`
		  }
		}
	`, map[string]any{"id": identifier}, &payload)
	if err != nil {
		return formSpecOutput{}, err
	}
	if payload.FormSpec == nil {
		return formSpecOutput{}, fmt.Errorf("form spec not found")
	}
	return *payload.FormSpec, nil
}

func runFormSpecShowBySlug(ctx context.Context, client *cliapi.Client, workspaceID, slug string) (*formSpecOutput, error) {
	var payload struct {
		FormSpecBySlug *formSpecOutput `json:"formSpecBySlug"`
	}
	err := client.Query(ctx, `
		query CLIFormSpecBySlug($workspaceID: ID!, $slug: String!) {
		  formSpecBySlug(workspaceID: $workspaceID, slug: $slug) {
		    `+formSpecSelection+`
		  }
		}
	`, map[string]any{
		"workspaceID": workspaceID,
		"slug":        slug,
	}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.FormSpecBySlug, nil
}

func runFormSubmissionShow(ctx context.Context, client *cliapi.Client, identifier string) (formSubmissionOutput, error) {
	var payload struct {
		FormSubmission *formSubmissionOutput `json:"formSubmission"`
	}
	err := client.Query(ctx, `
		query CLIFormSubmission($id: ID!) {
		  formSubmission(id: $id) {
		    `+formSubmissionSelection+`
		  }
		}
	`, map[string]any{"id": identifier}, &payload)
	if err != nil {
		return formSubmissionOutput{}, err
	}
	if payload.FormSubmission == nil {
		return formSubmissionOutput{}, fmt.Errorf("form submission not found")
	}
	return *payload.FormSubmission, nil
}

func runFormSpecCreate(ctx context.Context, client *cliapi.Client, input map[string]any) (formSpecOutput, error) {
	var payload struct {
		CreateFormSpec *formSpecOutput `json:"createFormSpec"`
	}
	err := client.Query(ctx, `
		mutation CLICreateFormSpec($input: CreateFormSpecInput!) {
		  createFormSpec(input: $input) {
		    `+formSpecSelection+`
		  }
		}
	`, map[string]any{"input": input}, &payload)
	if err != nil {
		return formSpecOutput{}, err
	}
	if payload.CreateFormSpec == nil {
		return formSpecOutput{}, fmt.Errorf("form spec creation returned no payload")
	}
	return *payload.CreateFormSpec, nil
}

func runFormSpecUpdate(ctx context.Context, client *cliapi.Client, specID string, input map[string]any) (formSpecOutput, error) {
	var payload struct {
		UpdateFormSpec *formSpecOutput `json:"updateFormSpec"`
	}
	err := client.Query(ctx, `
		mutation CLIUpdateFormSpec($id: ID!, $input: UpdateFormSpecInput!) {
		  updateFormSpec(id: $id, input: $input) {
		    `+formSpecSelection+`
		  }
		}
	`, map[string]any{
		"id":    specID,
		"input": input,
	}, &payload)
	if err != nil {
		return formSpecOutput{}, err
	}
	if payload.UpdateFormSpec == nil {
		return formSpecOutput{}, fmt.Errorf("form spec update returned no payload")
	}
	return *payload.UpdateFormSpec, nil
}

func runFormSubmissionCreate(ctx context.Context, client *cliapi.Client, input map[string]any) (formSubmissionOutput, error) {
	var payload struct {
		CreateFormSubmission *formSubmissionOutput `json:"createFormSubmission"`
	}
	err := client.Query(ctx, `
		mutation CLICreateFormSubmission($input: CreateFormSubmissionInput!) {
		  createFormSubmission(input: $input) {
		    `+formSubmissionSelection+`
		  }
		}
	`, map[string]any{"input": input}, &payload)
	if err != nil {
		return formSubmissionOutput{}, err
	}
	if payload.CreateFormSubmission == nil {
		return formSubmissionOutput{}, fmt.Errorf("form submission creation returned no payload")
	}
	return *payload.CreateFormSubmission, nil
}

func readRequiredJSONObjectFlagInput(path, inline, fieldName string) (map[string]any, error) {
	if strings.TrimSpace(path) == "" && strings.TrimSpace(inline) == "" {
		return nil, fmt.Errorf("--input-file or --input-json is required for %s", fieldName)
	}
	return readConfigInput(path, inline)
}

func printFormSpec(w io.Writer, spec formSpecOutput) {
	fmt.Fprintf(w, "id:\t%s\n", spec.ID)
	fmt.Fprintf(w, "workspace:\t%s\n", spec.WorkspaceID)
	fmt.Fprintf(w, "slug:\t%s\n", spec.Slug)
	fmt.Fprintf(w, "name:\t%s\n", spec.Name)
	fmt.Fprintf(w, "status:\t%s\n", spec.Status)
	fmt.Fprintf(w, "public:\t%t\n", spec.IsPublic)
	if spec.PublicKey != nil {
		fmt.Fprintf(w, "publicKey:\t%s\n", *spec.PublicKey)
	}
	if spec.DescriptionMarkdown != nil {
		fmt.Fprintf(w, "description:\t%s\n", *spec.DescriptionMarkdown)
	}
	if len(spec.SupportedChannels) > 0 {
		fmt.Fprintf(w, "channels:\t%s\n", strings.Join(spec.SupportedChannels, ","))
	}
	fmt.Fprintf(w, "createdAt:\t%s\n", spec.CreatedAt)
	fmt.Fprintf(w, "updatedAt:\t%s\n", spec.UpdatedAt)
}

func printFormSubmission(w io.Writer, submission formSubmissionOutput) {
	fmt.Fprintf(w, "id:\t%s\n", submission.ID)
	fmt.Fprintf(w, "workspace:\t%s\n", submission.WorkspaceID)
	fmt.Fprintf(w, "formSpecID:\t%s\n", submission.FormSpecID)
	fmt.Fprintf(w, "status:\t%s\n", submission.Status)
	fmt.Fprintf(w, "channel:\t%s\n", submission.Channel)
	if submission.SubmitterEmail != nil {
		fmt.Fprintf(w, "submitterEmail:\t%s\n", *submission.SubmitterEmail)
	}
	if submission.SubmitterName != nil {
		fmt.Fprintf(w, "submitterName:\t%s\n", *submission.SubmitterName)
	}
	if submission.CompletionToken != nil {
		fmt.Fprintf(w, "completionToken:\t%s\n", *submission.CompletionToken)
	}
	if submission.SubmittedAt != nil {
		fmt.Fprintf(w, "submittedAt:\t%s\n", *submission.SubmittedAt)
	}
	fmt.Fprintf(w, "createdAt:\t%s\n", submission.CreatedAt)
	fmt.Fprintf(w, "updatedAt:\t%s\n", submission.UpdatedAt)
	if len(submission.ValidationErrors) > 0 {
		fmt.Fprintf(w, "validationErrors:\t%s\n", strings.Join(submission.ValidationErrors, " | "))
	}
}
