// Package fenwick implements concurrency-safe Fenwick trees (binary indexed
// trees) for point updates and prefix/range aggregation queries.
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

// Tree stores values aggregated by injected Operations.
// Public indexes are zero-based; internal Fenwick indexes are one-based.
type Tree[T any] struct {
	mu   sync.RWMutex
	ops  Operations[T]
	tree []T
	vals []T
}

// New constructs a tree for models implementing Value.
func New[T Value](values []T) *Tree[T] {
	var zero T
	if len(values) > 0 {
		zero = values[0].Zero().(T)
	}
	return NewWithOperations(values, valueOperations[T]{zero: zero})
}

// NewNumeric constructs a tree for signed integers or floating-point values.
func NewNumeric[T Number](values []T) *Tree[T] {
	return NewWithOperations(values, NumericOperations[T]{})
}

// NewWithOperations constructs a tree using caller-provided aggregation
// behavior. The input slice is copied and construction takes O(n).
func NewWithOperations[T any](values []T, ops Operations[T]) *Tree[T] {
	if ops == nil {
		panic("fenwick: operations must not be nil")
	}

	t := &Tree[T]{
		ops:  ops,
		tree: make([]T, len(values)+1),
		vals: append([]T(nil), values...),
	}
	zero := ops.Zero()
	for i := range t.tree {
		t.tree[i] = zero
	}
	for i, value := range values {
		internal := i + 1
		t.tree[internal] = ops.Add(t.tree[internal], value)
		parent := internal + lowbit(internal)
		if parent < len(t.tree) {
			t.tree[parent] = ops.Add(t.tree[parent], t.tree[internal])
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
	t.vals[index] = t.ops.Add(t.vals[index], delta)
	t.addLocked(index+1, delta)
	return nil
}

// Apply atomically applies a batch of point mutations in slice order.
//
// All mutations are validated before any value is changed. The tree lock is
// acquired once, so readers and writers observe either the state before the
// whole batch or the state after it. An empty batch is a no-op.
func (t *Tree[T]) Apply(mutations ...Mutation[T]) error {
	if len(mutations) == 0 {
		return nil
	}
	if t == nil {
		return indexError(mutations[0].Index, 0)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, mutation := range mutations {
		if err := validateIndex(mutation.Index, len(t.vals)); err != nil {
			return err
		}
		if err := validateMutationKind(mutation.Kind); err != nil {
			return err
		}
	}

	for _, mutation := range mutations {
		switch mutation.Kind {
		case MutationAdd:
			t.vals[mutation.Index] = t.ops.Add(t.vals[mutation.Index], mutation.Value)
			t.addLocked(mutation.Index+1, mutation.Value)
		case MutationSet:
			delta := t.ops.Sub(mutation.Value, t.vals[mutation.Index])
			t.vals[mutation.Index] = mutation.Value
			t.addLocked(mutation.Index+1, delta)
		}
	}

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
	delta := t.ops.Sub(value, t.vals[index])
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
	return t.ops.Sub(t.prefixSumLocked(right+1), t.prefixSumLocked(left)), nil
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

func (t *Tree[T]) addLocked(index int, delta T) {
	for i := index; i < len(t.tree); i += lowbit(i) {
		t.tree[i] = t.ops.Add(t.tree[i], delta)
	}
}

func (t *Tree[T]) prefixSumLocked(index int) T {
	sum := t.ops.Zero()
	for i := index; i > 0; i -= lowbit(i) {
		sum = t.ops.Add(sum, t.tree[i])
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
