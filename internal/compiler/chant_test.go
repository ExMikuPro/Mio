package compiler

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"Mio/internal/ast"
	"Mio/internal/code"
	"Mio/internal/lexer"
	"Mio/internal/parser"
	"Mio/internal/resolver"
	"Mio/internal/token"
	"Mio/internal/value"
)

func TestCompileChant(t *testing.T) {
	tests := []struct {
		source       string
		constants    []value.Value
		instructions code.Instructions
	}{
		{`chant(123); chant(hai); chant(iie); chant("你好🌸"); chant(mu);`,
			[]value.Value{value.NewInt(123), value.NewString("你好🌸")},
			code.Instructions{
				byte(code.OpConstant), 0, 0, byte(code.OpChant),
				byte(code.OpTrue), byte(code.OpChant), byte(code.OpFalse), byte(code.OpChant),
				byte(code.OpConstant), 0, 1, byte(code.OpChant), byte(code.OpNil), byte(code.OpChant),
			}},
		{`((chant(mu))); 42;`, []value.Value{value.NewInt(42)}, code.Instructions{
			byte(code.OpNil), byte(code.OpChant), byte(code.OpConstant), 0, 0, byte(code.OpPop),
		}},
		{`moshi (hai) { chant(1); } sore { chant(2); } chant(3);`,
			[]value.Value{value.NewInt(1), value.NewInt(2), value.NewInt(3)}, code.Instructions{
				byte(code.OpTrue), byte(code.OpJumpIfFalse), 0, 11,
				byte(code.OpConstant), 0, 0, byte(code.OpChant), byte(code.OpJump), 0, 15,
				byte(code.OpConstant), 0, 1, byte(code.OpChant),
				byte(code.OpConstant), 0, 2, byte(code.OpChant),
			}},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			bc, err := compileGlobalSource(t, tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bc.Instructions, tt.instructions) || !reflect.DeepEqual(bc.Constants, tt.constants) {
				t.Fatalf("bytecode: %v\ninstructions:\n%s\nwant:\n%s", bc.Constants, bc.Instructions, tt.instructions)
			}
		})
	}
}

func TestCompileUserFunctionCall(t *testing.T) {
	bytecode, err := compileGlobalSource(t, `f(); waza f() -> unit { kaesu; }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytecode.Functions) != 1 || bytecode.Functions[0].ID != 0 {
		t.Fatalf("functions: %#v", bytecode.Functions)
	}
	want := code.Instructions{byte(code.OpCall), 0, 0}
	if !bytes.Equal(bytecode.Instructions, want) {
		t.Fatalf("instructions:\n%s", bytecode.Instructions.String())
	}
}

func TestCompileChantRequiresBinding(t *testing.T) {
	call := &ast.CallExpression{Function: &ast.Identifier{Name: "chant"}, Arguments: []ast.Expression{&ast.IntegerLiteral{Value: 1}}}
	for _, tt := range []struct {
		compiler *Compiler
		want     string
	}{
		{New(), "requires a name resolver"},
		{NewWithResolver(resolver.New()), "missing symbol binding"},
	} {
		err := tt.compiler.Compile(call)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("expected %q, got %v", tt.want, err)
		}
		if len(tt.compiler.instructions) != 0 {
			t.Fatal("unbound call emitted instructions")
		}
	}
}

func TestCompileCallGuardsWithoutTypeCheck(t *testing.T) {
	for _, tt := range []struct{ source, want string }{
		{`chant();`, "expects 1 argument"},
		{`chant(1, 2);`, "expects 1 argument"},
		{`kaku chant = 1; chant(2);`, `cannot call variable "chant"`},
		{`(chant)(1);`, "call target must be a function name"},
	} {
		t.Run(tt.source, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.source, token.V01)
			if err != nil {
				t.Fatal(err)
			}
			program, err := parser.New(l).ParseProgram()
			if err != nil {
				t.Fatal(err)
			}
			r := resolver.New()
			if err := r.ResolveProgram(program); err != nil {
				t.Fatal(err)
			}
			c := NewWithResolver(r)
			// 直接编译最后的调用，避免变量声明产生其他指令。
			err = c.Compile(program.Statements[len(program.Statements)-1])
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
			if len(c.instructions) != 0 {
				t.Fatal("invalid call emitted instructions")
			}
		})
	}
}
