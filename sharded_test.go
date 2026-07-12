package fenwick

import (
	"errors"
	"math/rand"
	"runtime"
	"slices"
	"sync"
	"testing"
)

func TestShardedTree(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount[int64]([]int64{3, 2, 5, 1, 4}, 3)
	if got := ft.Len(); got != 5 {
		t.Fatalf("Len()=%d, want 5", got)
	}
	if got := ft.ShardCount(); got != 3 {
		t.Fatalf("ShardCount()=%d, want 3", got)
	}
	if got := ft.Total(); got != 15 {
		t.Fatalf("Total()=%d, want 15", got)
	}

	prefix, err := ft.PrefixSum(2)
	if err != nil || prefix != 10 {
		t.Fatalf("PrefixSum(2)=(%d,%v), want (10,nil)", prefix, err)
	}
	exactPrefix, err := ft.ExactPrefixSum(2)
	if err != nil || exactPrefix != 10 {
		t.Fatalf("ExactPrefixSum(2)=(%d,%v), want (10,nil)", exactPrefix, err)
	}

	rangeSum, err := ft.RangeSum(1, 3)
	if err != nil || rangeSum != 8 {
		t.Fatalf("RangeSum(1,3)=(%d,%v), want (8,nil)", rangeSum, err)
	}
	exactRange, err := ft.ExactRangeSum(1, 3)
	if err != nil || exactRange != 8 {
		t.Fatalf("ExactRangeSum(1,3)=(%d,%v), want (8,nil)", exactRange, err)
	}

	if err := ft.Add(2, 10); err != nil {
		t.Fatal(err)
	}
	if err := ft.Set(4, -9); err != nil {
		t.Fatal(err)
	}
	if got := ft.ExactTotal(); got != 12 {
		t.Fatalf("ExactTotal()=%d, want 12", got)
	}
	if got := ft.Values(); !equalInt64s(got, []int64{3, 2, 15, 1, -9}) {
		t.Fatalf("Values()=%v", got)
	}
}

func TestShardedErrors(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount[int64](nil, 4)
	if ft.Len() != 0 || ft.Total() != 0 || ft.ExactTotal() != 0 {
		t.Fatalf("unexpected empty state")
	}
	if _, err := ft.At(0); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("At error=%v", err)
	}
	if _, err := ft.RangeSum(0, 0); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("RangeSum error=%v", err)
	}
}

func TestShardedRandomAgainstNaive(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(77))
	const n = 513
	values := make([]int64, n)
	for i := range values {
		values[i] = int64(rng.Intn(2001) - 1000)
	}
	ft := NewNumericShardedWithCount[int64](values, 17)
	naive := append([]int64(nil), values...)

	for step := 0; step < 20_000; step++ {
		switch rng.Intn(4) {
		case 0:
			i := rng.Intn(n)
			delta := int64(rng.Intn(101) - 50)
			if err := ft.Add(i, delta); err != nil {
				t.Fatal(err)
			}
			naive[i] += delta
		case 1:
			i := rng.Intn(n)
			value := int64(rng.Intn(2001) - 1000)
			if err := ft.Set(i, value); err != nil {
				t.Fatal(err)
			}
			naive[i] = value
		case 2:
			left := rng.Intn(n)
			right := left + rng.Intn(n-left)
			want := naiveRange(naive, left, right)
			got, err := ft.RangeSum(left, right)
			if err != nil || got != want {
				t.Fatalf("step=%d RangeSum(%d,%d)=(%d,%v), want %d", step, left, right, got, err, want)
			}
		case 3:
			left := rng.Intn(n)
			right := left + rng.Intn(n-left)
			want := naiveRange(naive, left, right)
			got, err := ft.ExactRangeSum(left, right)
			if err != nil || got != want {
				t.Fatalf("step=%d ExactRangeSum(%d,%d)=(%d,%v), want %d", step, left, right, got, err, want)
			}
		}
	}
}

