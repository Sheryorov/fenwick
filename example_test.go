package fenwick_test

import (
	"fmt"

	"github.com/Sheryorov/fenwick"
)

func ExampleTree() {
	ft := fenwick.New[int64]([]int64{3, 2, 5, 1, 4})

	sum, _ := ft.RangeSum(1, 3)
	fmt.Println(sum)

	_ = ft.Set(2, 10)
	sum, _ = ft.RangeSum(1, 3)
	fmt.Println(sum)

	// Output:
	// 8
	// 13
}
