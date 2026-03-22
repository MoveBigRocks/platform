package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

const caseSelection = `
id
caseID
workspaceID
subject
status
priority
teamID
queueID
queue {
  id
  name
}
contact {
  id
  email
  name
}
assignee {
  id
  email
  name
}
originatingConversationID
createdAt
updatedAt
resolvedAt
`

const caseConversationSelection = `
id
status
title
handlingTeamID
lastActivityAt
`

const caseCommunicationSelection = `
id
direction
channel
subject
body
createdAt
`

const caseWorkThreadSelection = `
id
kind
communicationID
conversationMessageID
conversationSessionID
channel
direction
role
visibility
subject
body
createdAt
`

const caseShowSelection = caseSelection + `
originatingConversation {
  ` + caseConversationSelection + `
}
communications {
  ` + caseCommunicationSelection + `
}
workThread {
  ` + caseWorkThreadSelection + `
}
`

type caseConversationOutput struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	Title          *string `json:"title,omitempty"`
	HandlingTeamID *string `json:"handlingTeamID,omitempty"`
	LastActivityAt string  `json:"lastActivityAt"`
}

type caseCommunicationOutput struct {
	ID        string  `json:"id"`
	Direction string  `json:"direction"`
	Channel   string  `json:"channel"`
	Subject   *string `json:"subject,omitempty"`
	Body      string  `json:"body"`
	CreatedAt string  `json:"createdAt"`
}

type caseWorkThreadEntryOutput struct {
	ID                    string  `json:"id"`
	Kind                  string  `json:"kind"`
	CommunicationID       *string `json:"communicationID,omitempty"`
	ConversationMessageID *string `json:"conversationMessageID,omitempty"`
	ConversationSessionID *string `json:"conversationSessionID,omitempty"`
	Channel               *string `json:"channel,omitempty"`
	Direction             *string `json:"direction,omitempty"`
	Role                  *string `json:"role,omitempty"`
	Visibility            *string `json:"visibility,omitempty"`
	Subject               *string `json:"subject,omitempty"`
	Body                  string  `json:"body"`
	CreatedAt             string  `json:"createdAt"`
}

type caseOutput struct {
	ID                        string                      `json:"id"`
	CaseID                    string                      `json:"caseID"`
	WorkspaceID               string                      `json:"workspaceID"`
	Subject                   string                      `json:"subject"`
	Status                    string                      `json:"status"`
	Priority                  string                      `json:"priority"`
	TeamID                    *string                     `json:"teamID,omitempty"`
	QueueID                   *string                     `json:"queueID,omitempty"`
	Queue                     *namedResource              `json:"queue,omitempty"`
	Contact                   *contactOutput              `json:"contact,omitempty"`
	Assignee                  *userOutput                 `json:"assignee,omitempty"`
	OriginatingConversationID *string                     `json:"originatingConversationID,omitempty"`
	OriginatingConversation   *caseConversationOutput     `json:"originatingConversation,omitempty"`
	Communications            []caseCommunicationOutput   `json:"communications,omitempty"`
	WorkThread                []caseWorkThreadEntryOutput `json:"workThread,omitempty"`
	CreatedAt                 string                      `json:"createdAt"`
	UpdatedAt                 string                      `json:"updatedAt"`
	ResolvedAt                *string                     `json:"resolvedAt,omitempty"`
}

type namedResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type contactOutput struct {
	ID    string  `json:"id"`
	Email string  `json:"email"`
	Name  *string `json:"name"`
}

