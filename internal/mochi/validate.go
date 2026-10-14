package mochi

import (
	"fmt"
	"unicode/utf8"

	"Mio/internal/code"
	"Mio/internal/compiler"
	"Mio/internal/value"
)

type instructionReference struct {
	offset int
	value  int
}

// Validate 验证字节码结构及索引引用
// 该函数不检查 Stack 深度和运行时类型。
func Validate(bytecode *compiler.Bytecode) error {
	if bytecode == nil {
		return fmt.Errorf("nil bytecode")
	}
	if len(bytecode.Constants) > MaxConstants {
		return fmt.Errorf("constant count %d exceeds limit %d", len(bytecode.Constants), MaxConstants)
	}
	if bytecode.GlobalsCount < 0 || bytecode.GlobalsCount > MaxGlobals {
		return fmt.Errorf("global slot count %d is outside range 0..%d", bytecode.GlobalsCount, MaxGlobals)
	}
	if len(bytecode.Instructions) > MaxInstructionBytes {
		return fmt.Errorf("instruction length %d exceeds limit %d", len(bytecode.Instructions), MaxInstructionBytes)
	}

	for i, constant := range bytecode.Constants {
		switch constant.Kind() {
		case value.KindInt:
		case value.KindString:
			text, _ := constant.Text()
			if len(text) > MaxStringBytes {
				return fmt.Errorf("constant %d string length %d exceeds limit %d", i, len(text), MaxStringBytes)
			}
			if !utf8.ValidString(text) {
				return fmt.Errorf("constant %d contains invalid UTF-8", i)
			}
		default:
			return fmt.Errorf("constant %d has unsupported kind %s", i, constant.Kind())
		}
	}

	boundaries := make([]bool, len(bytecode.Instructions)+1)
	boundaries[len(bytecode.Instructions)] = true
	references := make([]instructionReference, 0)

	for offset := 0; offset < len(bytecode.Instructions); {
		boundaries[offset] = true
		op := code.Opcode(bytecode.Instructions[offset])
		definition, err := code.Lookup(op)
		if err != nil {
			return fmt.Errorf("unknown opcode at offset %d: %w", offset, err)
		}

		operands, read, err := code.ReadOperands(definition, bytecode.Instructions[offset+1:])
		if err != nil {
			return fmt.Errorf("truncated operands at offset %d: %w", offset, err)
		}
		if 1+read <= 0 || offset+1+read > len(bytecode.Instructions) {
			return fmt.Errorf("invalid instruction width at offset %d", offset)
		}

		switch op {
		case code.OpConstant:
			if operands[0] >= len(bytecode.Constants) {
				return fmt.Errorf("constant index %d out of range at offset %d", operands[0], offset)
			}
		case code.OpSetGlobal, code.OpGetGlobal:
			if operands[0] >= bytecode.GlobalsCount {
				return fmt.Errorf("global slot index %d out of range at offset %d", operands[0], offset)
			}
		case code.OpJump, code.OpJumpIfFalse:
			references = append(references, instructionReference{offset: offset, value: operands[0]})
		}

		offset += 1 + read
	}

	for _, reference := range references {
		if reference.value > len(bytecode.Instructions) {
			return fmt.Errorf("jump target %d out of range at offset %d", reference.value, reference.offset)
		}
		if !boundaries[reference.value] {
			return fmt.Errorf("jump target %d is not an instruction boundary at offset %d", reference.value, reference.offset)
		}
	}

	return nil
}
