package code

import "testing"

// 测试函数 Opcode 编号保持固定
func TestFunctionOpcodeNumbers(t *testing.T) {
	tests := []struct {
		op   Opcode
		want byte
	}{
		{OpConstant, 1},
		{OpChant, 22},
		{OpSetLocal, 23},
		{OpGetLocal, 24},
		{OpCall, 25},
		{OpReturn, 26},
		{OpReturnValue, 27},
	}
	for _, tt := range tests {
		if byte(tt.op) != tt.want {
			t.Fatalf("opcode %s = %d, want %d", definitions[tt.op].Name, tt.op, tt.want)
		}
	}
}
