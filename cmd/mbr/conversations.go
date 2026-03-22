package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runConversations(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printConversationsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr conversations list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		status := fs.String("status", "", "Conversation status filter")
		channel := fs.String("channel", "", "Conversation channel filter")
		catalogNodeID := fs.String("catalog-node", "", "Primary catalog node ID filter")
		contactID := fs.String("contact", "", "Primary contact ID filter")
		caseID := fs.String("case", "", "Linked case ID filter")
		limit := fs.Int("limit", 20, "Maximum number of conversations to return")
		offset := fs.Int("offset", 0, "Number of conversations to skip")
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
		if *offset < 0 {
			fmt.Fprintln(stderr, "--offset must be greater than or equal to 0")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		sessions, err := runConversationList(ctx, client, conversationListInput{
			WorkspaceID:          workspaceValue,
			Status:               strings.TrimSpace(*status),
			Channel:              strings.TrimSpace(*channel),
			PrimaryCatalogNodeID: strings.TrimSpace(*catalogNodeID),
			PrimaryContactID:     strings.TrimSpace(*contactID),
			LinkedCaseID:         strings.TrimSpace(*caseID),
			Limit:                *limit,
			Offset:               *offset,
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, sessions, stderr)
		}
		if len(sessions) == 0 {
			fmt.Fprintln(stdout, "no conversations found")
			return 0
		}
		for _, session := range sessions {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", session.ID, session.Status, session.Channel, coalesce(session.Title, "untitled"))
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr conversations show", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":     true,
			"--api-url": true,
			"--token":   true,
			"--json":    false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "conversation identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		session, err := runConversationShow(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, session, stderr)
		}

		fmt.Fprintf(stdout, "id:\t%s\n", session.ID)
		fmt.Fprintf(stdout, "workspace:\t%s\n", session.WorkspaceID)
		fmt.Fprintf(stdout, "status:\t%s\n", session.Status)
		fmt.Fprintf(stdout, "channel:\t%s\n", session.Channel)
		fmt.Fprintf(stdout, "title:\t%s\n", coalesce(session.Title, "untitled"))
		if session.PrimaryContactID != nil {
			fmt.Fprintf(stdout, "primaryContactID:\t%s\n", *session.PrimaryContactID)
		}
		if session.PrimaryCatalogNodeID != nil {
			fmt.Fprintf(stdout, "primaryCatalogNodeID:\t%s\n", *session.PrimaryCatalogNodeID)
		}
		if session.LinkedCaseID != nil {
			fmt.Fprintf(stdout, "linkedCaseID:\t%s\n", *session.LinkedCaseID)
		}
		fmt.Fprintf(stdout, "openedAt:\t%s\n", session.OpenedAt)
		fmt.Fprintf(stdout, "lastActivityAt:\t%s\n", session.LastActivityAt)
		fmt.Fprintf(stdout, "participants:\t%d\n", len(session.Participants))
		fmt.Fprintf(stdout, "messages:\t%d\n", len(session.Messages))
		if session.WorkingState != nil {
			fmt.Fprintf(stdout, "requiresOperatorReview:\t%t\n", session.WorkingState.RequiresOperatorReview)
		}
		for _, message := range session.Messages {
			fmt.Fprintf(stdout, "\n[%s] %s/%s %s\n", message.CreatedAt, message.Role, message.Visibility, message.Kind)
			if message.ContentText != nil && strings.TrimSpace(*message.ContentText) != "" {
				fmt.Fprintln(stdout, *message.ContentText)
			} else if len(message.Content) > 0 {
				data, err := json.MarshalIndent(message.Content, "", "  ")
				if err == nil {
					fmt.Fprintln(stdout, string(data))
				}
			}
		}
		return 0
	case "reply":
		fs := flag.NewFlagSet("mbr conversations reply", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		participantID := fs.String("participant", "", "Participant ID")
		role := fs.String("role", "assistant", "Message role")
		kind := fs.String("kind", "text", "Message kind")
		visibility := fs.String("visibility", "customer", "Message visibility")
		content := fs.String("content", "", "Inline message text")
		filePath := fs.String("file", "", "Path to a file containing message text")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":         true,
			"--api-url":     true,
			"--token":       true,
			"--participant": true,
			"--role":        true,
			"--kind":        true,
			"--visibility":  true,
			"--content":     true,
			"--file":        true,
			"--json":        false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "conversation identifier is required")
			return 2
		}
		contentValue, err := readOptionalTextInput(strings.TrimSpace(*filePath), *content, "content")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if contentValue == nil || strings.TrimSpace(*contentValue) == "" {
			fmt.Fprintln(stderr, "one of --file or --content is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		message, err := runConversationReply(ctx, client, conversationReplyInput{
			SessionID:     strings.TrimSpace(positionals[0]),
			ParticipantID: strings.TrimSpace(*participantID),
			Role:          strings.TrimSpace(*role),
			Kind:          strings.TrimSpace(*kind),
			Visibility:    strings.TrimSpace(*visibility),
			ContentText:   strings.TrimSpace(*contentValue),
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, message, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", message.ID, message.Role, message.Visibility, coalesce(message.ContentText, ""))
		return 0
	case "handoff":
		fs := flag.NewFlagSet("mbr conversations handoff", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		teamID := fs.String("team", "", "Target handling team ID")
		queueID := fs.String("queue", "", "Target queue ID")
		operatorUserID := fs.String("operator", "", "Target operator user ID")
		reason := fs.String("reason", "", "Handoff reason")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":      true,
			"--api-url":  true,
			"--token":    true,
			"--team":     true,
			"--queue":    true,
			"--operator": true,
			"--reason":   true,
			"--json":     false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "conversation identifier is required")
			return 2
		}
		if strings.TrimSpace(*queueID) == "" {
			fmt.Fprintln(stderr, "--queue is required")
			return 2
		}
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		session, err := runConversationHandoff(ctx, client, strings.TrimSpace(positionals[0]), strings.TrimSpace(*teamID), strings.TrimSpace(*queueID), strings.TrimSpace(*operatorUserID), strings.TrimSpace(*reason))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, session, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", session.ID, session.Status, coalesce(session.HandlingTeamID, ""), coalesce(session.Title, "untitled"))
		return 0
	case "escalate":
		fs := flag.NewFlagSet("mbr conversations escalate", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		teamID := fs.String("team", "", "Target handling team ID")
		queueID := fs.String("queue", "", "Target queue ID")
		operatorUserID := fs.String("operator", "", "Target operator user ID")
		subject := fs.String("subject", "", "Case subject override")
		description := fs.String("description", "", "Case description override")
		priority := fs.String("priority", "", "Case priority")
		category := fs.String("category", "", "Case category")
		reason := fs.String("reason", "", "Escalation reason")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":         true,
			"--api-url":     true,
			"--token":       true,
			"--team":        true,
			"--queue":       true,
			"--operator":    true,
			"--subject":     true,
			"--description": true,
			"--priority":    true,
			"--category":    true,
			"--reason":      true,
			"--json":        false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "conversation identifier is required")
			return 2
		}
		if strings.TrimSpace(*queueID) == "" {
			fmt.Fprintln(stderr, "--queue is required")
			return 2
		}
		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		caseObj, err := runConversationEscalate(ctx, client, conversationEscalateInput{
			SessionID:      strings.TrimSpace(positionals[0]),
			TeamID:         strings.TrimSpace(*teamID),
			QueueID:        strings.TrimSpace(*queueID),
			OperatorUserID: strings.TrimSpace(*operatorUserID),
			Subject:        strings.TrimSpace(*subject),
			Description:    strings.TrimSpace(*description),
			Priority:       strings.TrimSpace(*priority),
			Category:       strings.TrimSpace(*category),
			Reason:         strings.TrimSpace(*reason),
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, caseObj, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", caseObj.ID, caseObj.Status, caseObj.Subject)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown conversations command %q\n\n", args[0])
		printConversationsUsage(stderr)
		return 2
	}
}

