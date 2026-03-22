package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/movebigrocks/platform/internal/cliapi"
)

func runAutomation(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAutomationUsage(stderr)
		return 2
	}

	switch args[0] {
	case "rules":
		return runAutomationRules(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown automation command %q\n\n", args[0])
		printAutomationUsage(stderr)
		return 2
	}
}

func runAutomationRules(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printAutomationUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("mbr automation rules list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		limit := fs.Int("limit", 50, "Maximum number of rules to return")
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

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "automation"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		filter := map[string]any{
			"workspaceID": workspaceValue,
			"first":       *limit,
		}

		var payload struct {
			AdminRules struct {
				Edges []struct {
					Node ruleOutput `json:"node"`
				} `json:"edges"`
				TotalCount int `json:"totalCount"`
			} `json:"adminRules"`
		}
		err = client.Query(ctx, `
			query CLIAdminRules($filter: AdminRuleFilter) {
			  adminRules(filter: $filter) {
			    totalCount
			    edges {
			      node {
			        `+ruleSelection+`
			      }
			    }
			  }
			}
		`, map[string]any{"filter": filter}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		rules := make([]ruleOutput, 0, len(payload.AdminRules.Edges))
		for _, edge := range payload.AdminRules.Edges {
			rules = append(rules, edge.Node)
		}
		if *jsonOutput {
			return writeJSON(stdout, map[string]any{
				"totalCount": payload.AdminRules.TotalCount,
				"rules":      rules,
			}, stderr)
		}
		if len(rules) == 0 {
			fmt.Fprintln(stdout, "no automation rules found")
			return 0
		}
		for _, rule := range rules {
			fmt.Fprintf(stdout, "%s\t%t\t%d\t%s\n", rule.ID, rule.IsActive, rule.Priority, rule.Title)
		}
		return 0
	case "create":
		fs := flag.NewFlagSet("mbr automation rules create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		instanceURL := registerInstanceURLFlag(fs)
		token := fs.String("token", "", "Bearer token")
		workspaceID := fs.String("workspace", "", "Workspace ID")
		title := fs.String("title", "", "Rule title")
		description := fs.String("description", "", "Rule description")
		conditionsFile := fs.String("conditions-file", "", "Path to JSON array of rule conditions")
		conditionsJSON := fs.String("conditions-json", "", "Inline JSON array of rule conditions")
		actionsFile := fs.String("actions-file", "", "Path to JSON array of rule actions")
		actionsJSON := fs.String("actions-json", "", "Inline JSON array of rule actions")
		activeFlag := newOptionalBoolFlag()
		fs.Var(activeFlag, "active", "Activate the rule immediately")
		priority := fs.Int("priority", 0, "Rule priority")
		maxPerHour := fs.Int("max-per-hour", 0, "Maximum executions per hour")
		maxPerDay := fs.Int("max-per-day", 0, "Maximum executions per day")
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
		if strings.TrimSpace(*title) == "" {
			fmt.Fprintln(stderr, "--title is required")
			return 2
		}

		cfg, err := loadCLIConfig(*instanceURL, *token)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if err := requireSessionAuth(cfg, "automation"); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		client := newCLIClient(cfg)

		conditions, err := readJSONArrayInput(*conditionsFile, *conditionsJSON, "conditions")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		actions, err := readJSONArrayInput(*actionsFile, *actionsJSON, "actions")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		input := map[string]any{
			"workspaceID": workspaceValue,
			"title":       *title,
			"conditions":  conditions,
			"actions":     actions,
			"priority":    *priority,
		}
		if value := strings.TrimSpace(*description); value != "" {
			input["description"] = value
		}
		if activeFlag.set {
			input["isActive"] = activeFlag.value
		}
		if *maxPerHour > 0 {
			input["maxExecutionsPerHour"] = *maxPerHour
		}
		if *maxPerDay > 0 {
			input["maxExecutionsPerDay"] = *maxPerDay
		}

		var payload struct {
			AdminCreateRule ruleOutput `json:"adminCreateRule"`
		}
		err = client.Query(ctx, `
			mutation CLICreateRule($input: CreateRuleInput!) {
			  adminCreateRule(input: $input) {
			    `+ruleSelection+`
			  }
			}
		`, map[string]any{"input": input}, &payload)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if *jsonOutput {
			return writeJSON(stdout, payload.AdminCreateRule, stderr)
		}
		fmt.Fprintf(stdout, "%s\t%t\t%d\t%s\n", payload.AdminCreateRule.ID, payload.AdminCreateRule.IsActive, payload.AdminCreateRule.Priority, payload.AdminCreateRule.Title)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown automation rules command %q\n\n", args[0])
		printAutomationUsage(stderr)
		return 2
	}
}
