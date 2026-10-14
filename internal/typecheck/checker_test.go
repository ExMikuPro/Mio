package typecheck

import (
	"strings"
	"testing"

	"Mio/internal/ast"
	"Mio/internal/token"
)

// 测试基础字面量的类型推断
func TestCheckLiteralExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expression
		want Kind
	}{
		{"整数", &ast.IntegerLiteral{Value: 123}, KindInt},
		{"布尔", &ast.BooleanLiteral{Value: true}, KindBool},
		{"字符串", &ast.StringLiteral{Value: "Hello Mio"}, KindString},
		{"空值", &ast.NilLiteral{}, KindNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New().checkExpression(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// 测试一元表达式与括号分组
func TestCheckPrefixExpressions(t *testing.T) {
	tests := []struct {
		name    string
		expr    ast.Expression
		want    Kind
		wantErr string
	}{
		{
			name: "整数取负",
			expr: &ast.PrefixExpression{
				Operator: token.Minus,
				Right:    &ast.IntegerLiteral{Value: 10},
			},
			want: KindInt,
		},
		{
			name: "布尔逻辑非",
			expr: &ast.PrefixExpression{
				Operator: token.Bang,
				Right:    &ast.BooleanLiteral{Value: true},
			},
			want: KindBool,
		},
		{
			name: "括号分组",
			expr: &ast.GroupedExpression{
				Expr: &ast.IntegerLiteral{Value: 123},
			},
			want: KindInt,
		},
		{
			name: "嵌套取负",
			expr: &ast.PrefixExpression{
				Operator: token.Minus,
				Right: &ast.PrefixExpression{
					Operator: token.Minus,
					Right:    &ast.IntegerLiteral{Value: 10},
				},
			},
			want: KindInt,
		},
		{
			name: "布尔值不能取负",
			expr: &ast.PrefixExpression{
				Operator: token.Minus,
				Right:    &ast.BooleanLiteral{Value: true},
			},
			want:    KindInvalid,
			wantErr: "requires int64",
		},
		{
			name: "整数不能逻辑非",
			expr: &ast.PrefixExpression{
				Operator: token.Bang,
				Right:    &ast.IntegerLiteral{Value: 10},
			},
			want:    KindInvalid,
			wantErr: "requires bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New().checkExpression(tt.expr)
			assertExpressionResult(
				t, got, err, tt.want, tt.wantErr,
			)
		})
	}
}

// 测试整数算术运算及非法类型组合
func TestCheckArithmeticExpressions(t *testing.T) {
	tests := []struct {
		name    string
		expr    ast.Expression
		want    Kind
		wantErr string
	}{
		{
			name: "加法",
			expr: &ast.InfixExpression{
				Left:     &ast.IntegerLiteral{Value: 10},
				Operator: token.Plus,
				Right:    &ast.IntegerLiteral{Value: 20},
			},
			want: KindInt,
		},
		{
			name: "减法",
			expr: &ast.InfixExpression{
				Left:     &ast.IntegerLiteral{Value: 10},
				Operator: token.Minus,
				Right:    &ast.IntegerLiteral{Value: 20},
			},
			want: KindInt,
		},
		{
			name: "乘法",
			expr: &ast.InfixExpression{
				Left:     &ast.IntegerLiteral{Value: 10},
				Operator: token.Asterisk,
				Right:    &ast.IntegerLiteral{Value: 20},
			},
			want: KindInt,
		},
		{
			name: "除法",
			expr: &ast.InfixExpression{
				Left:     &ast.IntegerLiteral{Value: 20},
				Operator: token.Slash,
				Right:    &ast.IntegerLiteral{Value: 10},
			},
			want: KindInt,
		},
		{
			name: "嵌套算术",
			expr: &ast.InfixExpression{
				Left: &ast.InfixExpression{
					Left:     &ast.IntegerLiteral{Value: 10},
					Operator: token.Plus,
					Right:    &ast.IntegerLiteral{Value: 20},
				},
				Operator: token.Asterisk,
				Right:    &ast.IntegerLiteral{Value: 3},
			},
			want: KindInt,
		},
		{
			name: "整数与字符串不能相加",
			expr: &ast.InfixExpression{
				Left:     &ast.IntegerLiteral{Value: 10},
				Operator: token.Plus,
				Right:    &ast.StringLiteral{Value: "20"},
			},
			want:    KindInvalid,
			wantErr: "requires int64 operands",
		},
		{
			name: "布尔值不能相加",
			expr: &ast.InfixExpression{
				Left:     &ast.BooleanLiteral{Value: true},
				Operator: token.Plus,
				Right:    &ast.BooleanLiteral{Value: false},
			},
			want:    KindInvalid,
			wantErr: "requires int64 operands",
		},
		{
			name: "拒绝赋值运算符作为二元表达式",
			expr: &ast.InfixExpression{
				Left:     &ast.IntegerLiteral{Value: 10},
				Operator: token.Assign,
				Right:    &ast.IntegerLiteral{Value: 10},
			},
			want:    KindInvalid,
			wantErr: "unsupported infix operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New().checkExpression(tt.expr)
			assertExpressionResult(
				t, got, err, tt.want, tt.wantErr,
			)
		})
	}
}

// 测试比较与逻辑运算的类型规则
func TestCheckComparisonAndLogicalExpressions(t *testing.T) {
	tests := []struct {
		name    string
		left    ast.Expression
		op      token.Type
		right   ast.Expression
		want    Kind
		wantErr string
	}{
		{
			"整数相等",
			&ast.IntegerLiteral{Value: 10},
			token.Equal,
			&ast.IntegerLiteral{Value: 20},
			KindBool, "",
		},
		{
			"布尔不等",
			&ast.BooleanLiteral{Value: true},
			token.NotEqual,
			&ast.BooleanLiteral{Value: false},
			KindBool, "",
		},
		{
			"字符串相等",
			&ast.StringLiteral{Value: "Mio"},
			token.Equal,
			&ast.StringLiteral{Value: "Mio"},
			KindBool, "",
		},
		{
			"整数小于",
			&ast.IntegerLiteral{Value: 10},
			token.Less,
			&ast.IntegerLiteral{Value: 20},
			KindBool, "",
		},
		{
			"整数小于等于",
			&ast.IntegerLiteral{Value: 10},
			token.LessEqual,
			&ast.IntegerLiteral{Value: 20},
			KindBool, "",
		},
		{
			"整数大于",
			&ast.IntegerLiteral{Value: 20},
			token.Greater,
			&ast.IntegerLiteral{Value: 10},
			KindBool, "",
		},
		{
			"整数大于等于",
			&ast.IntegerLiteral{Value: 20},
			token.GreaterEqual,
			&ast.IntegerLiteral{Value: 10},
			KindBool, "",
		},
		{
			"逻辑与",
			&ast.BooleanLiteral{Value: true},
			token.And,
			&ast.BooleanLiteral{Value: false},
			KindBool, "",
		},
		{
			"逻辑或",
			&ast.BooleanLiteral{Value: true},
			token.Or,
			&ast.BooleanLiteral{Value: false},
			KindBool, "",
		},
		{
			"类型不同不能相等比较",
			&ast.IntegerLiteral{Value: 10},
			token.Equal,
			&ast.StringLiteral{Value: "10"},
			KindInvalid, "requires matching types",
		},
		{
			"布尔值不能比大小",
			&ast.BooleanLiteral{Value: true},
			token.Greater,
			&ast.BooleanLiteral{Value: false},
			KindInvalid, "requires int64 operands",
		},
		{
			"整数不能逻辑与",
			&ast.IntegerLiteral{Value: 10},
			token.And,
			&ast.IntegerLiteral{Value: 20},
			KindInvalid, "requires bool operands",
		},
		{
			"空值暂不支持相等比较",
			&ast.NilLiteral{},
			token.Equal,
			&ast.NilLiteral{},
			KindInvalid, "does not support nil operands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.InfixExpression{
				Left:     tt.left,
				Operator: tt.op,
				Right:    tt.right,
			}

			got, err := New().checkExpression(expr)
			assertExpressionResult(
				t, got, err, tt.want, tt.wantErr,
			)
		})
	}
}

