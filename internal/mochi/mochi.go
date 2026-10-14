// Package mochi 编解码并验证 Mio 字节码文件。
//
// Mochi v1 使用确定性的大端字节序布局。
// Mochi v2 在 v1 主指令流之后追加函数表。
//
//	5 字节     文件签名 "MOCHI"
//	uint16    格式版本
//	uint32    常量数量
//	重复的常量数据：
//	  uint8   类型标签（1 = int64，2 = UTF-8 字符串）
//	  int64   整数值，或
//	  uint32  字符串字节数及对应的 UTF-8 数据
//	uint32    全局槽位数量
//	uint32    指令字节数
//	[]byte    指令数据
//
//	仅 v2：
//	uint32    函数数量
//	重复的函数数据：
//	  uint16 FunctionID
//	  uint16 名称字节数及对应的 UTF-8 数据
//	  uint32 参数数量
//	  uint32 局部槽位数量
//	  uint8  返回值标志
//	  uint32 指令字节数及对应的指令数据
//
// 两个版本都不包含填充和校验和，也不允许文件尾数据。
package mochi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"Mio/internal/compiler"
	"Mio/internal/value"
)

const (
	// Version 表示 Encode 写入的 Mochi 格式版本
	Version uint16 = 2
	// VersionV1 表示兼容读取的旧格式版本
	VersionV1 uint16 = 1
	// Extension 表示 Mochi 字节码文件扩展名
	Extension = ".mochi"

	// 编码和解码使用相同的资源限制
	MaxFileSize          = 64 << 20
	MaxConstants         = 1 << 16
	MaxGlobals           = 1 << 16
	MaxStringBytes       = 16 << 20
	MaxInstructionBytes  = 16 << 20
	MaxFunctions         = 1 << 16
	MaxFunctionNameBytes = 0xffff

	constantTagInt    byte = 1
	constantTagString byte = 2
	fileSignature          = "MOCHI"
)

// Encode 验证字节码并写入确定的 Mochi v2 数据
func Encode(w io.Writer, bytecode *compiler.Bytecode) error {
	if w == nil {
		return fmt.Errorf("mochi: nil writer")
	}
	if err := Validate(bytecode); err != nil {
		return fmt.Errorf("mochi: invalid bytecode: %w", err)
	}

	size, err := encodedSize(bytecode)
	if err != nil {
		return fmt.Errorf("mochi: %w", err)
	}
	if size > MaxFileSize {
		return fmt.Errorf("mochi: encoded file size %d exceeds limit %d", size, MaxFileSize)
	}

	if err := writeFull(w, []byte(fileSignature)); err != nil {
		return fmt.Errorf("mochi: write signature: %w", err)
	}
	if err := writeUint16(w, Version); err != nil {
		return fmt.Errorf("mochi: write version: %w", err)
	}
	if err := writeUint32(w, uint32(len(bytecode.Constants))); err != nil {
		return fmt.Errorf("mochi: write constant count: %w", err)
	}

	for i, constant := range bytecode.Constants {
		switch constant.Kind() {
		case value.KindInt:
			integer, _ := constant.Int64()
			if err := writeFull(w, []byte{constantTagInt}); err != nil {
				return fmt.Errorf("mochi: write constant %d tag: %w", i, err)
			}
			if err := writeUint64(w, uint64(integer)); err != nil {
				return fmt.Errorf("mochi: write constant %d integer: %w", i, err)
			}

		case value.KindString:
			text, _ := constant.Text()
			if err := writeFull(w, []byte{constantTagString}); err != nil {
				return fmt.Errorf("mochi: write constant %d tag: %w", i, err)
			}
			if err := writeUint32(w, uint32(len(text))); err != nil {
				return fmt.Errorf("mochi: write constant %d string length: %w", i, err)
			}
			if err := writeFull(w, []byte(text)); err != nil {
				return fmt.Errorf("mochi: write constant %d string: %w", i, err)
			}
		}
	}

	if err := writeUint32(w, uint32(bytecode.GlobalsCount)); err != nil {
		return fmt.Errorf("mochi: write global slot count: %w", err)
	}
	if err := writeUint32(w, uint32(len(bytecode.Instructions))); err != nil {
		return fmt.Errorf("mochi: write instruction length: %w", err)
	}
	if err := writeFull(w, bytecode.Instructions); err != nil {
		return fmt.Errorf("mochi: write instructions: %w", err)
	}
	if err := writeUint32(w, uint32(len(bytecode.Functions))); err != nil {
		return fmt.Errorf("mochi: write function count: %w", err)
	}
	for i, function := range bytecode.Functions {
		if err := writeUint16(w, uint16(function.ID)); err != nil {
			return fmt.Errorf("mochi: write function %d ID: %w", i, err)
		}
		if err := writeUint16(w, uint16(len(function.Name))); err != nil {
			return fmt.Errorf("mochi: write function %d name length: %w", i, err)
		}
		if err := writeFull(w, []byte(function.Name)); err != nil {
			return fmt.Errorf("mochi: write function %d name: %w", i, err)
		}
		if err := writeUint32(w, uint32(function.ParameterCount)); err != nil {
			return fmt.Errorf("mochi: write function %d parameter count: %w", i, err)
		}
		if err := writeUint32(w, uint32(function.LocalCount)); err != nil {
			return fmt.Errorf("mochi: write function %d local count: %w", i, err)
		}
		flags := byte(0)
		if function.ReturnsValue {
			flags = 1
		}
		if err := writeFull(w, []byte{flags}); err != nil {
			return fmt.Errorf("mochi: write function %d flags: %w", i, err)
		}
		if err := writeUint32(w, uint32(len(function.Instructions))); err != nil {
			return fmt.Errorf("mochi: write function %d instruction length: %w", i, err)
		}
		if err := writeFull(w, function.Instructions); err != nil {
			return fmt.Errorf("mochi: write function %d instructions: %w", i, err)
		}
	}

	return nil
}

