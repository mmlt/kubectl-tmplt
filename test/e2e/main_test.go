package e2e

import (
	"flag"
	"k8s.io/klog"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup
	defer klog.Flush()
	klog.InitFlags(nil)
	flag.Set("v", "5")
	flag.Set("alsologtostderr", "true")

	// Run.
	exitVal := m.Run()

	// Teardown.

	os.Exit(exitVal)
}
