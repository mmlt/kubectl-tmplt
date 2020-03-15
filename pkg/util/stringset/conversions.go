package stringset

// NewFromStringMap collects the keys of a [string]string and returns a Set of strings.
func NewFromStringMap(items map[string]string) Set {
	answer := New()
	for k := range items {
		answer.Add(k)
	}
	return answer
}
