package cliapi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	envDisableCredentialStore = "MBR_DISABLE_KEYCHAIN"
	credentialServiceName     = "com.movebigrocks.cli"
	credentialStoreTimeout    = 5 * time.Second
)

type credentialStore interface {
	Name() string
	Save(account, secret string) error
	Load(account string) (string, error)
	Delete(account string) error
}

var newCredentialStore = defaultCredentialStore

func defaultCredentialStore() credentialStore {
	if strings.TrimSpace(os.Getenv(envDisableCredentialStore)) == "1" {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("security"); err == nil {
			return macOSCredentialStore{}
		}
	case "linux":
		if _, err := exec.LookPath("secret-tool"); err == nil {
			return linuxSecretToolCredentialStore{}
		}
	}
	return nil
}

func credentialKey(authMode AuthMode, instanceURL string) string {
	return fmt.Sprintf("%s:%s", authMode, strings.TrimSpace(instanceURL))
}

type macOSCredentialStore struct{}

func (macOSCredentialStore) Name() string { return "macos-keychain" }

func (macOSCredentialStore) Save(account, secret string) error {
	_, _, err := runCredentialCommand("security", "add-generic-password", "-U", "-s", credentialServiceName, "-a", account, "-w", secret)
	if err != nil {
		return fmt.Errorf("save macOS keychain credential: %w", err)
	}
	return nil
}

func (macOSCredentialStore) Load(account string) (string, error) {
	stdout, _, err := runCredentialCommand("security", "find-generic-password", "-s", credentialServiceName, "-a", account, "-w")
	if err != nil {
		return "", fmt.Errorf("load macOS keychain credential: %w", err)
	}
	return strings.TrimSpace(stdout), nil
}

func (macOSCredentialStore) Delete(account string) error {
	_, _, err := runCredentialCommand("security", "delete-generic-password", "-s", credentialServiceName, "-a", account)
	if err != nil && !strings.Contains(err.Error(), "could not be found") {
		return fmt.Errorf("delete macOS keychain credential: %w", err)
	}
	return nil
}

type linuxSecretToolCredentialStore struct{}

func (linuxSecretToolCredentialStore) Name() string { return "linux-secret-tool" }

func (linuxSecretToolCredentialStore) Save(account, secret string) error {
	_, _, err := runCredentialCommandWithInput(secret, "secret-tool", "store", "--label=Move Big Rocks CLI", "service", credentialServiceName, "account", account)
	if err != nil {
		return fmt.Errorf("save secret-tool credential: %w", err)
	}
	return nil
}

func (linuxSecretToolCredentialStore) Load(account string) (string, error) {
	stdout, _, err := runCredentialCommand("secret-tool", "lookup", "service", credentialServiceName, "account", account)
	if err != nil {
		return "", fmt.Errorf("load secret-tool credential: %w", err)
	}
	return strings.TrimSpace(stdout), nil
}

func (linuxSecretToolCredentialStore) Delete(account string) error {
	_, _, err := runCredentialCommand("secret-tool", "clear", "service", credentialServiceName, "account", account)
	if err != nil {
		return fmt.Errorf("delete secret-tool credential: %w", err)
	}
	return nil
}

func runCredentialCommand(name string, args ...string) (string, string, error) {
	return runCredentialCommandWithInput("", name, args...)
}

func runCredentialCommandWithInput(stdin, name string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), credentialStoreTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("%s timed out", name)
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return stdout.String(), stderr.String(), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
	}
	return stdout.String(), stderr.String(), nil
}
