package fenwick

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
)

func TestTree(t *testing.T) {
	t.Parallel()

	ft := NewNumeric[int64]([]int64{3, 2, 5, 1, 4})

	if got := ft.Len(); got != 5 {
		t.Fatalf("Len()=%d, want 5", got)
	}
	if got := ft.Total(); got != 15 {
		t.Fatalf("Total()=%d, want 15", got)
	}

	prefix, err := ft.PrefixSum(2)
	if err != nil || prefix != 10 {
		t.Fatalf("PrefixSum(2)=(%d,%v), want (10,nil)", prefix, err)
	}

	rangeSum, err := ft.RangeSum(1, 3)
	if err != nil || rangeSum != 8 {
		t.Fatalf("RangeSum(1,3)=(%d,%v), want (8,nil)", rangeSum, err)
	}

	if err := ft.Add(2, 10); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got, _ := ft.At(2); got != 15 {
		t.Fatalf("At(2)=%d, want 15", got)
	}

	if err := ft.Set(2, -7); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, _ := ft.RangeSum(1, 3); got != -4 {
		t.Fatalf("RangeSum(1,3)=%d, want -4", got)
	}
}

func TestEmptyAndErrors(t *testing.T) {
	t.Parallel()

	ft := NewNumeric[int64](nil)
	if ft.Len() != 0 || ft.Total() != 0 {
		t.Fatalf("empty tree has Len=%d Total=%d", ft.Len(), ft.Total())
	}

	if _, err := ft.At(0); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("At(0) error=%v, want ErrIndexOutOfRange", err)
	}
	if err := ft.Add(-1, 1); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("Add(-1) error=%v, want ErrIndexOutOfRange", err)
	}
	if _, err := ft.RangeSum(1, 0); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("RangeSum(1,0) error=%v, want ErrInvalidRange", err)
	}
}

func TestValuesReturnsCopy(t *testing.T) {
	t.Parallel()

	ft := NewNumeric[int64]([]int64{1, 2, 3})
	values := ft.Values()
	values[0] = 999

	got, err := ft.At(0)
	if err != nil || got != 1 {
		t.Fatalf("At(0)=(%d,%v), want (1,nil)", got, err)
	}
}

func TestRandomAgainstNaive(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42))
	const n = 256
	values := make([]int64, n)
	for i := range values {
		values[i] = int64(rng.Intn(2001) - 1000)
	}

	ft := NewNumeric[int64](values)
	naive := append([]int64(nil), values...)

	for step := 0; step < 10_000; step++ {
		switch rng.Intn(3) {
		case 0:
			i := rng.Intn(n)
			delta := int64(rng.Intn(201) - 100)
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
			var want int64
			for _, value := range naive[left : right+1] {
				want += value
			}
			got, err := ft.RangeSum(left, right)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("step=%d RangeSum(%d,%d)=%d, want %d", step, left, right, got, want)
			}
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	ft := NewNumeric[int64](make([]int64, 128))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for i := 0; i < 1_000; i++ {
				index := (i + offset) % ft.Len()
				if err := ft.Add(index, 1); err != nil {
					t.Errorf("Add: %v", err)
					return
				}
				if _, err := ft.PrefixSum(index); err != nil {
					t.Errorf("PrefixSum: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	if got := ft.Total(); got != 8_000 {
		t.Fatalf("Total()=%d, want 8000", got)
	}
}

func FuzzTreeAgainstNaive(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4}, uint8(17))
	f.Add([]byte{}, uint8(0))

	f.Fuzz(func(t *testing.T, data []byte, seed uint8) {
		if len(data) > 128 {
			data = data[:128]
		}

		values := make([]int64, len(data))
		for i, value := range data {
			values[i] = int64(int8(value))
		}
		ft := NewNumeric[int64](values)
		naive := append([]int64(nil), values...)

		if len(naive) == 0 {
			if ft.Total() != 0 {
				t.Fatalf("empty Total()=%d", ft.Total())
			}
			return
		}

		rng := rand.New(rand.NewSource(int64(seed)))
		for step := 0; step < 100; step++ {
			i := rng.Intn(len(naive))
			if step%2 == 0 {
				delta := int64(rng.Intn(31) - 15)
				if err := ft.Add(i, delta); err != nil {
					t.Fatal(err)
				}
				naive[i] += delta
			} else {
				left := rng.Intn(len(naive))
				right := left + rng.Intn(len(naive)-left)
				var want int64
				for _, value := range naive[left : right+1] {
					want += value
				}
				got, err := ft.RangeSum(left, right)
				if err != nil || got != want {
					t.Fatalf("RangeSum(%d,%d)=(%d,%v), want (%d,nil)", left, right, got, err, want)
				}
			}
		}
	})
}

