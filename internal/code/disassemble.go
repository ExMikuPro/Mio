package code

import (
	"fmt"
	"strings"
)

// String 将字节码格式化为带偏移量的指令列表
func (ins Instructions) String() string {
	var out strings.Builder

	for offset := 0; offset < len(ins); {
		// 读取当前操作码
		opcode := Opcode(ins[offset])

		// 查询指令定义
		definition, err := Lookup(opcode)
		if err != nil {
			fmt.Fprintf(
				&out,
				"%04d ERROR: %v\n",
				offset,
				err,
			)

			offset++
			continue
		}

		// 跳过操作码并解码操作数
		operands, read, err := ReadOperands(
			definition,
			ins[offset+1:],
		)
		if err != nil {
			fmt.Fprintf(
				&out,
				"%04d ERROR: %v\n",
				offset,
				err,
			)
			break
		}

		// 输出指令偏移量和名称
		fmt.Fprintf(
			&out,
			"%04d %s",
			offset,
			definition.Name,
		)

		// 输出操作数
		for _, operand := range operands {
			fmt.Fprintf(&out, " %d", operand)
		}

		out.WriteByte('\n')

		// 移动到下一条指令
		offset += 1 + read
	}

	return out.String()
}
