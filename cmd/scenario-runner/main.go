package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	"github.com/movebigrocks/platform/internal/testutil/synth"
)

func main() {
	// Parse flags
	var (
		dataDir       = flag.String("data-dir", "./data/synth", "Directory for synthetic data storage")
		numWorkspaces = flag.Int("workspaces", 2, "Number of workspaces to create")
		numAgents     = flag.Int("agents", 3, "Number of agents per workspace")
		numCases      = flag.Int("cases", 20, "Number of cases per workspace")
		runScenarios  = flag.Bool("scenarios", true, "Run interactive scenarios after generation")
		dryRun        = flag.Bool("dry-run", false, "Show what would be created without saving")
		verbose       = flag.Bool("verbose", true, "Enable verbose output")
		clean         = flag.Bool("clean", false, "Clean existing data before generating")
	)
	flag.Parse()

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Move Big Rocks Synthetic Data Generator & Scenarios         ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Clean existing data if requested
	if *clean {
		fmt.Printf("Cleaning existing data in %s...\n", *dataDir)
		os.RemoveAll(*dataDir)
	}

	// Ensure data directory exists
	absDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving data directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(absDataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Data directory: %s\n\n", absDataDir)

	// Create store (filesystem-backed)
	store, err := stores.NewStore(absDataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create store: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Configure generator
	genConfig := &synth.Config{
		NumWorkspaces:      *numWorkspaces,
		NumAdminsPerWs:     1,
		NumAgentsPerWs:     *numAgents,
		NumCustomersPerWs:  *numCases * 2, // More customers than cases
		NumCasesPerWs:      *numCases,
		NumMessagesPerCase: 4,
		CaseSpreadDays:     30,
		Verbose:            *verbose,
		DryRun:             *dryRun,
	}

	// Phase 1: Generate base data
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("Phase 1: Generating Base Data")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	generator := synth.NewGenerator(store, genConfig)
	data, err := generator.Generate(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating data: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	printDataSummary(data)

	// Phase 2: Run scenarios
	if *runScenarios && !*dryRun {
		// Create test services for scenario testing (uses real service layer)
		testServices := synth.NewTestServices(store)

		// Track total results across all scenario types
		var allResults []*synth.ScenarioResult

		// Phase 2a: Case Lifecycle Scenarios
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2a: Running Case Lifecycle Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		for _, wsData := range data.Workspaces {
			fmt.Printf("\n📁 Workspace: %s\n", wsData.Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			scenarioRunner := synth.NewScenarioRunner(testServices, *verbose)
			results, err := scenarioRunner.RunAllScenarios(ctx, wsData.Workspace.ID, wsData.Agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running case scenarios: %v\n", err)
				continue
			}
			allResults = append(allResults, results...)
			printScenarioResults(results)
		}

		// Phase 2b: Platform Scenarios (workspace/user management)
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2b: Running Platform Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		platformRunner := synth.NewPlatformScenarioRunner(testServices, *verbose)
		platformResults, err := platformRunner.RunAllPlatformScenarios(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running platform scenarios: %v\n", err)
		} else {
			allResults = append(allResults, platformResults...)
			printScenarioResults(platformResults)
		}

		// Phase 2c: Observability Scenarios (error monitoring)
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2c: Running Observability Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		observabilityRunner := synth.NewObservabilityScenarioRunner(testServices, *verbose)
		// Use first workspace for observability tests
		if len(data.Workspaces) > 0 {
			fmt.Printf("\n📁 Testing in Workspace: %s\n", data.Workspaces[0].Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			observabilityResults, err := observabilityRunner.RunAllObservabilityScenarios(ctx, data.Workspaces[0].Workspace.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running observability scenarios: %v\n", err)
			} else {
				allResults = append(allResults, observabilityResults...)
				printScenarioResults(observabilityResults)
			}
		}

		// Phase 2d: Automation Scenarios (rules and jobs)
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2d: Running Automation Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		automationRunner := synth.NewAutomationScenarioRunner(testServices, *verbose)
		// Use first workspace and its agents for automation tests
		if len(data.Workspaces) > 0 && len(data.Workspaces[0].Agents) > 0 {
			fmt.Printf("\n📁 Testing in Workspace: %s\n", data.Workspaces[0].Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			automationResults, err := automationRunner.RunAllAutomationScenarios(ctx, data.Workspaces[0].Workspace.ID, data.Workspaces[0].Agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running automation scenarios: %v\n", err)
			} else {
				allResults = append(allResults, automationResults...)
				printScenarioResults(automationResults)
			}
		}

		// Phase 2e: Event Bus Scenarios (event-driven architecture)
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2e: Running Event Bus Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		eventBusRunner := synth.NewEventBusScenarioRunner(testServices, absDataDir, *verbose)
		eventBusResults, err := eventBusRunner.RunAllEventBusScenarios(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running event bus scenarios: %v\n", err)
		} else {
			allResults = append(allResults, eventBusResults...)
			printScenarioResults(eventBusResults)
		}

		// Phase 2f: Forms System Scenarios
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2f: Running Forms System Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		if len(data.Workspaces) > 0 {
			fmt.Printf("\n📁 Testing in Workspace: %s\n", data.Workspaces[0].Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			formsRunner := synth.NewFormsScenarioRunner(testServices, *verbose)
			formsResults, err := formsRunner.RunAllFormsScenarios(ctx, data.Workspaces[0].Workspace.ID, data.Workspaces[0].Agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running forms scenarios: %v\n", err)
			} else {
				allResults = append(allResults, formsResults...)
				printScenarioResults(formsResults)
			}
		}

		// Phase 2g: Knowledge Resource Scenarios
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2g: Running Knowledge Resource Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		if len(data.Workspaces) > 0 {
			fmt.Printf("\n📁 Testing in Workspace: %s\n", data.Workspaces[0].Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			knowledgeRunner := synth.NewKnowledgeScenarioRunner(testServices, *verbose)
			knowledgeResults, err := knowledgeRunner.RunAllKnowledgeScenarios(ctx, data.Workspaces[0].Workspace.ID, data.Workspaces[0].Agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running knowledge resource scenarios: %v\n", err)
			} else {
				allResults = append(allResults, knowledgeResults...)
				printScenarioResults(knowledgeResults)
			}
		}

		// Phase 2h: Email Processing Scenarios
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2h: Running Email Processing Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		if len(data.Workspaces) > 0 {
			fmt.Printf("\n📁 Testing in Workspace: %s\n", data.Workspaces[0].Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			emailRunner := synth.NewEmailScenarioRunner(testServices, *verbose)
			emailResults, err := emailRunner.RunAllEmailScenarios(ctx, data.Workspaces[0].Workspace.ID, data.Workspaces[0].Agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running email scenarios: %v\n", err)
			} else {
				allResults = append(allResults, emailResults...)
				printScenarioResults(emailResults)
			}
		}

		// Phase 2i: Error-Support Integration Scenarios
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2i: Running Error-Support Integration Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		if len(data.Workspaces) > 0 {
			fmt.Printf("\n📁 Testing in Workspace: %s\n", data.Workspaces[0].Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			integrationRunner := synth.NewIntegrationScenarioRunner(testServices, *verbose)
			integrationResults, err := integrationRunner.RunAllIntegrationScenarios(ctx, data.Workspaces[0].Workspace.ID, data.Workspaces[0].Agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running integration scenarios: %v\n", err)
			} else {
				allResults = append(allResults, integrationResults...)
				printScenarioResults(integrationResults)
			}
		}

		// Phase 2j: Attachments Scenarios
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Phase 2j: Running Attachments Scenarios")
		fmt.Println("═══════════════════════════════════════════════════════════════")

		if len(data.Workspaces) > 0 {
			fmt.Printf("\n📁 Testing in Workspace: %s\n", data.Workspaces[0].Workspace.Name)
			fmt.Println("───────────────────────────────────────────────────────────────")

			attachmentsRunner := synth.NewAttachmentsScenarioRunner(testServices, *verbose)
			attachmentsResults, err := attachmentsRunner.RunAllAttachmentsScenarios(ctx, data.Workspaces[0].Workspace.ID, data.Workspaces[0].Agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running attachments scenarios: %v\n", err)
			} else {
				allResults = append(allResults, attachmentsResults...)
				printScenarioResults(attachmentsResults)
			}
		}

		// Print overall summary
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("Overall Scenario Summary")
		fmt.Println("═══════════════════════════════════════════════════════════════")
		printOverallSummary(allResults)
	}

	// Print filesystem state
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("Filesystem State")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	printFilesystemState(absDataDir)

	fmt.Println()
	fmt.Println("✅ Synthetic data generation complete!")
	fmt.Printf("   Data location: %s\n", absDataDir)
}

func printDataSummary(data *synth.GeneratedData) {
	fmt.Println()
	fmt.Println("📊 Generation Summary")
	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("   Super Admin: %s <%s>\n", data.SuperAdmin.Name, data.SuperAdmin.Email)
	fmt.Printf("   Workspaces:  %d\n", len(data.Workspaces))
	fmt.Printf("   Total Cases: %d\n", data.TotalCases)
	fmt.Printf("   Total Emails: %d\n", data.TotalEmails)
	fmt.Printf("   Duration:    %v\n", data.EndTime.Sub(data.StartTime))
	fmt.Println()

	for _, ws := range data.Workspaces {
		fmt.Printf("   📁 %s (%s)\n", ws.Workspace.Name, ws.Workspace.Slug)
		fmt.Printf("      Admins: %d, Agents: %d, Customers: %d\n",
			len(ws.Admins), len(ws.Agents), len(ws.Customers))
		fmt.Printf("      Cases: %d, Teams: %d\n", len(ws.Cases), len(ws.Teams))

		// Case status breakdown
		statusCounts := make(map[string]int)
		for _, c := range ws.Cases {
			statusCounts[string(c.Case.Status)]++
		}
		fmt.Printf("      Status: ")
		for status, count := range statusCounts {
			fmt.Printf("%s=%d ", status, count)
		}
		fmt.Println()
	}
}

func printScenarioResults(results []*synth.ScenarioResult) {
	successCount := 0
	totalVerifications := 0
	passedVerifications := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
		for _, v := range r.Verifications {
			totalVerifications++
			if v.Passed {
				passedVerifications++
			}
		}
	}

	fmt.Printf("\n📋 Scenario Results: %d/%d passed, %d/%d verifications passed\n",
		successCount, len(results), passedVerifications, totalVerifications)
	fmt.Println("───────────────────────────────────────────────────────────────")

	for _, r := range results {
		status := "✅"
		if !r.Success {
			status = "❌"
		}
		fmt.Printf("   %s %s\n", status, r.Name)
		if r.Success {
			// Only show case details if CaseID is set (case lifecycle scenarios)
			if len(r.CaseID) >= 8 {
				fmt.Printf("      Case: %s, Final: %s, Messages: %d, Duration: %v\n",
					r.CaseID[:8]+"...", r.FinalStatus, r.TotalMessages, r.Duration.Round(time.Millisecond))
				fmt.Printf("      Transitions: ")
				for i, t := range r.StateTransitions {
					if i > 0 {
						fmt.Print(" → ")
					}
					fmt.Printf("%s", t.ToStatus)
				}
				fmt.Println()
			} else {
				// For non-case scenarios, just show duration
				fmt.Printf("      Duration: %v\n", r.Duration.Round(time.Millisecond))
			}
			// Show verification count
			passed := 0
			for _, v := range r.Verifications {
				if v.Passed {
					passed++
				}
			}
			fmt.Printf("      Verifications: %d/%d passed\n", passed, len(r.Verifications))
		} else if r.Error != nil {
			fmt.Printf("      Error: %v\n", r.Error)
			// Show failed verifications
			for _, v := range r.Verifications {
				if !v.Passed {
					fmt.Printf("      ✗ %s: %s\n", v.Check, v.Details)
				}
			}
		}
	}
}

func printOverallSummary(results []*synth.ScenarioResult) {
	successCount := 0
	totalVerifications := 0
	passedVerifications := 0

	for _, r := range results {
		if r.Success {
			successCount++
		}
		for _, v := range r.Verifications {
			totalVerifications++
			if v.Passed {
				passedVerifications++
			}
		}
	}

	fmt.Printf("\n🎯 Total Scenarios: %d/%d passed (%.1f%%)\n",
		successCount, len(results), float64(successCount)*100/float64(len(results)))
	fmt.Printf("   Verifications: %d/%d passed (%.1f%%)\n",
		passedVerifications, totalVerifications, float64(passedVerifications)*100/float64(totalVerifications))

	// Show any failures
	failedCount := 0
	for _, r := range results {
		if !r.Success {
			failedCount++
			if failedCount == 1 {
				fmt.Println("\n❌ Failed Scenarios:")
			}
			fmt.Printf("   - %s", r.Name)
			if r.Error != nil {
				fmt.Printf(": %v", r.Error)
			}
			fmt.Println()
		}
	}

	if failedCount == 0 {
		fmt.Println("\n✅ All scenarios passed!")
	}
}

func printFilesystemState(dataDir string) {
	// Count files by type
	counts := make(map[string]int)

	// Walk directory tree (ignore walk errors: continue with partial state)
	_ = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Get relative path (ignore errors: skip file on failure)
		rel, _ := filepath.Rel(dataDir, path)

		// Categorize based on path components
		switch {
		case strings.Contains(rel, "/users/") || strings.Contains(rel, "_global/users/"):
			counts["users"]++
		case strings.HasSuffix(rel, "workspace.json"):
			counts["workspaces"]++
		case strings.HasSuffix(rel, "settings.json"):
			counts["settings"]++
		case strings.Contains(rel, "/cases/"):
			counts["cases"]++
		case strings.Contains(rel, "/emails/inbound/"):
			counts["inbound_emails"]++
		case strings.Contains(rel, "/emails/outbound/"):
			counts["outbound_emails"]++
		case strings.Contains(rel, "/user_roles/"):
			counts["user_roles"]++
		case strings.Contains(rel, "/_idx/") || strings.Contains(rel, "/_counters/"):
			counts["indexes"]++
		default:
			counts["other"]++
		}
		return nil
	})

	fmt.Printf("   Users:           %d files\n", counts["users"])
	fmt.Printf("   Workspaces:      %d files\n", counts["workspaces"])
	fmt.Printf("   Settings:        %d files\n", counts["settings"])
	fmt.Printf("   User Roles:      %d files\n", counts["user_roles"])
	fmt.Printf("   Cases:           %d files\n", counts["cases"])
	fmt.Printf("   Inbound Emails:  %d files\n", counts["inbound_emails"])
	fmt.Printf("   Outbound Emails: %d files\n", counts["outbound_emails"])
	fmt.Printf("   Indexes:         %d files\n", counts["indexes"])
}
