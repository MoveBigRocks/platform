package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

type agentOutput struct {
	ID           string                     `json:"id"`
	WorkspaceID  string                     `json:"workspaceID"`
	Name         string                     `json:"name"`
	Description  *string                    `json:"description,omitempty"`
	OwnerID      string                     `json:"ownerID"`
	Status       string                     `json:"status"`
	StatusReason *string                    `json:"statusReason,omitempty"`
	CreatedAt    string                     `json:"createdAt"`
	UpdatedAt    string                     `json:"updatedAt"`
	CreatedByID  string                     `json:"createdByID"`
	Tokens       []agentTokenOutput         `json:"tokens,omitempty"`
	Membership   *workspaceMembershipOutput `json:"membership,omitempty"`
}

type agentTokenOutput struct {
	ID          string  `json:"id"`
	AgentID     string  `json:"agentID"`
	TokenPrefix string  `json:"tokenPrefix"`
	Name        string  `json:"name"`
	ExpiresAt   *string `json:"expiresAt,omitempty"`
	RevokedAt   *string `json:"revokedAt,omitempty"`
	LastUsedAt  *string `json:"lastUsedAt,omitempty"`
	LastUsedIP  *string `json:"lastUsedIP,omitempty"`
	UseCount    int     `json:"useCount"`
	CreatedAt   string  `json:"createdAt"`
	CreatedByID string  `json:"createdByID"`
}

type agentTokenResultOutput struct {
	Token          agentTokenOutput `json:"token"`
	PlaintextToken string           `json:"plaintextToken"`
}

type workspaceMembershipOutput struct {
	ID            string                      `json:"id"`
	WorkspaceID   string                      `json:"workspaceID"`
	PrincipalID   string                      `json:"principalID"`
	PrincipalType string                      `json:"principalType"`
	Role          string                      `json:"role"`
	Permissions   []string                    `json:"permissions"`
	Constraints   membershipConstraintsOutput `json:"constraints"`
	GrantedAt     string                      `json:"grantedAt"`
	ExpiresAt     *string                     `json:"expiresAt,omitempty"`
	RevokedAt     *string                     `json:"revokedAt,omitempty"`
}

type membershipConstraintsOutput struct {
	RateLimitPerMinute      *int     `json:"rateLimitPerMinute,omitempty"`
	RateLimitPerHour        *int     `json:"rateLimitPerHour,omitempty"`
	AllowedIPs              []string `json:"allowedIPs,omitempty"`
	AllowedProjectIDs       []string `json:"allowedProjectIDs,omitempty"`
	AllowedTeamIDs          []string `json:"allowedTeamIDs,omitempty"`
	AllowDelegatedRouting   bool     `json:"allowDelegatedRouting"`
	DelegatedRoutingTeamIDs []string `json:"delegatedRoutingTeamIDs,omitempty"`
	ActiveHoursStart        *string  `json:"activeHoursStart,omitempty"`
	ActiveHoursEnd          *string  `json:"activeHoursEnd,omitempty"`
	ActiveTimezone          *string  `json:"activeTimezone,omitempty"`
	ActiveDays              []int    `json:"activeDays,omitempty"`
}

type agentMembershipConstraintFlags struct {
	rateLimitPerMinute    int
	rateLimitPerHour      int
	allowedIPs            string
	allowedProjects       string
	allowedTeams          string
	allowDelegatedRouting *optionalBoolFlag
	delegatedTeams        string
	activeHoursStart      string
	activeHoursEnd        string
	activeTimezone        string
	activeDays            string
}

const agentTokenSelection = `
id
agentID
tokenPrefix
name
expiresAt
revokedAt
lastUsedAt
lastUsedIP
useCount
createdAt
createdByID
`

const agentMembershipConstraintsSelection = `
rateLimitPerMinute
rateLimitPerHour
allowedIPs
allowedProjectIDs
allowedTeamIDs
allowDelegatedRouting
delegatedRoutingTeamIDs
activeHoursStart
activeHoursEnd
activeTimezone
activeDays
`

const agentMembershipSelection = `
id
workspaceID
principalID
principalType
role
permissions
constraints {
  ` + agentMembershipConstraintsSelection + `
}
grantedAt
expiresAt
revokedAt
`

