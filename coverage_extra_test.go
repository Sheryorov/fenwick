package fenwick

import (
	"errors"
	"runtime"
	"slices"
	"testing"
)

func TestTreeAllErrorAndNoOpBranches(t *testing.T) {
	t.Parallel()

	var nilTree *Tree[int64]
	if err := nilTree.Add(0, 1); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil Add error=%v", err)
	}
	if err := nilTree.Set(0, 1); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil Set error=%v", err)
	}
	if _, err := nilTree.PrefixSum(0); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil PrefixSum error=%v", err)
	}
	if _, err := nilTree.RangeSum(0, 0); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("nil RangeSum error=%v", err)
	}

	ft := NewNumeric([]int64{10, 20, 30})
	for _, index := range []int{-1, 3} {
		if _, err := ft.At(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("At(%d) error=%v", index, err)
		}
		if err := ft.Add(index, 1); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Add(%d) error=%v", index, err)
		}
		if err := ft.Set(index, 1); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Set(%d) error=%v", index, err)
		}
		if _, err := ft.PrefixSum(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("PrefixSum(%d) error=%v", index, err)
		}
	}

	invalidRanges := [][2]int{
		{-1, 0},
		{1, 0},
		{0, 3},
	}
	for _, r := range invalidRanges {
		if _, err := ft.RangeSum(r[0], r[1]); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("RangeSum(%d,%d) error=%v", r[0], r[1], err)
		}
	}

	before := ft.Values()

	if err := ft.Add(1, 0); err != nil {
		t.Fatal(err)
	}
	if err := ft.Set(1, 20); err != nil {
		t.Fatal(err)
	}
	if got := ft.Values(); !slices.Equal(got, before) {
		t.Fatalf("no-op mutations changed values: got %v want %v", got, before)
	}

	copyValues := ft.Values()
	copyValues[0] = -999

	got, err := ft.At(0)
	if err != nil || got != 10 {
		t.Fatalf("Values exposed internal storage: At(0)=(%d,%v)", got, err)
	}
}

func TestTreeEmptyAndAllNumberKinds(t *testing.T) {
	t.Parallel()

	empty := NewNumeric[int64](nil)
	if empty.Len() != 0 || empty.Total() != 0 || len(empty.Values()) != 0 {
		t.Fatalf("bad empty tree state")
	}

	type Small int8

	ft8 := NewNumeric([]Small{1, 2, 3})
	if got := ft8.Total(); got != 6 {
		t.Fatalf("int8 Total=%v", got)
	}

	ft32 := NewNumeric([]float32{1.25, 2.5})
	if got := ft32.Total(); got != float32(3.75) {
		t.Fatalf("float32 Total=%v", got)
	}
}

func TestNewShardedAutomaticAndConstructorEdges(t *testing.T) {
	t.Parallel()

	values := []int64{1, 2, 3}

	auto := NewNumericSharded(values)
	if auto.Len() != len(values) {
		t.Fatalf("Len=%d", auto.Len())
	}

	wantMax := runtime.GOMAXPROCS(0) * 4
	if wantMax > len(values) {
		wantMax = len(values)
	}

	if auto.ShardCount() != wantMax {
		t.Fatalf("ShardCount=%d want %d", auto.ShardCount(), wantMax)
	}

	capped := NewNumericShardedWithCount(values, 100)
	if capped.ShardCount() != len(values) {
		t.Fatalf("capped ShardCount=%d", capped.ShardCount())
	}

	empty := NewNumericShardedWithCount[int64](nil, 2)
	if empty.Len() != 0 ||
		empty.ShardCount() != 0 ||
		empty.Total() != 0 ||
		empty.ExactTotal() != 0 ||
		empty.Values() != nil {
		t.Fatalf("bad empty sharded state")
	}
}

