package fenwick

// Operations defines the additive algebra used by a Fenwick tree.
//
// It is intentionally separate from the stored model type. This allows domain
// models to remain free of Fenwick-specific methods and lets callers inject
// aggregation behavior at construction time.
//
// Implementations must provide an additive identity and satisfy the usual
// additive group laws required by range sums: Add must be associative, Zero
// must be an identity, and Sub must undo Add.
type Operations[T any] interface {
	Zero() T
	Add(a, b T) T
	Sub(a, b T) T
}

// OperationFuncs adapts functions to Operations. It is convenient when the
// aggregation behavior is local to the caller and does not warrant a named
// operations type.
type OperationFuncs[T any] struct {
	ZeroFunc func() T
	AddFunc  func(a, b T) T
	SubFunc  func(a, b T) T
}

func (f OperationFuncs[T]) Zero() T      { return f.ZeroFunc() }
func (f OperationFuncs[T]) Add(a, b T) T { return f.AddFunc(a, b) }
func (f OperationFuncs[T]) Sub(a, b T) T { return f.SubFunc(a, b) }

// Value is the legacy self-describing model interface. Types implementing it
// can be passed to New and NewSharded directly. New code that should keep
// domain models independent from this package can use NewWithOperations and
// inject an Operations implementation instead.
type Value interface {
	Add(other Value) Value
	Sub(other Value) Value
	Zero() Value
}

type valueOperations[T Value] struct{ zero T }

func (o valueOperations[T]) Zero() T    { return o.zero }
func (valueOperations[T]) Add(a, b T) T { return a.Add(b).(T) }
func (valueOperations[T]) Sub(a, b T) T { return a.Sub(b).(T) }

// Number contains the built-in signed integer and floating-point families.
// Unsigned types are deliberately excluded because Set may require a negative
// delta when replacing a larger value with a smaller one.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~float32 | ~float64
}

// NumericOperations implements Operations for signed numeric types.
type NumericOperations[T Number] struct{}

func (NumericOperations[T]) Zero() T      { return 0 }
func (NumericOperations[T]) Add(a, b T) T { return a + b }
func (NumericOperations[T]) Sub(a, b T) T { return a - b }
