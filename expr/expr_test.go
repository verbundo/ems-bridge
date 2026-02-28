package expr

import (
	"reflect"
	"testing"
)

func TestEval(t *testing.T) {
	tests := []struct {
		expr string
		want any
	}{
		// String literals
		{`"hello"`, "hello"},
		{`'world'`, "world"},

		// Booleans
		{`true`, true},
		{`false`, false},

		// Integers
		{`42`, 42},
		{`-7`, -7},

		// Floats
		{`3.14`, 3.14},

		// Arithmetic
		{`1 + 2`, 3},
		{`10 - 4`, 6},
		{`3 * 4`, 12},
		{`10 / 2`, float64(5)}, // expr-lang returns float64 for division

		// String concatenation
		{`"foo" + "bar"`, "foobar"},

		// Arrays
		{`[]`, []any{}},
		{`["txt"]`, []any{"txt"}},
		{`["txt", "csv"]`, []any{"txt", "csv"}},
		{`[1, 2, 3]`, []any{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := Eval(tt.expr)
			if err != nil {
				t.Fatalf("Eval(%q) error: %v", tt.expr, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Eval(%q) = %v (%T), want %v (%T)", tt.expr, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestEval_Invalid(t *testing.T) {
	_, err := Eval(`@@@`)
	if err == nil {
		t.Error("Eval(invalid) expected error, got nil")
	}
}

func TestString(t *testing.T) {
	got, err := String(`"hello"`)
	if err != nil || got != "hello" {
		t.Errorf(`String("hello") = %q, %v`, got, err)
	}

	got, err = String(`'world'`)
	if err != nil || got != "world" {
		t.Errorf(`String('world') = %q, %v`, got, err)
	}

	_, err = String(`42`)
	if err == nil {
		t.Error("String(42) expected type error, got nil")
	}
}

func TestBool(t *testing.T) {
	if got, err := Bool(`true`); err != nil || got != true {
		t.Errorf("Bool(true) = %v, %v", got, err)
	}
	if got, err := Bool(`false`); err != nil || got != false {
		t.Errorf("Bool(false) = %v, %v", got, err)
	}

	_, err := Bool(`"yes"`)
	if err == nil {
		t.Error(`Bool("yes") expected type error, got nil`)
	}
}

func TestStringSlice(t *testing.T) {
	got, err := StringSlice(`['txt', 'csv']`)
	if err != nil {
		t.Fatalf("StringSlice error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"txt", "csv"}) {
		t.Errorf("StringSlice(['txt','csv']) = %v", got)
	}

	// Empty list
	got, err = StringSlice(`[]`)
	if err != nil || len(got) != 0 {
		t.Errorf("StringSlice([]) = %v, %v", got, err)
	}

	// Non-list
	_, err = StringSlice(`"notalist"`)
	if err == nil {
		t.Error(`StringSlice("notalist") expected type error, got nil`)
	}
}
