package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runContext(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printContextUsage(stderr)
		return 2
	}

	switch args[0] {
	case "view":
		fs := flag.NewFlagSet("mbr context view", flag.ContinueOnError)
		fs.SetOutput(stderr)
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "unexpected arguments")
			return 2
		}

		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		configPath, err := cliapi.ConfigPath()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		instanceURLValue := strings.TrimSpace(stored.InstanceURL)
		if instanceURLValue == "" {
			instanceURLValue = strings.TrimSpace(stored.APIURL)
		}

		payload := map[string]any{
			"configPath":    configPath,
			"instanceURL":   instanceURLValue,
			"adminBaseURL":  stored.AdminBaseURL,
			"workspaceID":   stored.CurrentWorkspaceID,
			"teamID":        stored.CurrentTeamID,
			"authMode":      stored.AuthMode,
			"hasToken":      stored.Token != "",
			"hasSession":    stored.SessionToken != "",
			"credentialKey": stored.CredentialKey,
		}
		if *jsonOutput {
			return writeJSON(stdout, payload, stderr)
		}

		fmt.Fprintf(stdout, "configPath:\t%s\n", configPath)
		fmt.Fprintf(stdout, "instanceURL:\t%s\n", instanceURLValue)
		fmt.Fprintf(stdout, "adminBaseURL:\t%s\n", stored.AdminBaseURL)
		fmt.Fprintf(stdout, "workspace:\t%s\n", stored.CurrentWorkspaceID)
		fmt.Fprintf(stdout, "team:\t%s\n", stored.CurrentTeamID)
		fmt.Fprintf(stdout, "authMode:\t%s\n", stored.AuthMode)
		fmt.Fprintf(stdout, "hasToken:\t%t\n", stored.Token != "")
		fmt.Fprintf(stdout, "hasSession:\t%t\n", stored.SessionToken != "")
		return 0
	case "set":
		fs := flag.NewFlagSet("mbr context set", flag.ContinueOnError)
		fs.SetOutput(stderr)
		workspaceID := fs.String("workspace", "", "Current workspace ID")
		teamID := fs.String("team", "", "Current team ID")
		clearTeam := fs.Bool("clear-team", false, "Clear the stored team")
		clear := fs.Bool("clear", false, "Clear the stored workspace and team")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() != 0 {
			fmt.Fprintln(stderr, "unexpected arguments")
			return 2
		}
		if !*clear && strings.TrimSpace(*workspaceID) == "" && strings.TrimSpace(*teamID) == "" && !*clearTeam {
			fmt.Fprintln(stderr, "at least one of --workspace, --team, --clear-team, or --clear is required")
			return 2
		}
		if *clear && (strings.TrimSpace(*workspaceID) != "" || strings.TrimSpace(*teamID) != "" || *clearTeam) {
			fmt.Fprintln(stderr, "--clear cannot be combined with --workspace, --team, or --clear-team")
			return 2
		}
		if *clearTeam && strings.TrimSpace(*teamID) != "" {
			fmt.Fprintln(stderr, "--clear-team cannot be combined with --team")
			return 2
		}

		stored, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		workspaceValue := strings.TrimSpace(*workspaceID)
		if *clear {
			workspaceValue = ""
		}
		teamValue := strings.TrimSpace(*teamID)
		if teamValue != "" && workspaceValue == "" {
			workspaceValue = stored.CurrentWorkspaceID
		}
		if teamValue != "" && strings.TrimSpace(workspaceValue) == "" {
			fmt.Fprintln(stderr, "--workspace is required when setting --team")
			return 2
		}

		var workspacePtr *string
		if *clear || strings.TrimSpace(*workspaceID) != "" {
			workspacePtr = &workspaceValue
		}
		var teamPtr *string
		if teamValue != "" {
			teamPtr = &teamValue
		}

		configPath, err := cliapi.SaveStoredContext(workspacePtr, teamPtr, *clear || *clearTeam)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		updated, err := cliapi.LoadStoredConfig()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		payload := map[string]any{
			"configPath":  configPath,
			"workspaceID": updated.CurrentWorkspaceID,
			"teamID":      updated.CurrentTeamID,
		}
		if *jsonOutput {
			return writeJSON(stdout, payload, stderr)
		}
		fmt.Fprintf(stdout, "configPath:\t%s\n", configPath)
		fmt.Fprintf(stdout, "workspace:\t%s\n", updated.CurrentWorkspaceID)
		fmt.Fprintf(stdout, "team:\t%s\n", updated.CurrentTeamID)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown context command %q\n\n", args[0])
		printContextUsage(stderr)
		return 2
	}
}
