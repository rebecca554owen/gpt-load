package utils

// UintSet 是使用空结构的高效 uint 值集合。
type UintSet map[uint]struct{}

// NewUintSet 创建带有给定 ID 的新 UintSet。
func NewUintSet(ids ...uint) UintSet {
	set := make(UintSet, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// Add 将 ID 添加到集合。
func (s UintSet) Add(id uint) {
	s[id] = struct{}{}
}

// Contains 检查集合是否包含给定 ID。
func (s UintSet) Contains(id uint) bool {
	_, ok := s[id]
	return ok
}

// Remove 从集合中移除 ID。
func (s UintSet) Remove(id uint) {
	delete(s, id)
}

// Len 返回集合中的元素数量。
func (s UintSet) Len() int {
	return len(s)
}

// ToSlice 返回集合中所有 ID 的切片。
func (s UintSet) ToSlice() []uint {
	ids := make([]uint, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	return ids
}