func TestShardedConcurrentUpdates(t *testing.T) {
	const (
		n       = 1 << 14
		workers = 16
		loops   = 20_000
	)
	ft := NewNumericShardedWithCount[int64](make([]int64, n), workers)

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := worker * (n / workers)
			width := n / workers
			for i := 0; i < loops; i++ {
				index := start + i%width
				if err := ft.Add(index, 1); err != nil {
					t.Errorf("Add: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if got := ft.ExactTotal(); got != workers*loops {
		t.Fatalf("ExactTotal()=%d, want %d", got, workers*loops)
	}
}

func TestShardedTreeApplyEmptyBatch(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount(
		[]int64{1, 2, 3, 4},
		2,
	)

	before := ft.Values()

	if err := ft.Apply(); err != nil {
		t.Fatalf("Apply() error=%v", err)
	}

	if got := ft.Values(); !slices.Equal(got, before) {
		t.Fatalf("Apply() changed values: got %v want %v", got, before)
	}
}

func TestShardedTreeApplyInvalidMutationDoesNotPartiallyApply(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount(
		[]int64{1, 2, 3, 4},
		2,
	)

	err := ft.Apply(
		AddMutation(0, int64(10)),
		Mutation[int64]{
			Index: 3,
			Kind:  MutationKind(99),
			Value: 20,
		},
	)
	if err == nil {
		t.Fatal("Apply() expected error")
	}

	if got := ft.Values(); !slices.Equal(got, []int64{1, 2, 3, 4}) {
		t.Fatalf("Apply() partially mutated tree: %v", got)
	}
}

func BenchmarkConcurrentAddSingleMutex(b *testing.B) {
	benchmarkConcurrentAdd(b, func(n int) addTarget { return NewNumeric[int64](make([]int64, n)) })
}

func BenchmarkConcurrentAddSharded(b *testing.B) {
	benchmarkConcurrentAdd(b, func(n int) addTarget {
		return NewNumericShardedWithCount[int64](make([]int64, n), max(1, runtime.GOMAXPROCS(0)*4))
	})
}

func BenchmarkShardedRangeSumFast(b *testing.B) {
	values := make([]int64, 1<<20)
	ft := NewNumericShardedWithCount[int64](values, max(1, runtime.GOMAXPROCS(0)*4))
	mask := len(values) - 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		left := i & (mask >> 1)
		if _, err := ft.RangeSum(left, left+(1<<18)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShardedRangeSumExact(b *testing.B) {
	values := make([]int64, 1<<20)
	ft := NewNumericShardedWithCount[int64](values, max(1, runtime.GOMAXPROCS(0)*4))
	mask := len(values) - 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		left := i & (mask >> 1)
		if _, err := ft.ExactRangeSum(left, left+(1<<18)); err != nil {
			b.Fatal(err)
		}
	}
}

func TestNilShardedTreeOperations(t *testing.T) {
	t.Parallel()

	var ft *ShardedTree[int64]

	if ft.Len() != 0 {
		t.Fatalf("Nil ShardedTree Len: got %d, want 0", ft.Len())
	}

	if ft.ShardCount() != 0 {
		t.Fatalf("Nil ShardedTree ShardCount: got %d, want 0", ft.ShardCount())
	}

	if ft.Total() != 0 {
		t.Fatalf("Nil ShardedTree Total: got %d, want 0", ft.Total())
	}

	if ft.ExactTotal() != 0 {
		t.Fatalf("Nil ShardedTree ExactTotal: got %d, want 0", ft.ExactTotal())
	}

	if values := ft.Values(); values != nil {
		t.Fatalf("Nil ShardedTree Values: got %v, want nil", values)
	}
}

func TestShardedSingleShard(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount[int64]([]int64{5}, 1)

	if ft.ShardCount() != 1 {
		t.Fatalf("Single shard count: got %d, want 1", ft.ShardCount())
	}

	if total := ft.Total(); total != 5 {
		t.Fatalf("Single shard total: got %d, want 5", total)
	}

	sum, _ := ft.ExactRangeSum(0, 0)
	if sum != 5 {
		t.Fatalf("Single shard range sum: got %d, want 5", sum)
	}
}

func TestShardedAddZeroDelta(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount[int64]([]int64{1, 2, 3, 4, 5}, 2)
	initial, _ := ft.At(1)

	// Adding zero should not change value
	if err := ft.Add(1, 0); err != nil {
		t.Fatalf("Add(1, 0): %v", err)
	}

	got, _ := ft.At(1)
	if got != initial {
		t.Fatalf("Add(1, 0) changed value: got %d, want %d", got, initial)
	}
}

func TestShardedPrefixSumCrossShard(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount[int64]([]int64{1, 2, 3, 4, 5}, 2)

	// Test prefix sum crossing shard boundary
	sum, _ := ft.ExactPrefixSum(3)
	if sum != 10 { // 1+2+3+4
		t.Fatalf("PrefixSum(3): got %d, want 10", sum)
	}
}

type addTarget interface {
	Add(index int, delta int64) error
	Len() int
}

func benchmarkConcurrentAdd(b *testing.B, factory func(n int) addTarget) {
	const n = 1 << 20
	ft := factory(n)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			index := (i*2654435761 + 1013904223) & (ft.Len() - 1)
			if err := ft.Add(index, 1); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func naiveRange(values []int64, left, right int) int64 {
	var sum int64
	for _, value := range values[left : right+1] {
		sum += value
	}
	return sum
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestShardedInjectedOperationsWithDomainModel(t *testing.T) {
	t.Parallel()

	values := []metricsModel{{Count: 1, Sum: 1}, {Count: 2, Sum: 2}, {Count: 3, Sum: 3}, {Count: 4, Sum: 4}}
	ft := NewShardedWithOperationsAndCount(values, 2, metricsOperations{})

	got, err := ft.ExactRangeSum(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := metricsModel{Count: 9, Sum: 9}
	if got != want {
		t.Fatalf("ExactRangeSum(1,3)=%+v, want %+v", got, want)
	}
}

func TestShardedApplyBatchAcrossShards(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount([]int64{1, 2, 3, 4, 5, 6}, 3)
	err := ft.Apply(
		AddMutation(0, int64(10)),
		SetMutation(2, int64(30)),
		AddMutation(5, int64(-1)),
		AddMutation(2, int64(2)),
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []int64{11, 2, 32, 4, 5, 5}
	if got := ft.Values(); !equalInt64s(got, want) {
		t.Fatalf("Values()=%v, want %v", got, want)
	}
	if got := ft.ExactTotal(); got != 59 {
		t.Fatalf("ExactTotal()=%d, want 59", got)
	}
}

func TestShardedApplyValidatesBeforeMutation(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount([]int64{1, 2, 3, 4}, 2)
	before := ft.Values()

	err := ft.Apply(
		SetMutation(0, int64(100)),
		AddMutation(4, int64(1)),
	)
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("Apply error=%v, want ErrIndexOutOfRange", err)
	}
	if got := ft.Values(); !equalInt64s(got, before) {
		t.Fatalf("invalid batch partially applied: got %v want %v", got, before)
	}
}

func TestShardedApplyEmptyAndNil(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount([]int64{1}, 1)
	if err := ft.Apply(); err != nil {
		t.Fatalf("empty Apply: %v", err)
	}

	var nilTree *ShardedTree[int64]
	if err := nilTree.Apply(); err != nil {
		t.Fatalf("empty nil Apply: %v", err)
	}
	if err := nilTree.Apply(AddMutation(0, int64(1))); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil Apply error=%v", err)
	}
}

func TestShardedApplyWithInjectedOperations(t *testing.T) {
	t.Parallel()

	values := []metricsModel{{Count: 1, Sum: 1}, {Count: 2, Sum: 2}, {Count: 3, Sum: 3}}
	ft := NewShardedWithOperationsAndCount(values, 2, metricsOperations{})

	err := ft.Apply(
		AddMutation(0, metricsModel{Count: 4, Sum: 0.5}),
		SetMutation(2, metricsModel{Count: 10, Sum: 10}),
	)
	if err != nil {
		t.Fatal(err)
	}

	got := ft.ExactTotal()
	want := metricsModel{Count: 17, Sum: 13.5}
	if got != want {
		t.Fatalf("ExactTotal()=%+v, want %+v", got, want)
	}
}
