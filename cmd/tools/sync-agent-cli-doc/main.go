package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/movebigrocks/platform/internal/clispec"
)

const (
	commandSurfaceStart = "<!-- BEGIN GENERATED CLI COMMAND SURFACE -->"
	commandSurfaceEnd   = "<!-- END GENERATED CLI COMMAND SURFACE -->"
	authMatrixStart     = "<!-- BEGIN GENERATED CLI AUTH MATRIX -->"
	authMatrixEnd       = "<!-- END GENERATED CLI AUTH MATRIX -->"
)

func main() {
	check := flag.Bool("check", false, "Verify that generated sections are up to date")
	file := flag.String("file", filepath.Join("docs", "AGENT_CLI.md"), "Path to the agent CLI markdown file")
	flag.Parse()

	path := filepath.Clean(*file)
	original, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	updated, err := syncAgentCLIDoc(string(original))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync %s: %v\n", path, err)
		os.Exit(1)
	}

	if *check {
		if updated != string(original) {
			fmt.Fprintf(os.Stderr, "%s is out of date; run go run ./cmd/tools/sync-agent-cli-doc\n", path)
			os.Exit(1)
		}
		fmt.Printf("%s is up to date\n", path)
		return
	}

	if updated == string(original) {
		fmt.Printf("%s already up to date\n", path)
		return
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("updated %s\n", path)
}

func syncAgentCLIDoc(content string) (string, error) {
	commandSurface := renderCommandSurface()
	authMatrix := renderAuthMatrix()

	updated, err := replaceGeneratedSection(content, commandSurfaceStart, commandSurfaceEnd, commandSurface)
	if err != nil {
		return "", err
	}
	updated, err = replaceGeneratedSection(updated, authMatrixStart, authMatrixEnd, authMatrix)
	if err != nil {
		return "", err
	}
	return updated, nil
}

func replaceGeneratedSection(content, startMarker, endMarker, replacement string) (string, error) {
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("missing section markers %q / %q", startMarker, endMarker)
	}

	var output strings.Builder
	output.WriteString(content[:start+len(startMarker)])
	output.WriteString("\n")
	output.WriteString(replacement)
	if !strings.HasSuffix(replacement, "\n") {
		output.WriteString("\n")
	}
	output.WriteString(content[end:])
	return output.String(), nil
}

func renderCommandSurface() string {
	var buf bytes.Buffer
	buf.WriteString("> Generated from `internal/clispec`. Run `go run ./cmd/tools/sync-agent-cli-doc` to regenerate.\n\n")
	buf.WriteString("```text\n")
	for _, command := range clispec.Commands() {
		buf.WriteString(command.Usage)
		buf.WriteByte('\n')
	}
	buf.WriteString("```\n")
	return buf.String()
}

func renderAuthMatrix() string {
	var buf bytes.Buffer
	buf.WriteString("> Generated from `internal/clispec`. Run `go run ./cmd/tools/sync-agent-cli-doc` to regenerate.\n\n")
	buf.WriteString("| Command | Auth | JSON | Operation | Idempotency |\n")
	buf.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, command := range clispec.Commands() {
		fmt.Fprintf(&buf, "| `%s` | %s | %s | %s | %s |\n",
			strings.Join(command.Path, " "),
			clispec.AuthModeLabel(command.AuthMode),
			boolLabel(command.SupportsJSON),
			string(command.Operation),
			string(command.Idempotency),
		)
	}
	return buf.String()
}

func boolLabel(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
