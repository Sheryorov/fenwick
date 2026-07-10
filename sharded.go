package fenwick

import (
	"runtime"
	"sync"
)

// ShardedTree splits values into independently locked contiguous Fenwick trees.
type ShardedTree[T any] struct {
	ops       Operations[T]
	length    int
	shardSize int
	shards    []fenwickShard[T]
}

type fenwickShard[T any] struct {
	mu    sync.RWMutex
	tree  []T
	vals  []T
	total T
	_     [56]byte
}

// NewSharded constructs a sharded tree for Value models.
func NewSharded[T Value](values []T) *ShardedTree[T] {
	return NewShardedWithOperations(values, legacyValueOperations(values))
}

// NewShardedWithCount constructs a sharded tree for Value models with an
// explicit shard count.
func NewShardedWithCount[T Value](values []T, shardCount int) *ShardedTree[T] {
	return NewShardedWithOperationsAndCount(values, shardCount, legacyValueOperations(values))
}

func legacyValueOperations[T Value](values []T) valueOperations[T] {
	var zero T
	if len(values) > 0 {
		zero = values[0].Zero().(T)
	}
	return valueOperations[T]{zero: zero}
}

// NewNumericSharded constructs a sharded tree for signed numeric values.
func NewNumericSharded[T Number](values []T) *ShardedTree[T] {
	return NewShardedWithOperations(values, NumericOperations[T]{})
}

// NewNumericShardedWithCount constructs a numeric sharded tree with an
// explicit shard count.
func NewNumericShardedWithCount[T Number](values []T, shardCount int) *ShardedTree[T] {
	return NewShardedWithOperationsAndCount(values, shardCount, NumericOperations[T]{})
}

// NewShardedWithOperations chooses the shard count from GOMAXPROCS.
func NewShardedWithOperations[T any](values []T, ops Operations[T]) *ShardedTree[T] {
	shards := runtime.GOMAXPROCS(0) * 4
	if shards < 1 {
		shards = 1
	}
	return NewShardedWithOperationsAndCount(values, shards, ops)
}

// NewShardedWithOperationsAndCount injects aggregation behavior and uses an
// explicit shard count.
func NewShardedWithOperationsAndCount[T any](values []T, shardCount int, ops Operations[T]) *ShardedTree[T] {
	if ops == nil {
		panic("fenwick: operations must not be nil")
	}
	if shardCount <= 0 {
		panic("fenwick: shard count must be positive")
	}
	if len(values) == 0 {
		return &ShardedTree[T]{ops: ops, shardSize: 1}
	}
	if shardCount > len(values) {
		shardCount = len(values)
	}

	shardSize := (len(values) + shardCount - 1) / shardCount
	t := &ShardedTree[T]{ops: ops, length: len(values), shardSize: shardSize, shards: make([]fenwickShard[T], shardCount)}
	zero := ops.Zero()
	for shardIndex := range t.shards {
		start := shardIndex * shardSize
		if start >= len(values) {
			break
		}
		end := min(start+shardSize, len(values))
		s := &t.shards[shardIndex]
		s.vals = append([]T(nil), values[start:end]...)
		s.tree = make([]T, len(s.vals)+1)
		for i := range s.tree {
			s.tree[i] = zero
		}
		s.total = zero
		for i, value := range s.vals {
			s.total = ops.Add(s.total, value)
			internal := i + 1
			s.tree[internal] = ops.Add(s.tree[internal], value)
			parent := internal + lowbit(internal)
			if parent < len(s.tree) {
				s.tree[parent] = ops.Add(s.tree[parent], s.tree[internal])
			}
		}
	}
	return t
}

func (t *ShardedTree[T]) Len() int {
	if t == nil {
		return 0
	}
	return t.length
}
func (t *ShardedTree[T]) ShardCount() int {
	if t == nil {
		return 0
	}
	return len(t.shards)
}

