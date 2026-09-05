package dsl

import (
	"fmt"
	"regexp"
	"strings"
)

// OpFunc is the implementation of a single pipeline operation: given the
// current value and its arguments, produce the next value.
type OpFunc func(current string, args []Arg) (string, error)

// OpSpec describes one registered operation: its arity, the argument
// names used for JSON serialization (in positional order), and its
// implementation.
type OpSpec struct {
	MinArgs  int
	MaxArgs  int
	ArgNames []string
	Fn       OpFunc
}

var registry = map[string]OpSpec{
	"trim": {
		MinArgs: 0, MaxArgs: 0,
		Fn: func(current string, _ []Arg) (string, error) {
			return strings.TrimSpace(current), nil
		},
	},
	"trimLeft": {
		MinArgs: 1, MaxArgs: 1, ArgNames: []string{"cutset"},
		Fn: func(current string, args []Arg) (string, error) {
			cutset, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			return strings.TrimLeft(current, cutset), nil
		},
	},
	"trimRight": {
		MinArgs: 1, MaxArgs: 1, ArgNames: []string{"cutset"},
		Fn: func(current string, args []Arg) (string, error) {
			cutset, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			return strings.TrimRight(current, cutset), nil
		},
	},
	"upper": {
		MinArgs: 0, MaxArgs: 0,
		Fn: func(current string, _ []Arg) (string, error) {
			return strings.ToUpper(current), nil
		},
	},
	"lower": {
		MinArgs: 0, MaxArgs: 0,
		Fn: func(current string, _ []Arg) (string, error) {
			return strings.ToLower(current), nil
		},
	},
	"replace": {
		MinArgs: 2, MaxArgs: 2, ArgNames: []string{"old", "new"},
		Fn: func(current string, args []Arg) (string, error) {
			old, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			next, err := argString(args, 1)
			if err != nil {
				return "", err
			}
			return strings.ReplaceAll(current, old, next), nil
		},
	},
	"replaceN": {
		MinArgs: 3, MaxArgs: 3, ArgNames: []string{"old", "new", "n"},
		Fn: func(current string, args []Arg) (string, error) {
			old, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			next, err := argString(args, 1)
			if err != nil {
				return "", err
			}
			n, err := argInt(args, 2)
			if err != nil {
				return "", err
			}
			return strings.Replace(current, old, next, n), nil
		},
	},
	"replaceRegex": {
		MinArgs: 2, MaxArgs: 2, ArgNames: []string{"pattern", "repl"},
		Fn: func(current string, args []Arg) (string, error) {
			pattern, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			repl, err := argString(args, 1)
			if err != nil {
				return "", err
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return "", fmt.Errorf("invalid regex %q: %w", pattern, err)
			}
			return re.ReplaceAllString(current, repl), nil
		},
	},
	"deleteSubstring": {
		MinArgs: 1, MaxArgs: 1, ArgNames: []string{"substr"},
		Fn: func(current string, args []Arg) (string, error) {
			substr, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			return strings.ReplaceAll(current, substr, ""), nil
		},
	},
	"deleteRange": {
		MinArgs: 2, MaxArgs: 2, ArgNames: []string{"start", "end"},
		Fn: func(current string, args []Arg) (string, error) {
			start, end, err := runeRange(args, current)
			if err != nil {
				return "", err
			}
			runes := []rune(current)
			return string(runes[:start]) + string(runes[end:]), nil
		},
	},
	"insertAt": {
		MinArgs: 2, MaxArgs: 2, ArgNames: []string{"pos", "text"},
		Fn: func(current string, args []Arg) (string, error) {
			pos, err := argInt(args, 0)
			if err != nil {
				return "", err
			}
			text, err := argString(args, 1)
			if err != nil {
				return "", err
			}
			runes := []rune(current)
			if pos < 0 || pos > len(runes) {
				return "", fmt.Errorf("insertAt position %d out of range for value of length %d", pos, len(runes))
			}
			return string(runes[:pos]) + text + string(runes[pos:]), nil
		},
	},
	"slice": {
		MinArgs: 2, MaxArgs: 2, ArgNames: []string{"start", "end"},
		Fn: func(current string, args []Arg) (string, error) {
			start, end, err := runeRange(args, current)
			if err != nil {
				return "", err
			}
			return string([]rune(current)[start:end]), nil
		},
	},
	"prefix": {
		MinArgs: 1, MaxArgs: 1, ArgNames: []string{"text"},
		Fn: func(current string, args []Arg) (string, error) {
			text, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			return text + current, nil
		},
	},
	"suffix": {
		MinArgs: 1, MaxArgs: 1, ArgNames: []string{"text"},
		Fn: func(current string, args []Arg) (string, error) {
			text, err := argString(args, 0)
			if err != nil {
				return "", err
			}
			return current + text, nil
		},
	},
	"padLeft": {
		MinArgs: 2, MaxArgs: 2, ArgNames: []string{"length", "pad"},
		Fn: func(current string, args []Arg) (string, error) {
			return pad(current, args, true)
		},
	},
	"padRight": {
		MinArgs: 2, MaxArgs: 2, ArgNames: []string{"length", "pad"},
		Fn: func(current string, args []Arg) (string, error) {
			return pad(current, args, false)
		},
	},
	"default": {
		MinArgs: 1, MaxArgs: 1, ArgNames: []string{"value"},
		Fn: func(current string, args []Arg) (string, error) {
			if current != "" {
				return current, nil
			}
			return argString(args, 0)
		},
	},
	"mapValues": {
		MinArgs: 1, MaxArgs: 2, ArgNames: []string{"table", "fallback"},
		Fn: func(current string, args []Arg) (string, error) {
			if len(args) == 0 || args[0].M == nil {
				return "", fmt.Errorf("mapValues requires a lookup table as its first argument")
			}
			if v, ok := args[0].M[current]; ok {
				return v, nil
			}
			if len(args) == 2 {
				return argString(args, 1)
			}
			return current, nil
		},
	},
}

