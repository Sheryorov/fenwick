package fenwick

import (
	"errors"
	"math/rand"
	"slices"
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

type Metrics struct {
	Count int64
	Sum   float64
}

type MetricsOperations struct{}

func (MetricsOperations) Zero() Metrics {
	return Metrics{}
}

func (MetricsOperations) Add(a, b Metrics) Metrics {
	return Metrics{
		Count: a.Count + b.Count,
		Sum:   a.Sum + b.Sum,
	}
}

func (MetricsOperations) Sub(a, b Metrics) Metrics {
	return Metrics{
		Count: a.Count - b.Count,
		Sum:   a.Sum - b.Sum,
	}
}

func TestTreeWithOperations(t *testing.T) {
	t.Parallel()

	values := []Metrics{
		{Count: 1, Sum: 1.5},
		{Count: 2, Sum: 2.5},
		{Count: 3, Sum: 4},
		{Count: 4, Sum: 6},
	}

	ft := NewWithOperations(values, MetricsOperations{})

	if got := ft.Len(); got != len(values) {
		t.Fatalf("Len()=%d want %d", got, len(values))
	}

	if got := ft.Total(); got != (Metrics{Count: 10, Sum: 14}) {
		t.Fatalf("Total()=%+v", got)
	}

	prefix, err := ft.PrefixSum(2)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 6, Sum: 8}); prefix != want {
		t.Fatalf("PrefixSum(2)=%+v want %+v", prefix, want)
	}

	sum, err := ft.RangeSum(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 9, Sum: 12.5}); sum != want {
		t.Fatalf("RangeSum(1,3)=%+v want %+v", sum, want)
	}

	value, err := ft.At(1)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 2, Sum: 2.5}); value != want {
		t.Fatalf("At(1)=%+v want %+v", value, want)
	}

	if err := ft.Add(1, Metrics{Count: 5, Sum: 1}); err != nil {
		t.Fatal(err)
	}

	value, err = ft.At(1)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 7, Sum: 3.5}); value != want {
		t.Fatalf("At(1) after Add=%+v want %+v", value, want)
	}

	if err := ft.Set(2, Metrics{Count: 10, Sum: 20}); err != nil {
		t.Fatal(err)
	}

	value, err = ft.At(2)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 10, Sum: 20}); value != want {
		t.Fatalf("At(2) after Set=%+v want %+v", value, want)
	}

	if got := ft.Total(); got != (Metrics{Count: 22, Sum: 31}) {
		t.Fatalf("Total() after mutations=%+v", got)
	}
}

func TestTreeWithOperationFuncs(t *testing.T) {
	t.Parallel()

	ops := OperationFuncs[Metrics]{
		ZeroFunc: func() Metrics {
			return Metrics{}
		},
		AddFunc: func(a, b Metrics) Metrics {
			return Metrics{
				Count: a.Count + b.Count,
				Sum:   a.Sum + b.Sum,
			}
		},
		SubFunc: func(a, b Metrics) Metrics {
			return Metrics{
				Count: a.Count - b.Count,
				Sum:   a.Sum - b.Sum,
			}
		},
	}

	ft := NewWithOperations(
		[]Metrics{
			{Count: 2, Sum: 3},
			{Count: 4, Sum: 5},
			{Count: 6, Sum: 7},
		},
		ops,
	)

	if got := ft.Total(); got != (Metrics{Count: 12, Sum: 15}) {
		t.Fatalf("Total()=%+v", got)
	}

	if err := ft.Add(0, Metrics{Count: 3, Sum: 2}); err != nil {
		t.Fatal(err)
	}

	sum, err := ft.RangeSum(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 9, Sum: 10}); sum != want {
		t.Fatalf("RangeSum(0,1)=%+v want %+v", sum, want)
	}
}

func TestTreeWithOperationsInputAndValuesIsolation(t *testing.T) {
	t.Parallel()

	values := []Metrics{
		{Count: 1, Sum: 1},
		{Count: 2, Sum: 2},
	}

	ft := NewWithOperations(values, MetricsOperations{})

	values[0] = Metrics{Count: 100, Sum: 100}

	got, err := ft.At(0)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 1, Sum: 1}); got != want {
		t.Fatalf("constructor retained input slice: got %+v want %+v", got, want)
	}

	copied := ft.Values()
	copied[0] = Metrics{Count: 200, Sum: 200}

	got, err = ft.At(0)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 1, Sum: 1}); got != want {
		t.Fatalf("Values exposed internal storage: got %+v want %+v", got, want)
	}
}