// Decode 读取并验证 Mochi v1 或 v2 数据
// 返回的常量和指令切片均为新分配。
func Decode(r io.Reader) (*compiler.Bytecode, error) {
	if r == nil {
		return nil, fmt.Errorf("mochi: nil reader")
	}

	limited := &io.LimitedReader{R: r, N: MaxFileSize + 1}

	var signature [len(fileSignature)]byte
	if err := readFull(limited, signature[:], "signature"); err != nil {
		return nil, err
	}
	if string(signature[:]) != fileSignature {
		return nil, fmt.Errorf("mochi: invalid file signature")
	}

	version, err := readUint16(limited, "version")
	if err != nil {
		return nil, err
	}
	if version != VersionV1 && version != Version {
		return nil, fmt.Errorf("mochi: unsupported format version %d", version)
	}

	constantCount, err := readUint32(limited, "constant count")
	if err != nil {
		return nil, err
	}
	if constantCount > MaxConstants {
		return nil, fmt.Errorf("mochi: constant count %d exceeds limit %d", constantCount, MaxConstants)
	}
	declaredSize := uint64(len(fileSignature) + 2 + 4)

	constants := make([]value.Value, int(constantCount))
	for i := range constants {
		var tag [1]byte
		if err := readFull(limited, tag[:], fmt.Sprintf("constant %d tag", i)); err != nil {
			return nil, err
		}
		declaredSize++

		switch tag[0] {
		case constantTagInt:
			declaredSize += 8
			if err := checkDeclaredSize(declaredSize); err != nil {
				return nil, err
			}
			bits, err := readUint64(limited, fmt.Sprintf("constant %d integer", i))
			if err != nil {
				return nil, err
			}
			constants[i] = value.NewInt(int64(bits))

		case constantTagString:
			length, err := readUint32(limited, fmt.Sprintf("constant %d string length", i))
			if err != nil {
				return nil, err
			}
			if length > MaxStringBytes {
				return nil, fmt.Errorf("mochi: constant %d string length %d exceeds limit %d", i, length, MaxStringBytes)
			}
			declaredSize += 4 + uint64(length)
			if err := checkDeclaredSize(declaredSize); err != nil {
				return nil, err
			}

			data := make([]byte, int(length))
			if err := readFull(limited, data, fmt.Sprintf("constant %d string data", i)); err != nil {
				return nil, err
			}
			if !utf8.Valid(data) {
				return nil, fmt.Errorf("mochi: constant %d contains invalid UTF-8", i)
			}
			constants[i] = value.NewString(string(data))

		default:
			return nil, fmt.Errorf("mochi: constant %d has invalid type tag %d", i, tag[0])
		}
	}

	globalCount, err := readUint32(limited, "global slot count")
	if err != nil {
		return nil, err
	}
	if globalCount > MaxGlobals {
		return nil, fmt.Errorf("mochi: global slot count %d exceeds limit %d", globalCount, MaxGlobals)
	}
	declaredSize += 4

	instructionLength, err := readUint32(limited, "instruction length")
	if err != nil {
		return nil, err
	}
	if instructionLength > MaxInstructionBytes {
		return nil, fmt.Errorf("mochi: instruction length %d exceeds limit %d", instructionLength, MaxInstructionBytes)
	}
	declaredSize += 4 + uint64(instructionLength)
	if err := checkDeclaredSize(declaredSize); err != nil {
		return nil, err
	}

	instructions := make([]byte, int(instructionLength))
	if err := readFull(limited, instructions, "instruction data"); err != nil {
		return nil, err
	}

	var functions []compiler.CompiledFunction
	if version == Version {
		functionCount, err := readUint32(limited, "function count")
		if err != nil {
			return nil, err
		}
		if functionCount > MaxFunctions {
			return nil, fmt.Errorf("mochi: function count %d exceeds limit %d", functionCount, MaxFunctions)
		}
		declaredSize += 4
		if functionCount > 0 {
			functions = make([]compiler.CompiledFunction, int(functionCount))
		}
		for i := range functions {
			id, err := readUint16(limited, fmt.Sprintf("function %d ID", i))
			if err != nil {
				return nil, err
			}
			nameLength, err := readUint16(limited, fmt.Sprintf("function %d name length", i))
			if err != nil {
				return nil, err
			}
			declaredSize += 4 + uint64(nameLength)
			if err := checkDeclaredSize(declaredSize); err != nil {
				return nil, err
			}
			name := make([]byte, int(nameLength))
			if err := readFull(limited, name, fmt.Sprintf("function %d name", i)); err != nil {
				return nil, err
			}
			if !utf8.Valid(name) {
				return nil, fmt.Errorf("mochi: function %d name contains invalid UTF-8", i)
			}
			parameterCount, err := readUint32(limited, fmt.Sprintf("function %d parameter count", i))
			if err != nil {
				return nil, err
			}
			localCount, err := readUint32(limited, fmt.Sprintf("function %d local count", i))
			if err != nil {
				return nil, err
			}
			var flags [1]byte
			if err := readFull(limited, flags[:], fmt.Sprintf("function %d flags", i)); err != nil {
				return nil, err
			}
			if flags[0]&^byte(1) != 0 {
				return nil, fmt.Errorf("mochi: function %d has invalid flags %d", i, flags[0])
			}
			length, err := readUint32(limited, fmt.Sprintf("function %d instruction length", i))
			if err != nil {
				return nil, err
			}
			if length > MaxInstructionBytes {
				return nil, fmt.Errorf("mochi: function %d instruction length %d exceeds limit %d", i, length, MaxInstructionBytes)
			}
			declaredSize += 4 + 4 + 1 + 4 + uint64(length)
			if err := checkDeclaredSize(declaredSize); err != nil {
				return nil, err
			}
			functionInstructions := make([]byte, int(length))
			if err := readFull(limited, functionInstructions, fmt.Sprintf("function %d instruction data", i)); err != nil {
				return nil, err
			}
			functions[i] = compiler.CompiledFunction{
				ID:             int(id),
				Name:           string(name),
				Instructions:   functionInstructions,
				ParameterCount: int(parameterCount),
				LocalCount:     int(localCount),
				ReturnsValue:   flags[0]&1 != 0,
			}
		}
	}

	var extra [1]byte
	if _, err := io.ReadFull(limited, extra[:]); err == nil {
		return nil, fmt.Errorf("mochi: trailing file data")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("mochi: check trailing data: %w", err)
	}

	bytecode := &compiler.Bytecode{
		Instructions: instructions,
		Constants:    constants,
		GlobalsCount: int(globalCount),
		Functions:    functions,
	}
	if err := Validate(bytecode); err != nil {
		return nil, fmt.Errorf("mochi: invalid bytecode: %w", err)
	}

	return bytecode, nil
}

