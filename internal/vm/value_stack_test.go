package vm

import (
	"testing"

	"Mio/internal/compiler"
	"Mio/internal/value"
)

// 测试整数执行结果使用 Value 保存
func TestVMValueStack(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{
			name:  "整数加法",
			input: "10 + 20;",
			want:  30,
		},
		{
			name:  "整数减法",
			input: "10 - 3;",
			want:  7,
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
			name:  "一元取负",
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

			// 获取统一运行时值
			result, ok := machine.LastPoppedValue()
			if !ok {
				t.Fatal("expected popped value")
			}

			// 检查运行时类型
			if result.Kind() != value.KindInt {
				t.Fatalf(
					"got kind %s, want int64",
					result.Kind(),
				)
			}

			// 读取整数结果
			number, ok := result.Int64()
			if !ok || number != tt.want {
				t.Fatalf(
					"got %d, ok=%v, want %d",
					number,
					ok,
					tt.want,
				)
			}

			// 旧接口应返回相同的整数
			legacy, ok := machine.LastPoppedStackElem()
			if !ok || legacy != tt.want {
				t.Fatalf(
					"legacy result: got %d, ok=%v",
					legacy,
					ok,
				)
			}

			// 表达式语句执行后栈应为空
			if len(machine.stack) != 0 {
				t.Fatalf(
					"expected empty stack, got %v",
					machine.stack,
				)
			}
		})
	}
}

// 测试重新执行时清理上一次的结果
func TestVMValueReset(t *testing.T) {
	machine := New(compileSource(t, "1 + 2;"))

	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}

	if _, ok := machine.LastPoppedValue(); !ok {
		t.Fatal("expected first execution result")
	}

	// 使用空字节码再次执行
	machine.bytecode = &compiler.Bytecode{}

	if err := machine.Run(); err != nil {
		t.Fatal(err)
	}

	if _, ok := machine.LastPoppedValue(); ok {
		t.Fatal("expected last popped value to be reset")
	}

	if _, ok := machine.LastPoppedStackElem(); ok {
		t.Fatal("expected legacy result to be reset")
	}
}
