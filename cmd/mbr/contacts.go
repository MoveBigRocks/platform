package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runContacts(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printContactsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr contacts list", flag.ContinueOnError)
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
			Contacts []contactOutput `json:"contacts"`
		}
		err = client.Query(ctx, `
			query CLIContacts($workspaceID: ID!) {
			  contacts(workspaceID: $workspaceID) {
			    id
			    email
			    name
			  }
			}
		`, map[string]any{"workspaceID": workspaceValue}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		if *jsonOutput {
			return writeJSON(stdout, payload.Contacts, stderr)
		}
		if len(payload.Contacts) == 0 {
			fmt.Fprintln(stdout, "no contacts found")
			return 0
		}
		for _, contact := range payload.Contacts {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", contact.Email, coalesce(contact.Name, "unknown"), contact.ID)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown contacts command %q\n\n", args[0])
		printContactsUsage(stderr)
		return 2
	}
}
