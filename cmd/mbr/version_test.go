package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestVersionReportsEmbeddedBuildProvenance(t *testing.T) {
	originalVersion, originalCommit, originalDate := cliVersion, cliGitCommit, cliBuildDate
	t.Cleanup(func() {
		cliVersion, cliGitCommit, cliBuildDate = originalVersion, originalCommit, originalDate
	})
	cliVersion = "v1.2.3"
	cliGitCommit = "abc123"
	cliBuildDate = "2026-07-16T08:00:00Z"

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"version", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run version code = %d, stderr = %s", code, stderr.String())
	}

	var got versionOutput
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got.Version != cliVersion || got.GitCommit != cliGitCommit || got.BuildDate != cliBuildDate {
		t.Fatalf("version output = %#v", got)
	}
}

func TestRootVersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run --version code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("expected version output")
	}
}
