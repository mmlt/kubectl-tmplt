package exe

import (
	"bytes"
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
func Run(log logr.Logger, options *Opt, stdin string, cmd string, args ...string) (stdout string, stderr string, err error) {
	log.V(2).Info("Run", "cmd", cmd, "args", args)

	c := exec.Command(cmd, args...)

	if options != nil {
		c.Env = options.Env
		c.Dir = options.Dir
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
	log.V(3).Info("Run-result", "stderr", stderr, "stdout", stdout)
	if err != nil {
		return "", "", fmt.Errorf("%s %v: %w - %s", cmd, args, err, stderr)
	}

	return
}
