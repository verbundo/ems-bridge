package expr

import (
	"fmt"

	exprlib "github.com/expr-lang/expr"
)

// Eval compiles and evaluates an expression string, returning the result as any.
func Eval(s string) (any, error) {
	program, err := exprlib.Compile(s)
	if err != nil {
		return nil, fmt.Errorf("compiling expression %q: %w", s, err)
	}
	result, err := exprlib.Run(program, nil)
	if err != nil {
		return nil, fmt.Errorf("evaluating expression %q: %w", s, err)
	}
	return result, nil
}

// String compiles and evaluates s, returning the result as a string.
func String(s string) (string, error) {
	v, err := Eval(s)
	if err != nil {
		return "", err
	}
	str, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T from %q", v, s)
	}
	return str, nil
}

// Bool compiles and evaluates s, returning the result as a bool.
func Bool(s string) (bool, error) {
	v, err := Eval(s)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("expected bool, got %T from %q", v, s)
	}
	return b, nil
}

// StringSlice compiles and evaluates s, returning the result as a []string.
func StringSlice(s string) ([]string, error) {
	v, err := Eval(s)
	if err != nil {
		return nil, err
	}
	switch val := v.(type) {
	case []string:
		return val, nil
	case []any:
		strs := make([]string, len(val))
		for i, item := range val {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected []string but item %d is %T", i, item)
			}
			strs[i] = str
		}
		return strs, nil
	default:
		return nil, fmt.Errorf("expected []string, got %T from %q", v, s)
	}
}
