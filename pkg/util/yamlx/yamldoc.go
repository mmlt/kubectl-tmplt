package yamlx

import (
	"bufio"
	"bytes"
	yaml2 "gopkg.in/yaml.v2"
)

// IsEmpty returns true when yaml doesn't contain any context.
func IsEmpty(yaml []byte) bool {
	d := Values{}
	err := yaml2.Unmarshal(yaml, &d)

	return err == nil && len(d) == 0
}

// SplitDoc splits a yaml text at "---" boundaries.
// A doc is limited to 5MB.
func SplitDoc(yaml []byte) ([][]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(yaml))
	scanner.Buffer(make([]byte, 128*1024), 5*1024*1024)
	scanner.Split(splitYAMLDocument)
	var result [][]byte
	for scanner.Scan() {
		b := scanner.Bytes()
		b2 := make([]byte, len(b))
		copy(b2, b)
		result = append(result, b2)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// Code below is from https://github.com/kubernetes/apimachinery/pkg/util/yaml/decoder.go

const yamlSeparator = "\n---"
const separator = "---"

// splitYAMLDocument is a bufio.SplitFunc for splitting YAML streams into individual documents.
func splitYAMLDocument(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	sep := len([]byte(yamlSeparator))
	if i := bytes.Index(data, []byte(yamlSeparator)); i >= 0 {
		// We have a potential document terminator
		i += sep
		after := data[i:]
		if len(after) == 0 {
			// we can't read any more characters
			if atEOF {
				return len(data), data[:len(data)-sep], nil
			}
			return 0, nil, nil
		}
		if j := bytes.IndexByte(after, '\n'); j >= 0 {
			return i + j + 1, data[0 : i-sep], nil
		}
		return 0, nil, nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