const agentListSelection = `
id
workspaceID
name
description
ownerID
status
statusReason
createdAt
updatedAt
createdByID
`

const agentDetailSelection = `
` + agentListSelection + `
tokens {
  ` + agentTokenSelection + `
}
membership {
  ` + agentMembershipSelection + `
}
`

func runAgents(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAgentsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr agents list", flag.ContinueOnError)
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

		agents, err := runAgentList(ctx, client, workspaceValue)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, agents, stderr)
		}
		if len(agents) == 0 {
			fmt.Fprintln(stdout, "no agents found")
			return 0
		}
		for _, agent := range agents {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", agent.ID, agent.Name, agent.Status)
		}
		return 0
	case "show":
		fs := flag.NewFlagSet("mbr agents show", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "agent identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		agent, err := runAgentShow(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, agent, stderr)
		}
		printAgent(stdout, agent)
		return 0
	case "create":
		fs := flag.NewFlagSet("mbr agents create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		name := fs.String("name", "", "Agent name")
		description := fs.String("description", "", "Agent description")
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
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		agent, err := runAgentCreate(ctx, client, workspaceValue, strings.TrimSpace(*name), strings.TrimSpace(*description))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, agent, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", agent.ID, agent.Name, agent.Status)
		return 0
	case "update":
		fs := flag.NewFlagSet("mbr agents update", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		name := fs.String("name", "", "Updated agent name")
		description := fs.String("description", "", "Updated agent description")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":         true,
			"--api-url":     true,
			"--token":       true,
			"--name":        true,
			"--description": true,
			"--json":        false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "agent identifier is required")
			return 2
		}
		if strings.TrimSpace(*name) == "" && strings.TrimSpace(*description) == "" {
			fmt.Fprintln(stderr, "at least one of --name or --description is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		agent, err := runAgentUpdate(ctx, client, strings.TrimSpace(positionals[0]), strings.TrimSpace(*name), strings.TrimSpace(*description))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, agent, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", agent.ID, agent.Name, agent.Status)
		return 0
	case "suspend":
		fs := flag.NewFlagSet("mbr agents suspend", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		reason := fs.String("reason", "", "Suspension reason")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":     true,
			"--api-url": true,
			"--token":   true,
			"--reason":  true,
			"--json":    false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "agent identifier is required")
			return 2
		}
		if strings.TrimSpace(*reason) == "" {
			fmt.Fprintln(stderr, "--reason is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		agent, err := runAgentSuspend(ctx, client, strings.TrimSpace(positionals[0]), strings.TrimSpace(*reason))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, agent, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", agent.ID, agent.Name, agent.Status)
		return 0
	case "activate":
		fs := flag.NewFlagSet("mbr agents activate", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "agent identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		agent, err := runAgentActivate(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, agent, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", agent.ID, agent.Name, agent.Status)
		return 0
	case "revoke":
		fs := flag.NewFlagSet("mbr agents revoke", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		reason := fs.String("reason", "", "Revocation reason")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":     true,
			"--api-url": true,
			"--token":   true,
			"--reason":  true,
			"--json":    false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "agent identifier is required")
			return 2
		}
		if strings.TrimSpace(*reason) == "" {
			fmt.Fprintln(stderr, "--reason is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		agent, err := runAgentRevoke(ctx, client, strings.TrimSpace(positionals[0]), strings.TrimSpace(*reason))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, agent, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", agent.ID, agent.Name, agent.Status)
		return 0
	case "tokens":
		return runAgentTokens(ctx, args[1:], stdout, stderr)
	case "memberships":
		return runAgentMemberships(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown agents command %q\n\n", args[0])
		printAgentsUsage(stderr)
		return 2
	}
}

