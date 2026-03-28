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
description
status
priority
category
channel
teamID
queueID
queue {
  id
  name
}
contactID
contactEmail
contactName
contact {
  id
  email
  name
}
assigneeID
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
fromName
fromUserID
fromAgentID
isInternal
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
	ID          string  `json:"id"`
	Direction   string  `json:"direction"`
	Channel     string  `json:"channel"`
	Subject     *string `json:"subject,omitempty"`
	Body        string  `json:"body"`
	FromName    *string `json:"fromName,omitempty"`
	FromUserID  *string `json:"fromUserID,omitempty"`
	FromAgentID *string `json:"fromAgentID,omitempty"`
	IsInternal  bool    `json:"isInternal"`
	CreatedAt   string  `json:"createdAt"`
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
	Description               *string                     `json:"description,omitempty"`
	Status                    string                      `json:"status"`
	Priority                  string                      `json:"priority"`
	Category                  *string                     `json:"category,omitempty"`
	Channel                   string                      `json:"channel"`
	TeamID                    *string                     `json:"teamID,omitempty"`
	QueueID                   *string                     `json:"queueID,omitempty"`
	Queue                     *namedResource              `json:"queue,omitempty"`
	ContactID                 *string                     `json:"contactID,omitempty"`
	ContactEmail              *string                     `json:"contactEmail,omitempty"`
	ContactName               *string                     `json:"contactName,omitempty"`
	Contact                   *contactOutput              `json:"contact,omitempty"`
	AssigneeID                *string                     `json:"assigneeID,omitempty"`
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

type createCaseInput struct {
	WorkspaceID  string
	Subject      string
	Description  string
	Priority     string
	Category     string
	QueueID      string
	ContactID    string
	ContactEmail string
	ContactName  string
}