func TestTreeWithOperationsErrorsAndNoOp(t *testing.T) {
	t.Parallel()

	ft := NewWithOperations(
		[]Metrics{
			{Count: 1, Sum: 1},
			{Count: 2, Sum: 2},
		},
		MetricsOperations{},
	)

	for _, index := range []int{-1, 2} {
		if _, err := ft.At(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("At(%d) error=%v", index, err)
		}
		if err := ft.Add(index, Metrics{}); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Add(%d) error=%v", index, err)
		}
		if err := ft.Set(index, Metrics{}); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Set(%d) error=%v", index, err)
		}
		if _, err := ft.PrefixSum(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("PrefixSum(%d) error=%v", index, err)
		}
	}

	for _, r := range [][2]int{
		{-1, 0},
		{1, 0},
		{0, 2},
	} {
		if _, err := ft.RangeSum(r[0], r[1]); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("RangeSum(%d,%d) error=%v", r[0], r[1], err)
		}
	}

	before := ft.Values()

	if err := ft.Add(1, Metrics{}); err != nil {
		t.Fatal(err)
	}
	if err := ft.Set(1, before[1]); err != nil {
		t.Fatal(err)
	}

	if got := ft.Values(); !slices.Equal(got, before) {
		t.Fatalf("no-op changed values: got %+v want %+v", got, before)
	}
}

func TestEmptyTreeWithOperations(t *testing.T) {
	t.Parallel()

	ft := NewWithOperations[Metrics](nil, MetricsOperations{})

	if got := ft.Len(); got != 0 {
		t.Fatalf("Len()=%d", got)
	}
	if got := ft.Total(); got != (Metrics{}) {
		t.Fatalf("Total()=%+v", got)
	}
	if got := ft.Values(); len(got) != 0 {
		t.Fatalf("Values()=%+v", got)
	}
}

func TestShardedTreeWithOperations(t *testing.T) {
	t.Parallel()

	ft := NewShardedWithOperationsAndCount(
		[]Metrics{
			{Count: 1, Sum: 1},
			{Count: 2, Sum: 2},
			{Count: 3, Sum: 3},
			{Count: 4, Sum: 4},
			{Count: 5, Sum: 5},
			{Count: 6, Sum: 6},
		},
		3,
		MetricsOperations{},
	)

	if got := ft.ShardCount(); got != 3 {
		t.Fatalf("ShardCount()=%d want 3", got)
	}

	if got := ft.Total(); got != (Metrics{Count: 21, Sum: 21}) {
		t.Fatalf("Total()=%+v", got)
	}

	if got := ft.ExactTotal(); got != (Metrics{Count: 21, Sum: 21}) {
		t.Fatalf("ExactTotal()=%+v", got)
	}

	fast, err := ft.RangeSum(1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 14, Sum: 14}); fast != want {
		t.Fatalf("RangeSum(1,4)=%+v want %+v", fast, want)
	}

	exact, err := ft.ExactRangeSum(1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 14, Sum: 14}); exact != want {
		t.Fatalf("ExactRangeSum(1,4)=%+v want %+v", exact, want)
	}

	if err := ft.Add(2, Metrics{Count: 7, Sum: 0.5}); err != nil {
		t.Fatal(err)
	}

	if err := ft.Set(4, Metrics{Count: 20, Sum: 10}); err != nil {
		t.Fatal(err)
	}

	if got := ft.ExactTotal(); got != (Metrics{Count: 43, Sum: 26.5}) {
		t.Fatalf("ExactTotal() after mutations=%+v", got)
	}

	value, err := ft.At(4)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 20, Sum: 10}); value != want {
		t.Fatalf("At(4)=%+v want %+v", value, want)
	}
}

