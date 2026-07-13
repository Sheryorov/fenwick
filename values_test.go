package fenwick

import "testing"

func TestValueImplementations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "Int",
			test: func(t *testing.T) {
				a, b := Int(10), Int(4)

				if got := a.Add(b); got != Int(14) {
					t.Fatalf("Add() = %v, want 14", got)
				}
				if got := a.Sub(b); got != Int(6) {
					t.Fatalf("Sub() = %v, want 6", got)
				}
				if got := a.Zero(); got != Int(0) {
					t.Fatalf("Zero() = %v, want 0", got)
				}
			},
		},
		{
			name: "Int64",
			test: func(t *testing.T) {
				a, b := Int64(100), Int64(35)

				if got := a.Add(b); got != Int64(135) {
					t.Fatalf("Add() = %v, want 135", got)
				}
				if got := a.Sub(b); got != Int64(65) {
					t.Fatalf("Sub() = %v, want 65", got)
				}
				if got := a.Zero(); got != Int64(0) {
					t.Fatalf("Zero() = %v, want 0", got)
				}
			},
		},
		{
			name: "Uint",
			test: func(t *testing.T) {
				a, b := Uint(10), Uint(4)

				if got := a.Add(b); got != Uint(14) {
					t.Fatalf("Add() = %v, want 14", got)
				}
				if got := a.Sub(b); got != Uint(6) {
					t.Fatalf("Sub() = %v, want 6", got)
				}
				if got := a.Zero(); got != Uint(0) {
					t.Fatalf("Zero() = %v, want 0", got)
				}
			},
		},
		{
			name: "Uint64",
			test: func(t *testing.T) {
				a, b := Uint64(100), Uint64(35)

				if got := a.Add(b); got != Uint64(135) {
					t.Fatalf("Add() = %v, want 135", got)
				}
				if got := a.Sub(b); got != Uint64(65) {
					t.Fatalf("Sub() = %v, want 65", got)
				}
				if got := a.Zero(); got != Uint64(0) {
					t.Fatalf("Zero() = %v, want 0", got)
				}
			},
		},
		{
			name: "Float32",
			test: func(t *testing.T) {
				a, b := Float32(10.5), Float32(4.25)

				if got := a.Add(b); got != Float32(14.75) {
					t.Fatalf("Add() = %v, want 14.75", got)
				}
				if got := a.Sub(b); got != Float32(6.25) {
					t.Fatalf("Sub() = %v, want 6.25", got)
				}
				if got := a.Zero(); got != Float32(0) {
					t.Fatalf("Zero() = %v, want 0", got)
				}
			},
		},
		{
			name: "Float64",
			test: func(t *testing.T) {
				a, b := Float64(100.5), Float64(35.25)

				if got := a.Add(b); got != Float64(135.75) {
					t.Fatalf("Add() = %v, want 135.75", got)
				}
				if got := a.Sub(b); got != Float64(65.25) {
					t.Fatalf("Sub() = %v, want 65.25", got)
				}
				if got := a.Zero(); got != Float64(0) {
					t.Fatalf("Zero() = %v, want 0", got)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.test(t)
		})
	}
}
