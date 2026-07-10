package fenwick

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// ShardedTree is a Fenwick tree optimized for concurrent point updates.
//
// Values are split into independent contiguous shards. Updates to different
// shards proceed in parallel. Public indexes are zero-based.
//
// Fast query methods (PrefixSum, RangeSum, Total) use atomic shard totals and
// lock only boundary shards. During concurrent writes they are race-free but
// do not provide a single linearizable snapshot across multiple shards.
// Use ExactPrefixSum, ExactRangeSum, ExactTotal, or Values when a consistent
// cross-shard snapshot is required.
type ShardedTree struct {
	length    int
	shardSize int
	shards    []fenwickShard
}

type fenwickShard struct {
	mu sync.RWMutex

	tree []int64 // one-based local Fenwick storage
	vals []int64 // zero-based local values

	// total is maintained atomically for lock-free reads of complete shards.
	total atomic.Int64

	// Keep hot counters for adjacent shards on different cache lines on common
	// 64-byte-cache-line architectures. This is a performance hint, not an API
	// guarantee.
	_ [56]byte
}

// NewSharded builds a ShardedTree in O(n). The shard count is selected from
// GOMAXPROCS and capped by the number of values.
func NewSharded(values []int64) *ShardedTree {
	shards := runtime.GOMAXPROCS(0) * 4
	if shards < 1 {
		shards = 1
	}
	return NewShardedWithCount(values, shards)
}

// NewShardedWithCount builds a ShardedTree in O(n) using shardCount shards.
// It panics when shardCount is not positive. The input slice is copied.
func NewShardedWithCount(values []int64, shardCount int) *ShardedTree {
	if shardCount <= 0 {
		panic("fenwick: shard count must be positive")
	}

	if len(values) == 0 {
		return &ShardedTree{shardSize: 1}
	}
	if shardCount > len(values) {
		shardCount = len(values)
	}

	shardSize := (len(values) + shardCount - 1) / shardCount
	t := &ShardedTree{
		length:    len(values),
		shardSize: shardSize,
		shards:    make([]fenwickShard, shardCount),
	}

	for shardIndex := range t.shards {
		start := shardIndex * shardSize
		if start >= len(values) {
			break
		}
		end := min(start+shardSize, len(values))
		s := &t.shards[shardIndex]
		s.vals = append([]int64(nil), values[start:end]...)
		s.tree = make([]int64, len(s.vals)+1)

		var total int64
		for i, value := range s.vals {
			total += value
			internal := i + 1
			s.tree[internal] += value
			parent := internal + lowbit(internal)
			if parent < len(s.tree) {
				s.tree[parent] += s.tree[internal]
			}
		}
		s.total.Store(total)
	}

	return t
}

// Len returns the number of stored values.
func (t *ShardedTree) Len() int {
	if t == nil {
		return 0
	}
	return t.length
}

// ShardCount returns the number of shards.
func (t *ShardedTree) ShardCount() int {
	if t == nil {
		return 0
	}
	return len(t.shards)
}

// At returns the value at index.
func (t *ShardedTree) At(index int) (int64, error) {
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return 0, err
	}
	s := &t.shards[shardIndex]
	s.mu.RLock()
	value := s.vals[localIndex]
	s.mu.RUnlock()
	return value, nil
}

// Add increments the value at index by delta in O(log shardSize).
func (t *ShardedTree) Add(index int, delta int64) error {
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return err
	}
	if delta == 0 {
		return nil
	}

	s := &t.shards[shardIndex]
	s.mu.Lock()
	s.vals[localIndex] += delta
	s.addLocked(localIndex+1, delta)
	s.total.Add(delta)
	s.mu.Unlock()
	return nil
}

// Set replaces the value at index in O(log shardSize).
func (t *ShardedTree) Set(index int, value int64) error {
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return err
	}

	s := &t.shards[shardIndex]
	s.mu.Lock()
	delta := value - s.vals[localIndex]
	if delta != 0 {
		s.vals[localIndex] = value
		s.addLocked(localIndex+1, delta)
		s.total.Add(delta)
	}
	s.mu.Unlock()
	return nil
}

// PrefixSum returns the inclusive sum values[0:index+1]. It is optimized for
// throughput and is not a linearizable cross-shard snapshot during concurrent
// writes. Use ExactPrefixSum when snapshot consistency is required.
func (t *ShardedTree) PrefixSum(index int) (int64, error) {
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return 0, err
	}

	var sum int64
	for i := 0; i < shardIndex; i++ {
		sum += t.shards[i].total.Load()
	}

	s := &t.shards[shardIndex]
	s.mu.RLock()
	sum += s.prefixSumLocked(localIndex + 1)
	s.mu.RUnlock()
	return sum, nil
}

