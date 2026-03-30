package extensionruntime

import (
	"encoding/json"
	"net/http"
	"testing"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	publicruntime "github.com/movebigrocks/platform/pkg/extensionsruntime"
)

func TestApplyRuntimeIdentityHeadersIncludesEffectiveExtensionConfig(t *testing.T) {
	defaultConfig := shareddomain.NewTypedCustomFields()
	defaultConfig.SetString("mode", "b2b")
	defaultConfig.SetBool("showTotals", true)

	installedConfig := shareddomain.NewTypedCustomFields()
	installedConfig.SetString("mode", "agency")

	extension := &platformdomain.InstalledExtension{
		ID:          "ext_123",
		WorkspaceID: "ws_123",
		Slug:        "sales-pipeline",
		Manifest: platformdomain.ExtensionManifest{
			Publisher:     "demandops",
			Slug:          "sales-pipeline",
			DefaultConfig: defaultConfig,
		},
		Config: installedConfig,
	}

	headers := http.Header{}
	applyRuntimeIdentityHeaders(headers, extension)

	if got := headers.Get(publicruntime.HeaderExtensionID); got != "ext_123" {
		t.Fatalf("expected extension id header, got %q", got)
	}
	if got := headers.Get(publicruntime.HeaderWorkspaceID); got != "ws_123" {
		t.Fatalf("expected workspace header, got %q", got)
	}

	var config map[string]any
	if err := json.Unmarshal([]byte(headers.Get(publicruntime.HeaderExtensionConfigJSON)), &config); err != nil {
		t.Fatalf("decode config header: %v", err)
	}
	if got := config["mode"]; got != "agency" {
		t.Fatalf("expected installed config override, got %#v", got)
	}
	if got := config["showTotals"]; got != true {
		t.Fatalf("expected default config to remain present, got %#v", got)
	}
}
