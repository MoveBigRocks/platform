package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

func runTeams(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printTeamsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr teams list", flag.ContinueOnError)
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
			Teams []teamOutput `json:"teams"`
		}
		err = client.Query(ctx, `
			query CLITeams($workspaceID: ID!) {
			  teams(workspaceID: $workspaceID) {
			    id
			    workspaceID
			    name
			    description
			    emailAddress
			    responseTimeHours
			    resolutionTimeHours
			    autoAssign
			    autoAssignKeywords
			    isActive
			    createdAt
			    updatedAt
			  }
			}
		`, map[string]any{"workspaceID": workspaceValue}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.Teams, stderr)
		}
		if len(payload.Teams) == 0 {
			fmt.Fprintln(stdout, "no teams found")
			return 0
		}
		for _, team := range payload.Teams {
			fmt.Fprintf(stdout, "%s\t%s\t%t\n", team.ID, team.Name, team.IsActive)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr teams show", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "team identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		var payload struct {
			Team *teamOutput `json:"team"`
		}
		err = client.Query(ctx, `
			query CLITeam($id: ID!) {
			  team(id: $id) {
			    id
			    workspaceID
			    name
			    description
			    emailAddress
			    responseTimeHours
			    resolutionTimeHours
			    autoAssign
			    autoAssignKeywords
			    isActive
			    createdAt
			    updatedAt
			  }
			}
		`, map[string]any{"id": strings.TrimSpace(positionals[0])}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if payload.Team == nil {
			fmt.Fprintln(stderr, "team not found")
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.Team, stderr)
		}
		fmt.Fprintf(stdout, "id:\t%s\n", payload.Team.ID)
		fmt.Fprintf(stdout, "workspace:\t%s\n", payload.Team.WorkspaceID)
		fmt.Fprintf(stdout, "name:\t%s\n", payload.Team.Name)
		if payload.Team.Description != nil {
			fmt.Fprintf(stdout, "description:\t%s\n", *payload.Team.Description)
		}
		if payload.Team.EmailAddress != nil {
			fmt.Fprintf(stdout, "email:\t%s\n", *payload.Team.EmailAddress)
		}
		fmt.Fprintf(stdout, "responseTimeHours:\t%d\n", payload.Team.ResponseTimeHours)
		fmt.Fprintf(stdout, "resolutionTimeHours:\t%d\n", payload.Team.ResolutionTimeHours)
		fmt.Fprintf(stdout, "autoAssign:\t%t\n", payload.Team.AutoAssign)
		if len(payload.Team.AutoAssignKeywords) > 0 {
			fmt.Fprintf(stdout, "autoAssignKeywords:\t%s\n", strings.Join(payload.Team.AutoAssignKeywords, ","))
		}
		fmt.Fprintf(stdout, "active:\t%t\n", payload.Team.IsActive)
		fmt.Fprintf(stdout, "createdAt:\t%s\n", payload.Team.CreatedAt)
		fmt.Fprintf(stdout, "updatedAt:\t%s\n", payload.Team.UpdatedAt)
		return 0
	case "create":
		fs := flag.NewFlagSet("mbr teams create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		name := fs.String("name", "", "Team name")
		description := fs.String("description", "", "Team description")
		emailAddress := fs.String("email", "", "Team email address")
		responseTimeHours := fs.Int("response-hours", 0, "Target response time in hours")
		resolutionTimeHours := fs.Int("resolution-hours", 0, "Target resolution time in hours")
		autoAssign := fs.Bool("auto-assign", false, "Enable auto-assignment")
		autoAssignKeywords := fs.String("auto-assign-keywords", "", "Comma-separated auto-assignment keywords")
		inactive := fs.Bool("inactive", false, "Create the team as inactive")
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
		if err := requireSessionAuth(cfg, "teams"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		input := map[string]any{
			"workspaceID":         workspaceValue,
			"name":                *name,
			"description":         strings.TrimSpace(*description),
			"emailAddress":        strings.TrimSpace(*emailAddress),
			"responseTimeHours":   *responseTimeHours,
			"resolutionTimeHours": *resolutionTimeHours,
			"autoAssign":          *autoAssign,
			"isActive":            !*inactive,
		}
		if keywords := commaSeparatedValues(*autoAssignKeywords); len(keywords) > 0 {
			input["autoAssignKeywords"] = keywords
		}

		var payload struct {
			CreateTeam teamOutput `json:"createTeam"`
		}
		err = client.Query(ctx, `
			mutation CLICreateTeam($input: CreateTeamInput!) {
			  createTeam(input: $input) {
			    id
			    workspaceID
			    name
			    description
			    emailAddress
			    responseTimeHours
			    resolutionTimeHours
			    autoAssign
			    autoAssignKeywords
			    isActive
			    createdAt
			    updatedAt
			  }
			}
		`, map[string]any{"input": input}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.CreateTeam, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%t\n", payload.CreateTeam.ID, payload.CreateTeam.Name, payload.CreateTeam.IsActive)
		return 0
	case "members":
		if len(args) < 2 {
			printTeamsUsage(stderr)
			return 2
		}
		switch args[1] {
		case "list":
			fs := flag.NewFlagSet("mbr teams members list", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			teamID := fs.String("team", "", "Team ID")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			if err := fs.Parse(args[2:]); err != nil {
				return 2
			}
			stored, err := cliapi.LoadStoredConfig()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			teamValue, err := requireTeamID(*teamID, stored)
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
				Team struct {
					ID      string             `json:"id"`
					Members []teamMemberOutput `json:"members"`
				} `json:"team"`
			}
			err = client.Query(ctx, `
				query CLITeamMembers($id: ID!) {
				  team(id: $id) {
				    id
				    members {
				      id
				      teamID
				      userID
				      workspaceID
				      role
				      isActive
				      joinedAt
				      createdAt
				      updatedAt
				    }
				  }
				}
			`, map[string]any{"id": teamValue}, &payload)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, payload.Team.Members, stderr)
			}
			if len(payload.Team.Members) == 0 {
				fmt.Fprintln(stdout, "no team members found")
				return 0
			}
			for _, member := range payload.Team.Members {
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%t\n", member.ID, member.UserID, member.Role, member.IsActive)
			}
			return 0
		case "add":
			fs := flag.NewFlagSet("mbr teams members add", flag.ContinueOnError)
			fs.SetOutput(stderr)
			instanceURL := registerInstanceURLFlag(fs)
			token := fs.String("token", "", "Bearer token")
			teamID := fs.String("team", "", "Team ID")
			userID := fs.String("user", "", "User ID")
			role := fs.String("role", string(platformdomain.TeamMemberRoleMember), "Team role")
			jsonOutput := fs.Bool("json", false, "Emit JSON output")
			if err := fs.Parse(args[2:]); err != nil {
				return 2
			}
			stored, err := cliapi.LoadStoredConfig()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			teamValue, err := requireTeamID(*teamID, stored)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			if strings.TrimSpace(*userID) == "" {
				fmt.Fprintln(stderr, "--user is required")
				return 2
			}

			cfg, err := loadCLIConfig(*instanceURL, *token)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			if err := requireSessionAuth(cfg, "teams"); err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			client := newCLIClient(cfg)

			var payload struct {
				AddTeamMember teamMemberOutput `json:"addTeamMember"`
			}
			err = client.Query(ctx, `
				mutation CLIAddTeamMember($input: AddTeamMemberInput!) {
				  addTeamMember(input: $input) {
				    id
				    teamID
				    userID
				    workspaceID
				    role
				    isActive
				    joinedAt
				    createdAt
				    updatedAt
				  }
				}
			`, map[string]any{
				"input": map[string]any{
					"teamID": teamValue,
					"userID": *userID,
					"role":   strings.TrimSpace(*role),
				},
			}, &payload)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if *jsonOutput {
				return writeJSON(stdout, payload.AddTeamMember, stderr)
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%t\n", payload.AddTeamMember.ID, payload.AddTeamMember.UserID, payload.AddTeamMember.Role, payload.AddTeamMember.IsActive)
			return 0
		default:
			fmt.Fprintf(stderr, "unknown teams members command %q\n\n", args[1])
			printTeamsUsage(stderr)
			return 2
		}
	default:
		fmt.Fprintf(stderr, "unknown teams command %q\n\n", args[0])
		printTeamsUsage(stderr)
		return 2
	}
}
