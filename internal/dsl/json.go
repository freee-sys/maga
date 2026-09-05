package dsl

import (
	"encoding/json"
	"fmt"
)

type jsonOp struct {
	Op   string                     `json:"op"`
	Args map[string]json.RawMessage `json:"args,omitempty"`
}

// MarshalJSON encodes the pipeline as the structured form the UI's
// op-dropdown + typed-arg editor writes: an ordered array of
// {"op": name, "args": {argName: value}}.
func (p Pipeline) MarshalJSON() ([]byte, error) {
	ops := make([]jsonOp, len(p))
	for i, op := range p {
		spec, ok := registry[op.Name]
		if !ok {
			return nil, fmt.Errorf("unknown dsl op %q", op.Name)
		}

		args := make(map[string]json.RawMessage, len(op.Args))
		for j, arg := range op.Args {
			name := argNameFor(spec, j)
			raw, err := marshalArg(arg)
			if err != nil {
				return nil, fmt.Errorf("op %q arg %q: %w", op.Name, name, err)
			}
			args[name] = raw
		}

		ops[i] = jsonOp{Op: op.Name, Args: args}
	}
	return json.Marshal(ops)
}

// UnmarshalJSON makes Pipeline a json.Unmarshaler, so it round-trips
// correctly through encoding/json even nested inside another structure
// (e.g. map[string]Pipeline for a rule's per-group transforms) rather
// than only via the standalone FromJSON entry point.
func (p *Pipeline) UnmarshalJSON(data []byte) error {
	parsed, err := FromJSON(data)
	if err != nil {
		return err
	}
	*p = parsed
	return nil
}

// FromJSON decodes the structured form produced by MarshalJSON back into
// a Pipeline. Every op must be known and every argument must map to a
// name in that op's registry entry; unknown ops or malformed argument
// values are reported as errors rather than silently dropped.
func FromJSON(raw []byte) (Pipeline, error) {
	var ops []jsonOp
	if err := json.Unmarshal(raw, &ops); err != nil {
		return nil, fmt.Errorf("invalid pipeline JSON: %w", err)
	}

	pipeline := make(Pipeline, len(ops))
	for i, jo := range ops {
		spec, ok := registry[jo.Op]
		if !ok {
			return nil, fmt.Errorf("unknown dsl op %q", jo.Op)
		}

		args := make([]Arg, len(spec.ArgNames))
		for j, name := range spec.ArgNames {
			rawArg, present := jo.Args[name]
			if !present {
				if j < spec.MinArgs {
					return nil, fmt.Errorf("op %q missing required arg %q", jo.Op, name)
				}
				args = args[:j]
				break
			}
			arg, err := unmarshalArg(rawArg)
			if err != nil {
				return nil, fmt.Errorf("op %q arg %q: %w", jo.Op, name, err)
			}
			args[j] = arg
		}

		pipeline[i] = Op{Name: jo.Op, Args: args}
	}
	return pipeline, nil
}

// argNameFor returns the registered name for the i-th argument of spec,
// falling back to a positional placeholder if the registry entry doesn't
// name that many arguments (shouldn't happen for valid pipelines, but
// keeps MarshalJSON total rather than panicking on malformed input).
func argNameFor(spec OpSpec, i int) string {
	if i < len(spec.ArgNames) {
		return spec.ArgNames[i]
	}
	return fmt.Sprintf("arg%d", i)
}

func marshalArg(a Arg) (json.RawMessage, error) {
	switch {
	case a.S != nil:
		return json.Marshal(*a.S)
	case a.I != nil:
		return json.Marshal(*a.I)
	case a.M != nil:
		return json.Marshal(a.M)
	default:
		return nil, fmt.Errorf("argument has no value")
	}
}

func unmarshalArg(raw json.RawMessage) (Arg, error) {
	var asMap map[string]string
	if err := json.Unmarshal(raw, &asMap); err == nil {
		return Arg{M: asMap}, nil
	}
	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return Arg{I: &asInt}, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return Arg{S: &asString}, nil
	}
	return Arg{}, fmt.Errorf("value %s is not a string, integer, or lookup table", raw)
}
