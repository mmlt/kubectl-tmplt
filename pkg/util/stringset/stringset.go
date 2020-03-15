// Package stringset implements Set operations on strings.
package stringset

// Set provides operations like union, intersect, difference on Sets with strings.
type Set map[string]struct{}

// New creates a set with a elements.
func New(a ...string) Set {
	r := make(Set)
	for _, i := range a {
		r.Add(i)
	}
	return r
}

// ToSlice returns the elements of the receiver as a slice.
func (set Set) ToSlice() []string {
	var r []string
	for v := range set {
		r = append(r, v)
	}
	return r
}

// Add adds s to the receiver.
// Returns false if s is already in the receiver.
func (set Set) Add(s string) bool {
	_, found := set[s]
	set[s] = struct{}{}
	return !found //
}

// Contains returns true if s is in the receiver.
func (set Set) Contains(s string) bool {
	_, found := set[s]
	return found
}

// ContainsAll returns true if all s's are in the receiver.
func (set Set) ContainsAll(s ...string) bool {
	for _, v := range s {
		if !set.Contains(v) {
			return false
		}
	}
	return true
}

// IsSubset returns true if all items of other are in the receiver.
func (set Set) IsSubset(other Set) bool {
	for elem := range other {
		if !set.Contains(elem) {
			return false
		}
	}
	return true
}

// Union returns a new set with items of the receiver and items of other.
func (set Set) Union(other Set) Set {
	r := New()

	for elem := range set {
		r.Add(elem)
	}
	for elem := range other {
		r.Add(elem)
	}
	return r
}

// Intersect returns a new set with items that exist only in both sets.
func (set Set) Intersect(other Set) Set {
	r := New()
	// loop over smaller set
	if set.Cardinality() < other.Cardinality() {
		for elem := range set {
			if other.Contains(elem) {
				r.Add(elem)
			}
		}
	} else {
		for elem := range other {
			if set.Contains(elem) {
				r.Add(elem)
			}
		}
	}
	return r
}

// Difference returns a set with items in the receiver that are not in other.
func (set Set) Difference(other Set) Set {
	r := New()
	for elem := range set {
		if !other.Contains(elem) {
			r.Add(elem)
		}
	}
	return r
}

// SymmetricDifference returns a new set with items of the receiver or other that are not in both sets.
func (set Set) SymmetricDifference(other Set) Set {
	aDiff := set.Difference(other)
	bDiff := other.Difference(set)
	return aDiff.Union(bDiff)
}

// Remove allows the removal of a single item in the set.
func (set Set) Remove(i string) {
	delete(set, i)
}

// Cardinality returns how many items are currently in the set.
func (set Set) Cardinality() int {
	return len(set)
}

// Equal determines if two sets are equal to each other.
// If they both are the same size and have the same items they are considered equal.
func (set Set) Equal(other Set) bool {
	if set.Cardinality() != other.Cardinality() {
		return false
	}
	for elem := range set {
		if !other.Contains(elem) {
			return false
		}
	}
	return true
}

// Clone returns a clone of the set.
// Does NOT clone the underlying elements.
func (set Set) Clone() Set {
	clonedSet := New()
	for elem := range set {
		clonedSet.Add(elem)
	}
	return clonedSet
}
