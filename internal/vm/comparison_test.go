package vm

import (
	"strings"
	"testing"

	"Mio/internal/code"
	"Mio/internal/compiler"
	"Mio/internal/value"
)

// 测试比较表达式的执行结果
func TestVMComparisonExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"整数相等", "10 == 10;", true},
		{"整数不等", "10 != 20;", true},
		{"相等结果为假", "10 == 20;", false},
		{"整数小于", "5 < 10;", true},
		{"整数小于等于", "10 <= 10;", true},
		{"整数大于", "10 > 5;", true},
		{"整数大于等于", "10 >= 10;", true},
		{"比较顺序", "5 > 10;", false},
		{"布尔相等", "hai == hai;", true},
		{"布尔不等", "hai != iie;", true},
		{"算术与比较组合", "(1 + 2) > 2;", true},
		{"比较结果取反", "!(10 < 5);", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytecode := compileSource(t, tt.input)
			machine := New(bytecode)

			if err := machine.Run(); err != nil {
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

			got, ok := result.Bool()
			if !ok || got != tt.want {
				t.Fatalf(
					"got %v, ok=%v, want %v",
					got,
					ok,
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

// 测试非法比较操作的运行时错误
func TestVMComparisonErrors(t *testing.T) {
	tests := []struct {
		name     string
		bytecode *compiler.Bytecode
		wantErr  string
	}{
		{
			name: "比较操作数不足",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpEqual),
				},
			},
			wantErr: "stack underflow",
		},
		{
			name: "整数与布尔值不能直接比较相等",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpConstant), 0, 0,
					byte(code.OpTrue),
					byte(code.OpEqual),
				},
				Constants: []int64{10},
			},
			wantErr: "requires matching types",
		},
		{
			name: "布尔值不能比较大小",
			bytecode: &compiler.Bytecode{
				Instructions: code.Instructions{
					byte(code.OpTrue),
					byte(code.OpFalse),
					byte(code.OpGreater),
				},
			},
			wantErr: "requires int64 operands",
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

// 测试字符串运行时值的相等比较
func TestVMStringComparison(t *testing.T) {
	machine := New(nil)

	// 当前 Compiler 尚未支持字符串常量编译
	machine.stack = []value.Value{
		value.NewString("Mio"),
		value.NewString("Mio"),
	}

	if err := machine.executeComparison(
		code.OpEqual,
		"OpEqual",
	); err != nil {
		t.Fatal(err)
	}

	if len(machine.stack) != 1 {
		t.Fatalf(
			"got stack size %d, want 1",
			len(machine.stack),
		)
	}

	result, ok := machine.stack[0].Bool()
	if !ok || !result {
		t.Fatalf(
			"got %v, ok=%v, want true",
			result,
			ok,
		)
	}
}
