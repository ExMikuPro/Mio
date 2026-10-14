package compiler

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"Mio/internal/code"
	"Mio/internal/lexer"
	"Mio/internal/parser"
	"Mio/internal/resolver"
	"Mio/internal/token"
	"Mio/internal/typecheck"
	"Mio/internal/value"
)

// compileGlobalSource 对经过语义检查的源码生成字节码
func compileGlobalSource(
	t *testing.T,
	input string,
) (*Bytecode, error) {
	t.Helper()

	l, err := lexer.NewChecked(input, token.V01)
	if err != nil {
		t.Fatal(err)
	}

	p := parser.New(l)
	program, err := p.ParseProgram()
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Err(); err != nil {
		t.Fatal(err)
	}

	r := resolver.New()
	if err := r.ResolveProgram(program); err != nil {
		t.Fatal(err)
	}

	checker := typecheck.NewWithResolver(r)
	if err := checker.CheckProgram(program); err != nil {
		t.Fatal(err)
	}

	c := NewWithResolver(r)
	if err := c.Compile(program); err != nil {
		return nil, err
	}

	return c.Bytecode(), nil
}

// 测试全局变量声明、引用与赋值
func TestCompileGlobalVariables(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		constants    []value.Value
		globalsCount int
		instructions code.Instructions
	}{
		{
			name: "声明与引用",
			input: `
kaku age = 18;
kaku next = age + 1;
next;
`,
			constants:    []value.Value{value.NewInt(18), value.NewInt(1)},
			globalsCount: 2,
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpSetGlobal), 0, 0,
				byte(code.OpGetGlobal), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpAdd),
				byte(code.OpSetGlobal), 0, 1,
				byte(code.OpGetGlobal), 0, 1,
				byte(code.OpPop),
			},
		},
		{
			name: "变量重新赋值",
			input: `
kaku age = 18;
age = 20;
age;
`,
			constants:    []value.Value{value.NewInt(18), value.NewInt(20)},
			globalsCount: 1,
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpSetGlobal), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpSetGlobal), 0, 0,
				byte(code.OpGetGlobal), 0, 0,
				byte(code.OpPop),
			},
		},
		{
			name: "条件分支修改全局变量",
			input: `
kaku age = 18;
moshi (hai) {
    age = age + 1;
}
age;
`,
			constants:    []value.Value{value.NewInt(18), value.NewInt(1)},
			globalsCount: 1,
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpSetGlobal), 0, 0,
				byte(code.OpTrue),
				byte(code.OpJumpIfFalse), 0, 20,
				byte(code.OpGetGlobal), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpAdd),
				byte(code.OpSetGlobal), 0, 0,
				byte(code.OpGetGlobal), 0, 0,
				byte(code.OpPop),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compileGlobalSource(t, tt.input)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got.Constants, tt.constants) {
				t.Fatalf(
					"constants: got %v, want %v",
					got.Constants,
					tt.constants,
				)
			}

			if got.GlobalsCount != tt.globalsCount {
				t.Fatalf(
					"global count: got %d, want %d",
					got.GlobalsCount,
					tt.globalsCount,
				)
			}

			if !bytes.Equal(got.Instructions, tt.instructions) {
				t.Fatalf(
					"instructions:\ngot:\n%s\nwant:\n%s",
					got.Instructions.String(),
					tt.instructions.String(),
				)
			}
		})
	}
}

// 测试尚未支持的代码块局部变量
func TestCompileLocalVariableNotYetSupported(t *testing.T) {
	input := `
moshi (hai) {
    kaku local = 10;
}
`

	_, err := compileGlobalSource(t, input)
	if err == nil {
		t.Fatal("expected compiler error")
	}

	if !strings.Contains(
		err.Error(),
		"block-local variable",
	) {
		t.Fatalf(
			"expected local variable error, got %v",
			err,
		)
	}
}
