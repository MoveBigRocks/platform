package main

import (
	"context"
	"fmt"
	"io"
)

type rootCommandHandler func(context.Context, []string, io.Writer, io.Writer) int

var rootCommandHandlers = map[string]rootCommandHandler{
	"auth":              runAuth,
	"sandboxes":         runSandboxes,
	"context":           runContext,
	"spec":              runSpec,
	"workspaces":        runWorkspaces,
	"teams":             runTeams,
	"agents":            runAgents,
	"catalog":           runCatalog,
	"knowledge":         runKnowledge,
	"concepts":          runConcepts,
	"artifacts":         runArtifacts,
	"queues":            runQueues,
	"conversations":     runConversations,
	"forms":             runForms,
	"automation":        runAutomation,
	"cases":             runCases,
	"contacts":          runContacts,
	"attachments":       runAttachments,
	"health":            runHealth,
	"extensions":        runExtensions,
	"extensions-skills": runExtensionSkill,
	"extensions-events": runExtensionEvents,
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootUsage(stderr)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		printRootUsage(stdout)
		return 0
	}

	handler, ok := rootCommandHandlers[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printRootUsage(stderr)
		return 2
	}
	return handler(ctx, args[1:], stdout, stderr)
}