func parseCSVValues(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func runCases(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printCasesUsage(stderr)
		return 2
	}

	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("mbr cases create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		subject := fs.String("subject", "", "Case subject")
		description := fs.String("description", "", "Case description")
		priority := fs.String("priority", "", "Case priority")
		category := fs.String("category", "", "Case category")
		queueID := fs.String("queue", "", "Queue ID")
		contactID := fs.String("contact-id", "", "Contact ID")
		contactEmail := fs.String("contact-email", "", "Contact email")
		contactName := fs.String("contact-name", "", "Contact name")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*subject) == "" {
			fmt.Fprintln(stderr, "--subject is required")
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
		caseObj, err := runCaseCreate(ctx, client, createCaseInput{
			WorkspaceID:  workspaceValue,
			Subject:      strings.TrimSpace(*subject),
			Description:  strings.TrimSpace(*description),
			Priority:     strings.TrimSpace(*priority),
			Category:     strings.TrimSpace(*category),
			QueueID:      strings.TrimSpace(*queueID),
			ContactID:    strings.TrimSpace(*contactID),
			ContactEmail: strings.TrimSpace(*contactEmail),
			ContactName:  strings.TrimSpace(*contactName),
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", caseObj.CaseID, caseObj.Status, caseObj.Priority, caseObj.Subject)
		return 0
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
		if caseObj.Description != nil {
			fmt.Fprintf(stdout, "description:\t%s\n", *caseObj.Description)
		}
		fmt.Fprintf(stdout, "status:\t%s\n", caseObj.Status)
		fmt.Fprintf(stdout, "priority:\t%s\n", caseObj.Priority)
		fmt.Fprintf(stdout, "channel:\t%s\n", caseObj.Channel)
		if caseObj.Category != nil {
			fmt.Fprintf(stdout, "category:\t%s\n", *caseObj.Category)
		}
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
		} else if caseObj.ContactEmail != nil {
			fmt.Fprintf(stdout, "contact:\t%s <%s>\n", coalesce(caseObj.ContactName, "unknown"), *caseObj.ContactEmail)
		}
		if caseObj.Assignee != nil {
			fmt.Fprintf(stdout, "assignee:\t%s <%s>\n", caseObj.Assignee.Name, caseObj.Assignee.Email)
		} else if caseObj.AssigneeID != nil {
			fmt.Fprintf(stdout, "assignee:\t%s\n", *caseObj.AssigneeID)
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
	case "assign":
		fs := flag.NewFlagSet("mbr cases assign", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for human-readable case IDs")
		assigneeID := fs.String("assignee", "", "Assignee user ID")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--assignee":  true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "case identifier is required")
			return 2
		}
		if strings.TrimSpace(*assigneeID) == "" {
			fmt.Fprintln(stderr, "--assignee is required")
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
		caseObj, err := runCaseAssign(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, optionalString(strings.TrimSpace(*assigneeID)))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", caseObj.CaseID, coalesce(caseObj.AssigneeID, ""), caseObj.Subject)
		return 0
	case "unassign":
		fs := flag.NewFlagSet("mbr cases unassign", flag.ContinueOnError)
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
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
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
		caseObj, err := runCaseAssign(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, nil)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\n", caseObj.CaseID, caseObj.Subject)
		return 0
	case "set-priority":
		fs := flag.NewFlagSet("mbr cases set-priority", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for human-readable case IDs")
		priority := fs.String("priority", "", "New case priority")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--priority":  true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "case identifier is required")
			return 2
		}
		if strings.TrimSpace(*priority) == "" {
			fmt.Fprintln(stderr, "--priority is required")
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
		caseObj, err := runCaseSetPriority(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, strings.TrimSpace(*priority))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", caseObj.CaseID, caseObj.Priority, caseObj.Subject)
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
	case "add-note":
		fs := flag.NewFlagSet("mbr cases add-note", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for human-readable case IDs")
		body := fs.String("body", "", "Internal note body")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--body":      true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "case identifier is required")
			return 2
		}
		if strings.TrimSpace(*body) == "" {
			fmt.Fprintln(stderr, "--body is required")
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
		comm, err := runCaseAddNote(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, strings.TrimSpace(*body))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, comm, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\n", comm.ID, comm.CreatedAt)
		return 0
	case "reply":
		fs := flag.NewFlagSet("mbr cases reply", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID for human-readable case IDs")
		subject := fs.String("subject", "", "Reply subject")
		body := fs.String("body", "", "Reply body")
		to := fs.String("to", "", "Comma-separated recipient emails")
		cc := fs.String("cc", "", "Comma-separated CC emails")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":       true,
			"--api-url":   true,
			"--token":     true,
			"--workspace": true,
			"--subject":   true,
			"--body":      true,
			"--to":        true,
			"--cc":        true,
			"--json":      false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "case identifier is required")
			return 2
		}
		if strings.TrimSpace(*body) == "" {
			fmt.Fprintln(stderr, "--body is required")
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
		comm, err := runCaseReply(ctx, client, strings.TrimSpace(positionals[0]), workspaceValue, strings.TrimSpace(*subject), strings.TrimSpace(*body), parseCSVValues(*to), parseCSVValues(*cc))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, comm, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\n", comm.ID, comm.CreatedAt)
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

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copyValue := strings.TrimSpace(value)
	return &copyValue
}

func runCaseCreate(ctx context.Context, client *cliapi.Client, input createCaseInput) (caseOutput, error) {
	mutationInput := map[string]any{
		"workspaceID": input.WorkspaceID,
		"subject":     input.Subject,
	}
	if input.Description != "" {
		mutationInput["description"] = input.Description
	}
	if input.Priority != "" {
		mutationInput["priority"] = strings.ToLower(input.Priority)
	}
	if input.Category != "" {
		mutationInput["category"] = input.Category
	}
	if input.QueueID != "" {
		mutationInput["queueID"] = input.QueueID
	}
	if input.ContactID != "" {
		mutationInput["contactID"] = input.ContactID
	}
	if input.ContactEmail != "" {
		mutationInput["contactEmail"] = input.ContactEmail
	}
	if input.ContactName != "" {
		mutationInput["contactName"] = input.ContactName
	}

	var payload struct {
		CreateCase *caseOutput `json:"createCase"`
	}
	err := client.Query(ctx, `
		mutation CLICreateCase($input: CreateCaseInput!) {
		  createCase(input: $input) {
		    `+caseSelection+`
		  }
		}
	`, map[string]any{
		"input": mutationInput,
	}, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.CreateCase == nil {
		return caseOutput{}, fmt.Errorf("case creation returned no case")
	}
	return *payload.CreateCase, nil
}

func runCaseAssign(ctx context.Context, client *cliapi.Client, identifier, workspaceID string, assigneeID *string) (caseOutput, error) {
	caseObj, err := runCaseShow(ctx, client, identifier, workspaceID)
	if err != nil {
		return caseOutput{}, err
	}

	var payload struct {
		AssignCase *caseOutput `json:"assignCase"`
	}
	variables := map[string]any{
		"id": caseObj.ID,
	}
	if assigneeID != nil {
		variables["assigneeID"] = *assigneeID
	} else {
		variables["assigneeID"] = nil
	}
	err = client.Query(ctx, `
		mutation CLIAssignCase($id: ID!, $assigneeID: ID) {
		  assignCase(id: $id, assigneeID: $assigneeID) {
		    `+caseSelection+`
		  }
		}
	`, variables, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.AssignCase == nil {
		return caseOutput{}, fmt.Errorf("case assignment returned no case")
	}
	return *payload.AssignCase, nil
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
		"status": strings.ToLower(status),
	}, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.UpdateCaseStatus == nil {
		return caseOutput{}, fmt.Errorf("case status update returned no case")
	}
	return *payload.UpdateCaseStatus, nil
}

func runCaseSetPriority(ctx context.Context, client *cliapi.Client, identifier, workspaceID, priority string) (caseOutput, error) {
	caseObj, err := runCaseShow(ctx, client, identifier, workspaceID)
	if err != nil {
		return caseOutput{}, err
	}
	var payload struct {
		SetCasePriority *caseOutput `json:"setCasePriority"`
	}
	err = client.Query(ctx, `
		mutation CLISetCasePriority($id: ID!, $priority: CasePriority!) {
		  setCasePriority(id: $id, priority: $priority) {
		    `+caseSelection+`
		  }
		}
	`, map[string]any{
		"id":       caseObj.ID,
		"priority": strings.ToLower(priority),
	}, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.SetCasePriority == nil {
		return caseOutput{}, fmt.Errorf("case priority update returned no case")
	}
	return *payload.SetCasePriority, nil
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

func runCaseAddNote(ctx context.Context, client *cliapi.Client, identifier, workspaceID, body string) (caseCommunicationOutput, error) {
	caseObj, err := runCaseShow(ctx, client, identifier, workspaceID)
	if err != nil {
		return caseCommunicationOutput{}, err
	}
	var payload struct {
		AddCaseNote *caseCommunicationOutput `json:"addCaseNote"`
	}
	err = client.Query(ctx, `
		mutation CLIAddCaseNote($id: ID!, $body: String!) {
		  addCaseNote(id: $id, body: $body) {
		    `+caseCommunicationSelection+`
		  }
		}
	`, map[string]any{
		"id":   caseObj.ID,
		"body": body,
	}, &payload)
	if err != nil {
		return caseCommunicationOutput{}, err
	}
	if payload.AddCaseNote == nil {
		return caseCommunicationOutput{}, fmt.Errorf("case note mutation returned no communication")
	}
	return *payload.AddCaseNote, nil
}

func runCaseReply(ctx context.Context, client *cliapi.Client, identifier, workspaceID, subject, body string, toEmails, ccEmails []string) (caseCommunicationOutput, error) {
	caseObj, err := runCaseShow(ctx, client, identifier, workspaceID)
	if err != nil {
		return caseCommunicationOutput{}, err
	}
	input := map[string]any{
		"body": body,
	}
	if subject != "" {
		input["subject"] = subject
	}
	if len(toEmails) > 0 {
		input["toEmails"] = toEmails
	}
	if len(ccEmails) > 0 {
		input["ccEmails"] = ccEmails
	}

	var payload struct {
		ReplyToCase *caseCommunicationOutput `json:"replyToCase"`
	}
	err = client.Query(ctx, `
		mutation CLIReplyToCase($id: ID!, $input: ReplyToCaseInput!) {
		  replyToCase(id: $id, input: $input) {
		    `+caseCommunicationSelection+`
		  }
		}
	`, map[string]any{
		"id":    caseObj.ID,
		"input": input,
	}, &payload)
	if err != nil {
		return caseCommunicationOutput{}, err
	}
	if payload.ReplyToCase == nil {
		return caseCommunicationOutput{}, fmt.Errorf("case reply returned no communication")
	}
	return *payload.ReplyToCase, nil
}