func runAgentTokens(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAgentsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr agents tokens list", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "agent identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		tokens, err := runAgentTokensList(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, tokens, stderr)
		}
		if len(tokens) == 0 {
			fmt.Fprintln(stdout, "no agent tokens found")
			return 0
		}
		for _, token := range tokens {
			state := "active"
			if token.RevokedAt != nil {
				state = "revoked"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", token.ID, token.TokenPrefix, token.Name, state)
		}
		return 0
	case "create":
		fs := flag.NewFlagSet("mbr agents tokens create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		name := fs.String("name", "", "Token name")
		expiresInDays := fs.Int("expires-in-days", 0, "Token expiry in days")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":             true,
			"--api-url":         true,
			"--token":           true,
			"--name":            true,
			"--expires-in-days": true,
			"--json":            false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "agent identifier is required")
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
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		result, err := runAgentTokenCreate(ctx, client, strings.TrimSpace(positionals[0]), strings.TrimSpace(*name), *expiresInDays)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "tokenID:\t%s\n", result.Token.ID)
		fmt.Fprintf(stdout, "tokenPrefix:\t%s\n", result.Token.TokenPrefix)
		fmt.Fprintf(stdout, "plaintextToken:\t%s\n", result.PlaintextToken)
		if result.Token.ExpiresAt != nil {
			fmt.Fprintf(stdout, "expiresAt:\t%s\n", *result.Token.ExpiresAt)
		}
		return 0
	case "revoke":
		fs := flag.NewFlagSet("mbr agents tokens revoke", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "token identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		result, err := runAgentTokenRevoke(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, result, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", result.ID, result.TokenPrefix, result.Name)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown agents tokens command %q\n\n", args[0])
		printAgentsUsage(stderr)
		return 2
	}
}

func runAgentMemberships(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAgentsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "show":
		fs := flag.NewFlagSet("mbr agents memberships show", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "agent identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		membership, err := runAgentMembershipShow(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, membership, stderr)
		}
		if membership == nil {
			fmt.Fprintln(stdout, "no active workspace membership found")
			return 0
		}
		printWorkspaceMembership(stdout, *membership)
		return 0
	case "grant":
		fs := flag.NewFlagSet("mbr agents memberships grant", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		role := fs.String("role", "", "Workspace role")
		permissions := fs.String("permissions", "", "Comma-separated permissions")
		expiresInDays := fs.Int("expires-in-days", 0, "Membership expiry in days")
		inputFile := fs.String("input-file", "", "Path to membership grant JSON")
		inputJSON := fs.String("input-json", "", "Inline membership grant JSON")
		rateLimitPerMinute := fs.Int("rate-limit-per-minute", 0, "Per-minute request limit")
		rateLimitPerHour := fs.Int("rate-limit-per-hour", 0, "Per-hour request limit")
		allowedIPs := fs.String("allowed-ips", "", "Comma-separated allowed IPs")
		allowedProjects := fs.String("allowed-projects", "", "Comma-separated allowed project IDs")
		allowedTeams := fs.String("allowed-teams", "", "Comma-separated allowed team IDs")
		allowDelegatedRouting := newOptionalBoolFlag()
		fs.Var(allowDelegatedRouting, "allow-delegated-routing", "Allow delegated routing for this membership")
		delegatedTeams := fs.String("delegated-routing-teams", "", "Comma-separated delegated-routing team IDs")
		activeHoursStart := fs.String("active-hours-start", "", "Active hours start (HH:MM)")
		activeHoursEnd := fs.String("active-hours-end", "", "Active hours end (HH:MM)")
		activeTimezone := fs.String("active-timezone", "", "Active-hours timezone")
		activeDays := fs.String("active-days", "", "Comma-separated active days (1=Mon,7=Sun)")
		jsonOutput := fs.Bool("json", false, "Emit JSON output")
		flagArgs, positionals := splitSinglePositionalArgs(args[1:], map[string]bool{
			"--url":                     true,
			"--api-url":                 true,
			"--token":                   true,
			"--workspace":               true,
			"--role":                    true,
			"--permissions":             true,
			"--expires-in-days":         true,
			"--input-file":              true,
			"--input-json":              true,
			"--rate-limit-per-minute":   true,
			"--rate-limit-per-hour":     true,
			"--allowed-ips":             true,
			"--allowed-projects":        true,
			"--allowed-teams":           true,
			"--allow-delegated-routing": false,
			"--delegated-routing-teams": true,
			"--active-hours-start":      true,
			"--active-hours-end":        true,
			"--active-timezone":         true,
			"--active-days":             true,
			"--json":                    false,
		})
		if err := fs.Parse(flagArgs); err != nil {
			return 2
		}
		if len(positionals) != 1 || strings.TrimSpace(positionals[0]) == "" {
			fmt.Fprintln(stderr, "agent identifier is required")
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
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		baseInput, err := readOptionalJSONObjectInput(strings.TrimSpace(*inputFile), strings.TrimSpace(*inputJSON), "input")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		constraintFlags := agentMembershipConstraintFlags{
			rateLimitPerMinute:    *rateLimitPerMinute,
			rateLimitPerHour:      *rateLimitPerHour,
			allowedIPs:            *allowedIPs,
			allowedProjects:       *allowedProjects,
			allowedTeams:          *allowedTeams,
			allowDelegatedRouting: allowDelegatedRouting,
			delegatedTeams:        *delegatedTeams,
			activeHoursStart:      *activeHoursStart,
			activeHoursEnd:        *activeHoursEnd,
			activeTimezone:        *activeTimezone,
			activeDays:            *activeDays,
		}
		input, err := buildAgentMembershipGrantInput(baseInput, workspaceValue, strings.TrimSpace(positionals[0]), strings.TrimSpace(*role), strings.TrimSpace(*permissions), *expiresInDays, constraintFlags)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		membership, err := runAgentMembershipGrant(ctx, client, input)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, membership, stderr)
		}
		printWorkspaceMembership(stdout, membership)
		return 0
	case "revoke":
		fs := flag.NewFlagSet("mbr agents memberships revoke", flag.ContinueOnError)
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
			fmt.Fprintln(stderr, "membership identifier is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "agents"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		membership, err := runAgentMembershipRevoke(ctx, client, strings.TrimSpace(positionals[0]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, membership, stderr)
		}
		printWorkspaceMembership(stdout, membership)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown agents memberships command %q\n\n", args[0])
		printAgentsUsage(stderr)
		return 2
	}
}

