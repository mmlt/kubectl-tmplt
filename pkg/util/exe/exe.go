package exe

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	"os/exec"
)

// Opt are the exec options, see https://godoc.org/os/exec#Cmd for details.
type Opt struct {
	// Path is the working directory.
	Dir string
	// Env is the execution environment.
	Env []string
}

// Run executes 'cmd' with 'stdin', 'args' and 'options'.
// Upon completion it returns stdout and stderr.
// Ctx is optional.
func Run(ctx context.Context, log logr.Logger, options *Opt, stdin string, cmd string, args ...string) (stdout string, stderr string, err error) {
	log.V(2).Info("Run", "cmd", cmd, "args", args)

	var c *exec.Cmd
	if ctx != nil {
		c = exec.CommandContext(ctx, cmd, args...)
	} else {
		c = exec.Command(cmd, args...)
	}

	if options != nil {
		if options.Env != nil {
			c.Env = options.Env
		}
		if options.Dir != "" {
			c.Dir = options.Dir
		}
	}

	if stdin != "" {
		sin, err := c.StdinPipe()
		if err != nil {
			log.Error(err, "should not happen")
			return "", "", err
		}

		go func() {
			defer sin.Close()
			io.WriteString(sin, stdin)
		}()
	}

	var sout, serr bytes.Buffer
	c.Stdout, c.Stderr = &sout, &serr
	err = c.Run()
	stdout, stderr = string(sout.Bytes()), string(serr.Bytes())
	if err != nil && err.Error() != "signal: killed" {
		// Do not consider 'signal: killed' an error as the log line might cause the user to think something went wrong.
		// Signal kill is the result of port-forward being stopped by context Cancel().
		log.V(3).Info("Run-result", "error", nil, "stderr", stderr, "stdout", stdout)
		return "", "", fmt.Errorf("%s %v: %w - %s", cmd, args, err, stderr)
	}
	log.V(3).Info("Run-result", "error", err, "stderr", stderr, "stdout", stdout)

	return
}
