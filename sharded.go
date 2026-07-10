package fenwick

import (
	"runtime"
	"sync"
)

// ShardedTree is a Fenwick tree optimized for concurrent point updates.
//
// Values are split into independent contiguous shards. Updates to different
// shards proceed in parallel. Public indexes are zero-based.
//
// Fast query methods (PrefixSum, RangeSum, Total) use shard totals and
// lock only boundary shards. During concurrent writes they are race-free but
// do not provide a single linearizable snapshot across multiple shards.
// Use ExactPrefixSum, ExactRangeSum, ExactTotal, or Values when a consistent
// cross-shard snapshot is required.
type ShardedTree[T Numeric] struct {
	length    int
	shardSize int
	shards    []fenwickShard[T]
}

type fenwickShard[T Numeric] struct {
	mu sync.RWMutex

	tree []T // one-based local Fenwick storage
	vals []T // zero-based local values

	// total is maintained under mu lock for thread-safe access.
	total T

	// Keep hot counters for adjacent shards on different cache lines on common
	// 64-byte-cache-line architectures. This is a performance hint, not an API
	// guarantee.
	_ [56]byte
}

// NewSharded builds a ShardedTree in O(n). The shard count is selected from
// GOMAXPROCS and capped by the number of values.
func NewSharded[T Numeric](values []T) *ShardedTree[T] {
	shards := runtime.GOMAXPROCS(0) * 4
	if shards < 1 {
		shards = 1
	}
	return NewShardedWithCount(values, shards)
}

// NewShardedWithCount builds a ShardedTree in O(n) using shardCount shards.
// It panics when shardCount is not positive. The input slice is copied.
func NewShardedWithCount[T Numeric](values []T, shardCount int) *ShardedTree[T] {
	if shardCount <= 0 {
		panic("fenwick: shard count must be positive")
	}

	if len(values) == 0 {
		return &ShardedTree[T]{shardSize: 1}
	}
	if shardCount > len(values) {
		shardCount = len(values)
	}

	shardSize := (len(values) + shardCount - 1) / shardCount
	t := &ShardedTree[T]{
		length:    len(values),
		shardSize: shardSize,
		shards:    make([]fenwickShard[T], shardCount),
	}

	for shardIndex := range t.shards {
		start := shardIndex * shardSize
		if start >= len(values) {
			break
		}
		end := min(start+shardSize, len(values))
		s := &t.shards[shardIndex]
		s.vals = append([]T(nil), values[start:end]...)
		s.tree = make([]T, len(s.vals)+1)

		var total T
		for i, value := range s.vals {
			total += value
			internal := i + 1
			s.tree[internal] += value
			parent := internal + lowbit(internal)
			if parent < len(s.tree) {
				s.tree[parent] += s.tree[internal]
			}
		}
		s.total = total
	}

	return t
}

// Len returns the number of stored values.
func (t *ShardedTree[T]) Len() int {
	if t == nil {
		return 0
	}
	return t.length
}

// ShardCount returns the number of shards.
func (t *ShardedTree[T]) ShardCount() int {
	if t == nil {
		return 0
	}
	return len(t.shards)
}

// At returns the value at index.
func (t *ShardedTree[T]) At(index int) (T, error) {
	var zero T
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return zero, err
	}
	s := &t.shards[shardIndex]
	s.mu.RLock()
	value := s.vals[localIndex]
	s.mu.RUnlock()
	return value, nil
}

// Add increments the value at index by delta in O(log shardSize).
func (t *ShardedTree[T]) Add(index int, delta T) error {
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return err
	}
	var zero T
	if delta == zero {
		return nil
	}

	s := &t.shards[shardIndex]
	s.mu.Lock()
	s.vals[localIndex] += delta
	s.addLocked(localIndex+1, delta)
	s.total += delta
	s.mu.Unlock()
	return nil
}

// Set replaces the value at index in O(log shardSize).
func (t *ShardedTree[T]) Set(index int, value T) error {
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return err
	}

	s := &t.shards[shardIndex]
	s.mu.Lock()
	delta := value - s.vals[localIndex]
	var zero T
	if delta != zero {
		s.vals[localIndex] = value
		s.addLocked(localIndex+1, delta)
		s.total += delta
	}
	s.mu.Unlock()
	return nil
}

// PrefixSum returns the inclusive sum values[0:index+1]. It is optimized for
// throughput and is not a linearizable cross-shard snapshot during concurrent
// writes. Use ExactPrefixSum when snapshot consistency is required.
func (t *ShardedTree[T]) PrefixSum(index int) (T, error) {
	var zero T
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return zero, err
	}

	var sum T
	for i := 0; i < shardIndex; i++ {
		s := &t.shards[i]
		s.mu.RLock()
		sum += s.total
		s.mu.RUnlock()
	}

	s := &t.shards[shardIndex]
	s.mu.RLock()
	sum += s.prefixSumLocked(localIndex + 1)
	s.mu.RUnlock()
	return sum, nil
}

