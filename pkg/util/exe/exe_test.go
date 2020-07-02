package exe

import (
	"context"
	"github.com/go-logr/stdr"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRun(t *testing.T) {
	var tests = map[string]struct {
		options    *Opt
		cmd        string
		args       []string
		in         string
		wantErr    string
		wantStdout string
		wantStderr string
	}{
		"simple_args": {
			cmd:        "echo",
			args:       []string{"-n", "hello world"},
			wantStdout: "hello world",
		},
		"want_an_error": {
			cmd:     "ls",
			args:    []string{"nonexisting"},
			wantErr: "ls [nonexisting]: exit status 2 - ls: cannot access 'nonexisting': No such file or directory\n",
		},
		"use_stdin": {
			cmd:        "base64",
			args:       []string{"-d"},
			in:         "aGVsbG8gd29ybGQ=",
			wantStdout: "hello world",
		},
		"environment": {
			options: &Opt{
				Env: []string{"SONG=HappyHappyJoyJoy"},
			},
			cmd:        "env",
			wantStdout: "SONG=HappyHappyJoyJoy\n",
		},
		"pwd": {
			options: &Opt{
				Dir: "/tmp",
			},
			cmd:        "pwd",
			wantStdout: "/tmp\n",
		},
	}

	log := stdr.New(nil)

	for name, tst := range tests {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := Run(context.Background(), log, tst.options, tst.in, tst.cmd, tst.args...)
			if tst.wantErr != "" {
				assert.EqualError(t, err, tst.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tst.wantStdout, stdout)
			assert.Equal(t, tst.wantStderr, stderr)
		})
	}
}
