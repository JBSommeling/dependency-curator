package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

type DefaultRunner struct{}

func NewDefaultRunner() *DefaultRunner {
	return &DefaultRunner{}
}

func (r *DefaultRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// npm commands return exit code 1 for outdated/audit, which is not an error
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return stdout.Bytes(), nil
			}
		}
		return nil, fmt.Errorf("command %s failed: %w\nstderr: %s", name, err, stderr.String())
	}

	return stdout.Bytes(), nil
}
