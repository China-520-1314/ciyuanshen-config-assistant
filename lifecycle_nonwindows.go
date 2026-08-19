//go:build !windows

package main

import (
	"context"
	"os/exec"
)

// The Windows-specific raw command-line setting is unavailable on Unix. This
// fallback keeps command construction inspectable in cross-platform tests.
func newWindowsShellCommand(ctx context.Context, line string) *exec.Cmd {
	return exec.CommandContext(ctx, "cmd.exe", "/D", "/S", "/C", line)
}
