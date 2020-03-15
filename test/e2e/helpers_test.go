// helper functions to create, delete files during testing.
package e2e_test

import (
	"github.com/otiai10/copy"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Testfiles holds the path of the temporary directory used to create test files in.
type testfiles string

// TestFilesNew creates a temporary directory for testing.
func testFilesNew() testfiles {
	p, err := ioutil.TempDir("", "testfiles")
	if err != nil {
		panic(err)
	}
	return testfiles(p)
}

// Path returns the absolute path of 'path' in the test directory.
func (tf testfiles) Path(path ...string) string {
	p := append([]string{string(tf)}, path...)
	return filepath.Join(p...)
}

// MustRemove removes the file at 'path' from the test directory.
func (tf testfiles) MustRemove(path string) {
	ap := tf.Path(path)
	err := os.Remove(ap)
	if err != nil {
		panic(err)
	}
}

// MustRemoveAll removes the test directory.
func (tf testfiles) MustRemoveAll() {
	err := os.RemoveAll(string(tf))
	if err != nil {
		panic(err)
	}
}

// MustCreate create a file at 'path' with content 'text' in the test directory.
func (tf testfiles) MustCreate(path, text string) {
	if path == "" {
		return
	}
	p := tf.Path(path)
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
func (tf testfiles) MustCopy(src, dst string) {
	err := copy.Copy(src, tf.Path(dst))

	if err != nil {
		panic(err)
	}
}
