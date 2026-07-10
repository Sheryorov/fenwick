package fenwick

import "fmt"

// MutationKind identifies how a batch mutation changes a value.
type MutationKind uint8

const (
	// MutationAdd increments the current value by Mutation.Value.
	MutationAdd MutationKind = iota + 1
	// MutationSet replaces the current value with Mutation.Value.
	MutationSet
)

// Mutation describes one point change applied by Apply.
//
// Mutations are evaluated in slice order. This matters when multiple
// mutations target the same index.
type Mutation[T any] struct {
	Index int
	Kind  MutationKind
	Value T
}

// AddMutation creates an additive point mutation.
func AddMutation[T any](index int, delta T) Mutation[T] {
	return Mutation[T]{Index: index, Kind: MutationAdd, Value: delta}
}

// SetMutation creates a replacement point mutation.
func SetMutation[T any](index int, value T) Mutation[T] {
	return Mutation[T]{Index: index, Kind: MutationSet, Value: value}
}

func validateMutationKind(kind MutationKind) error {
	switch kind {
	case MutationAdd, MutationSet:
		return nil
	default:
		return fmt.Errorf("fenwick: invalid mutation kind %d", kind)
	}
}
