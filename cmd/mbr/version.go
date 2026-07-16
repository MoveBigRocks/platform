package main

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// These values are replaced by scripts/build-cli-release.sh using Go
// linker flags. Development builds remain explicit instead of pretending to
// be a published release.
var (
	cliVersion   = "dev"
	cliGitCommit = "unknown"
	cliBuildDate = "unknown"
)

type versionOutput struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
}

func runVersion(_ context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mbr version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "Emit JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected arguments")
		return 2
	}

	output := versionOutput{
		Version:   cliVersion,
		GitCommit: cliGitCommit,
		BuildDate: cliBuildDate,
	}
	if *jsonOutput {
		return writeJSON(stdout, output, stderr)
	}

	fmt.Fprintf(stdout, "mbr %s\ncommit %s\nbuilt %s\n", output.Version, output.GitCommit, output.BuildDate)
	return 0
}