func TestShardedTreeWithOperationsReadPaths(t *testing.T) {
	t.Parallel()

	ft := NewShardedWithOperationsAndCount(
		[]Metrics{
			{Count: 1, Sum: 10},
			{Count: 2, Sum: 20},
			{Count: 3, Sum: 30},
			{Count: 4, Sum: 40},
			{Count: 5, Sum: 50},
			{Count: 6, Sum: 60},
		},
		3,
		MetricsOperations{},
	)

	checks := []struct {
		left  int
		right int
		want  Metrics
	}{
		{0, 0, Metrics{Count: 1, Sum: 10}},
		{0, 1, Metrics{Count: 3, Sum: 30}},
		{1, 4, Metrics{Count: 14, Sum: 140}},
		{2, 5, Metrics{Count: 18, Sum: 180}},
	}

	for _, tc := range checks {
		fast, err := ft.RangeSum(tc.left, tc.right)
		if err != nil {
			t.Fatal(err)
		}
		if fast != tc.want {
			t.Fatalf(
				"RangeSum(%d,%d)=%+v want %+v",
				tc.left,
				tc.right,
				fast,
				tc.want,
			)
		}

		exact, err := ft.ExactRangeSum(tc.left, tc.right)
		if err != nil {
			t.Fatal(err)
		}
		if exact != tc.want {
			t.Fatalf(
				"ExactRangeSum(%d,%d)=%+v want %+v",
				tc.left,
				tc.right,
				exact,
				tc.want,
			)
		}
	}

	wantPrefixes := []Metrics{
		{Count: 1, Sum: 10},
		{Count: 3, Sum: 30},
		{Count: 6, Sum: 60},
		{Count: 10, Sum: 100},
		{Count: 15, Sum: 150},
		{Count: 21, Sum: 210},
	}

	for index, want := range wantPrefixes {
		fast, err := ft.PrefixSum(index)
		if err != nil {
			t.Fatal(err)
		}
		if fast != want {
			t.Fatalf("PrefixSum(%d)=%+v want %+v", index, fast, want)
		}

		exact, err := ft.ExactPrefixSum(index)
		if err != nil {
			t.Fatal(err)
		}
		if exact != want {
			t.Fatalf(
				"ExactPrefixSum(%d)=%+v want %+v",
				index,
				exact,
				want,
			)
		}
	}
}

func TestShardedTreeWithOperationFuncs(t *testing.T) {
	t.Parallel()

	ops := OperationFuncs[Metrics]{
		ZeroFunc: func() Metrics {
			return Metrics{}
		},
		AddFunc: func(a, b Metrics) Metrics {
			return Metrics{
				Count: a.Count + b.Count,
				Sum:   a.Sum + b.Sum,
			}
		},
		SubFunc: func(a, b Metrics) Metrics {
			return Metrics{
				Count: a.Count - b.Count,
				Sum:   a.Sum - b.Sum,
			}
		},
	}

	ft := NewShardedWithOperationsAndCount(
		[]Metrics{
			{Count: 1, Sum: 2},
			{Count: 3, Sum: 4},
			{Count: 5, Sum: 6},
			{Count: 7, Sum: 8},
		},
		2,
		ops,
	)

	if got := ft.ExactTotal(); got != (Metrics{Count: 16, Sum: 20}) {
		t.Fatalf("ExactTotal()=%+v", got)
	}

	if err := ft.Add(3, Metrics{Count: 1, Sum: 2}); err != nil {
		t.Fatal(err)
	}

	if got := ft.ExactTotal(); got != (Metrics{Count: 17, Sum: 22}) {
		t.Fatalf("ExactTotal() after Add=%+v", got)
	}
}

func TestShardedTreeWithOperationsErrorsAndNoOp(t *testing.T) {
	t.Parallel()

	ft := NewShardedWithOperationsAndCount(
		[]Metrics{
			{Count: 1, Sum: 1},
			{Count: 2, Sum: 2},
			{Count: 3, Sum: 3},
		},
		2,
		MetricsOperations{},
	)

	for _, index := range []int{-1, 3} {
		if _, err := ft.At(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("At(%d) error=%v", index, err)
		}
		if err := ft.Add(index, Metrics{}); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Add(%d) error=%v", index, err)
		}
		if err := ft.Set(index, Metrics{}); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Set(%d) error=%v", index, err)
		}
		if _, err := ft.PrefixSum(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("PrefixSum(%d) error=%v", index, err)
		}
		if _, err := ft.ExactPrefixSum(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("ExactPrefixSum(%d) error=%v", index, err)
		}
	}

	for _, r := range [][2]int{
		{-1, 0},
		{2, 1},
		{0, 3},
	} {
		if _, err := ft.RangeSum(r[0], r[1]); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("RangeSum(%d,%d) error=%v", r[0], r[1], err)
		}
		if _, err := ft.ExactRangeSum(r[0], r[1]); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("ExactRangeSum(%d,%d) error=%v", r[0], r[1], err)
		}
	}

	before := ft.Values()

	if err := ft.Add(1, Metrics{}); err != nil {
		t.Fatal(err)
	}
	if err := ft.Set(1, before[1]); err != nil {
		t.Fatal(err)
	}

	if got := ft.Values(); !slices.Equal(got, before) {
		t.Fatalf("no-op changed values: got %+v want %+v", got, before)
	}

	copied := ft.Values()
	copied[0] = Metrics{Count: 100, Sum: 100}

	got, err := ft.At(0)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Metrics{Count: 1, Sum: 1}); got != want {
		t.Fatalf("Values exposed internal storage: got %+v want %+v", got, want)
	}
}