func runAgentList(ctx context.Context, client *cliapi.Client, workspaceID string) ([]agentOutput, error) {
	var payload struct {
		Agents []agentOutput `json:"agents"`
	}
	err := client.Query(ctx, `
		query CLIAgents($workspaceID: ID!) {
		  agents(workspaceID: $workspaceID) {
		    `+agentListSelection+`
		  }
		}
	`, map[string]any{"workspaceID": workspaceID}, &payload)
	if err != nil {
		return nil, err
	}
	return payload.Agents, nil
}

func runAgentShow(ctx context.Context, client *cliapi.Client, agentID string) (agentOutput, error) {
	var payload struct {
		Agent *agentOutput `json:"agent"`
	}
	err := client.Query(ctx, `
		query CLIAgent($id: ID!) {
		  agent(id: $id) {
		    `+agentDetailSelection+`
		  }
		}
	`, map[string]any{"id": agentID}, &payload)
	if err != nil {
		return agentOutput{}, err
	}
	if payload.Agent == nil {
		return agentOutput{}, fmt.Errorf("agent not found")
	}
	return *payload.Agent, nil
}

func runAgentCreate(ctx context.Context, client *cliapi.Client, workspaceID, name, description string) (agentOutput, error) {
	var payload struct {
		CreateAgent *agentOutput `json:"createAgent"`
	}
	input := map[string]any{
		"workspaceID": workspaceID,
		"name":        name,
	}
	if description != "" {
		input["description"] = description
	}
	err := client.Query(ctx, `
		mutation CLICreateAgent($input: CreateAgentInput!) {
		  createAgent(input: $input) {
		    `+agentListSelection+`
		  }
		}
	`, map[string]any{"input": input}, &payload)
	if err != nil {
		return agentOutput{}, err
	}
	if payload.CreateAgent == nil {
		return agentOutput{}, fmt.Errorf("agent creation returned no payload")
	}
	return *payload.CreateAgent, nil
}

