package utils

// UintSet is a memory-efficient set of uint values using empty structs.
type UintSet map[uint]struct{}

// NewUintSet creates a new UintSet with the given IDs.
func NewUintSet(ids ...uint) UintSet {
	set := make(UintSet, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// Add adds an ID to the set.
func (s UintSet) Add(id uint) {
	s[id] = struct{}{}
}

// Contains checks if the set contains the given ID.
func (s UintSet) Contains(id uint) bool {
	_, ok := s[id]
	return ok
}

// Remove removes an ID from the set.
func (s UintSet) Remove(id uint) {
	delete(s, id)
}

// Len returns the number of elements in the set.
func (s UintSet) Len() int {
	return len(s)
}

// ToSlice returns a slice of all IDs in the set.
func (s UintSet) ToSlice() []uint {
	ids := make([]uint, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	return ids
}
