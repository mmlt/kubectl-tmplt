package yamlx

// Values represent a YAML object.
type Values map[string]interface{}

// Merge overrides values into base and return the new values.
// No argument values are modified.
// Value precedence is from left (lowest) to right (highest)
func Merge(base Values, overrides ...Values) Values {
	result := deepCopy(base)
	for _, v := range overrides {
		merge(result, v)
	}
	return result
}

// Merge merges src values into dst values.
func merge(dst, src Values) {
	for key, sv := range src {
		dv, found := dst[key]

		sm, sIsMap := sv.(Values)
		dm, dIsMap := dv.(Values)
		if found && sIsMap && dIsMap {
			merge(dm, sm)
		} else {
			dst[key] = sv
		}
	}
}

// DeepCopy Values.
func deepCopy(mp Values) Values {
	c := make(Values)
	for k, v := range mp {
		vm, ok := v.(Values)
		if ok {
			c[k] = deepCopy(vm)
		} else {
			c[k] = v
		}
	}

	return c
}