func encodedSize(bytecode *compiler.Bytecode) (uint64, error) {
	size := uint64(len(fileSignature) + 2 + 4 + 4 + 4 + 4)
	for i, constant := range bytecode.Constants {
		switch constant.Kind() {
		case value.KindInt:
			size += 1 + 8
		case value.KindString:
			text, _ := constant.Text()
			size += 1 + 4 + uint64(len(text))
		default:
			return 0, fmt.Errorf("constant %d has unsupported kind %s", i, constant.Kind())
		}
	}
	size += uint64(len(bytecode.Instructions))
	for _, function := range bytecode.Functions {
		size += 2 + 2 + uint64(len(function.Name)) + 4 + 4 + 1 + 4 + uint64(len(function.Instructions))
	}
	return size, nil
}

func checkDeclaredSize(size uint64) error {
	if size > MaxFileSize {
		return fmt.Errorf("mochi: declared file size %d exceeds limit %d", size, MaxFileSize)
	}
	return nil
}

func readFull(r io.Reader, data []byte, field string) error {
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("mochi: truncated %s: %w", field, err)
	}
	return nil
}

func readUint16(r io.Reader, field string) (uint16, error) {
	var data [2]byte
	if err := readFull(r, data[:], field); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(data[:]), nil
}

func readUint32(r io.Reader, field string) (uint32, error) {
	var data [4]byte
	if err := readFull(r, data[:], field); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(data[:]), nil
}

func readUint64(r io.Reader, field string) (uint64, error) {
	var data [8]byte
	if err := readFull(r, data[:], field); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(data[:]), nil
}

func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func writeUint16(w io.Writer, value uint16) error {
	var data [2]byte
	binary.BigEndian.PutUint16(data[:], value)
	return writeFull(w, data[:])
}

func writeUint32(w io.Writer, value uint32) error {
	var data [4]byte
	binary.BigEndian.PutUint32(data[:], value)
	return writeFull(w, data[:])
}

func writeUint64(w io.Writer, value uint64) error {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], value)
	return writeFull(w, data[:])
}
