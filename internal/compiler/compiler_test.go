package compiler

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"Mio/internal/ast"
	"Mio/internal/code"
	"Mio/internal/lexer"
	"Mio/internal/parser"
	"Mio/internal/token"
)

// 测试整数表达式的字节码编译
func TestCompileIntegerExpressions(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		constants    []int64
		instructions code.Instructions
	}{
		{
			name:      "整数加法",
			input:     "1 + 2;",
			constants: []int64{1, 2},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpAdd),
				byte(code.OpPop),
			},
		},
		{
			name:      "嵌套加法",
			input:     "1 + (2 + 3);",
			constants: []int64{1, 2, 3},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpConstant), 0, 2,
				byte(code.OpAdd),
				byte(code.OpAdd),
				byte(code.OpPop),
			},
		},
		{
			name:      "多条表达式语句",
			input:     "1; 2;",
			constants: []int64{1, 2},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpPop),
				byte(code.OpConstant), 0, 1,
				byte(code.OpPop),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
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

			compiler := New()

			if err := compiler.Compile(program); err != nil {
				t.Fatal(err)
			}

			result := compiler.Bytecode()

			// 检查常量池
			if !reflect.DeepEqual(result.Constants, tt.constants) {
				t.Fatalf(
					"constants: got %v, want %v",
					result.Constants,
					tt.constants,
				)
			}

			// 检查实际生成的字节码
			if !bytes.Equal(result.Instructions, tt.instructions) {
				t.Fatalf(
					"instructions:\ngot:\n%s\nwant:\n%s",
					result.Instructions.String(),
					tt.instructions.String(),
				)
			}
		})
	}
}

// 测试常量池索引溢出
func TestCompileConstantOverflow(t *testing.T) {
	compiler := New()

	// 填满两个字节可表示的常量索引
	compiler.constants = make([]int64, 1<<16)

	err := compiler.Compile(
		&ast.IntegerLiteral{Value: 123},
	)

	if err == nil ||
		!strings.Contains(err.Error(), "constant pool index overflow") {
		t.Fatalf("expected constant pool overflow, got %v", err)
	}
}

// 测试扩展整数运算的字节码编译
func TestCompileExtendedArithmetic(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		constants    []int64
		instructions code.Instructions
	}{
		{
			name:      "整数减法",
			input:     "10 - 3;",
			constants: []int64{10, 3},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpSub),
				byte(code.OpPop),
			},
		},
		{
			name:      "整数乘法",
			input:     "6 * 7;",
			constants: []int64{6, 7},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpMul),
				byte(code.OpPop),
			},
		},
		{
			name:      "整数除法",
			input:     "20 / 4;",
			constants: []int64{20, 4},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpDiv),
				byte(code.OpPop),
			},
		},
		{
			name:      "一元负号",
			input:     "-10;",
			constants: []int64{10},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpNeg),
				byte(code.OpPop),
			},
		},
		{
			name:      "算术优先级",
			input:     "1 + 2 * 3;",
			constants: []int64{1, 2, 3},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpConstant), 0, 2,
				byte(code.OpMul),
				byte(code.OpAdd),
				byte(code.OpPop),
			},
		},
		{
			name:      "括号与一元负号",
			input:     "-(10 - 3);",
			constants: []int64{10, 3},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpSub),
				byte(code.OpNeg),
				byte(code.OpPop),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
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

			compiler := New()
			if err := compiler.Compile(program); err != nil {
				t.Fatal(err)
			}

			result := compiler.Bytecode()

			if !reflect.DeepEqual(result.Constants, tt.constants) {
				t.Fatalf(
					"constants: got %v, want %v",
					result.Constants,
					tt.constants,
				)
			}

			if !bytes.Equal(result.Instructions, tt.instructions) {
				t.Fatalf(
					"instructions:\ngot:\n%s\nwant:\n%s",
					result.Instructions.String(),
					tt.instructions.String(),
				)
			}
		})
	}
}