// RangeSum returns the inclusive sum values[left:right+1]. During concurrent
// writes it is race-free but not a linearizable cross-shard snapshot.
func (t *ShardedTree) RangeSum(left, right int) (int64, error) {
	if err := t.validateRange(left, right); err != nil {
		return 0, err
	}
	return t.rangeSumFast(left, right), nil
}

// Total returns the sum of atomically observed shard totals. During concurrent
// writes it is not guaranteed to represent one global instant.
func (t *ShardedTree) Total() int64 {
	if t == nil {
		return 0
	}
	var sum int64
	for i := range t.shards {
		sum += t.shards[i].total.Load()
	}
	return sum
}

// ExactPrefixSum returns a linearizable cross-shard prefix sum by read-locking
// every involved shard in ascending order.
func (t *ShardedTree) ExactPrefixSum(index int) (int64, error) {
	shardIndex, localIndex, err := t.locate(index)
	if err != nil {
		return 0, err
	}

	t.lockReadRange(0, shardIndex)
	defer t.unlockReadRange(0, shardIndex)

	var sum int64
	for i := 0; i < shardIndex; i++ {
		sum += t.shards[i].prefixSumLocked(len(t.shards[i].vals))
	}
	sum += t.shards[shardIndex].prefixSumLocked(localIndex + 1)
	return sum, nil
}

// ExactRangeSum returns a linearizable inclusive range sum by read-locking all
// intersecting shards in ascending order.
func (t *ShardedTree) ExactRangeSum(left, right int) (int64, error) {
	if err := t.validateRange(left, right); err != nil {
		return 0, err
	}
	firstShard, leftLocal, _ := t.locate(left)
	lastShard, rightLocal, _ := t.locate(right)

	t.lockReadRange(firstShard, lastShard)
	defer t.unlockReadRange(firstShard, lastShard)

	if firstShard == lastShard {
		s := &t.shards[firstShard]
		return s.prefixSumLocked(rightLocal+1) - s.prefixSumLocked(leftLocal), nil
	}

	var sum int64
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
func (t *ShardedTree) ExactTotal() int64 {
	if t == nil || len(t.shards) == 0 {
		return 0
	}
	t.lockReadRange(0, len(t.shards)-1)
	defer t.unlockReadRange(0, len(t.shards)-1)

	var sum int64
	for i := range t.shards {
		sum += t.shards[i].prefixSumLocked(len(t.shards[i].vals))
	}
	return sum
}

// Values returns a consistent copy of all values.
func (t *ShardedTree) Values() []int64 {
	if t == nil || t.length == 0 {
		return nil
	}
	t.lockReadRange(0, len(t.shards)-1)
	defer t.unlockReadRange(0, len(t.shards)-1)

	values := make([]int64, 0, t.length)
	for i := range t.shards {
		values = append(values, t.shards[i].vals...)
	}
	return values
}

func (t *ShardedTree) rangeSumFast(left, right int) int64 {
	firstShard, leftLocal, _ := t.locate(left)
	lastShard, rightLocal, _ := t.locate(right)

	if firstShard == lastShard {
		s := &t.shards[firstShard]
		s.mu.RLock()
		sum := s.prefixSumLocked(rightLocal+1) - s.prefixSumLocked(leftLocal)
		s.mu.RUnlock()
		return sum
	}

	var sum int64
	first := &t.shards[firstShard]
	first.mu.RLock()
	sum += first.prefixSumLocked(len(first.vals)) - first.prefixSumLocked(leftLocal)
	first.mu.RUnlock()

	for i := firstShard + 1; i < lastShard; i++ {
		sum += t.shards[i].total.Load()
	}

	last := &t.shards[lastShard]
	last.mu.RLock()
	sum += last.prefixSumLocked(rightLocal + 1)
	last.mu.RUnlock()
	return sum
}

func (t *ShardedTree) locate(index int) (shardIndex, localIndex int, err error) {
	if t == nil || index < 0 || index >= t.length {
		return 0, 0, indexError(index, t.Len())
	}
	shardIndex = index / t.shardSize
	localIndex = index - shardIndex*t.shardSize
	return shardIndex, localIndex, nil
}

func (t *ShardedTree) validateRange(left, right int) error {
	if t == nil || left < 0 || right < left || right >= t.length {
		return rangeError(left, right, t.Len())
	}
	return nil
}

func (t *ShardedTree) lockReadRange(first, last int) {
	for i := first; i <= last; i++ {
		t.shards[i].mu.RLock()
	}
}

func (t *ShardedTree) unlockReadRange(first, last int) {
	for i := last; i >= first; i-- {
		t.shards[i].mu.RUnlock()
	}
}

func (s *fenwickShard) addLocked(internalIndex int, delta int64) {
	for i := internalIndex; i < len(s.tree); i += lowbit(i) {
		s.tree[i] += delta
	}
}

func (s *fenwickShard) prefixSumLocked(internalIndex int) int64 {
	var sum int64
	for i := internalIndex; i > 0; i -= lowbit(i) {
		sum += s.tree[i]
	}
	return sum
}
