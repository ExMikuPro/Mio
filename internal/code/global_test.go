package code

import "testing"

// 测试全局变量指令的编码与解码
func TestGlobalInstructions(t *testing.T) {
	tests := []struct {
		name string
		op   Opcode
	}{
		{"保存全局变量", OpSetGlobal},
		{"读取全局变量", OpGetGlobal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ins, err := Make(tt.op, 258)
			if err != nil {
				t.Fatal(err)
			}

			if len(ins) != 3 ||
				ins[0] != byte(tt.op) ||
				ins[1] != 1 ||
				ins[2] != 2 {
				t.Fatalf("incorrect encoding: %v", ins)
			}

			def, err := Lookup(tt.op)
			if err != nil {
				t.Fatal(err)
			}

			operands, read, err := ReadOperands(def, ins[1:])
			if err != nil {
				t.Fatal(err)
			}

			if read != 2 ||
				len(operands) != 1 ||
				operands[0] != 258 {
				t.Fatalf(
					"incorrect decoding: %v, read=%d",
					operands,
					read,
				)
			}
		})
	}
}