func runAgentUpdate(ctx context.Context, client *cliapi.Client, agentID, name, description string) (agentOutput, error) {
	var payload struct {
		UpdateAgent *agentOutput `json:"updateAgent"`
	}
	input := map[string]any{}
	if name != "" {
		input["name"] = name
	}
	if description != "" {
		input["description"] = description
	}
	err := client.Query(ctx, `
		mutation CLIUpdateAgent($id: ID!, $input: UpdateAgentInput!) {
		  updateAgent(id: $id, input: $input) {
		    `+agentListSelection+`
		  }
		}
	`, map[string]any{"id": agentID, "input": input}, &payload)
	if err != nil {
		return agentOutput{}, err
	}
	if payload.UpdateAgent == nil {
		return agentOutput{}, fmt.Errorf("agent update returned no payload")
	}
	return *payload.UpdateAgent, nil
}

func runAgentSuspend(ctx context.Context, client *cliapi.Client, agentID, reason string) (agentOutput, error) {
	var payload struct {
		SuspendAgent *agentOutput `json:"suspendAgent"`
	}
	err := client.Query(ctx, `
		mutation CLISuspendAgent($id: ID!, $reason: String!) {
		  suspendAgent(id: $id, reason: $reason) {
		    `+agentListSelection+`
		  }
		}
	`, map[string]any{"id": agentID, "reason": reason}, &payload)
	if err != nil {
		return agentOutput{}, err
	}
	if payload.SuspendAgent == nil {
		return agentOutput{}, fmt.Errorf("agent suspension returned no payload")
	}
	return *payload.SuspendAgent, nil
}

func runAgentActivate(ctx context.Context, client *cliapi.Client, agentID string) (agentOutput, error) {
	var payload struct {
		ActivateAgent *agentOutput `json:"activateAgent"`
	}
	err := client.Query(ctx, `
		mutation CLIActivateAgent($id: ID!) {
		  activateAgent(id: $id) {
		    `+agentListSelection+`
		  }
		}
	`, map[string]any{"id": agentID}, &payload)
	if err != nil {
		return agentOutput{}, err
	}
	if payload.ActivateAgent == nil {
		return agentOutput{}, fmt.Errorf("agent activation returned no payload")
	}
	return *payload.ActivateAgent, nil
}

func runAgentRevoke(ctx context.Context, client *cliapi.Client, agentID, reason string) (agentOutput, error) {
	var payload struct {
		RevokeAgent *agentOutput `json:"revokeAgent"`
	}
	err := client.Query(ctx, `
		mutation CLIRevokeAgent($id: ID!, $reason: String!) {
		  revokeAgent(id: $id, reason: $reason) {
		    `+agentListSelection+`
		  }
		}
	`, map[string]any{"id": agentID, "reason": reason}, &payload)
	if err != nil {
		return agentOutput{}, err
	}
	if payload.RevokeAgent == nil {
		return agentOutput{}, fmt.Errorf("agent revocation returned no payload")
	}
	return *payload.RevokeAgent, nil
}

func runAgentTokensList(ctx context.Context, client *cliapi.Client, agentID string) ([]agentTokenOutput, error) {
	var payload struct {
		Agent *struct {
			Tokens []agentTokenOutput `json:"tokens"`
		} `json:"agent"`
	}
	err := client.Query(ctx, `
		query CLIAgentTokens($id: ID!) {
		  agent(id: $id) {
		    id
		    tokens {
		      `+agentTokenSelection+`
		    }
		  }
		}
	`, map[string]any{"id": agentID}, &payload)
	if err != nil {
		return nil, err
	}
	if payload.Agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	return payload.Agent.Tokens, nil
}

func runAgentTokenCreate(ctx context.Context, client *cliapi.Client, agentID, name string, expiresInDays int) (agentTokenResultOutput, error) {
	var payload struct {
		CreateAgentToken *agentTokenResultOutput `json:"createAgentToken"`
	}
	input := map[string]any{
		"agentID": agentID,
		"name":    name,
	}
	if expiresInDays > 0 {
		input["expiresInDays"] = expiresInDays
	}
	err := client.Query(ctx, `
		mutation CLICreateAgentToken($input: CreateAgentTokenInput!) {
		  createAgentToken(input: $input) {
		    plaintextToken
		    token {
		      `+agentTokenSelection+`
		    }
		  }
		}
	`, map[string]any{"input": input}, &payload)
	if err != nil {
		return agentTokenResultOutput{}, err
	}
	if payload.CreateAgentToken == nil {
		return agentTokenResultOutput{}, fmt.Errorf("agent token creation returned no payload")
	}
	return *payload.CreateAgentToken, nil
}

