package vm

import (
	"Mio/internal/code"
	"Mio/internal/compiler"
	"Mio/internal/value"
	"strings"
	"testing"
)

// 测试条件语句的字节码执行
func TestVMIfStatements(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       int64
		wantResult bool
	}{
		{
			name:       "条件为真",
			input:      "moshi (hai) { 10; } sore { 20; }",
			want:       10,
			wantResult: true,
		},
		{
			name:       "条件为假",
			input:      "moshi (iie) { 10; } sore { 20; }",
			want:       20,
			wantResult: true,
		},
		{
			name:       "无 sore 且条件为真",
			input:      "moshi (hai) { 10; }",
			want:       10,
			wantResult: true,
		},
		{
			name:       "无 sore 且条件为假",
			input:      "moshi (iie) { 10; }",
			wantResult: false,
		},
		{
			name: "比较表达式控制分支",
			input: `moshi (10 > 5) {
                100;
            } sore {
                200;
            }`,
			want:       100,
			wantResult: true,
		},
		{
			name: "嵌套条件语句",
			input: `moshi (hai) {
                moshi (iie) {
                    10;
                } sore {
                    20;
                }
            }`,
			want:       20,
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytecode := compileSource(t, tt.input)
			machine := New(bytecode)

			if err := machine.Run(); err != nil {
				t.Fatal(err)
			}

			result, ok := machine.LastPoppedValue()

			if ok != tt.wantResult {
				t.Fatalf(
					"got result presence %v, want %v",
					ok,
					tt.wantResult,
				)
			}

			if tt.wantResult {
				number, valid := result.Int64()
				if !valid || number != tt.want {
					t.Fatalf(
						"got %d, valid=%v, want %d",
						number,
						valid,
						tt.want,
					)
				}
			}

			// 条件执行完毕后栈应为空
			if len(machine.stack) != 0 {
				t.Fatalf(
					"expected empty stack, got %v",
					machine.stack,
				)
			}
		})
	}
}

// 测试逻辑运算的运行时短路行为
func TestVMShortCircuit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr string
	}{
		{
			name:  "逻辑与左侧为假",
			input: "iie && (10 / 0 > 1);",
			want:  false,
		},
		{
			name:  "逻辑或左侧为真",
			input: "hai || (10 / 0 > 1);",
			want:  true,
		},
		{
			name:  "逻辑与执行右侧",
			input: "hai && (10 > 5);",
			want:  true,
		},
		{
			name:  "逻辑或执行右侧",
			input: "iie || (10 > 5);",
			want:  true,
		},
		{
			name:  "嵌套短路表达式",
			input: "(hai && iie) || hai;",
			want:  true,
		},
		{
			name:    "逻辑与不能跳过必要的右侧计算",
			input:   "hai && (10 / 0 > 1);",
			wantErr: "division by zero",
		},
		{
			name:    "逻辑或不能跳过必要的右侧计算",
			input:   "iie || (10 / 0 > 1);",
			wantErr: "division by zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytecode := compileSource(t, tt.input)
			machine := New(bytecode)

			err := machine.Run()

			if tt.wantErr != "" {
				if err == nil ||
					!strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf(
						"expected %q, got %v",
						tt.wantErr,
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			result, ok := machine.LastPoppedValue()
			if !ok {
				t.Fatal("expected popped value")
			}

			if result.Kind() != value.KindBool {
				t.Fatalf(
					"got %s, want bool",
					result.Kind(),
				)
			}

			got, valid := result.Bool()
			if !valid || got != tt.want {
				t.Fatalf(
					"got %v, valid=%v, want %v",
					got,
					valid,
					tt.want,
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

// 测试非法跳转指令的运行时错误
func TestVMJumpErrors(t *testing.T) {
	tests := []struct {
		name     string
		bytecode *compiler.Bytecode
		wantErr  string
	}{
		{
			name: "无条件跳转地址越界",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpJump), 0, 100,
				},
			},
			wantErr: "jump target",
		},
		{
			name: "条件跳转栈为空",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpJumpIfFalse), 0, 3,
				},
			},
			wantErr: "stack underflow",
		},
		{
			name: "条件跳转要求 bool",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpConstant), 0, 0,
					byte(code.OpJumpIfFalse), 0, 6,
				},
				Constants: []value.Value{value.NewInt(10)},
			},
			wantErr: "requires bool",
		},
		{
			name: "条件跳转地址越界",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpFalse),
					byte(code.OpJumpIfFalse), 0, 100,
				},
			},
			wantErr: "jump target",
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
