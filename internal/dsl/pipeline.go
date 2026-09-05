// Package dsl implements a small, fixed-vocabulary pipeline of string
// operations used to transform values captured by a rule's hostname regex
// before they become part of a cluster key. It is deliberately not a
// general-purpose scripting language: every operation is a named Go
// function from a closed registry, so there is no eval and no code
// injection surface.
package dsl

import "fmt"

// Arg is a single operation argument: a string, an integer, or a
// string-to-string lookup table, never more than one at a time.
type Arg struct {
	S *string
	I *int
	M map[string]string
}

// MapArg builds a lookup-table argument, for ops like mapValues.
func MapArg(m map[string]string) Arg {
	return Arg{M: m}
}

// Op is one pipeline step: an operation name plus its arguments.
type Op struct {
	Name string
	Args []Arg
}

// Pipeline is an ordered sequence of operations applied to a string.
type Pipeline []Op

// Apply runs the pipeline against input, threading the result of each step
// into the next. It stops and returns an error on the first unknown op or
// argument mismatch.
func (p Pipeline) Apply(input string) (string, error) {
	value := input
	for _, op := range p {
		spec, ok := registry[op.Name]
		if !ok {
			return "", fmt.Errorf("unknown dsl op %q", op.Name)
		}
		if len(op.Args) < spec.MinArgs || len(op.Args) > spec.MaxArgs {
			return "", fmt.Errorf("dsl op %q expects %d-%d args, got %d", op.Name, spec.MinArgs, spec.MaxArgs, len(op.Args))
		}
		next, err := spec.Fn(value, op.Args)
		if err != nil {
			return "", fmt.Errorf("dsl op %q: %w", op.Name, err)
		}
		value = next
	}
	return value, nil
}
