//go:build !linux

package supervisor

import "os/exec"

// applyPdeathsig is a no-op on non-Linux. macOS and others lack parent-death
// signaling, so the supervisor relies solely on its explicit Stop path for
// child lifecycle on those hosts (relevant mostly for local development and
// tests).
func applyPdeathsig(cmd *exec.Cmd) {}