func TestShardedAllErrorAndNoOpBranches(t *testing.T) {
	t.Parallel()

	var nilTree *ShardedTree[int64]

	if _, err := nilTree.At(0); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil At: %v", err)
	}
	if err := nilTree.Add(0, 1); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil Add: %v", err)
	}
	if err := nilTree.Set(0, 1); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil Set: %v", err)
	}
	if _, err := nilTree.PrefixSum(0); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil PrefixSum: %v", err)
	}
	if _, err := nilTree.RangeSum(0, 0); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("nil RangeSum: %v", err)
	}
	if _, err := nilTree.ExactPrefixSum(0); !errors.Is(err, ErrIndexOutOfRange) {
		t.Fatalf("nil ExactPrefixSum: %v", err)
	}
	if _, err := nilTree.ExactRangeSum(0, 0); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("nil ExactRangeSum: %v", err)
	}

	ft := NewNumericShardedWithCount(
		[]int64{1, 2, 3, 4, 5, 6},
		3,
	)

	for _, index := range []int{-1, 6} {
		if _, err := ft.At(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("At(%d): %v", index, err)
		}
		if err := ft.Add(index, 1); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Add(%d): %v", index, err)
		}
		if err := ft.Set(index, 1); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("Set(%d): %v", index, err)
		}
		if _, err := ft.PrefixSum(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("PrefixSum(%d): %v", index, err)
		}
		if _, err := ft.ExactPrefixSum(index); !errors.Is(err, ErrIndexOutOfRange) {
			t.Fatalf("ExactPrefixSum(%d): %v", index, err)
		}
	}

	invalidRanges := [][2]int{
		{-1, 0},
		{2, 1},
		{0, 6},
	}
	for _, r := range invalidRanges {
		if _, err := ft.RangeSum(r[0], r[1]); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("RangeSum%v: %v", r, err)
		}
		if _, err := ft.ExactRangeSum(r[0], r[1]); !errors.Is(err, ErrInvalidRange) {
			t.Fatalf("ExactRangeSum%v: %v", r, err)
		}
	}

	before := ft.Values()

	if err := ft.Add(2, 0); err != nil {
		t.Fatal(err)
	}
	if err := ft.Set(2, before[2]); err != nil {
		t.Fatal(err)
	}
	if got := ft.Values(); !slices.Equal(got, before) {
		t.Fatalf("no-op changed values")
	}

	copied := ft.Values()
	copied[0] = 999

	if got, _ := ft.At(0); got != 1 {
		t.Fatalf("Values exposed storage: %d", got)
	}
}

func TestShardedReadPathsSameAndCrossShard(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount(
		[]int64{1, 2, 3, 4, 5, 6},
		3,
	)

	checks := []struct {
		left  int
		right int
		want  int64
	}{
		{0, 0, 1},
		{0, 1, 3},
		{1, 4, 14},
		{2, 5, 18},
	}

	for _, tc := range checks {
		fast, err := ft.RangeSum(tc.left, tc.right)
		if err != nil || fast != tc.want {
			t.Fatalf(
				"fast [%d,%d]=(%d,%v), want %d",
				tc.left,
				tc.right,
				fast,
				err,
				tc.want,
			)
		}

		exact, err := ft.ExactRangeSum(tc.left, tc.right)
		if err != nil || exact != tc.want {
			t.Fatalf(
				"exact [%d,%d]=(%d,%v), want %d",
				tc.left,
				tc.right,
				exact,
				err,
				tc.want,
			)
		}
	}

	for index, want := range []int64{1, 3, 6, 10, 15, 21} {
		fast, err := ft.PrefixSum(index)
		if err != nil || fast != want {
			t.Fatalf(
				"PrefixSum(%d)=(%d,%v), want %d",
				index,
				fast,
				err,
				want,
			)
		}

		exact, err := ft.ExactPrefixSum(index)
		if err != nil || exact != want {
			t.Fatalf(
				"ExactPrefixSum(%d)=(%d,%v), want %d",
				index,
				exact,
				err,
				want,
			)
		}
	}
}

func TestShardedFloatAndNamedType(t *testing.T) {
	t.Parallel()

	ft := NewNumericShardedWithCount(
		[]float64{1.5, -0.5, 2},
		2,
	)

	if got := ft.ExactTotal(); got != 3 {
		t.Fatalf("float total=%v", got)
	}

	if err := ft.Set(1, 0.5); err != nil {
		t.Fatal(err)
	}

	if got, err := ft.ExactRangeSum(0, 2); err != nil || got != 4 {
		t.Fatalf("float range=(%v,%v)", got, err)
	}

	type Score int32

	named := NewNumericShardedWithCount(
		[]Score{1, 2, 3},
		2,
	)
	if got := named.Total(); got != 6 {
		t.Fatalf("named total=%v", got)
	}
}