func runAgentTokenRevoke(ctx context.Context, client *cliapi.Client, tokenID string) (agentTokenOutput, error) {
	var payload struct {
		RevokeAgentToken *agentTokenOutput `json:"revokeAgentToken"`
	}
	err := client.Query(ctx, `
		mutation CLIRevokeAgentToken($id: ID!) {
		  revokeAgentToken(id: $id) {
		    `+agentTokenSelection+`
		  }
		}
	`, map[string]any{"id": tokenID}, &payload)
	if err != nil {
		return agentTokenOutput{}, err
	}
	if payload.RevokeAgentToken == nil {
		return agentTokenOutput{}, fmt.Errorf("agent token revocation returned no payload")
	}
	return *payload.RevokeAgentToken, nil
}

func runAgentMembershipShow(ctx context.Context, client *cliapi.Client, agentID string) (*workspaceMembershipOutput, error) {
	var payload struct {
		Agent *struct {
			Membership *workspaceMembershipOutput `json:"membership"`
		} `json:"agent"`
	}
	err := client.Query(ctx, `
		query CLIAgentMembership($id: ID!) {
		  agent(id: $id) {
		    id
		    membership {
		      `+agentMembershipSelection+`
		    }
		  }
		}
	`, map[string]any{"id": agentID}, &payload)
	if err != nil {
		return nil, err
	}
	if payload.Agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	return payload.Agent.Membership, nil
}

func runAgentMembershipGrant(ctx context.Context, client *cliapi.Client, input map[string]any) (workspaceMembershipOutput, error) {
	var payload struct {
		GrantAgentMembership *workspaceMembershipOutput `json:"grantAgentMembership"`
	}
	err := client.Query(ctx, `
		mutation CLIGrantAgentMembership($input: GrantMembershipInput!) {
		  grantAgentMembership(input: $input) {
		    `+agentMembershipSelection+`
		  }
		}
	`, map[string]any{"input": input}, &payload)
	if err != nil {
		return workspaceMembershipOutput{}, err
	}
	if payload.GrantAgentMembership == nil {
		return workspaceMembershipOutput{}, fmt.Errorf("membership grant returned no payload")
	}
	return *payload.GrantAgentMembership, nil
}

func runAgentMembershipRevoke(ctx context.Context, client *cliapi.Client, membershipID string) (workspaceMembershipOutput, error) {
	var payload struct {
		RevokeAgentMembership *workspaceMembershipOutput `json:"revokeAgentMembership"`
	}
	err := client.Query(ctx, `
		mutation CLIRevokeAgentMembership($id: ID!) {
		  revokeAgentMembership(id: $id) {
		    `+agentMembershipSelection+`
		  }
		}
	`, map[string]any{"id": membershipID}, &payload)
	if err != nil {
		return workspaceMembershipOutput{}, err
	}
	if payload.RevokeAgentMembership == nil {
		return workspaceMembershipOutput{}, fmt.Errorf("membership revoke returned no payload")
	}
	return *payload.RevokeAgentMembership, nil
}