func (t *ShardedTree[T]) At(index int) (T, error) {
	var zero T
	si, li, err := t.locate(index)
	if err != nil {
		return zero, err
	}
	s := &t.shards[si]
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vals[li], nil
}
func (t *ShardedTree[T]) Add(index int, delta T) error {
	si, li, err := t.locate(index)
	if err != nil {
		return err
	}
	s := &t.shards[si]
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vals[li] = t.ops.Add(s.vals[li], delta)
	s.addLocked(t.ops, li+1, delta)
	s.total = t.ops.Add(s.total, delta)
	return nil
}

// Apply atomically applies a batch of point mutations in slice order.
//
// All mutations are validated before locks are acquired. Every touched shard
// is then write-locked in ascending order, preventing deadlocks and ensuring
// exact readers observe either the state before the whole batch or the state
// after it. Fast cross-shard readers retain their documented non-snapshot
// semantics. An empty batch is a no-op.
func (t *ShardedTree[T]) Apply(mutations ...Mutation[T]) error {
	if len(mutations) == 0 {
		return nil
	}
	if t == nil {
		return indexError(mutations[0].Index, 0)
	}

	touched := make([]bool, len(t.shards))
	for _, mutation := range mutations {
		si, _, err := t.locate(mutation.Index)
		if err != nil {
			return err
		}
		if err := validateMutationKind(mutation.Kind); err != nil {
			return err
		}
		touched[si] = true
	}

	for i, isTouched := range touched {
		if isTouched {
			t.shards[i].mu.Lock()
		}
	}
	defer func() {
		for i := len(touched) - 1; i >= 0; i-- {
			if touched[i] {
				t.shards[i].mu.Unlock()
			}
		}
	}()

	for _, mutation := range mutations {
		si := mutation.Index / t.shardSize
		li := mutation.Index - si*t.shardSize
		s := &t.shards[si]

		switch mutation.Kind {
		case MutationAdd:
			s.vals[li] = t.ops.Add(s.vals[li], mutation.Value)
			s.addLocked(t.ops, li+1, mutation.Value)
			s.total = t.ops.Add(s.total, mutation.Value)
		case MutationSet:
			delta := t.ops.Sub(mutation.Value, s.vals[li])
			s.vals[li] = mutation.Value
			s.addLocked(t.ops, li+1, delta)
			s.total = t.ops.Add(s.total, delta)
		}
	}

	return nil
}

