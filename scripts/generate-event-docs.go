//go:build ignore

// generate-event-docs.go generates event documentation from pkg/eventbus/types.go
// Run with: go run scripts/generate-event-docs.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type EventType struct {
	Name     string
	Value    string
	Category string
	Comment  string
}

func main() {
	// Find project root
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	typesFile := filepath.Join(wd, "pkg", "eventbus", "types.go")
	outputFile := filepath.Join(wd, "docs", "EVENTS.md")

	events, err := parseTypesFile(typesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing types.go: %v\n", err)
		os.Exit(1)
	}

	if err := generateMarkdown(events, outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating markdown: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s with %d event types\n", outputFile, len(events))
}

func parseTypesFile(filename string) ([]EventType, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []EventType
	var currentComment string

	// Regex to match:
	// - TypeXxx = EventType{"xxx.yyy"} (compatibility form)
	// - TypeXxx = EventType{value: "xxx.yyy", version: 1} (named fields)
	typeRegex := regexp.MustCompile(`^\s*(Type\w+)\s*=\s*EventType\{\s*(?:value:\s*)?"([^"]+)"`)
	commentRegex := regexp.MustCompile(`^\s*//\s*(.*)`)
	separatorRegex := regexp.MustCompile(`^\s*//\s*=+\s*$`)
	emptyCommentRegex := regexp.MustCompile(`^\s*//\s*$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		// Skip visual separators and empty comment lines
		if separatorRegex.MatchString(trimmedLine) || emptyCommentRegex.MatchString(trimmedLine) {
			currentComment = ""
			continue
		}

		// Check for comment
		if matches := commentRegex.FindStringSubmatch(line); matches != nil {
			if currentComment != "" {
				currentComment += " "
			}
			currentComment += matches[1]
			continue
		}

		// Check for type definition
		if matches := typeRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			if name == "TypeUnknown" {
				currentComment = ""
				continue
			}
			value := matches[2]
			category := extractCategory(value)

			events = append(events, EventType{
				Name:     name,
				Value:    value,
				Category: category,
				Comment:  strings.TrimSpace(currentComment),
			})
			currentComment = ""
			continue
		}

		// Reset comment if we hit a non-comment, non-type line (but not empty or var/parentheses)
		trimmed := trimmedLine
		if trimmed != "" && trimmed != "var (" && trimmed != ")" {
			currentComment = ""
		}
	}

	return events, scanner.Err()
}

func extractCategory(value string) string {
	parts := strings.Split(value, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

func generateMarkdown(events []EventType, filename string) error {
	// Ensure docs directory exists
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Group by category
	byCategory := make(map[string][]EventType)
	for _, e := range events {
		byCategory[e.Category] = append(byCategory[e.Category], e)
	}

	// Sort categories
	var categories []string
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	// Write header
	fmt.Fprintf(file, "# Event Types\n\n")
	fmt.Fprintf(file, "> Auto-generated from `pkg/eventbus/types.go` on %s\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(file, "> Run `go run scripts/generate-event-docs.go` to regenerate\n\n")
	fmt.Fprintf(file, "Total: **%d event types**\n\n", len(events))

	// Table of contents
	fmt.Fprintf(file, "## Categories\n\n")
	for _, cat := range categories {
		fmt.Fprintf(file, "- [%s](#%s) (%d events)\n", strings.Title(cat), cat, len(byCategory[cat]))
	}
	fmt.Fprintf(file, "\n---\n\n")

	// Events by category
	for _, cat := range categories {
		catEvents := byCategory[cat]
		fmt.Fprintf(file, "## %s\n\n", strings.Title(cat))
		fmt.Fprintf(file, "| Constant | Event Type | Description |\n")
		fmt.Fprintf(file, "|----------|------------|-------------|\n")

		for _, e := range catEvents {
			comment := e.Comment
			if comment == "" {
				comment = "-"
			}
			fmt.Fprintf(file, "| `%s` | `%s` | %s |\n", e.Name, e.Value, comment)
		}
		fmt.Fprintf(file, "\n")
	}

	return nil
}
