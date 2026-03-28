package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/container"
	"github.com/movebigrocks/platform/internal/platform/extensionbundle"
	"github.com/movebigrocks/platform/internal/platform/extensiondesiredstate"
	"github.com/movebigrocks/platform/internal/platform/extensionreconcile"
	"github.com/movebigrocks/platform/internal/platform/extensionruntime"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "render-runtime-manifest":
		fs := flag.NewFlagSet("reconcile-extensions render-runtime-manifest", flag.ContinueOnError)
		fs.SetOutput(stderr)
		desiredStatePath := fs.String("desired-state", "", "Path to extensions/desired-state.yaml")
		outputPath := fs.String("output", "-", "Path to write JSON output, or - for stdout")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*desiredStatePath) == "" {
			fmt.Fprintln(stderr, "--desired-state is required")
			return 2
		}
		doc, err := extensiondesiredstate.LoadFile(*desiredStatePath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		manifest, err := extensionreconcile.BuildRuntimeManifest(ctx, doc, extensionreconcile.DefaultBundleLoader{
			Config: extensionbundle.DefaultResolverConfigFromEnv(),
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := writeJSON(*outputPath, manifest, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "plan", "apply", "check":
		fs := flag.NewFlagSet("reconcile-extensions "+args[0], flag.ContinueOnError)
		fs.SetOutput(stderr)
		desiredStatePath := fs.String("desired-state", "", "Path to extensions/desired-state.yaml")
		outputPath := fs.String("output", "-", "Path to write JSON output, or - for stdout")
		runtimeManifestPath := fs.String("runtime-manifest-out", "", "Optional path to write generated runtime manifest JSON")
		actor := fs.String("actor", "system:reconcile-extensions", "Actor identity recorded in reconciliation artifacts")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if strings.TrimSpace(*desiredStatePath) == "" {
			fmt.Fprintln(stderr, "--desired-state is required")
			return 2
		}

		doc, err := extensiondesiredstate.LoadFile(*desiredStatePath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}

		app, err := openApp()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer app.Close()

		engine := app.Engine(strings.TrimSpace(*actor))

		switch args[0] {
		case "plan":
			plan, err := engine.Plan(ctx, doc, *desiredStatePath)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if err := maybeWriteRuntimeManifest(*runtimeManifestPath, plan.RuntimeManifest, stdout); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if err := writeJSON(*outputPath, plan, stdout); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		case "apply":
			result, err := engine.Apply(ctx, doc, *desiredStatePath)
			if err := maybeWriteRuntimeManifest(*runtimeManifestPath, result.RuntimeManifest, stdout); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if writeErr := writeJSON(*outputPath, result, stdout); writeErr != nil {
				fmt.Fprintln(stderr, writeErr)
				return 1
			}
			if err != nil || !result.Clean {
				if err != nil {
					fmt.Fprintln(stderr, err)
				}
				return 1
			}
			return 0
		default:
			result, err := engine.Check(ctx, doc, *desiredStatePath)
			if err := maybeWriteRuntimeManifest(*runtimeManifestPath, result.RuntimeManifest, stdout); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if writeErr := writeJSON(*outputPath, result, stdout); writeErr != nil {
				fmt.Fprintln(stderr, writeErr)
				return 1
			}
			if err != nil || !result.Clean {
				if err != nil {
					fmt.Fprintln(stderr, err)
				}
				return 1
			}
			return 0
		}
	default:
		printUsage(stderr)
		return 2
	}
}

type app struct {
	container      *container.Container
	serviceTargets *extensionruntime.Registry
	runtime        *extensionruntime.Runtime
}

func openApp() (*app, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	c, err := container.New(cfg, container.Options{
		Version:   "reconcile-extensions",
		GitCommit: "unknown",
		BuildDate: "unknown",
	})
	if err != nil {
		return nil, fmt.Errorf("initialize container: %w", err)
	}

	serviceTargets := extensionruntime.NewRegistry(c)
	runtime := extensionruntime.NewRuntime(
		serviceTargets,
		extensionruntime.WithBackgroundRuntimeDeps(c.EventBus, c.Store.Extensions(), c.Store.Workspaces(), c.Logger),
	)
	c.Platform.Extension.SetActivationRuntime(runtime)
	c.Platform.Extension.SetHealthRuntime(runtime)
	c.Platform.Extension.SetDiagnosticsRuntime(runtime)
	c.Platform.Extension.SetPrivilegedRuntime(runtime)

	return &app{
		container:      c,
		serviceTargets: serviceTargets,
		runtime:        runtime,
	}, nil
}

func (a *app) Close() {
	if a == nil {
		return
	}
	if a.runtime != nil {
		a.runtime.Stop()
	}
	if a.serviceTargets != nil {
		_ = a.serviceTargets.Close()
	}
	if a.container != nil && a.container.Store != nil {
		_ = a.container.Store.Close()
	}
}

func (a *app) Engine(actor string) *extensionreconcile.Engine {
	engine := extensionreconcile.NewEngine(
		extensionreconcile.DefaultBundleLoader{Config: extensionbundle.DefaultResolverConfigFromEnv()},
		a.container.Store.Extensions(),
		a.container.Store.Workspaces(),
		a.container.Platform.Extension,
	)
	engine.Actor = actor
	return engine
}

func writeJSON(path string, value any, stdout io.Writer) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	data = append(data, '\n')
	if strings.TrimSpace(path) == "" || strings.TrimSpace(path) == "-" {
		_, err = stdout.Write(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(filepath.Clean(path), data, 0o600); err != nil {
		return fmt.Errorf("write json output: %w", err)
	}
	return nil
}

func maybeWriteRuntimeManifest(path string, manifest extensionreconcile.RuntimeManifest, stdout io.Writer) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return writeJSON(path, manifest, stdout)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  reconcile-extensions render-runtime-manifest --desired-state <path> [--output <path|->]")
	fmt.Fprintln(w, "  reconcile-extensions plan --desired-state <path> [--output <path|->] [--runtime-manifest-out <path>] [--actor <id>]")
	fmt.Fprintln(w, "  reconcile-extensions apply --desired-state <path> [--output <path|->] [--runtime-manifest-out <path>] [--actor <id>]")
	fmt.Fprintln(w, "  reconcile-extensions check --desired-state <path> [--output <path|->] [--runtime-manifest-out <path>] [--actor <id>]")
}