// 测试布尔值与逻辑非表达式编译
func TestCompileBooleanExpressions(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		instructions code.Instructions
	}{
		{
			name:  "布尔真值",
			input: "hai;",
			instructions: code.Instructions{
				byte(code.OpTrue),
				byte(code.OpPop),
			},
		},
		{
			name:  "布尔假值",
			input: "iie;",
			instructions: code.Instructions{
				byte(code.OpFalse),
				byte(code.OpPop),
			},
		},
		{
			name:  "逻辑非",
			input: "!hai;",
			instructions: code.Instructions{
				byte(code.OpTrue),
				byte(code.OpNot),
				byte(code.OpPop),
			},
		},
		{
			name:  "嵌套逻辑非",
			input: "!!iie;",
			instructions: code.Instructions{
				byte(code.OpFalse),
				byte(code.OpNot),
				byte(code.OpNot),
				byte(code.OpPop),
			},
		},
		{
			name:  "括号分组",
			input: "!(hai);",
			instructions: code.Instructions{
				byte(code.OpTrue),
				byte(code.OpNot),
				byte(code.OpPop),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
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

			compiler := New()

			if err := compiler.Compile(program); err != nil {
				t.Fatal(err)
			}

			result := compiler.Bytecode()

			// 布尔字面量不占用整数常量池
			if len(result.Constants) != 0 {
				t.Fatalf(
					"expected empty constant pool, got %v",
					result.Constants,
				)
			}

			// 检查生成的指令
			if !bytes.Equal(
				result.Instructions,
				tt.instructions,
			) {
				t.Fatalf(
					"instructions:\ngot:\n%s\nwant:\n%s",
					result.Instructions.String(),
					tt.instructions.String(),
				)
			}
		})
	}
}

// 测试比较表达式的字节码编译
func TestCompileComparisonExpressions(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		constants    []int64
		instructions code.Instructions
	}{
		{
			name:      "整数相等",
			input:     "10 == 10;",
			constants: []int64{10, 10},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpEqual),
				byte(code.OpPop),
			},
		},
		{
			name:      "整数不等",
			input:     "10 != 20;",
			constants: []int64{10, 20},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpNotEqual),
				byte(code.OpPop),
			},
		},
		{
			name:      "整数小于",
			input:     "5 < 10;",
			constants: []int64{5, 10},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpLess),
				byte(code.OpPop),
			},
		},
		{
			name:      "整数小于等于",
			input:     "5 <= 10;",
			constants: []int64{5, 10},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpLessEqual),
				byte(code.OpPop),
			},
		},
		{
			name:      "整数大于",
			input:     "10 > 5;",
			constants: []int64{10, 5},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpGreater),
				byte(code.OpPop),
			},
		},
		{
			name:      "整数大于等于",
			input:     "10 >= 5;",
			constants: []int64{10, 5},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpGreaterEqual),
				byte(code.OpPop),
			},
		},
		{
			name:  "布尔值不等比较",
			input: "hai != iie;",
			instructions: code.Instructions{
				byte(code.OpTrue),
				byte(code.OpFalse),
				byte(code.OpNotEqual),
				byte(code.OpPop),
			},
		},
		{
			name:      "算术与比较嵌套",
			input:     "(1 + 2) > 2;",
			constants: []int64{1, 2, 2},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpConstant), 0, 1,
				byte(code.OpAdd),
				byte(code.OpConstant), 0, 2,
				byte(code.OpGreater),
				byte(code.OpPop),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
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

			compiler := New()

			if err := compiler.Compile(program); err != nil {
				t.Fatal(err)
			}

			result := compiler.Bytecode()

			if !reflect.DeepEqual(result.Constants, tt.constants) {
				t.Fatalf(
					"constants: got %v, want %v",
					result.Constants,
					tt.constants,
				)
			}

			if !bytes.Equal(result.Instructions, tt.instructions) {
				t.Fatalf(
					"instructions:\ngot:\n%s\nwant:\n%s",
					result.Instructions.String(),
					tt.instructions.String(),
				)
			}
		})
	}
}

