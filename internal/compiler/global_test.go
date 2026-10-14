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

// 测试代码块变量按声明符号分配独立槽位
func TestCompileBlockVariableSlots(t *testing.T) {
	input := `
kaku value = 10;
moshi (hai) {
    kaku value = 20;
    chant(value);
}
chant(value);
`

	got, err := compileGlobalSource(t, input)
	if err != nil {
		t.Fatal(err)
	}

	want := code.Instructions{
		byte(code.OpConstant), 0, 0,
		byte(code.OpSetGlobal), 0, 0,
		byte(code.OpTrue),
		byte(code.OpJumpIfFalse), 0, 20,
		byte(code.OpConstant), 0, 1,
		byte(code.OpSetGlobal), 0, 1,
		byte(code.OpGetGlobal), 0, 1,
		byte(code.OpChant),
		byte(code.OpGetGlobal), 0, 0,
		byte(code.OpChant),
	}

	if got.GlobalsCount != 2 {
		t.Fatalf("global count: got %d, want 2", got.GlobalsCount)
	}
	if !bytes.Equal(got.Instructions, want) {
		t.Fatalf(
			"instructions:\ngot:\n%s\nwant:\n%s",
			got.Instructions.String(),
			want.String(),
		)
	}
}

// 测试不同分支中的同名变量使用不同槽位
func TestCompileBranchVariableSlots(t *testing.T) {
	input := `
moshi (hai) {
    kaku value = 10;
    chant(value);
} sore {
    kaku value = 20;
    chant(value);
}
`

	got, err := compileGlobalSource(t, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.GlobalsCount != 2 {
		t.Fatalf("global count: got %d, want 2", got.GlobalsCount)
	}

	disassembly := got.Instructions.String()
	for _, want := range []string{
		"OpSetGlobal 0",
		"OpGetGlobal 0",
		"OpSetGlobal 1",
		"OpGetGlobal 1",
	} {
		if !strings.Contains(disassembly, want) {
			t.Fatalf("disassembly missing %q:\n%s", want, disassembly)
		}
	}
}
