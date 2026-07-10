// Package fenwick implements a concurrency-safe Fenwick tree (binary indexed tree)
// for point updates and prefix/range sum queries.
package fenwick

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrIndexOutOfRange indicates that an index is outside [0, Len()).
	ErrIndexOutOfRange = errors.New("fenwick: index out of range")
	// ErrInvalidRange indicates that a range is malformed or outside [0, Len()).
	ErrInvalidRange = errors.New("fenwick: invalid range")
)

// Tree stores signed 64-bit values.
//
// Public indexes are zero-based. Internally, the Fenwick representation is
// one-based. All exported methods are safe for concurrent use.
//
// Arithmetic uses int64 and does not detect overflow.
type Tree struct {
	mu   sync.RWMutex
	tree []int64 // one-based Fenwick storage; tree[0] is unused
	vals []int64 // zero-based values, retained to support Set and At in O(1)
}

// New constructs a Tree from values in O(n) time. The input slice is copied,
// so later changes to values do not affect the Tree.
func New(values []int64) *Tree {
	t := &Tree{
		tree: make([]int64, len(values)+1),
		vals: append([]int64(nil), values...),
	}

	// Linear-time Fenwick construction. Each node starts with its own value,
	// then contributes its completed block sum to its parent.
	for i, value := range values {
		internal := i + 1
		t.tree[internal] += value

		parent := internal + lowbit(internal)
		if parent < len(t.tree) {
			t.tree[parent] += t.tree[internal]
		}
	}

	return t
}

// Len returns the number of stored values.
func (t *Tree) Len() int {
	if t == nil {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.vals)
}

// At returns the value at index.
func (t *Tree) At(index int) (int64, error) {
	if t == nil {
		return 0, indexError(index, 0)
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if err := validateIndex(index, len(t.vals)); err != nil {
		return 0, err
	}
	return t.vals[index], nil
}

// Add increments the value at index by delta in O(log n).
func (t *Tree) Add(index int, delta int64) error {
	if t == nil {
		return indexError(index, 0)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := validateIndex(index, len(t.vals)); err != nil {
		return err
	}

	t.vals[index] += delta
	t.addLocked(index+1, delta)
	return nil
}

// Set replaces the value at index in O(log n).
func (t *Tree) Set(index int, value int64) error {
	if t == nil {
		return indexError(index, 0)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := validateIndex(index, len(t.vals)); err != nil {
		return err
	}

	delta := value - t.vals[index]
	if delta == 0 {
		return nil
	}

	t.vals[index] = value
	t.addLocked(index+1, delta)
	return nil
}

// PrefixSum returns the inclusive sum values[0:index+1] in O(log n).
func (t *Tree) PrefixSum(index int) (int64, error) {
	if t == nil {
		return 0, indexError(index, 0)
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if err := validateIndex(index, len(t.vals)); err != nil {
		return 0, err
	}
	return t.prefixSumLocked(index + 1), nil
}

// RangeSum returns the inclusive sum values[left:right+1] in O(log n).
func (t *Tree) RangeSum(left, right int) (int64, error) {
	if t == nil {
		return 0, rangeError(left, right, 0)
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if err := validateRange(left, right, len(t.vals)); err != nil {
		return 0, err
	}

	rightSum := t.prefixSumLocked(right + 1)
	leftSum := t.prefixSumLocked(left)
	return rightSum - leftSum, nil
}

// Total returns the sum of all values. For an empty Tree, Total returns 0.
func (t *Tree) Total() int64 {
	if t == nil {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.prefixSumLocked(len(t.vals))
}

// Values returns a copy of the current values.
func (t *Tree) Values() []int64 {
	if t == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]int64(nil), t.vals...)
}

func (t *Tree) addLocked(internalIndex int, delta int64) {
	for i := internalIndex; i < len(t.tree); i += lowbit(i) {
		t.tree[i] += delta
	}
}

func (t *Tree) prefixSumLocked(internalIndex int) int64 {
	var sum int64
	for i := internalIndex; i > 0; i -= lowbit(i) {
		sum += t.tree[i]
	}
	return sum
}

func lowbit(i int) int {
	return i & -i
}

func validateIndex(index, length int) error {
	if index < 0 || index >= length {
		return indexError(index, length)
	}
	return nil
}

func validateRange(left, right, length int) error {
	if left < 0 || right < left || right >= length {
		return rangeError(left, right, length)
	}
	return nil
}

func indexError(index, length int) error {
	return fmt.Errorf("%w: index=%d length=%d", ErrIndexOutOfRange, index, length)
}

func rangeError(left, right, length int) error {
	return fmt.Errorf("%w: left=%d right=%d length=%d", ErrInvalidRange, left, right, length)
}
