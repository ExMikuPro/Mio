package ast

import (
	"testing"

	"Mio/internal/token"
)

// 测试整数表达式、语句和程序根节点的组合
func TestIntegerLiteralTree(t *testing.T) {
	start := token.Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	integerEnd := token.Position{
		Offset: 3,
		Line:   1,
		Column: 4,
	}

	statementEnd := token.Position{
		Offset: 4,
		Line:   1,
		Column: 5,
	}

	// 构造整数表达式
	integer := &IntegerLiteral{
		Value: 123,
		Range: Span{
			Start: start,
			End:   integerEnd,
		},
	}

	// 构造表达式语句
	statement := &ExpressionStatement{
		Expr: integer,
		Range: Span{
			Start: start,
			End:   statementEnd,
		},
	}

	// 构造程序根节点
	program := &Program{
		Statements: []Statement{statement},
		Range:      statement.Range,
	}

	// 检查程序节点
	if len(program.Statements) != 1 {
		t.Fatal("expected one statement")
	}

	// 检查整数字面量
	if integer.Value != 123 {
		t.Fatalf(
			"got %d, want 123",
			integer.Value,
		)
	}

	// 验证整数与语句的源码范围
	if integer.Span().End != integerEnd {
		t.Fatalf("incorrect integer span")
	}

	if statement.Span().End != statementEnd {
		t.Fatalf("incorrect statement span")
	}
}

// 测试标识符名称及源码范围
func TestIdentifierNode(t *testing.T) {
	start := token.Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	end := token.Position{
		Offset: 3,
		Line:   1,
		Column: 4,
	}

	identifier := &Identifier{
		Name: "age",
		Range: Span{
			Start: start,
			End:   end,
		},
	}

	// 检查标识符名称
	if identifier.Name != "age" {
		t.Fatalf(
			"got %q, want age",
			identifier.Name,
		)
	}

	// 验证标识符的起止位置
	if identifier.Span().Start != start {
		t.Fatal("incorrect start position")
	}

	if identifier.Span().End != end {
		t.Fatal("incorrect end position")
	}
}

// 测试中缀表达式的 AST 嵌套结构
func TestInfixExpressionTree(t *testing.T) {
	// 构造第一行的源码位置
	pos := func(offset int) token.Position {
		return token.Position{
			Offset: offset,
			Line:   1,
			Column: offset + 1,
		}
	}

	// 对应源码：1 + 2 * 3;
	one := &IntegerLiteral{
		Value: 1,
		Range: Span{Start: pos(0), End: pos(1)},
	}

	two := &IntegerLiteral{
		Value: 2,
		Range: Span{Start: pos(4), End: pos(5)},
	}

	three := &IntegerLiteral{
		Value: 3,
		Range: Span{Start: pos(8), End: pos(9)},
	}

	// 构造乘法子表达式
	multiply := &InfixExpression{
		Left:     two,
		Operator: token.Asterisk,
		Right:    three,
		Range:    Span{Start: pos(4), End: pos(9)},
	}

	// 构造加法根表达式
	add := &InfixExpression{
		Left:     one,
		Operator: token.Plus,
		Right:    multiply,
		Range:    Span{Start: pos(0), End: pos(9)},
	}

	// 验证加法位于 AST 根节点
	if add.Operator != token.Plus {
		t.Fatal("root operator should be PLUS")
	}

	// 验证乘法位于加法的右子树
	right, ok := add.Right.(*InfixExpression)
	if !ok {
		t.Fatal("right should be InfixExpression")
	}

	if right.Operator != token.Asterisk {
		t.Fatal("right operator should be ASTERISK")
	}

	// 验证完整表达式的源码范围
	if add.Span().Start != pos(0) ||
		add.Span().End != pos(9) {
		t.Fatal("incorrect expression span")
	}
}