func buildAgentMembershipGrantInput(base map[string]any, workspaceID, agentID, role, permissions string, expiresInDays int, flags agentMembershipConstraintFlags) (map[string]any, error) {
	input := map[string]any{}
	for key, value := range base {
		input[key] = value
	}
	input["workspaceID"] = workspaceID
	input["agentID"] = agentID
	if role != "" {
		input["role"] = role
	}
	if perms := commaSeparatedValues(permissions); len(perms) > 0 {
		input["permissions"] = perms
	}
	if expiresInDays > 0 {
		input["expiresInDays"] = expiresInDays
	}

	constraints, err := membershipConstraintInputMap(input)
	if err != nil {
		return nil, err
	}
	if flags.rateLimitPerMinute > 0 {
		constraints["rateLimitPerMinute"] = flags.rateLimitPerMinute
	}
	if flags.rateLimitPerHour > 0 {
		constraints["rateLimitPerHour"] = flags.rateLimitPerHour
	}
	if values := commaSeparatedValues(flags.allowedIPs); len(values) > 0 {
		constraints["allowedIPs"] = values
	}
	if values := commaSeparatedValues(flags.allowedProjects); len(values) > 0 {
		constraints["allowedProjectIDs"] = values
	}
	if values := commaSeparatedValues(flags.allowedTeams); len(values) > 0 {
		constraints["allowedTeamIDs"] = values
	}
	delegatedTeams := commaSeparatedValues(flags.delegatedTeams)
	if flags.allowDelegatedRouting != nil && flags.allowDelegatedRouting.set {
		constraints["allowDelegatedRouting"] = flags.allowDelegatedRouting.value
	}
	if len(delegatedTeams) > 0 {
		if flags.allowDelegatedRouting != nil && flags.allowDelegatedRouting.set && !flags.allowDelegatedRouting.value {
			return nil, fmt.Errorf("--delegated-routing-teams cannot be used when --allow-delegated-routing=false")
		}
		constraints["allowDelegatedRouting"] = true
		constraints["delegatedRoutingTeamIDs"] = delegatedTeams
	}
	if value := strings.TrimSpace(flags.activeHoursStart); value != "" {
		constraints["activeHoursStart"] = value
	}
	if value := strings.TrimSpace(flags.activeHoursEnd); value != "" {
		constraints["activeHoursEnd"] = value
	}
	if value := strings.TrimSpace(flags.activeTimezone); value != "" {
		constraints["activeTimezone"] = value
	}
	if values, err := commaSeparatedInts(flags.activeDays); err != nil {
		return nil, err
	} else if len(values) > 0 {
		constraints["activeDays"] = values
	}
	if len(constraints) > 0 {
		input["constraints"] = constraints
	} else {
		delete(input, "constraints")
	}

	roleValue, ok := input["role"].(string)
	if !ok || strings.TrimSpace(roleValue) == "" {
		return nil, fmt.Errorf("membership role is required; pass --role or provide it in --input-file/--input-json")
	}
	permissionValues, err := anyStringSlice(input["permissions"])
	if err != nil {
		return nil, fmt.Errorf("permissions must be a string array: %w", err)
	}
	if len(permissionValues) == 0 {
		return nil, fmt.Errorf("at least one permission is required; pass --permissions or provide them in --input-file/--input-json")
	}
	input["permissions"] = permissionValues
	return input, nil
}

func membershipConstraintInputMap(input map[string]any) (map[string]any, error) {
	raw, ok := input["constraints"]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}
	typed, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("constraints must be a JSON object")
	}
	constraints := make(map[string]any, len(typed))
	for key, value := range typed {
		constraints[key] = value
	}
	return constraints, nil
}

func anyStringSlice(value any) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []string:
		return uniqueStrings(typed), nil
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				items = append(items, text)
			}
		}
		return uniqueStrings(items), nil
	case string:
		return commaSeparatedValues(typed), nil
	default:
		return nil, fmt.Errorf("unexpected type %T", value)
	}
}

func commaSeparatedInts(value string) ([]int, error) {
	values := commaSeparatedValues(value)
	if len(values) == 0 {
		return nil, nil
	}
	parsed := make([]int, 0, len(values))
	for _, item := range values {
		var day int
		if _, err := fmt.Sscanf(item, "%d", &day); err != nil {
			return nil, fmt.Errorf("invalid integer value %q", item)
		}
		parsed = append(parsed, day)
	}
	return parsed, nil
}

