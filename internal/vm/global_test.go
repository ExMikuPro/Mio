package vm

import (
	"Mio/internal/code"
	"strings"
	"testing"

	"Mio/internal/compiler"
	"Mio/internal/lexer"
	"Mio/internal/parser"
	"Mio/internal/resolver"
	"Mio/internal/token"
	"Mio/internal/typecheck"
)

// compileVariableSource 编译经过语义检查的 Mio 源码
func compileVariableSource(
	t *testing.T,
	input string,
) *compiler.Bytecode {
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

	c := compiler.NewWithResolver(r)
	if err := c.Compile(program); err != nil {
		t.Fatal(err)
	}

	return c.Bytecode()
}

// 测试全局变量的运行时存储与读取
func TestVMGlobalVariables(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{
			name: "声明与读取",
			input: `
kaku age = 18;
kaku next = age + 1;
next;
`,
			want: 19,
		},
		{
			name: "变量重新赋值",
			input: `
kaku age = 18;
age = 20;
age;
`,
			want: 20,
		},
		{
			name: "多次读取变量",
			input: `
kaku age = 18;
age;
age;
`,
			want: 18,
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
			want: 19,
		},
		{
			name: "条件为假时不修改变量",
			input: `
kaku age = 18;

moshi (iie) {
    age = 30;
}

age;
`,
			want: 18,
		},
		{
			name: "sore 分支修改变量",
			input: `
kaku value = 1;

moshi (iie) {
    value = 10;
} sore {
    value = 20;
}

value;
`,
			want: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytecode := compileVariableSource(t, tt.input)
			machine := New(bytecode)

			if err := machine.Run(); err != nil {
				t.Fatal(err)
			}

			result, ok := machine.LastPoppedValue()
			if !ok {
				t.Fatal("expected popped value")
			}

			got, ok := result.Int64()
			if !ok || got != tt.want {
				t.Fatalf(
					"got %d, ok=%v, want %d",
					got,
					ok,
					tt.want,
				)
			}

			if len(machine.globals) != bytecode.GlobalsCount {
				t.Fatalf(
					"got %d global slots, want %d",
					len(machine.globals),
					bytecode.GlobalsCount,
				)
			}

			if len(machine.stack) != 0 {
				t.Fatalf(
					"expected empty stack, got %v",
					machine.stack,
				)
			}
		})
	}
}

// 测试非法全局变量字节码
func TestVMGlobalErrors(t *testing.T) {
	tests := []struct {
		name     string
		bytecode *compiler.Bytecode
		wantErr  string
	}{
		{
			name: "读取越界槽位",
			bytecode: &compiler.Bytecode{
				GlobalsCount: 1,
				Instructions: code.Instructions{
					byte(code.OpGetGlobal), 0, 1,
				},
			},
			wantErr: "out of range",
		},
		{
			name: "写入越界槽位",
			bytecode: &compiler.Bytecode{
				GlobalsCount: 1,
				Instructions: code.Instructions{
					byte(code.OpSetGlobal), 0, 1,
				},
			},
			wantErr: "out of range",
		},
		{
			name: "保存变量时栈为空",
			bytecode: &compiler.Bytecode{
				GlobalsCount: 1,
				Instructions: code.Instructions{
					byte(code.OpSetGlobal), 0, 0,
				},
			},
			wantErr: "stack underflow",
		},
		{
			name: "读取未初始化变量",
			bytecode: &compiler.Bytecode{
				GlobalsCount: 1,
				Instructions: code.Instructions{
					byte(code.OpGetGlobal), 0, 0,
				},
			},
			wantErr: "uninitialized",
		},
		{
			name: "非法槽位数量",
			bytecode: &compiler.Bytecode{
				GlobalsCount: -1,
			},
			wantErr: "invalid global slot count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := New(tt.bytecode)

			err := machine.Run()
			if err == nil {
				t.Fatal("expected VM error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf(
					"expected error containing %q, got %v",
					tt.wantErr,
					err,
				)
			}
		})
	}
}
