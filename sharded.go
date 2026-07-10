package fenwick

import (
	"runtime"
	"sync"
)

type ShardedTree[T Number] struct {
	length    int
	shardSize int
	shards    []fenwickShard[T]
}

type fenwickShard[T Number] struct {
	mu    sync.RWMutex
	tree  []T
	vals  []T
	total T
	_     [56]byte
}

func NewSharded[T Number](values []T) *ShardedTree[T] {
	count := runtime.GOMAXPROCS(0) * 4
	if count < 1 {
		count = 1
	}
	return NewShardedWithCount(values, count)
}

func NewShardedWithCount[T Number](values []T, shardCount int) *ShardedTree[T] {
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
	t := &ShardedTree[T]{length: len(values), shardSize: shardSize, shards: make([]fenwickShard[T], shardCount)}
	for si := range t.shards {
		start := si * shardSize
		if start >= len(values) {
			break
		}
		end := min(start+shardSize, len(values))
		s := &t.shards[si]
		s.vals = append([]T(nil), values[start:end]...)
		s.tree = make([]T, len(s.vals)+1)
		for i, value := range s.vals {
			s.total += value
			internal := i + 1
			s.tree[internal] += value
			parent := internal + lowbit(internal)
			if parent < len(s.tree) {
				s.tree[parent] += s.tree[internal]
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
	v := s.vals[li]
	s.mu.RUnlock()
	return v, nil
}
func (t *ShardedTree[T]) Add(index int, delta T) error {
	si, li, err := t.locate(index)
	if err != nil {
		return err
	}
	if delta == 0 {
		return nil
	}
	s := &t.shards[si]
	s.mu.Lock()
	s.vals[li] += delta
	s.addLocked(li+1, delta)
	s.total += delta
	s.mu.Unlock()
	return nil
}
func (t *ShardedTree[T]) Set(index int, value T) error {
	si, li, err := t.locate(index)
	if err != nil {
		return err
	}
	s := &t.shards[si]
	s.mu.Lock()
	delta := value - s.vals[li]
	if delta != 0 {
		s.vals[li] = value
		s.addLocked(li+1, delta)
		s.total += delta
	}
	s.mu.Unlock()
	return nil
}
func (t *ShardedTree[T]) PrefixSum(index int) (T, error) {
	var zero T
	si, li, err := t.locate(index)
	if err != nil {
		return zero, err
	}
	var sum T
	for i := 0; i < si; i++ {
		s := &t.shards[i]
		s.mu.RLock()
		sum += s.total
		s.mu.RUnlock()
	}
	s := &t.shards[si]
	s.mu.RLock()
	sum += s.prefixSumLocked(li + 1)
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
	var sum T
	if t == nil {
		return sum
	}
	for i := range t.shards {
		s := &t.shards[i]
		s.mu.RLock()
		sum += s.total
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
	var sum T
	for i := 0; i < si; i++ {
		sum += t.shards[i].total
	}
	sum += t.shards[si].prefixSumLocked(li + 1)
	return sum, nil
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
		return s.prefixSumLocked(rl+1) - s.prefixSumLocked(ll), nil
	}
	first := &t.shards[fs]
	sum := first.prefixSumLocked(len(first.vals)) - first.prefixSumLocked(ll)
	for i := fs + 1; i < ls; i++ {
		sum += t.shards[i].total
	}
	sum += t.shards[ls].prefixSumLocked(rl + 1)
	return sum, nil
}
func (t *ShardedTree[T]) ExactTotal() T {
	var sum T
	if t == nil || len(t.shards) == 0 {
		return sum
	}
	t.lockReadRange(0, len(t.shards)-1)
	defer t.unlockReadRange(0, len(t.shards)-1)
	for i := range t.shards {
		sum += t.shards[i].total
	}
	return sum
}
func (t *ShardedTree[T]) Values() []T {
	if t == nil || t.length == 0 {
		return nil
	}
	t.lockReadRange(0, len(t.shards)-1)
	defer t.unlockReadRange(0, len(t.shards)-1)
	out := make([]T, 0, t.length)
	for i := range t.shards {
		out = append(out, t.shards[i].vals...)
	}
	return out
}

func (t *ShardedTree[T]) rangeSumFast(left, right int) T {
	fs, ll, _ := t.locate(left)
	ls, rl, _ := t.locate(right)
	if fs == ls {
		s := &t.shards[fs]
		s.mu.RLock()
		sum := s.prefixSumLocked(rl+1) - s.prefixSumLocked(ll)
		s.mu.RUnlock()
		return sum
	}
	first := &t.shards[fs]
	first.mu.RLock()
	sum := first.prefixSumLocked(len(first.vals)) - first.prefixSumLocked(ll)
	first.mu.RUnlock()
	for i := fs + 1; i < ls; i++ {
		s := &t.shards[i]
		s.mu.RLock()
		sum += s.total
		s.mu.RUnlock()
	}
	last := &t.shards[ls]
	last.mu.RLock()
	sum += last.prefixSumLocked(rl + 1)
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
func (s *fenwickShard[T]) addLocked(i int, delta T) {
	for ; i < len(s.tree); i += lowbit(i) {
		s.tree[i] += delta
	}
}
func (s *fenwickShard[T]) prefixSumLocked(i int) T {
	var sum T
	for ; i > 0; i -= lowbit(i) {
		sum += s.tree[i]
	}
	return sum
}
