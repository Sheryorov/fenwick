package fenwick

import (
	"errors"
	"math/rand"
	"runtime"
	"sync"
	"testing"
)

func TestShardedTree(t *testing.T) {
	t.Parallel()

	ft := NewShardedWithCount[int64]([]int64{3, 2, 5, 1, 4}, 3)
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

	ft := NewShardedWithCount[int64](nil, 4)
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
	ft := NewShardedWithCount[int64](values, 17)
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
	ft := NewShardedWithCount[int64](make([]int64, n), workers)

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

func BenchmarkConcurrentAddSingleMutex(b *testing.B) {
	benchmarkConcurrentAdd(b, func(n int) addTarget { return New[int64](make([]int64, n)) })
}

func BenchmarkConcurrentAddSharded(b *testing.B) {
	benchmarkConcurrentAdd(b, func(n int) addTarget {
		return NewShardedWithCount[int64](make([]int64, n), max(1, runtime.GOMAXPROCS(0)*4))
	})
}

func BenchmarkShardedRangeSumFast(b *testing.B) {
	values := make([]int64, 1<<20)
	ft := NewShardedWithCount[int64](values, max(1, runtime.GOMAXPROCS(0)*4))
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
	ft := NewShardedWithCount[int64](values, max(1, runtime.GOMAXPROCS(0)*4))
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