func runCases(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printCasesUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr cases list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		status := fs.String("status", "", "Case status filter")
		priority := fs.String("priority", "", "Case priority filter")
		queueID := fs.String("queue", "", "Queue ID filter")
		assigneeID := fs.String("assignee", "", "Assignee ID filter")
		first := fs.Int("limit", 20, "Maximum number of cases to return")
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
		if *first <= 0 {
			fmt.Fprintln(stderr, "--limit must be greater than 0")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		filter := map[string]any{"first": *first}
		if value := strings.TrimSpace(*status); value != "" {
			filter["status"] = []string{value}
		}
		if value := strings.TrimSpace(*priority); value != "" {
			filter["priority"] = []string{value}
		}
		if value := strings.TrimSpace(*queueID); value != "" {
			filter["queueID"] = value
		}
		if value := strings.TrimSpace(*assigneeID); value != "" {
			filter["assigneeID"] = value
		}

		var payload struct {
			Cases struct {
				Edges []struct {
					Node caseOutput `json:"node"`
				} `json:"edges"`
				TotalCount int `json:"totalCount"`
			} `json:"cases"`
		}
		err = client.Query(ctx, `
			query CLICases($workspaceID: ID!, $filter: CaseFilter) {
			  cases(workspaceID: $workspaceID, filter: $filter) {
			    totalCount
			    edges {
			      node {
			        `+caseSelection+`
			      }
			    }
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

		cases := make([]caseOutput, 0, len(payload.Cases.Edges))
		for _, edge := range payload.Cases.Edges {
			cases = append(cases, edge.Node)
		}
		if *jsonOutput {
			return writeJSON(stdout, map[string]any{
				"totalCount": payload.Cases.TotalCount,
				"cases":      cases,
			}, stderr)
		}
		if len(cases) == 0 {
			fmt.Fprintln(stdout, "no cases found")
			return 0
		}
		for _, item := range cases {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", item.CaseID, item.Status, item.Priority, item.Subject)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr cases show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for human-readable case IDs")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
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
			fmt.Fprintln(stderr, "case identifier is required")
			return 2
		}
		identifier := strings.TrimSpace(positionals[0])
		if identifier == "" {
			fmt.Fprintln(stderr, "case identifier is required")
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

		caseObj, err := runCaseShow(ctx, client, identifier, workspaceValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}

		fmt.Fprintf(stdout, "id:\t%s\n", caseObj.ID)
		fmt.Fprintf(stdout, "caseID:\t%s\n", caseObj.CaseID)
		fmt.Fprintf(stdout, "workspace:\t%s\n", caseObj.WorkspaceID)
		fmt.Fprintf(stdout, "subject:\t%s\n", caseObj.Subject)
		fmt.Fprintf(stdout, "status:\t%s\n", caseObj.Status)
		fmt.Fprintf(stdout, "priority:\t%s\n", caseObj.Priority)
		if caseObj.TeamID != nil {
			fmt.Fprintf(stdout, "team:\t%s\n", *caseObj.TeamID)
		}
		if caseObj.Queue != nil {
			fmt.Fprintf(stdout, "queue:\t%s (%s)\n", caseObj.Queue.Name, caseObj.Queue.ID)
		} else if caseObj.QueueID != nil {
			fmt.Fprintf(stdout, "queue:\t%s\n", *caseObj.QueueID)
		}
		if caseObj.Contact != nil {
			fmt.Fprintf(stdout, "contact:\t%s <%s>\n", coalesce(caseObj.Contact.Name, "unknown"), caseObj.Contact.Email)
		}
		if caseObj.Assignee != nil {
			fmt.Fprintf(stdout, "assignee:\t%s <%s>\n", caseObj.Assignee.Name, caseObj.Assignee.Email)
		}
		if caseObj.OriginatingConversation != nil {
			fmt.Fprintf(stdout, "originatingConversation:\t%s (%s)\n", caseObj.OriginatingConversation.ID, caseObj.OriginatingConversation.Status)
		} else if caseObj.OriginatingConversationID != nil {
			fmt.Fprintf(stdout, "originatingConversation:\t%s\n", *caseObj.OriginatingConversationID)
		}
		fmt.Fprintf(stdout, "communications:\t%d\n", len(caseObj.Communications))
		fmt.Fprintf(stdout, "workThread:\t%d\n", len(caseObj.WorkThread))
		fmt.Fprintf(stdout, "createdAt:\t%s\n", caseObj.CreatedAt)
		fmt.Fprintf(stdout, "updatedAt:\t%s\n", caseObj.UpdatedAt)
		if caseObj.ResolvedAt != nil {
			fmt.Fprintf(stdout, "resolvedAt:\t%s\n", *caseObj.ResolvedAt)
		}
		for _, entry := range caseObj.WorkThread {
			headerParts := []string{entry.Kind}
			if entry.Channel != nil && strings.TrimSpace(*entry.Channel) != "" {
				headerParts = append(headerParts, *entry.Channel)
			}
			if entry.Direction != nil && strings.TrimSpace(*entry.Direction) != "" {
				headerParts = append(headerParts, *entry.Direction)
			}
			if entry.Role != nil && strings.TrimSpace(*entry.Role) != "" {
				headerParts = append(headerParts, *entry.Role)
			}
			if entry.Visibility != nil && strings.TrimSpace(*entry.Visibility) != "" {
				headerParts = append(headerParts, *entry.Visibility)
			}
			fmt.Fprintf(stdout, "\n[%s] %s\n", entry.CreatedAt, strings.Join(headerParts, "/"))
			if entry.Subject != nil && strings.TrimSpace(*entry.Subject) != "" {
				fmt.Fprintf(stdout, "%s\n", *entry.Subject)
			}
			if strings.TrimSpace(entry.Body) != "" {
				fmt.Fprintf(stdout, "%s\n", entry.Body)
			}
		}
		return 0
	case "set-status":
		fs := flag.NewFlagSet("mbr cases set-status", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for human-readable case IDs")
		status := fs.String("status", "", "New case status")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--status":    true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "case identifier is required")
			return 2
		}
		if strings.TrimSpace(*status) == "" {
			fmt.Fprintln(stderr, "--status is required")
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
		caseObj, err := runCaseSetStatus(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, strings.TrimSpace(*status))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", caseObj.CaseID, caseObj.Status, caseObj.Subject)
		return 0
	case "handoff":
		fs := flag.NewFlagSet("mbr cases handoff", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for human-readable case IDs")
		teamID := fs.String("team", "", "Target team ID")
		queueID := fs.String("queue", "", "Target queue ID")
		assigneeID := fs.String("assignee", "", "Target assignee user ID")
		reason := fs.String("reason", "", "Handoff reason")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--team":      true,
			"--queue":     true,
			"--assignee":  true,
			"--reason":    true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "case identifier is required")
			return 2
		}
		if strings.TrimSpace(*queueID) == "" {
			fmt.Fprintln(stderr, "--queue is required")
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
		caseObj, err := runCaseHandoff(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, strings.TrimSpace(*teamID), strings.TrimSpace(*queueID), strings.TrimSpace(*assigneeID), strings.TrimSpace(*reason))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", caseObj.CaseID, coalesce(caseObj.TeamID, ""), coalesce(caseObj.QueueID, ""), caseObj.Subject)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown cases command %q\n\n", args[0])
		printCasesUsage(stderr)
		return 2
	}
}

func runCaseShow(ctx context.Context, client *cliapi.Client, identifier, workspaceID string) (caseOutput, error) {
	if workspaceID != "" {
		var payload struct {
			CaseByHumanID *caseOutput `json:"caseByHumanID"`
		}
		err := client.Query(ctx, `
			query CLICaseByHumanID($workspaceID: ID!, $caseID: String!) {
			  caseByHumanID(workspaceID: $workspaceID, caseID: $caseID) {
			    `+caseShowSelection+`
			  }
			}
		`, map[string]any{
			"workspaceID": workspaceID,
			"caseID":      identifier,
		}, &payload)
		if err == nil && payload.CaseByHumanID != nil {
			return *payload.CaseByHumanID, nil
		}
		if err != nil {
			return caseOutput{}, err
		}
		return caseOutput{}, fmt.Errorf("case not found")
	}

	var payload struct {
		Case *caseOutput `json:"case"`
	}
	err := client.Query(ctx, `
		query CLICase($id: ID!) {
		  case(id: $id) {
		    `+caseShowSelection+`
		  }
		}
	`, map[string]any{"id": identifier}, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.Case == nil {
		return caseOutput{}, fmt.Errorf("case not found")
	}
	return *payload.Case, nil
}

func runCaseSetStatus(ctx context.Context, client *cliapi.Client, identifier, workspaceID, status string) (caseOutput, error) {
	caseObj, err := runCaseShow(ctx, client, identifier, workspaceID)
	if err != nil {
		return caseOutput{}, err
	}
	var payload struct {
		UpdateCaseStatus *caseOutput `json:"updateCaseStatus"`
	}
	err = client.Query(ctx, `
		mutation CLIUpdateCaseStatus($id: ID!, $status: CaseStatus!) {
		  updateCaseStatus(id: $id, status: $status) {
		    `+caseSelection+`
		  }
		}
	`, map[string]any{
		"id":     caseObj.ID,
		"status": strings.ToUpper(status),
	}, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.UpdateCaseStatus == nil {
		return caseOutput{}, fmt.Errorf("case status update returned no case")
	}
	return *payload.UpdateCaseStatus, nil
}

func runCaseHandoff(ctx context.Context, client *cliapi.Client, identifier, workspaceID, teamID, queueID, assigneeID, reason string) (caseOutput, error) {
	caseObj, err := runCaseShow(ctx, client, identifier, workspaceID)
	if err != nil {
		return caseOutput{}, err
	}
	input := map[string]any{
		"queueID": queueID,
	}
	if teamID != "" {
		input["teamID"] = teamID
	}
	if assigneeID != "" {
		input["assigneeID"] = assigneeID
	}
	if reason != "" {
		input["reason"] = reason
	}

	var payload struct {
		HandoffCase *caseOutput `json:"handoffCase"`
	}
	err = client.Query(ctx, `
		mutation CLIHandoffCase($id: ID!, $input: CaseHandoffInput!) {
		  handoffCase(id: $id, input: $input) {
		    `+caseSelection+`
		  }
		}
	`, map[string]any{
		"id":    caseObj.ID,
		"input": input,
	}, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.HandoffCase == nil {
		return caseOutput{}, fmt.Errorf("case handoff returned no case")
	}
	return *payload.HandoffCase, nil
}
