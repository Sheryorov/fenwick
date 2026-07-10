package fenwick

// Int64 is a Value wrapper for int64 that implements arithmetic operations.
type Int64 int64

func (i Int64) Add(other Value) Value {
	return i + other.(Int64)
}

func (i Int64) Sub(other Value) Value {
	return i - other.(Int64)
}

func (i Int64) Zero() Value {
	return Int64(0)
}

// Float64 is a Value wrapper for float64 that implements arithmetic operations.
type Float64 float64

func (f Float64) Add(other Value) Value {
	return f + other.(Float64)
}

func (f Float64) Sub(other Value) Value {
	return f - other.(Float64)
}

func (f Float64) Zero() Value {
	return Float64(0)
}

// Int is a Value wrapper for int that implements arithmetic operations.
type Int int

func (i Int) Add(other Value) Value {
	return i + other.(Int)
}

func (i Int) Sub(other Value) Value {
	return i - other.(Int)
}

func (i Int) Zero() Value {
	return Int(0)
}

// Uint64 is a Value wrapper for uint64 that implements arithmetic operations.
type Uint64 uint64

func (u Uint64) Add(other Value) Value {
	return u + other.(Uint64)
}

func (u Uint64) Sub(other Value) Value {
	return u - other.(Uint64)
}

func (u Uint64) Zero() Value {
	return Uint64(0)
}

// Uint is a Value wrapper for uint that implements arithmetic operations.
type Uint uint

func (u Uint) Add(other Value) Value {
	return u + other.(Uint)
}

func (u Uint) Sub(other Value) Value {
	return u - other.(Uint)
}

func (u Uint) Zero() Value {
	return Uint(0)
}

// Float32 is a Value wrapper for float32 that implements arithmetic operations.
type Float32 float32

func (f Float32) Add(other Value) Value {
	return f + other.(Float32)
}

func (f Float32) Sub(other Value) Value {
	return f - other.(Float32)
}

func (f Float32) Zero() Value {
	return Float32(0)
}