// RangeSum returns the inclusive sum values[left:right+1]. During concurrent
// writes it is race-free but not a linearizable cross-shard snapshot.
func (t *ShardedTree[T]) RangeSum(left, right int) (T, error) {
	var zero T
	if err := t.validateRange(left, right); err != nil {
		return zero, err
	}
	return t.rangeSumFast(left, right), nil
}

// Total returns the sum of observed shard totals. During concurrent
// writes it is not guaranteed to represent one global instant.
func (t *ShardedTree[T]) Total() T {
	var zero T
	if t == nil {
		return zero
	}
	var sum T
	for i := range t.shards {
		s := &t.shards[i]
		s.mu.RLock()
		sum += s.total
		s.mu.RUnlock()
	}
	return sum
}

// ExactPrefixSum returns a linearizable cross-shard prefix sum by read-locking
// every involved shard in ascending order.
func (t *ShardedTree[T]) ExactPrefixSum(index int) (T, error) {
	var zero T
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return zero, err
	}

	t.lockReadRange(0, shardIndex)
	defer t.unlockReadRange(0, shardIndex)

	var sum T
	for i := 0; i < shardIndex; i++ {
		sum += t.shards[i].prefixSumLocked(len(t.shards[i].vals))
	}
	sum += t.shards[shardIndex].prefixSumLocked(localIndex + 1)
	return sum, nil
}

// ExactRangeSum returns a linearizable inclusive range sum by read-locking all
// intersecting shards in ascending order.
func (t *ShardedTree[T]) ExactRangeSum(left, right int) (T, error) {
	var zero T
	if err := t.validateRange(left, right); err != nil {
		return zero, err
	}
	firstShard, leftLocal, _ := t.locate(left)
	lastShard, rightLocal, _ := t.locate(right)

	t.lockReadRange(firstShard, lastShard)
	defer t.unlockReadRange(firstShard, lastShard)

	if firstShard == lastShard {
		s := &t.shards[firstShard]
		return s.prefixSumLocked(rightLocal+1) - s.prefixSumLocked(leftLocal), nil
	}

	var sum T
	first := &t.shards[firstShard]
	sum += first.prefixSumLocked(len(first.vals)) - first.prefixSumLocked(leftLocal)

	for i := firstShard + 1; i < lastShard; i++ {
		sum += t.shards[i].prefixSumLocked(len(t.shards[i].vals))
	}

	last := &t.shards[lastShard]
	sum += last.prefixSumLocked(rightLocal + 1)
	return sum, nil
}

// ExactTotal returns a linearizable total by read-locking every shard.
func (t *ShardedTree[T]) ExactTotal() T {
	var zero T
	if t == nil || len(t.shards) == 0 {
		return zero
	}
	t.lockReadRange(0, len(t.shards)-1)
	defer t.unlockReadRange(0, len(t.shards)-1)

	var sum T
	for i := range t.shards {
		sum += t.shards[i].prefixSumLocked(len(t.shards[i].vals))
	}
	return sum
}

// Values returns a consistent copy of all values.
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
	firstShard, leftLocal, _ := t.locate(left)
	lastShard, rightLocal, _ := t.locate(right)

	if firstShard == lastShard {
		s := &t.shards[firstShard]
		s.mu.RLock()
		sum := s.prefixSumLocked(rightLocal+1) - s.prefixSumLocked(leftLocal)
		s.mu.RUnlock()
		return sum
	}

	var sum T
	first := &t.shards[firstShard]
	first.mu.RLock()
	sum += first.prefixSumLocked(len(first.vals)) - first.prefixSumLocked(leftLocal)
	first.mu.RUnlock()

	for i := firstShard + 1; i < lastShard; i++ {
		s := &t.shards[i]
		s.mu.RLock()
		sum += s.total
		s.mu.RUnlock()
	}

	last := &t.shards[lastShard]
	last.mu.RLock()
	sum += last.prefixSumLocked(rightLocal + 1)
	last.mu.RUnlock()
	return sum
}

func (t *ShardedTree[T]) locate(index int) (shardIndex, localIndex int, err error) {
	if t == nil || index < 0 || index >= t.length {
		return 0, 0, indexError(index, t.Len())
	}
	shardIndex = index / t.shardSize
	localIndex = index - shardIndex*t.shardSize
	return shardIndex, localIndex, nil
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

func (s *fenwickShard[T]) addLocked(internalIndex int, delta T) {
	for i := internalIndex; i < len(s.tree); i += lowbit(i) {
		s.tree[i] += delta
	}
}

func (s *fenwickShard[T]) prefixSumLocked(internalIndex int) T {
	var sum T
	for i := internalIndex; i > 0; i -= lowbit(i) {
		sum += s.tree[i]
	}
	return sum
}
