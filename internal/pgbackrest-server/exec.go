package pgbackrestserver

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
)

type BackrestCommand struct {
	Command string
	Opts    []string
}

func (b *BackrestCommand) Run(ctx context.Context) (*bytes.Buffer, *bytes.Buffer, error) {
	args := append([]string{b.Command}, b.Opts...)
	cmd := exec.CommandContext(ctx, "pgbackrest", args...)

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}

	err := cmd.Run()
	if err != nil {
		return nil, nil, err
	}

	return stdout, stderr, nil
}