func TestIntValueOperations(t *testing.T) {
	t.Parallel()

	a := Int(10)
	b := Int(4)

	if got := a.Add(b); got != Int(14) {
		t.Fatalf("Add()=%v want %v", got, Int(14))
	}

	if got := a.Sub(b); got != Int(6) {
		t.Fatalf("Sub()=%v want %v", got, Int(6))
	}

	if got := a.Zero(); got != Int(0) {
		t.Fatalf("Zero()=%v want %v", got, Int(0))
	}
}

func TestInt64ValueOperations(t *testing.T) {
	t.Parallel()

	a := Int64(100)
	b := Int64(35)

	if got := a.Add(b); got != Int64(135) {
		t.Fatalf("Add()=%v want %v", got, Int64(135))
	}

	if got := a.Sub(b); got != Int64(65) {
		t.Fatalf("Sub()=%v want %v", got, Int64(65))
	}

	if got := a.Zero(); got != Int64(0) {
		t.Fatalf("Zero()=%v want %v", got, Int64(0))
	}
}

func TestUintValueOperations(t *testing.T) {
	t.Parallel()

	a := Uint(10)
	b := Uint(4)

	if got := a.Add(b); got != Uint(14) {
		t.Fatalf("Add()=%v want %v", got, Uint(14))
	}

	if got := a.Sub(b); got != Uint(6) {
		t.Fatalf("Sub()=%v want %v", got, Uint(6))
	}

	if got := a.Zero(); got != Uint(0) {
		t.Fatalf("Zero()=%v want %v", got, Uint(0))
	}
}

func TestUint64ValueOperations(t *testing.T) {
	t.Parallel()

	a := Uint64(100)
	b := Uint64(35)

	if got := a.Add(b); got != Uint64(135) {
		t.Fatalf("Add()=%v want %v", got, Uint64(135))
	}

	if got := a.Sub(b); got != Uint64(65) {
		t.Fatalf("Sub()=%v want %v", got, Uint64(65))
	}

	if got := a.Zero(); got != Uint64(0) {
		t.Fatalf("Zero()=%v want %v", got, Uint64(0))
	}
}

func TestFloat32ValueOperations(t *testing.T) {
	t.Parallel()

	a := Float32(10.5)
	b := Float32(4.25)

	if got := a.Add(b); got != Float32(14.75) {
		t.Fatalf("Add()=%v want %v", got, Float32(14.75))
	}

	if got := a.Sub(b); got != Float32(6.25) {
		t.Fatalf("Sub()=%v want %v", got, Float32(6.25))
	}

	if got := a.Zero(); got != Float32(0) {
		t.Fatalf("Zero()=%v want %v", got, Float32(0))
	}
}

func TestFloat64ValueOperations(t *testing.T) {
	t.Parallel()

	a := Float64(100.5)
	b := Float64(35.25)

	if got := a.Add(b); got != Float64(135.75) {
		t.Fatalf("Add()=%v want %v", got, Float64(135.75))
	}

	if got := a.Sub(b); got != Float64(65.25) {
		t.Fatalf("Sub()=%v want %v", got, Float64(65.25))
	}

	if got := a.Zero(); got != Float64(0) {
		t.Fatalf("Zero()=%v want %v", got, Float64(0))
	}
}

func TestValueWrappersThroughTree(t *testing.T) {
	t.Parallel()

	t.Run("Int", func(t *testing.T) {
		t.Parallel()

		tree := New([]Int{1, 2, 3})

		if got := tree.Total(); got != Int(6) {
			t.Fatalf("Total()=%v want %v", got, Int(6))
		}

		if err := tree.Add(1, Int(5)); err != nil {
			t.Fatal(err)
		}

		if got := tree.Total(); got != Int(11) {
			t.Fatalf("Total() after Add=%v want %v", got, Int(11))
		}

		if err := tree.Set(2, Int(10)); err != nil {
			t.Fatal(err)
		}

		if got := tree.Total(); got != Int(18) {
			t.Fatalf("Total() after Set=%v want %v", got, Int(18))
		}
	})

	t.Run("Int64", func(t *testing.T) {
		t.Parallel()

		tree := New([]Int64{10, 20, 30})

		sum, err := tree.RangeSum(1, 2)
		if err != nil {
			t.Fatal(err)
		}

		if sum != Int64(50) {
			t.Fatalf("RangeSum()=%v want %v", sum, Int64(50))
		}
	})

	t.Run("Uint", func(t *testing.T) {
		t.Parallel()

		tree := New([]Uint{1, 2, 3})

		if got := tree.Total(); got != Uint(6) {
			t.Fatalf("Total()=%v want %v", got, Uint(6))
		}

		if err := tree.Set(1, Uint(5)); err != nil {
			t.Fatal(err)
		}

		if got := tree.Total(); got != Uint(9) {
			t.Fatalf("Total() after Set=%v want %v", got, Uint(9))
		}
	})

	t.Run("Uint64", func(t *testing.T) {
		t.Parallel()

		tree := New([]Uint64{10, 20, 30})

		if err := tree.Add(0, Uint64(5)); err != nil {
			t.Fatal(err)
		}

		if got := tree.Total(); got != Uint64(65) {
			t.Fatalf("Total()=%v want %v", got, Uint64(65))
		}
	})

	t.Run("Float32", func(t *testing.T) {
		t.Parallel()

		tree := New([]Float32{1.25, 2.5, 3.75})

		if got := tree.Total(); got != Float32(7.5) {
			t.Fatalf("Total()=%v want %v", got, Float32(7.5))
		}
	})

	t.Run("Float64", func(t *testing.T) {
		t.Parallel()

		tree := New([]Float64{1.5, 2.25, 3.75})

		sum, err := tree.RangeSum(0, 1)
		if err != nil {
			t.Fatal(err)
		}

		if sum != Float64(3.75) {
			t.Fatalf("RangeSum()=%v want %v", sum, Float64(3.75))
		}
	})
}