// 测试条件语句的跳转编译
func TestCompileIfStatements(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		constants    []int64
		instructions code.Instructions
	}{
		{
			name:      "无 sore 分支",
			input:     "moshi (hai) { 10; }",
			constants: []int64{10},
			instructions: code.Instructions{
				byte(code.OpTrue),
				byte(code.OpJumpIfFalse), 0, 8,
				byte(code.OpConstant), 0, 0,
				byte(code.OpPop),
			},
		},
		{
			name:      "包含 sore 分支",
			input:     "moshi (iie) { 10; } sore { 20; }",
			constants: []int64{10, 20},
			instructions: code.Instructions{
				byte(code.OpFalse),
				byte(code.OpJumpIfFalse), 0, 11,
				byte(code.OpConstant), 0, 0,
				byte(code.OpPop),
				byte(code.OpJump), 0, 15,
				byte(code.OpConstant), 0, 1,
				byte(code.OpPop),
			},
		},
		{
			name: "嵌套条件语句",
			input: `moshi (hai) {
                moshi (iie) {
                    1;
                } sore {
                    2;
                }
            }`,
			constants: []int64{1, 2},
			instructions: code.Instructions{
				byte(code.OpTrue),
				byte(code.OpJumpIfFalse), 0, 19,
				byte(code.OpFalse),
				byte(code.OpJumpIfFalse), 0, 15,
				byte(code.OpConstant), 0, 0,
				byte(code.OpPop),
				byte(code.OpJump), 0, 19,
				byte(code.OpConstant), 0, 1,
				byte(code.OpPop),
			},
		},
		{
			name: "绝对跳转地址",
			input: `1;
                moshi (iie) {
                    2;
                }`,
			constants: []int64{1, 2},
			instructions: code.Instructions{
				byte(code.OpConstant), 0, 0,
				byte(code.OpPop),
				byte(code.OpFalse),
				byte(code.OpJumpIfFalse), 0, 12,
				byte(code.OpConstant), 0, 1,
				byte(code.OpPop),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
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

			compiler := New()
			if err := compiler.Compile(program); err != nil {
				t.Fatal(err)
			}

			got := compiler.Bytecode()

			if !reflect.DeepEqual(got.Constants, tt.constants) {
				t.Fatalf(
					"constants: got %v, want %v",
					got.Constants,
					tt.constants,
				)
			}

			if !bytes.Equal(got.Instructions, tt.instructions) {
				t.Fatalf(
					"instructions:\ngot:\n%s\nwant:\n%s",
					got.Instructions.String(),
					tt.instructions.String(),
				)
			}
		})
	}
}

// 测试逻辑运算的短路字节码编译
func TestCompileShortCircuitExpressions(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		constants []int64
		want      string
	}{
		{
			name:  "逻辑与",
			input: "hai && iie;",
			want: "0000 OpTrue\n" +
				"0001 OpJumpIfFalse 8\n" +
				"0004 OpFalse\n" +
				"0005 OpJump 9\n" +
				"0008 OpFalse\n" +
				"0009 OpPop\n",
		},
		{
			name:  "逻辑或",
			input: "hai || iie;",
			want: "0000 OpTrue\n" +
				"0001 OpJumpIfFalse 8\n" +
				"0004 OpTrue\n" +
				"0005 OpJump 9\n" +
				"0008 OpFalse\n" +
				"0009 OpPop\n",
		},
		{
			name:      "逻辑与跳过右侧",
			input:     "iie && (10 > 5);",
			constants: []int64{10, 5},
			want: "0000 OpFalse\n" +
				"0001 OpJumpIfFalse 14\n" +
				"0004 OpConstant 0\n" +
				"0007 OpConstant 1\n" +
				"0010 OpGreater\n" +
				"0011 OpJump 15\n" +
				"0014 OpFalse\n" +
				"0015 OpPop\n",
		},
		{
			name:      "逻辑或跳过右侧",
			input:     "hai || (10 > 5);",
			constants: []int64{10, 5},
			want: "0000 OpTrue\n" +
				"0001 OpJumpIfFalse 8\n" +
				"0004 OpTrue\n" +
				"0005 OpJump 15\n" +
				"0008 OpConstant 0\n" +
				"0011 OpConstant 1\n" +
				"0014 OpGreater\n" +
				"0015 OpPop\n",
		},
		{
			name:  "嵌套逻辑表达式",
			input: "(hai && iie) || hai;",
			want: "0000 OpTrue\n" +
				"0001 OpJumpIfFalse 8\n" +
				"0004 OpFalse\n" +
				"0005 OpJump 9\n" +
				"0008 OpFalse\n" +
				"0009 OpJumpIfFalse 16\n" +
				"0012 OpTrue\n" +
				"0013 OpJump 17\n" +
				"0016 OpTrue\n" +
				"0017 OpPop\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := lexer.NewChecked(tt.input, token.V01)
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

			compiler := New()

			if err := compiler.Compile(program); err != nil {
				t.Fatal(err)
			}

			result := compiler.Bytecode()

			if !reflect.DeepEqual(
				result.Constants,
				tt.constants,
			) {
				t.Fatalf(
					"constants: got %v, want %v",
					result.Constants,
					tt.constants,
				)
			}

			got := result.Instructions.String()

			if got != tt.want {
				t.Fatalf(
					"instructions:\ngot:\n%s\nwant:\n%s",
					got,
					tt.want,
				)
			}
		})
	}
}