// 测试比较、逻辑非与逻辑与的嵌套组合
func TestCheckNestedLogicalExpression(t *testing.T) {
	expr := &ast.InfixExpression{
		Left: &ast.GroupedExpression{
			Expr: &ast.InfixExpression{
				Left:     &ast.IntegerLiteral{Value: 10},
				Operator: token.Less,
				Right:    &ast.IntegerLiteral{Value: 20},
			},
		},
		Operator: token.And,
		Right: &ast.PrefixExpression{
			Operator: token.Bang,
			Right:    &ast.BooleanLiteral{Value: false},
		},
	}

	got, err := New().checkExpression(expr)
	assertExpressionResult(t, got, err, KindBool, "")
}

// 校验推断结果与预期错误
func assertExpressionResult(
	t *testing.T,
	got Kind,
	err error,
	want Kind,
	wantErr string,
) {
	t.Helper()

	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	if wantErr == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}

	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf(
			"expected error containing %q, got %v",
			wantErr,
			err,
		)
	}
}

// 测试尚未实现的函数调用表达式
func TestUnsupportedExpression(t *testing.T) {
	expr := &ast.CallExpression{
		Function: &ast.Identifier{Name: "chant"},
	}

	got, err := New().checkExpression(expr)

	if got != KindInvalid ||
		err == nil ||
		!strings.Contains(err.Error(), "unsupported expression") {
		t.Fatalf(
			"expected unsupported expression error, got kind=%s err=%v",
			got,
			err,
		)
	}
}
