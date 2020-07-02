// helper functions to create, delete files during testing.
package e2e_test

import (
	"github.com/otiai10/copy"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Testdir is the path to a temporary directory used to create test files in.
type testdir string

// MustCreateTestDir creates a temporary directory for testing.
func MustCreateTestDir(pattern string) testdir {
	p, err := ioutil.TempDir("", pattern)
	if err != nil {
		panic(err)
	}
	return testdir(p)
}

// Path returns the absolute path of 'path' in the test directory.
func (td testdir) Path(path ...string) string {
	p := append([]string{string(td)}, path...)
	return filepath.Join(p...)
}

// MustRemove removes the file at 'path' from the test directory.
func (td testdir) MustRemove(path string) {
	ap := td.Path(path)
	err := os.Remove(ap)
	if err != nil {
		panic(err)
	}
}

// MustRemoveAll removes the test directory.
func (td testdir) MustRemoveAll() {
	err := os.RemoveAll(string(td))
	if err != nil {
		panic(err)
	}
}

// MustCreate create a file at 'path' with content 'text' in the test directory.
func (td testdir) MustCreate(path, text string) {
	if path == "" {
		return
	}
	p := td.Path(path)
	d := filepath.Dir(p)
	err := os.MkdirAll(d, 0700)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(p, []byte(text), 0600)
	if err != nil {
		panic(err)
	}
}

// MustCopy recursive copy of src files to dst in test directory.
func (td testdir) MustCopy(src, dst string) {
	err := copy.Copy(src, td.Path(dst))

	if err != nil {
		panic(err)
	}
}
