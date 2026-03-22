package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

type queueItemCaseOutput struct {
	ID         string  `json:"id"`
	HumanID    string  `json:"humanID"`
	Subject    *string `json:"subject,omitempty"`
	Status     string  `json:"status"`
	Priority   string  `json:"priority"`
	TeamID     *string `json:"teamID,omitempty"`
	AssigneeID *string `json:"assigneeID,omitempty"`
	UpdatedAt  string  `json:"updatedAt"`
}

type queueItemConversationOutput struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	Channel        string  `json:"channel"`
	Title          *string `json:"title,omitempty"`
	LinkedCaseID   *string `json:"linkedCaseID,omitempty"`
	LastActivityAt string  `json:"lastActivityAt"`
}

type queueItemOutput struct {
	ID                    string                       `json:"id"`
	WorkspaceID           string                       `json:"workspaceID"`
	QueueID               string                       `json:"queueID"`
	ItemKind              string                       `json:"itemKind"`
	CaseID                *string                      `json:"caseID,omitempty"`
	ConversationSessionID *string                      `json:"conversationSessionID,omitempty"`
	Case                  *queueItemCaseOutput         `json:"case,omitempty"`
	ConversationSession   *queueItemConversationOutput `json:"conversationSession,omitempty"`
	CreatedAt             string                       `json:"createdAt"`
	UpdatedAt             string                       `json:"updatedAt"`
}

func runQueues(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printQueuesUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr queues list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
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

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		var payload struct {
			Queues []queueOutput `json:"queues"`
		}
		err = client.Query(ctx, `
			query CLIQueues($workspaceID: ID!) {
			  queues(workspaceID: $workspaceID) {
			    id
			    workspaceID
			    slug
			    name
			    description
			  }
			}
		`, map[string]any{"workspaceID": workspaceValue}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.Queues, stderr)
		}
		if len(payload.Queues) == 0 {
			fmt.Fprintln(stdout, "no queues found")
			return 0
		}
		for _, queue := range payload.Queues {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", queue.ID, queue.Slug, queue.Name)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr queues show", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "queue identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		var payload struct {
			Queue *queueOutput `json:"queue"`
		}
		err = client.Query(ctx, `
			query CLIQueue($id: ID!) {
			  queue(id: $id) {
			    id
			    workspaceID
			    slug
			    name
			    description
			  }
			}
		`, map[string]any{"id": strings.TrimSpace(positionals[0])}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if payload.Queue == nil {
			fmt.Fprintln(stderr, "queue not found")
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.Queue, stderr)
		}
		fmt.Fprintf(stdout, "id:\t%s\n", payload.Queue.ID)
		fmt.Fprintf(stdout, "workspace:\t%s\n", payload.Queue.WorkspaceID)
		fmt.Fprintf(stdout, "slug:\t%s\n", payload.Queue.Slug)
		fmt.Fprintf(stdout, "name:\t%s\n", payload.Queue.Name)
		if payload.Queue.Description != nil {
			fmt.Fprintf(stdout, "description:\t%s\n", *payload.Queue.Description)
		}
		return 0
	case "create":
		fs := flag.NewFlagSet("mbr queues create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		name := fs.String("name", "", "Queue name")
		slug := fs.String("slug", "", "Queue slug")
		description := fs.String("description", "", "Queue description")
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

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)
		queue, err := runQueueCreate(ctx, client, workspaceValue, *name, *slug, *description)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, queue, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", queue.ID, queue.Slug, queue.Name)
		return 0
	case "items":
		fs := flag.NewFlagSet("mbr queues items", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "queue identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		items, err := runQueueItems(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, items, stderr)
		}
		if len(items) == 0 {
			fmt.Fprintln(stdout, "no queue items found")
			return 0
		}
		for _, item := range items {
			switch item.ItemKind {
			case "case":
				status := ""
				subject := ""
				if item.Case != nil {
					status = item.Case.Status
					subject = coalesce(item.Case.Subject, item.Case.HumanID)
				}
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", item.ID, item.ItemKind, status, subject)
			case "conversation_session":
				status := ""
				title := ""
				if item.ConversationSession != nil {
					status = item.ConversationSession.Status
					title = coalesce(item.ConversationSession.Title, "untitled")
				}
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", item.ID, item.ItemKind, status, title)
			default:
				fmt.Fprintf(stdout, "%s\t%s\n", item.ID, item.ItemKind)
			}
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown queues command %q\n\n", args[0])
		printQueuesUsage(stderr)
		return 2
	}
}

func runQueueItems(ctx context.Context, client *cliapi.Client, queueID string) ([]queueItemOutput, error) {
	var payload struct {
		Queue *struct {
			Items []queueItemOutput `json:"items"`
		} `json:"queue"`
	}
	err := client.Query(ctx, `
		query CLIQueueItems($id: ID!) {
		  queue(id: $id) {
		    items {
		      id
		      workspaceID
		      queueID
		      itemKind
		      caseID
		      conversationSessionID
		      createdAt
		      updatedAt
		      case {
		        id
		        humanID
		        subject
		        status
		        priority
		        teamID
		        assigneeID
		        updatedAt
		      }
		      conversationSession {
		        id
		        status
		        channel
		        title
		        linkedCaseID
		        lastActivityAt
		      }
		    }
		  }
		}
	`, map[string]any{"id": queueID}, &payload)
	if err != nil {
		return nil, err
	}
	if payload.Queue == nil {
		return nil, fmt.Errorf("queue not found")
	}
	return payload.Queue.Items, nil
}
