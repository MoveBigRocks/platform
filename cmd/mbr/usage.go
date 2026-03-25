package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/clispec"
)

func runSpec(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printSpecUsage(stderr)
		return 2
	}

	switch args[0] {
	case "export":
		fs := flag.NewFlagSet("mbr spec export", flag.ContinueOnError)
		fs.SetOutput(stderr)
		_ = fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "unexpected arguments")
			return 2
		}
		return writeJSON(stdout, clispec.CurrentSpec(), stderr)
	default:
		fmt.Fprintf(stderr, "unknown spec command %q\n\n", args[0])
		printSpecUsage(stderr)
		return 2
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	for _, command := range clispec.Commands() {
		fmt.Fprintf(w, "  %s\n", command.Usage)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Notes:")
	fmt.Fprintln(w, "  mbr auth login opens the browser when no token is supplied.")
	fmt.Fprintln(w, "  mbr context set stores the current workspace and team for later commands.")
	fmt.Fprintln(w, "  Session-backed auth is required for mbr workspaces create, mbr teams create/members add, mbr forms list/create, and mbr automation rules list/create.")
	fmt.Fprintln(w, "  mbr spec export emits the machine-readable CLI contract.")
}

func printAuthUsage(w io.Writer) {
	printUsageSection(w, "auth")
}

func printSpecUsage(w io.Writer) {
	printUsageSection(w, "spec")
}

func printContextUsage(w io.Writer) {
	printUsageSection(w, "context")
}

func printWorkspacesUsage(w io.Writer) {
	printUsageSection(w, "workspaces")
}

func printTeamsUsage(w io.Writer) {
	printUsageSection(w, "teams")
}

func printAgentsUsage(w io.Writer) {
	printUsageSection(w, "agents")
}

func printCatalogUsage(w io.Writer) {
	printUsageSection(w, "catalog")
}

func printFormsUsage(w io.Writer) {
	printUsageSection(w, "forms")
}

func printKnowledgeUsage(w io.Writer) {
	printUsageSection(w, "knowledge")
}

func printConceptsUsage(w io.Writer) {
	printUsageSection(w, "concepts")
}

func printArtifactsUsage(w io.Writer) {
	printUsageSection(w, "artifacts")
}

func printQueuesUsage(w io.Writer) {
	printUsageSection(w, "queues")
}

func printConversationsUsage(w io.Writer) {
	printUsageSection(w, "conversations")
}

func printAutomationUsage(w io.Writer) {
	printUsageSection(w, "automation")
}

func printAttachmentsUsage(w io.Writer) {
	printUsageSection(w, "attachments")
}

func printCasesUsage(w io.Writer) {
	printUsageSection(w, "cases")
}

func printContactsUsage(w io.Writer) {
	printUsageSection(w, "contacts")
}

func printHealthUsage(w io.Writer) {
	printUsageSection(w, "health")
}

func printExtensionsUsage(w io.Writer) {
	printUsageSection(w, "extensions")
}

func printExtensionSkillsUsage(w io.Writer) {
	printUsageSection(w, "extensions-skills")
}

func printExtensionEventsUsage(w io.Writer) {
	printUsageSection(w, "extensions-events")
}

func printUsageSection(w io.Writer, sectionKey string) {
	section, ok := clispec.Section(sectionKey)
	if !ok {
		fmt.Fprintln(w, "Usage:")
		return
	}

	fmt.Fprintln(w, "Usage:")
	for _, command := range clispec.CommandsForSection(sectionKey) {
		fmt.Fprintf(w, "  %s\n", command.Usage)
	}
	if len(section.Notes) == 0 {
		return
	}
	fmt.Fprintln(w)
	for _, note := range section.Notes {
		fmt.Fprintln(w, unwrapMarkdownCode(note))
	}
}

func unwrapMarkdownCode(input string) string {
	replacer := strings.NewReplacer("`", "")
	return replacer.Replace(input)
}
