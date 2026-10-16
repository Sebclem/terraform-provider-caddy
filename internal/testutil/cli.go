package testutil

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

type CLIResult struct {
	ExitCode       int
	Stdout, Stderr string
}

func RunCLI(ctx context.Context, bin, dir string, args ...string) (CLIResult, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	for _, s := range os.Environ() {
		if strings.HasPrefix(s, "TF_LOG") || strings.HasPrefix(s, "TF_CLI_ARGS") || strings.HasPrefix(s, "TF_REATTACH_PROVIDERS=") {
			continue
		}
		cmd.Env = append(cmd.Env, s)
	}
	cmd.Env = append(cmd.Env, "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1")
	err := cmd.Run()
	r := CLIResult{Stdout: out.String(), Stderr: errout.String()}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			r.ExitCode = exit.ExitCode()
			return r, nil
		}
		return r, err
	}
	return r, nil
}