func printAgent(w io.Writer, agent agentOutput) {
	fmt.Fprintf(w, "id:\t%s\n", agent.ID)
	fmt.Fprintf(w, "workspace:\t%s\n", agent.WorkspaceID)
	fmt.Fprintf(w, "name:\t%s\n", agent.Name)
	if agent.Description != nil {
		fmt.Fprintf(w, "description:\t%s\n", *agent.Description)
	}
	fmt.Fprintf(w, "status:\t%s\n", agent.Status)
	if agent.StatusReason != nil {
		fmt.Fprintf(w, "statusReason:\t%s\n", *agent.StatusReason)
	}
	fmt.Fprintf(w, "ownerID:\t%s\n", agent.OwnerID)
	fmt.Fprintf(w, "createdByID:\t%s\n", agent.CreatedByID)
	fmt.Fprintf(w, "createdAt:\t%s\n", agent.CreatedAt)
	fmt.Fprintf(w, "updatedAt:\t%s\n", agent.UpdatedAt)
	if agent.Membership != nil {
		fmt.Fprintln(w, "\nmembership:")
		printWorkspaceMembership(w, *agent.Membership)
	}
	if len(agent.Tokens) > 0 {
		fmt.Fprintln(w, "\ntokens:")
		for _, token := range agent.Tokens {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", token.ID, token.TokenPrefix, token.Name)
		}
	}
}

func printWorkspaceMembership(w io.Writer, membership workspaceMembershipOutput) {
	fmt.Fprintf(w, "id:\t%s\n", membership.ID)
	fmt.Fprintf(w, "workspace:\t%s\n", membership.WorkspaceID)
	fmt.Fprintf(w, "principalID:\t%s\n", membership.PrincipalID)
	fmt.Fprintf(w, "principalType:\t%s\n", membership.PrincipalType)
	fmt.Fprintf(w, "role:\t%s\n", membership.Role)
	fmt.Fprintf(w, "permissions:\t%s\n", strings.Join(membership.Permissions, ","))
	fmt.Fprintf(w, "grantedAt:\t%s\n", membership.GrantedAt)
	if membership.ExpiresAt != nil {
		fmt.Fprintf(w, "expiresAt:\t%s\n", *membership.ExpiresAt)
	}
	if membership.RevokedAt != nil {
		fmt.Fprintf(w, "revokedAt:\t%s\n", *membership.RevokedAt)
	}
	if membership.Constraints.RateLimitPerMinute != nil {
		fmt.Fprintf(w, "rateLimitPerMinute:\t%d\n", *membership.Constraints.RateLimitPerMinute)
	}
	if membership.Constraints.RateLimitPerHour != nil {
		fmt.Fprintf(w, "rateLimitPerHour:\t%d\n", *membership.Constraints.RateLimitPerHour)
	}
	if len(membership.Constraints.AllowedIPs) > 0 {
		fmt.Fprintf(w, "allowedIPs:\t%s\n", strings.Join(membership.Constraints.AllowedIPs, ","))
	}
	if len(membership.Constraints.AllowedProjectIDs) > 0 {
		fmt.Fprintf(w, "allowedProjectIDs:\t%s\n", strings.Join(membership.Constraints.AllowedProjectIDs, ","))
	}
	if len(membership.Constraints.AllowedTeamIDs) > 0 {
		fmt.Fprintf(w, "allowedTeamIDs:\t%s\n", strings.Join(membership.Constraints.AllowedTeamIDs, ","))
	}
	fmt.Fprintf(w, "allowDelegatedRouting:\t%t\n", membership.Constraints.AllowDelegatedRouting)
	if len(membership.Constraints.DelegatedRoutingTeamIDs) > 0 {
		fmt.Fprintf(w, "delegatedRoutingTeamIDs:\t%s\n", strings.Join(membership.Constraints.DelegatedRoutingTeamIDs, ","))
	}
	if membership.Constraints.ActiveHoursStart != nil {
		fmt.Fprintf(w, "activeHoursStart:\t%s\n", *membership.Constraints.ActiveHoursStart)
	}
	if membership.Constraints.ActiveHoursEnd != nil {
		fmt.Fprintf(w, "activeHoursEnd:\t%s\n", *membership.Constraints.ActiveHoursEnd)
	}
	if membership.Constraints.ActiveTimezone != nil {
		fmt.Fprintf(w, "activeTimezone:\t%s\n", *membership.Constraints.ActiveTimezone)
	}
	if len(membership.Constraints.ActiveDays) > 0 {
		values := make([]string, 0, len(membership.Constraints.ActiveDays))
		for _, day := range membership.Constraints.ActiveDays {
			values = append(values, fmt.Sprintf("%d", day))
		}
		fmt.Fprintf(w, "activeDays:\t%s\n", strings.Join(values, ","))
	}
}
