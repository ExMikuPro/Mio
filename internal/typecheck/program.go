package typecheck

import (
	"Mio/internal/ast"
	"Mio/internal/resolver"
	"fmt"
)

// CheckProgram 检查名称解析完成后的程序
func (c *Checker) CheckProgram(program *ast.Program) error {
	if c.resolver == nil {
		return fmt.Errorf("type checker requires a name resolver")
	}

	if program == nil {
		return fmt.Errorf("cannot type-check a nil program")
	}

	for _, stmt := range program.Statements {
		if err := c.checkStatement(stmt); err != nil {
			return err
		}
	}
	return nil
}

// checkStatement 检查变量声明赋值和表达式
func (c *Checker) checkStatement(stmt ast.Statement) error {
	switch n := stmt.(type) {

	// 变量声明
	case *ast.LetStatement:
		// 推断初始化表达式的类型
		kind, err := c.checkExpression(n.Value)
		if err != nil {
			return err
		}

		// 暂不允许仅通过 mu 推断变量类型
		if kind == KindNil {
			return fmt.Errorf(
				"cannot infer type of %q from nil",
				n.Name.Name,
			)
		}

		// 获取声明对应的符号
		symbol, ok := c.resolver.BindingOf(n.Name)
		if !ok {
			return fmt.Errorf(
				"missing symbol binding for declaration %q",
				n.Name.Name,
			)
		}

		// 保存变量静态类型
		c.types[symbol] = kind
		return nil

	case *ast.AssignStatement:
		symbol, ok := c.resolver.BindingOf(n.Name)
		if !ok {
			return fmt.Errorf(
				"missing symbol binding for assignment %q",
				n.Name.Name,
			)
		}

		// 本阶段只允许普通变量作为赋值目标
		if symbol.Kind != resolver.SymbolVariable {
			return fmt.Errorf(
				"cannot assign to %s %q",
				symbol.Kind,
				n.Name.Name,
			)
		}

		// 获取变量声明时确定的类型
		expected, ok := c.types[symbol]
		if !ok {
			return fmt.Errorf(
				"type of assignment target %q is not available",
				n.Name.Name,
			)
		}

		// 推断新值的类型
		actual, err := c.checkExpression(n.Value)
		if err != nil {
			return err
		}

		// 禁止赋值改变量类型
		if actual != expected {
			pos := n.Name.Span().Start
			return fmt.Errorf(
				"cannot assign %s to %q (declared %s) at %d:%d",
				actual,
				n.Name.Name,
				expected,
				pos.Line,
				pos.Column,
			)
		}
		return nil

	// 普通表达式语句
	case *ast.ExpressionStatement:
		_, err := c.checkExpression(n.Expr)
		return err

		// moshi / sore 条件语句
	case *ast.IfStatement:
		// 条件表达式必须是 bool
		conditionType, err := c.checkExpression(n.Condition)
		if err != nil {
			return err
		}

		if conditionType != KindBool {
			pos := n.Condition.Span().Start

			return fmt.Errorf(
				"moshi condition requires bool, got %s at %d:%d",
				conditionType,
				pos.Line,
				pos.Column,
			)
		}

		// 检查 moshi 分支
		if err := c.checkBlock(n.Consequence); err != nil {
			return err
		}

		// 检查可选的 sore 分支
		if n.Alternative != nil {
			return c.checkBlock(n.Alternative)
		}

		return nil

	default:
		return fmt.Errorf(
			"unsupported statement %T",
			stmt,
		)
	}
}

// checkBlock 检查代码块中的所有语句
func (c *Checker) checkBlock(block *ast.BlockStatement) error {
	if block == nil {
		return fmt.Errorf("missing block statement")
	}

	for _, stmt := range block.Statements {
		if err := c.checkStatement(stmt); err != nil {
			return err
		}
	}

	return nil
}