func argString(args []Arg, i int) (string, error) {
	if i >= len(args) || args[i].S == nil {
		return "", fmt.Errorf("argument %d must be a string", i)
	}
	return *args[i].S, nil
}

func argInt(args []Arg, i int) (int, error) {
	if i >= len(args) || args[i].I == nil {
		return 0, fmt.Errorf("argument %d must be an integer", i)
	}
	return *args[i].I, nil
}

// runeRange extracts and validates a [start,end) rune range for a value,
// shared by deleteRange and slice.
func runeRange(args []Arg, value string) (start, end int, err error) {
	start, err = argInt(args, 0)
	if err != nil {
		return 0, 0, err
	}
	end, err = argInt(args, 1)
	if err != nil {
		return 0, 0, err
	}
	length := len([]rune(value))
	if start < 0 || end > length || start > end {
		return 0, 0, fmt.Errorf("range [%d,%d) out of bounds for value of length %d", start, end, length)
	}
	return start, end, nil
}

func pad(current string, args []Arg, left bool) (string, error) {
	length, err := argInt(args, 0)
	if err != nil {
		return "", err
	}
	padStr, err := argString(args, 1)
	if err != nil {
		return "", err
	}
	if padStr == "" {
		return "", fmt.Errorf("pad string must not be empty")
	}
	runes := []rune(current)
	if len(runes) >= length {
		return current, nil
	}
	var b strings.Builder
	for b.Len()+len(current) < length && len([]rune(b.String()))+len(runes) < length {
		b.WriteString(padStr)
	}
	padding := []rune(b.String())
	needed := length - len(runes)
	if len(padding) > needed {
		padding = padding[:needed]
	}
	if left {
		return string(padding) + current, nil
	}
	return current + string(padding), nil
}
