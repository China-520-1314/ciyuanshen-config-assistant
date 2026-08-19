//go:build windows

package main

import (
	"context"
	"os/exec"
	"syscall"
)

// newWindowsShellCommand uses SysProcAttr.CmdLine because cmd.exe has a
// different quoting grammar from CommandLineToArgvW. Passing /C as a normal Go
// argument can strip the quotes around "Program Files" before cmd sees them.
func newWindowsShellCommand(ctx context.Context, line string) *exec.Cmd {
	command := exec.CommandContext(ctx, "cmd.exe", "/D", "/S", "/C", line)
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CmdLine:    `cmd.exe /D /S /C ` + line,
	}
	return command
}
