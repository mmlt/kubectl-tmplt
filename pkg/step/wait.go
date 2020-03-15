package step

// Wait is a step that waits for a condition C.
type Wait struct {
	C string `yaml:"wait"`
}
