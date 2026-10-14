package vm

import (
	"strings"
	"testing"

	"Mio/internal/code"
	"Mio/internal/compiler"
	"Mio/internal/lexer"
	"Mio/internal/parser"
	"Mio/internal/token"
)

// compileSource 将 Mio 源码编译为字节码
func compileSource(
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

	c := compiler.New()
	if err := c.Compile(program); err != nil {
		t.Fatal(err)
	}

	return c.Bytecode()
}

// 测试整数加法的字节码执行
func TestVMIntegerAddition(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{
			name:  "基础加法",
			input: "1 + 2;",
			want:  3,
		},
		{
			name:  "连续加法",
			input: "1 + 2 + 3;",
			want:  6,
		},
		{
			name:  "括号嵌套",
			input: "1 + (2 + 3);",
			want:  6,
		},
		{
			name:  "多条表达式语句",
			input: "1; 2 + 3;",
			want:  5,
		},
		{
			name:  "零值",
			input: "0;",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytecode := compileSource(t, tt.input)
			machine := New(bytecode)

			if err := machine.Run(); err != nil {
				t.Fatal(err)
			}

			got, ok := machine.LastPoppedStackElem()
			if !ok {
				t.Fatal("expected popped result")
			}

			if got != tt.want {
				t.Fatalf(
					"got %d, want %d",
					got,
					tt.want,
				)
			}

			// 表达式语句执行完后不应残留栈值
			if len(machine.stack) != 0 {
				t.Fatalf(
					"expected empty stack, got %v",
					machine.stack,
				)
			}
		})
	}
}

// 测试空程序的执行
func TestVMEmptyProgram(t *testing.T) {
	machine := New(compileSource(t, ""))

	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}

	if _, ok := machine.LastPoppedStackElem(); ok {
		t.Fatal("expected no result")
	}
}

// 测试虚拟机的非法字节码处理
func TestVMRunErrors(t *testing.T) {
	tests := []struct {
		name     string
		bytecode *compiler.Bytecode
		wantErr  string
	}{
		{
			name:    "空字节码对象",
			wantErr: "nil bytecode",
		},
		{
			name: "未知操作码",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{255},
			},
			wantErr: "unknown opcode",
		},
		{
			name: "常量指令不完整",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpConstant), 0,
				},
			},
			wantErr: "not enough bytes",
		},
		{
			name: "常量索引越界",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpConstant), 0, 1,
				},
				Constants: []int64{10},
			},
			wantErr: "out of range",
		},
		{
			name: "空栈弹出",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpPop),
				},
			},
			wantErr: "stack underflow",
		},
		{
			name: "加法操作数不足",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpConstant), 0, 0,
					byte(code.OpAdd),
				},
				Constants: []int64{1},
			},
			wantErr: "stack underflow",
		},

		{
			name: "减法操作数不足",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpSub),
				},
			},
			wantErr: "stack underflow",
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
					"expected error containing %q, got %q",
					tt.wantErr,
					err.Error(),
				)
			}
		})
	}
}

// 测试整数算术运算的字节码执行
func TestVMIntegerArithmetic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{
			name:  "整数减法",
			input: "10 - 3;",
			want:  7,
		},
		{
			name:  "减法操作数顺序",
			input: "3 - 10;",
			want:  -7,
		},
		{
			name:  "整数乘法",
			input: "6 * 7;",
			want:  42,
		},
		{
			name:  "整数除法",
			input: "20 / 4;",
			want:  5,
		},
		{
			name:  "整数除法截断",
			input: "7 / 2;",
			want:  3,
		},
		{
			name:  "负整数除法",
			input: "-7 / 2;",
			want:  -3,
		},
		{
			name:  "一元负号",
			input: "-10;",
			want:  -10,
		},
		{
			name:  "连续取负",
			input: "--10;",
			want:  10,
		},
		{
			name:  "算术优先级",
			input: "1 + 2 * 3;",
			want:  7,
		},
		{
			name:  "括号与一元取负",
			input: "-(10 - 3);",
			want:  -7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytecode := compileSource(t, tt.input)
			machine := New(bytecode)

			if err := machine.Run(); err != nil {
				t.Fatal(err)
			}

			got, ok := machine.LastPoppedStackElem()
			if !ok {
				t.Fatal("expected popped result")
			}

			if got != tt.want {
				t.Fatalf(
					"got %d, want %d",
					got,
					tt.want,
				)
			}

			// 表达式执行后不应残留栈元素
			if len(machine.stack) != 0 {
				t.Fatalf(
					"expected empty stack, got %v",
					machine.stack,
				)
			}
		})
	}
}