func (t *ShardedTree[T]) Set(index int, value T) error {
	si, li, err := t.locate(index)
	if err != nil {
		return err
	}
	s := &t.shards[si]
	s.mu.Lock()
	defer s.mu.Unlock()
	delta := t.ops.Sub(value, s.vals[li])
	s.vals[li] = value
	s.addLocked(t.ops, li+1, delta)
	s.total = t.ops.Add(s.total, delta)
	return nil
}
func (t *ShardedTree[T]) PrefixSum(index int) (T, error) {
	var zero T
	si, li, err := t.locate(index)
	if err != nil {
		return zero, err
	}
	sum := t.ops.Zero()
	for i := 0; i < si; i++ {
		s := &t.shards[i]
		s.mu.RLock()
		sum = t.ops.Add(sum, s.total)
		s.mu.RUnlock()
	}
	s := &t.shards[si]
	s.mu.RLock()
	sum = t.ops.Add(sum, s.prefixSumLocked(t.ops, li+1))
	s.mu.RUnlock()
	return sum, nil
}
func (t *ShardedTree[T]) RangeSum(left, right int) (T, error) {
	var zero T
	if err := t.validateRange(left, right); err != nil {
		return zero, err
	}
	return t.rangeSumFast(left, right), nil
}
func (t *ShardedTree[T]) Total() T {
	var zero T
	if t == nil {
		return zero
	}
	sum := t.ops.Zero()
	for i := range t.shards {
		s := &t.shards[i]
		s.mu.RLock()
		sum = t.ops.Add(sum, s.total)
		s.mu.RUnlock()
	}
	return sum
}
func (t *ShardedTree[T]) ExactPrefixSum(index int) (T, error) {
	var zero T
	si, li, err := t.locate(index)
	if err != nil {
		return zero, err
	}
	t.lockReadRange(0, si)
	defer t.unlockReadRange(0, si)
	sum := t.ops.Zero()
	for i := 0; i < si; i++ {
		sum = t.ops.Add(sum, t.shards[i].total)
	}
	return t.ops.Add(sum, t.shards[si].prefixSumLocked(t.ops, li+1)), nil
}
func (t *ShardedTree[T]) ExactRangeSum(left, right int) (T, error) {
	var zero T
	if err := t.validateRange(left, right); err != nil {
		return zero, err
	}
	fs, ll, _ := t.locate(left)
	ls, rl, _ := t.locate(right)
	t.lockReadRange(fs, ls)
	defer t.unlockReadRange(fs, ls)
	if fs == ls {
		s := &t.shards[fs]
		return t.ops.Sub(s.prefixSumLocked(t.ops, rl+1), s.prefixSumLocked(t.ops, ll)), nil
	}
	first := &t.shards[fs]
	sum := t.ops.Sub(first.total, first.prefixSumLocked(t.ops, ll))
	for i := fs + 1; i < ls; i++ {
		sum = t.ops.Add(sum, t.shards[i].total)
	}
	return t.ops.Add(sum, t.shards[ls].prefixSumLocked(t.ops, rl+1)), nil
}
func (t *ShardedTree[T]) ExactTotal() T {
	var zero T
	if t == nil || len(t.shards) == 0 {
		return zero
	}
	t.lockReadRange(0, len(t.shards)-1)
	defer t.unlockReadRange(0, len(t.shards)-1)
	sum := t.ops.Zero()
	for i := range t.shards {
		sum = t.ops.Add(sum, t.shards[i].total)
	}
	return sum
}
func (t *ShardedTree[T]) Values() []T {
	if t == nil || t.length == 0 {
		return nil
	}
	t.lockReadRange(0, len(t.shards)-1)
	defer t.unlockReadRange(0, len(t.shards)-1)
	values := make([]T, 0, t.length)
	for i := range t.shards {
		values = append(values, t.shards[i].vals...)
	}
	return values
}
func (t *ShardedTree[T]) rangeSumFast(left, right int) T {
	fs, ll, _ := t.locate(left)
	ls, rl, _ := t.locate(right)
	if fs == ls {
		s := &t.shards[fs]
		s.mu.RLock()
		defer s.mu.RUnlock()
		return t.ops.Sub(s.prefixSumLocked(t.ops, rl+1), s.prefixSumLocked(t.ops, ll))
	}
	first := &t.shards[fs]
	first.mu.RLock()
	sum := t.ops.Sub(first.total, first.prefixSumLocked(t.ops, ll))
	first.mu.RUnlock()
	for i := fs + 1; i < ls; i++ {
		s := &t.shards[i]
		s.mu.RLock()
		sum = t.ops.Add(sum, s.total)
		s.mu.RUnlock()
	}
	last := &t.shards[ls]
	last.mu.RLock()
	sum = t.ops.Add(sum, last.prefixSumLocked(t.ops, rl+1))
	last.mu.RUnlock()
	return sum
}
func (t *ShardedTree[T]) locate(index int) (int, int, error) {
	if t == nil || index < 0 || index >= t.length {
		return 0, 0, indexError(index, t.Len())
	}
	si := index / t.shardSize
	return si, index - si*t.shardSize, nil
}
func (t *ShardedTree[T]) validateRange(left, right int) error {
	if t == nil || left < 0 || right < left || right >= t.length {
		return rangeError(left, right, t.Len())
	}
	return nil
}
func (t *ShardedTree[T]) lockReadRange(first, last int) {
	for i := first; i <= last; i++ {
		t.shards[i].mu.RLock()
	}
}
func (t *ShardedTree[T]) unlockReadRange(first, last int) {
	for i := last; i >= first; i-- {
		t.shards[i].mu.RUnlock()
	}
}
func (s *fenwickShard[T]) addLocked(ops Operations[T], index int, delta T) {
	for i := index; i < len(s.tree); i += lowbit(i) {
		s.tree[i] = ops.Add(s.tree[i], delta)
	}
}
func (s *fenwickShard[T]) prefixSumLocked(ops Operations[T], index int) T {
	sum := ops.Zero()
	for i := index; i > 0; i -= lowbit(i) {
		sum = ops.Add(sum, s.tree[i])
	}
	return sum
}