type conversationEscalateInput struct {
	SessionID      string
	TeamID         string
	QueueID        string
	OperatorUserID string
	Subject        string
	Description    string
	Priority       string
	Category       string
	Reason         string
}

func runConversationHandoff(ctx context.Context, client *cliapi.Client, sessionID, teamID, queueID, operatorUserID, reason string) (conversationSessionOutput, error) {
	var payload struct {
		HandoffConversation *conversationSessionOutput `json:"handoffConversation"`
	}
	input := map[string]any{
		"queueID": queueID,
	}
	if teamID != "" {
		input["teamID"] = teamID
	}
	if operatorUserID != "" {
		input["operatorUserID"] = operatorUserID
	}
	if reason != "" {
		input["reason"] = reason
	}
	err := client.Query(ctx, `
		mutation CLIHandoffConversation($sessionID: ID!, $input: ConversationHandoffInput!) {
		  handoffConversation(sessionID: $sessionID, input: $input) {
		    `+conversationDetailSelection+`
		  }
		}
	`, map[string]any{
		"sessionID": sessionID,
		"input":     input,
	}, &payload)
	if err != nil {
		return conversationSessionOutput{}, err
	}
	if payload.HandoffConversation == nil {
		return conversationSessionOutput{}, fmt.Errorf("conversation handoff returned no payload")
	}
	return *payload.HandoffConversation, nil
}

func runConversationEscalate(ctx context.Context, client *cliapi.Client, input conversationEscalateInput) (caseOutput, error) {
	var payload struct {
		EscalateConversation *caseOutput `json:"escalateConversation"`
	}
	mutationInput := map[string]any{
		"queueID": input.QueueID,
	}
	if input.TeamID != "" {
		mutationInput["teamID"] = input.TeamID
	}
	if input.OperatorUserID != "" {
		mutationInput["operatorUserID"] = input.OperatorUserID
	}
	if input.Subject != "" {
		mutationInput["subject"] = input.Subject
	}
	if input.Description != "" {
		mutationInput["description"] = input.Description
	}
	if input.Priority != "" {
		mutationInput["priority"] = strings.ToUpper(input.Priority)
	}
	if input.Category != "" {
		mutationInput["category"] = input.Category
	}
	if input.Reason != "" {
		mutationInput["reason"] = input.Reason
	}
	err := client.Query(ctx, `
		mutation CLIEscalateConversation($sessionID: ID!, $input: EscalateConversationInput!) {
		  escalateConversation(sessionID: $sessionID, input: $input) {
		    `+caseSelection+`
		  }
		}
	`, map[string]any{
		"sessionID": input.SessionID,
		"input":     mutationInput,
	}, &payload)
	if err != nil {
		return caseOutput{}, err
	}
	if payload.EscalateConversation == nil {
		return caseOutput{}, fmt.Errorf("conversation escalation returned no case")
	}
	return *payload.EscalateConversation, nil
}
