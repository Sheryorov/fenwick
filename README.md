# Fenwick Tree for Go
[![Build](https://github.com/Sheryorov/fenwick/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/Sheryorov/fenwick/actions/workflows/build.yml)
[![Tests](https://github.com/Sheryorov/fenwick/actions/workflows/tests.yml/badge.svg?branch=master)](https://github.com/Sheryorov/fenwick/actions/workflows/tests.yml)
[![codecov](https://codecov.io/gh/Sheryorov/fenwick/branch/master/graph/badge.svg)](https://codecov.io/gh/Sheryorov/fenwick)
[![Go Reference](https://pkg.go.dev/badge/github.com/Sheryorov/fenwick.svg)](https://pkg.go.dev/github.com/Sheryorov/fenwick)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Sheryorov/fenwick)](https://github.com/Sheryorov/fenwick/blob/master/go.mod)
[![GitHub Release](https://img.shields.io/github/v/release/Sheryorov/fenwick)](https://github.com/Sheryorov/fenwick/releases)
[![GitHub issues](https://img.shields.io/github/issues/Sheryorov/fenwick)](https://github.com/Sheryorov/fenwick/issues)
[![GitHub stars](https://img.shields.io/github/stars/Sheryorov/fenwick?style=social)](https://github.com/Sheryorov/fenwick/stargazers)

A zero-based, concurrency-safe Fenwick tree (Binary Indexed Tree) for Go.

The package supports three integration styles:

1. **Built-in signed numeric types** through `NewNumeric`.
2. **Dependency injection** through `Operations[T]` and `NewWithOperations`.
3. **Legacy self-describing models** through the `Value` interface and `New`.

It also provides a sharded implementation for write-heavy concurrent workloads.

## Installation

```bash
go get github.com/Sheryorov/fenwick
```

## Core capabilities

- `O(n)` construction
- `O(log n)` point updates
- `O(log n)` prefix and range queries
- zero-based public indexes
- inclusive ranges
- concurrent-safe standard tree
- sharded tree for parallel updates
- fast and exact sharded query modes
- injectable aggregation behavior for domain models

## 1. Numeric values

Use `NewNumeric` for signed integer and floating-point types.

```go
package main

import (
    "fmt"

    "github.com/Sheryorov/fenwick"
)

func main() {
    tree := fenwick.NewNumeric([]int64{3, 2, 5, 1, 4})

    sum, err := tree.RangeSum(1, 3)
    if err != nil {
        panic(err)
    }

    fmt.Println(sum) // 8

    if err := tree.Set(2, 10); err != nil {
        panic(err)
    }

    fmt.Println(tree.Total()) // 20
}
```

Supported numeric families:

- `int`, `int8`, `int16`, `int32`, `int64`
- `float32`, `float64`
- named types with one of those underlying types

Unsigned integers are intentionally excluded because `Set` may require a negative delta.

## 2. Injecting domain-model operations

Domain models do not need to import or implement anything from this package.
Aggregation behavior is injected separately through `Operations[T]`.

```go
type Operations[T any] interface {
    Zero() T
    Add(a, b T) T
    Sub(a, b T) T
}
```

Example domain model:

```go
package main

import (
    "fmt"

    "github.com/Sheryorov/fenwick"
)

type Metrics struct {
    Requests int64
    Duration float64
}

type MetricsOperations struct{}

func (MetricsOperations) Zero() Metrics {
    return Metrics{}
}

func (MetricsOperations) Add(a, b Metrics) Metrics {
    return Metrics{
        Requests: a.Requests + b.Requests,
        Duration: a.Duration + b.Duration,
    }
}

func (MetricsOperations) Sub(a, b Metrics) Metrics {
    return Metrics{
        Requests: a.Requests - b.Requests,
        Duration: a.Duration - b.Duration,
    }
}

func main() {
    tree := fenwick.NewWithOperations(
        []Metrics{
            {Requests: 10, Duration: 1.2},
            {Requests: 20, Duration: 2.8},
            {Requests: 15, Duration: 1.5},
        },
        MetricsOperations{},
    )

    result, err := tree.RangeSum(0, 1)
    if err != nil {
        panic(err)
    }

    fmt.Printf("requests=%d duration=%.1f\n", result.Requests, result.Duration)
    // requests=30 duration=4.0
}
```

This is the recommended integration style when domain models should remain independent from the Fenwick package.

### Function-based injection

For small integrations, use `OperationFuncs[T]` instead of defining a named operations type.

```go
ops := fenwick.OperationFuncs[int64]{
    ZeroFunc: func() int64 { return 0 },
    AddFunc:  func(a, b int64) int64 { return a + b },
    SubFunc:  func(a, b int64) int64 { return a - b },
}

tree := fenwick.NewWithOperations([]int64{1, 2, 3}, ops)
```

## 3. Legacy `Value` models

A model may implement `Value` directly:

```go
type Value interface {
    Add(other Value) Value
    Sub(other Value) Value
    Zero() Value
}
```

Then it can be constructed with `New`:

```go
tree := fenwick.New([]fenwick.Int64{1, 2, 3})
```

Built-in wrappers are provided:

- `fenwick.Int`
- `fenwick.Int64`
- `fenwick.Float32`
- `fenwick.Float64`
- `fenwick.Uint`
- `fenwick.Uint64`

For new domain code, injected `Operations[T]` is usually safer because it avoids runtime type assertions inside the model.

## Standard tree API

```go
type Tree[T any]
```

| Method | Description | Complexity |
|---|---|---:|
| `NewNumeric(values)` | Build a numeric tree | `O(n)` |
| `NewWithOperations(values, ops)` | Build with injected behavior | `O(n)` |
| `New(values)` | Build from `Value` models | `O(n)` |
| `Len()` | Number of values | `O(1)` |
| `At(index)` | Read one value | `O(1)` |
| `Add(index, delta)` | Add a delta | `O(log n)` |
| `Set(index, value)` | Replace a value | `O(log n)` |
| `PrefixSum(index)` | Inclusive sum `[0, index]` | `O(log n)` |
| `RangeSum(left, right)` | Inclusive sum `[left, right]` | `O(log n)` |
| `Total()` | Sum all values | `O(log n)` |
| `Values()` | Return a copy | `O(n)` |

All exported methods are safe for concurrent use.

## Indexing and ranges

The public API is zero-based.

```go
values := []int64{3, 2, 5, 1, 4}
```

Valid indexes are `0` through `4`.

Ranges are inclusive:

```go
tree.RangeSum(1, 3)
```

returns `2 + 5 + 1`.

## Sharded tree

The sharded implementation divides values into independently locked contiguous trees.

### Numeric constructors

```go
tree := fenwick.NewNumericSharded(values)
tree := fenwick.NewNumericShardedWithCount(values, 32)
```

### Injected-operations constructors

```go
tree := fenwick.NewShardedWithOperations(values, ops)
tree := fenwick.NewShardedWithOperationsAndCount(values, 32, ops)
```

### Legacy `Value` constructors

```go
tree := fenwick.NewSharded(values)
tree := fenwick.NewShardedWithCount(values, 32)
```

### Fast queries

- `PrefixSum`
- `RangeSum`
- `Total`

Fast queries minimize lock duration. During concurrent writes across multiple shards, they are race-free but may combine values observed at slightly different moments.

### Exact queries

- `ExactPrefixSum`
- `ExactRangeSum`
- `ExactTotal`
- `Values`

Exact queries lock all involved shards in deterministic order and return one consistent cross-shard snapshot.

## Choosing an integration style

Use `NewNumeric` when:

- values are signed numbers or floats;
- direct arithmetic is sufficient;
- maximum simplicity and performance are desired.

Use `NewWithOperations` when:

- values are domain structs;
- aggregation behavior should be injected;
- models should not depend on this package;
- compile-time type safety is important.

Use `New` with `Value` when:

- existing models already implement the legacy interface;
- backward compatibility is required.

## Algebra requirements

Injected operations must obey:

- associativity: `Add(Add(a, b), c) == Add(a, Add(b, c))`
- identity: `Add(a, Zero()) == a`
- inverse behavior: `Sub(a, a) == Zero()`

Breaking these rules produces incorrect prefix and range results.

## Errors

```go
fenwick.ErrIndexOutOfRange
fenwick.ErrInvalidRange
```

Use `errors.Is`:

```go
value, err := tree.At(-1)
if errors.Is(err, fenwick.ErrIndexOutOfRange) {
    // handle invalid index
}
```

## Concurrency

`Tree[T]` uses one `sync.RWMutex`.

`ShardedTree[T]` uses one `sync.RWMutex` per shard, allowing updates to different shards to proceed concurrently.

Sharding is not automatically faster. Benchmark with the expected read/write ratio and index distribution.

## Limitations

- arithmetic overflow is not detected;
- floating-point accumulation follows IEEE-754 semantics;
- injected operations are trusted to satisfy the documented algebraic laws;
- `OperationFuncs` panics if a required function field is nil and used;
- legacy `Value` implementations may panic on incompatible runtime type assertions;
- fast sharded reads are not a single global snapshot during concurrent writes.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
go test -run='^$' -bench=. -benchmem ./...
```

## Repository

- Source: https://github.com/Sheryorov/fenwick
- Issues: https://github.com/Sheryorov/fenwick/issues
- Documentation: https://pkg.go.dev/github.com/Sheryorov/fenwick

## Reference

Peter M. Fenwick, *A New Data Structure for Cumulative Frequency Tables*, 1994.

## Atomic batch mutations

Both `Tree[T]` and `ShardedTree[T]` support applying multiple point changes with
one call:

```go
err := tree.Apply(
    fenwick.AddMutation(0, int64(5)),
    fenwick.SetMutation(3, int64(20)),
    fenwick.AddMutation(3, int64(-2)),
)
if err != nil {
    // no mutation was applied when validation failed
}
```

Mutations are evaluated in the order supplied. This is relevant when several
mutations target the same index.

`Tree.Apply` acquires its write lock once. Readers and writers observe either
the state before the complete batch or the state after it.

`ShardedTree.Apply` validates the complete batch first, groups the affected
indexes by shard, and locks only the touched shards in ascending order. Exact
read methods observe the complete batch atomically. Fast cross-shard reads keep
their documented non-snapshot semantics.

Available mutation helpers:

```go
fenwick.AddMutation(index, delta)
fenwick.SetMutation(index, value)
```

An empty batch is a no-op. Invalid indexes or mutation kinds are rejected
before any changes are made.
