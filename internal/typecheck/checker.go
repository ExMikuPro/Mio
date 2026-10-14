package typecheck

import (
	"fmt"

	"Mio/internal/ast"
	"Mio/internal/resolver"
	"Mio/internal/token"
)

// Checker 负责表达式的静态类型检查
type Checker struct {
	resolver *resolver.Resolver
	types    map[resolver.Symbol]Kind
}

// New 创建类型检查器
func New() *Checker {
	return &Checker{
		types: make(map[resolver.Symbol]Kind),
	}
}

// NewWithResolver 创建带符号绑定的类型检查器
func NewWithResolver(r *resolver.Resolver) *Checker {
	c := New()
	c.resolver = r
	return c
}

// checkExpression 推断并检查表达式的类型
func (c *Checker) checkExpression(expr ast.Expression) (Kind, error) {
	switch n := expr.(type) {

	// 基础字面量
	case *ast.IntegerLiteral:
		return KindInt, nil

	case *ast.BooleanLiteral:
		return KindBool, nil

	case *ast.StringLiteral:
		return KindString, nil

	case *ast.NilLiteral:
		return KindNil, nil

	// 括号不会改变内部表达式的类型
	case *ast.GroupedExpression:
		return c.checkExpression(n.Expr)

	// 一元表达式
	case *ast.PrefixExpression:
		right, err := c.checkExpression(n.Right)
		if err != nil {
			return KindInvalid, err
		}

		switch n.Operator {
		case token.Minus:
			if right != KindInt {
				return KindInvalid, fmt.Errorf(
					"operator '-' requires int64, got %s",
					right,
				)
			}

			return KindInt, nil

		case token.Bang:
			if right != KindBool {
				return KindInvalid, fmt.Errorf(
					"operator '!' requires bool, got %s",
					right,
				)
			}

			return KindBool, nil

		default:
			return KindInvalid, fmt.Errorf(
				"unsupported prefix operator %s",
				n.Operator,
			)
		}

	// 二元表达式
	case *ast.InfixExpression:
		// 检查左操作数类型
		left, err := c.checkExpression(n.Left)
		if err != nil {
			return KindInvalid, err
		}

		// 检查右操作数类型
		right, err := c.checkExpression(n.Right)
		if err != nil {
			return KindInvalid, err
		}

		// 根据运算符验证操作数类型
		switch n.Operator {

		// 算术运算
		case token.Plus,
			token.Minus,
			token.Asterisk,
			token.Slash:

			if left != KindInt || right != KindInt {
				return KindInvalid, fmt.Errorf(
					"operator %s requires int64 operands, got %s and %s",
					n.Operator,
					left,
					right,
				)
			}

			return KindInt, nil

		// 大小比较
		case token.Less,
			token.LessEqual,
			token.Greater,
			token.GreaterEqual:

			if left != KindInt || right != KindInt {
				return KindInvalid, fmt.Errorf(
					"operator %s requires int64 operands, got %s and %s",
					n.Operator,
					left,
					right,
				)
			}

			return KindBool, nil

		// 相等比较
		case token.Equal, token.NotEqual:
			if left != right {
				return KindInvalid, fmt.Errorf(
					"operator %s requires matching types, got %s and %s",
					n.Operator,
					left,
					right,
				)
			}

			// 暂时只支持基础非空类型
			switch left {
			case KindInt, KindBool, KindString:
				return KindBool, nil

			default:
				return KindInvalid, fmt.Errorf(
					"operator %s does not support %s operands",
					n.Operator,
					left,
				)
			}

		// 逻辑运算
		case token.And, token.Or:
			if left != KindBool || right != KindBool {
				return KindInvalid, fmt.Errorf(
					"operator %s requires bool operands, got %s and %s",
					n.Operator,
					left,
					right,
				)
			}

			return KindBool, nil

		default:
			return KindInvalid, fmt.Errorf(
				"unsupported infix operator %s",
				n.Operator,
			)
		}

	case *ast.Identifier:
		if c.resolver == nil {
			return KindInvalid, fmt.Errorf(
				"identifier %q requires resolved symbols",
				n.Name,
			)
		}

		// 获取 Resolver 确定的符号
		symbol, ok := c.resolver.BindingOf(n)
		if !ok {
			return KindInvalid, fmt.Errorf(
				"missing symbol binding for %q",
				n.Name,
			)
		}

		// 查询符号对应的静态类型
		kind, ok := c.types[symbol]
		if !ok {
			return KindInvalid, fmt.Errorf(
				"type of identifier %q is not available",
				n.Name,
			)
		}

		return kind, nil

	default:
		return KindInvalid, fmt.Errorf(
			"unsupported expression %T",
			expr,
		)
	}
}
