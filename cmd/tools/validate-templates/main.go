// validate-templates is a build-time tool that validates all HTML templates.
// It's run during CI/build verification to catch template errors before deployment.
//
// Usage: go run cmd/tools/validate-templates/main.go
//
// Exit codes:
//
//	0 - all templates valid
//	1 - template parsing errors found
package main

import (
	"fmt"
	"os"

	platformhandlers "github.com/movebigrocks/platform/pkg/extensionhost/platform/handlers"
)

func main() {
	hasErrors := false
	mainTemplates, partialTemplates, err := platformhandlers.AdminTemplateFiles()
	if err != nil {
		fmt.Printf("Failed to list embedded admin templates: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Validating embedded admin templates...\n")
	_, err = platformhandlers.ParseAdminTemplates()
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		hasErrors = true
		fmt.Printf("  Checking individual embedded templates:\n")
		for _, tmpl := range append(mainTemplates, partialTemplates...) {
			if _, err := platformhandlers.ParseAdminTemplateWithPartials(tmpl); err != nil {
				fmt.Printf("    FAIL: %s - %v\n", tmpl, err)
			}
		}
	} else {
		fmt.Printf("  OK: %d templates validated\n", len(mainTemplates)+len(partialTemplates))
	}

	if hasErrors {
		fmt.Println("\nTemplate validation FAILED")
		os.Exit(1)
	}

	fmt.Println("\nAll templates valid")
}