func TestValueWrappersThroughShardedTree(t *testing.T) {
	t.Parallel()

	tree := NewShardedWithCount(
		[]Int64{1, 2, 3, 4, 5, 6},
		3,
	)

	if got := tree.ExactTotal(); got != Int64(21) {
		t.Fatalf("ExactTotal()=%v want %v", got, Int64(21))
	}

	if err := tree.Add(2, Int64(10)); err != nil {
		t.Fatal(err)
	}

	if got := tree.ExactTotal(); got != Int64(31) {
		t.Fatalf("ExactTotal() after Add=%v want %v", got, Int64(31))
	}

	if err := tree.Set(4, Int64(20)); err != nil {
		t.Fatal(err)
	}

	if got := tree.ExactTotal(); got != Int64(46) {
		t.Fatalf("ExactTotal() after Set=%v want %v", got, Int64(46))
	}

	sum, err := tree.ExactRangeSum(1, 4)
	if err != nil {
		t.Fatal(err)
	}

	if sum != Int64(39) {
		t.Fatalf("ExactRangeSum()=%v want %v", sum, Int64(39))
	}
}

func TestValueOperationsPanicOnMismatchedConcreteType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "Int.Add",
			fn: func() {
				_ = Int(1).Add(Int64(2))
			},
		},
		{
			name: "Int.Sub",
			fn: func() {
				_ = Int(1).Sub(Int64(2))
			},
		},
		{
			name: "Int64.Add",
			fn: func() {
				_ = Int64(1).Add(Int(2))
			},
		},
		{
			name: "Int64.Sub",
			fn: func() {
				_ = Int64(1).Sub(Int(2))
			},
		},
		{
			name: "Uint.Add",
			fn: func() {
				_ = Uint(1).Add(Uint64(2))
			},
		},
		{
			name: "Uint.Sub",
			fn: func() {
				_ = Uint(1).Sub(Uint64(2))
			},
		},
		{
			name: "Uint64.Add",
			fn: func() {
				_ = Uint64(1).Add(Uint(2))
			},
		},
		{
			name: "Uint64.Sub",
			fn: func() {
				_ = Uint64(1).Sub(Uint(2))
			},
		},
		{
			name: "Float32.Add",
			fn: func() {
				_ = Float32(1).Add(Float64(2))
			},
		},
		{
			name: "Float32.Sub",
			fn: func() {
				_ = Float32(1).Sub(Float64(2))
			},
		},
		{
			name: "Float64.Add",
			fn: func() {
				_ = Float64(1).Add(Float32(2))
			},
		},
		{
			name: "Float64.Sub",
			fn: func() {
				_ = Float64(1).Sub(Float32(2))
			},
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic")
				}
			}()

			tc.fn()
		})
	}
}

func TestUnsignedSubtractionUnderflowUsesGoSemantics(t *testing.T) {
	t.Parallel()

	got := Uint(1).Sub(Uint(2))
	want := Uint(^uint(0))

	if got != want {
		t.Fatalf("Uint underflow=%v want %v", got, want)
	}

	got64 := Uint64(1).Sub(Uint64(2))
	want64 := Uint64(^uint64(0))

	if got64 != want64 {
		t.Fatalf("Uint64 underflow=%v want %v", got64, want64)
	}
}
