package workflowproof

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const DirEnv = "WORKFLOW_PROOF_DIR"

// WriteJSON writes a machine-readable workflow artifact when WORKFLOW_PROOF_DIR
// is configured. Tests can call this unconditionally; it becomes a no-op when
// artifact capture is not requested.
func WriteJSON(t testing.TB, name string, payload any) {
	t.Helper()

	root := strings.TrimSpace(os.Getenv(DirEnv))
	if root == "" {
		return
	}

	name = sanitizeName(name)
	if name == "" {
		name = "workflow"
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create workflow proof dir: %v", err)
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal workflow proof payload: %v", err)
	}

	path := filepath.Join(root, name+".json")
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("write workflow proof artifact %s: %v", path, err)
	}
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	return result
}
