// Package fenwick implements concurrency-safe Fenwick trees (binary indexed
// trees) for point updates and prefix/range sum queries.
package fenwick

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrIndexOutOfRange = errors.New("fenwick: index out of range")
	ErrInvalidRange    = errors.New("fenwick: invalid range")
)

// Number is the set of supported value types.
//
// Unsigned integers are intentionally excluded: Set may need to apply a
// negative delta when a value decreases, which cannot be represented safely by
// an unsigned type.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~float32 | ~float64
}

// Tree stores numeric values. Public indexes are zero-based; the internal
// Fenwick representation is one-based. All exported methods are safe for
// concurrent use.
type Tree[T Number] struct {
	mu   sync.RWMutex
	tree []T
	vals []T
}

// New constructs a Tree in O(n). The input slice is copied.
func New[T Number](values []T) *Tree[T] {
	t := &Tree[T]{
		tree: make([]T, len(values)+1),
		vals: append([]T(nil), values...),
	}
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

func (t *Tree[T]) Len() int {
	if t == nil {
		return 0
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.vals)
}

func (t *Tree[T]) At(index int) (T, error) {
	var zero T
	if t == nil {
		return zero, indexError(index, 0)
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := validateIndex(index, len(t.vals)); err != nil {
		return zero, err
	}
	return t.vals[index], nil
}

func (t *Tree[T]) Add(index int, delta T) error {
	if t == nil {
		return indexError(index, 0)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := validateIndex(index, len(t.vals)); err != nil {
		return err
	}
	if delta == 0 {
		return nil
	}
	t.vals[index] += delta
	t.addLocked(index+1, delta)
	return nil
}

func (t *Tree[T]) Set(index int, value T) error {
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

func (t *Tree[T]) PrefixSum(index int) (T, error) {
	var zero T
	if t == nil {
		return zero, indexError(index, 0)
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := validateIndex(index, len(t.vals)); err != nil {
		return zero, err
	}
	return t.prefixSumLocked(index + 1), nil
}

func (t *Tree[T]) RangeSum(left, right int) (T, error) {
	var zero T
	if t == nil {
		return zero, rangeError(left, right, 0)
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if err := validateRange(left, right, len(t.vals)); err != nil {
		return zero, err
	}
	return t.prefixSumLocked(right+1) - t.prefixSumLocked(left), nil
}

func (t *Tree[T]) Total() T {
	var zero T
	if t == nil {
		return zero
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.prefixSumLocked(len(t.vals))
}

func (t *Tree[T]) Values() []T {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]T(nil), t.vals...)
}

func (t *Tree[T]) addLocked(i int, delta T) {
	for ; i < len(t.tree); i += lowbit(i) {
		t.tree[i] += delta
	}
}

func (t *Tree[T]) prefixSumLocked(i int) T {
	var sum T
	for ; i > 0; i -= lowbit(i) {
		sum += t.tree[i]
	}
	return sum
}

func lowbit(i int) int { return i & -i }

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
