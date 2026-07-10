# fenwick

[![Build](https://github.com/Sheryorov/fenwick/workflows/Build/badge.svg)](https://github.com/Sheryorov/fenwick/actions)
[![Tests](https://github.com/Sheryorov/fenwick/workflows/Tests/badge.svg)](https://github.com/Sheryorov/fenwick/actions)
[![codecov](https://codecov.io/gh/Sheryorov/fenwick/graph/badge.svg)](https://codecov.io/gh/Sheryorov/fenwick)
[![Go Report Card](https://goreportcard.com/badge/github.com/Sheryorov/fenwick)](https://goreportcard.com/report/github.com/Sheryorov/fenwick)

A zero-based, concurrency-safe Fenwick tree for signed integer and floating-point types, supporting point updates and range sums.

## Supported Types

The `Tree[T]` and `ShardedTree[T]` types support:
- signed integers: `int`, `int8`, `int16`, `int32`, `int64`;
- floating point: `float32`, `float64`;
- named types whose underlying type is one of the above.

Unsigned integers are intentionally excluded because `Set` may need a negative
delta when a value decreases. Arithmetic overflow is not detected.

## API

### Tree[T]

- `New[T](values)` — O(n)
- `Len()` — O(1)
- `Add(index, delta)` — O(log n)
- `Set(index, value)` — O(log n)
- `At(index)` — O(1)
- `PrefixSum(index)` — O(log n), inclusive
- `RangeSum(left, right)` — O(log n), inclusive
- `Total()` — O(log n)
- `Values()` — O(n), returns a copy

The implementation copies its input, validates public indexes and ranges, and is safe for concurrent readers and writers using `sync.RWMutex`.

Arithmetic uses the generic type `T` and does not detect overflow.

## Verify

```bash
go test ./...
go test -race ./...
go test -run=^$ -fuzz=FuzzTreeAgainstNaive -fuzztime=10s
go test -bench=. -benchmem ./...
go vet ./...
```

## Usage

### Tree[T]

```go
// Works with any Numeric type
ft := fenwick.New([]int64{3, 2, 5, 1, 4})

sum, err := ft.RangeSum(1, 3) // 8
if err != nil {
    // handle invalid range
}

err = ft.Set(2, 10)

// With floats
ff := fenwick.New([]float64{1.5, 2.3, 3.7})
total := ff.Total() // 7.5
```

## Sharded tree for write-heavy concurrency

`ShardedTree[T]` splits values into independent contiguous Fenwick trees. Point
updates to different shards can execute concurrently.

```go
ft := fenwick.NewShardedWithCount([]int64{...}, 32)

_ = ft.Add(100, 5)
_ = ft.Set(200, 9)

// Maximum throughput. Race-free, but not a single cross-shard snapshot while
// concurrent writers are active.
sum, _ := ft.RangeSum(50, 250)

// Linearizable consistent snapshot across all intersecting shards.
exact, _ := ft.ExactRangeSum(50, 250)

_, _ = sum, exact
```

### Constructors

- `NewSharded[T](values)` chooses approximately `4 * GOMAXPROCS` shards.
- `NewShardedWithCount[T](values, shardCount)` uses an explicit shard count.

### Fast methods

- `PrefixSum`, `RangeSum`, `Total`
- lock only boundary shards;
- optimized for throughput;
- during concurrent writes, the result may combine states observed at slightly
  different instants.

### Strict snapshot methods

- `ExactPrefixSum`, `ExactRangeSum`, `ExactTotal`, `Values`
- lock all involved shards in ascending order;
- provide one consistent cross-shard view;
- have higher contention for large ranges.

Run correctness and race tests:

```bash
go test ./...
go test -race ./...
```

Compare concurrent update throughput:

```bash
go test -run '^$' -bench 'ConcurrentAdd' -benchmem
```

Always benchmark with the expected read/write ratio and index distribution.
Sharding helps most when many goroutines update different shards. A single
`Tree` can remain faster for low contention or read-heavy workloads because it
has fewer indirections and locks.