func BenchmarkTreeRangeSum(b *testing.B) {
	values := make([]int64, 1<<20)
	for i := range values {
		values[i] = int64(i)
	}
	ft := NewNumeric[int64](values)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		left := i & ((1 << 19) - 1)
		if _, err := ft.RangeSum(left, left+(1<<18)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTreeAdd(b *testing.B) {
	ft := NewNumeric[int64](make([]int64, 1<<20))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ft.Add(i&((1<<20)-1), 1); err != nil {
			b.Fatal(err)
		}
	}
}

func TestAddZeroDelta(t *testing.T) {
	t.Parallel()

	ft := NewNumeric[int64]([]int64{1, 2, 3})
	initial, _ := ft.At(1)

	// Adding zero should not change value
	if err := ft.Add(1, 0); err != nil {
		t.Fatalf("Add(1, 0): %v", err)
	}

	got, _ := ft.At(1)
	if got != initial {
		t.Fatalf("Add(1, 0) changed value: got %d, want %d", got, initial)
	}
	if sum := ft.Total(); sum != 6 {
		t.Fatalf("Total after zero add: got %d, want 6", sum)
	}
}

func TestSetZeroDelta(t *testing.T) {
	t.Parallel()

	ft := NewNumeric[int64]([]int64{1, 2, 3})

	// Setting to same value should work
	if err := ft.Set(1, 2); err != nil {
		t.Fatalf("Set(1, 2): %v", err)
	}

	got, _ := ft.At(1)
	if got != 2 {
		t.Fatalf("Set(1, 2): got %d", got)
	}
}

func TestNilTreeOperations(t *testing.T) {
	t.Parallel()

	var ft *Tree[int64]

	if ft.Len() != 0 {
		t.Fatalf("Nil tree Len: got %d, want 0", ft.Len())
	}

	if ft.Total() != 0 {
		t.Fatalf("Nil tree Total: got %d, want 0", ft.Total())
	}

	if values := ft.Values(); values != nil {
		t.Fatalf("Nil tree Values: got %v, want nil", values)
	}

	if _, err := ft.At(0); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("Nil tree At(0): error %v, want ErrIndexOutOfRange", err)
	}
}

type metricsModel struct {
	Count int64
	Sum   float64
}

type metricsOperations struct{}

func (metricsOperations) Zero() metricsModel { return metricsModel{} }
func (metricsOperations) Add(a, b metricsModel) metricsModel {
	return metricsModel{Count: a.Count + b.Count, Sum: a.Sum + b.Sum}
}
func (metricsOperations) Sub(a, b metricsModel) metricsModel {
	return metricsModel{Count: a.Count - b.Count, Sum: a.Sum - b.Sum}
}

func TestInjectedOperationsWithDomainModel(t *testing.T) {
	t.Parallel()

	values := []metricsModel{{Count: 2, Sum: 1.5}, {Count: 3, Sum: 2.5}, {Count: 5, Sum: 4}}
	ft := NewWithOperations(values, metricsOperations{})

	got, err := ft.RangeSum(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := metricsModel{Count: 8, Sum: 6.5}
	if got != want {
		t.Fatalf("RangeSum(1,2)=%+v, want %+v", got, want)
	}

	if err := ft.Set(1, metricsModel{Count: 10, Sum: 8}); err != nil {
		t.Fatal(err)
	}
	if total := ft.Total(); total != (metricsModel{Count: 17, Sum: 13.5}) {
		t.Fatalf("Total()=%+v", total)
	}
}

func TestOperationFuncsInjection(t *testing.T) {
	t.Parallel()

	ops := OperationFuncs[int64]{
		ZeroFunc: func() int64 { return 0 },
		AddFunc:  func(a, b int64) int64 { return a + b },
		SubFunc:  func(a, b int64) int64 { return a - b },
	}
	ft := NewWithOperations([]int64{1, 2, 3}, ops)
	if got := ft.Total(); got != 6 {
		t.Fatalf("Total()=%d, want 6", got)
	}
}

func TestNilOperationsPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	NewWithOperations[int64](nil, nil)
}
