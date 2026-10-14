package code

import "testing"

// 测试字节码反汇编输出
func TestInstructionsString(t *testing.T) {
	tests := []struct {
		name string
		ins  Instructions
		want string
	}{
		{
			name: "基础算术表达式",
			ins: Instructions{
				byte(OpConstant), 0, 0,
				byte(OpConstant), 0, 1,
				byte(OpAdd),
				byte(OpPop),
			},
			want: "0000 OpConstant 0\n" +
				"0003 OpConstant 1\n" +
				"0006 OpAdd\n" +
				"0007 OpPop\n",
		},
		{
			name: "最大常量索引",
			ins: Instructions{
				byte(OpConstant), 255, 255,
			},
			want: "0000 OpConstant 65535\n",
		},
		{
			name: "空指令序列",
			ins:  Instructions{},
			want: "",
		},
		{
			name: "未知操作码",
			ins: Instructions{
				255,
				byte(OpAdd),
			},
			want: "0000 ERROR: unknown opcode 255\n" +
				"0001 OpAdd\n",
		},
		{
			name: "操作数不完整",
			ins: Instructions{
				byte(OpConstant), 1,
			},
			want: "0000 ERROR: OpConstant operand 1: not enough bytes\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ins.String()

			if got != tt.want {
				t.Fatalf(
					"got:\n%s\nwant:\n%s",
					got,
					tt.want,
				)
			}
		})
	}
}
