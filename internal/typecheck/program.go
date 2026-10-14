package typecheck

import (
	"fmt"

	"Mio/internal/ast"
	"Mio/internal/resolver"
)

// CheckProgram 对完成名称解析的程序执行类型检查
func (c *Checker) CheckProgram(program *ast.Program) error {
	if c.resolver == nil {
		return fmt.Errorf("type checker requires a name resolver")
	}

	if program == nil {
		return fmt.Errorf("cannot type-check a nil program")
	}

	return c.checkStatements(program.Statements)
}

// checkStatements 先注册函数签名，再检查语句
func (c *Checker) checkStatements(statements []ast.Statement) error {
	// 第一遍：预注册当前作用域的函数签名
	for _, stmt := range statements {
		fn, ok := stmt.(*ast.FunctionStatement)
		if !ok {
			continue
		}

		if err := c.registerFunctionSignature(fn); err != nil {
			return err
		}
	}

	// 第二遍：检查语句及函数体
	for _, stmt := range statements {
		if err := c.checkStatement(stmt); err != nil {
			return err
		}
	}

	return nil
}

// checkStatement 检查语句的静态类型约束
func (c *Checker) checkStatement(stmt ast.Statement) error {
	switch n := stmt.(type) {

	// 变量声明
	case *ast.LetStatement:
		kind, err := c.checkExpression(n.Value)
		if err != nil {
			return err
		}

		// 暂不允许从 mu 推断变量类型
		if kind == KindNil {
			return fmt.Errorf(
				"cannot infer type of %q from nil",
				n.Name.Name,
			)
		}

		// 不允许将无返回值表达式保存为变量
		if kind == KindUnit {
			return fmt.Errorf(
				"cannot infer type of %q from unit",
				n.Name.Name,
			)
		}

		symbol, ok := c.resolver.BindingOf(n.Name)
		if !ok {
			return fmt.Errorf(
				"missing symbol binding for declaration %q",
				n.Name.Name,
			)
		}

		c.types[symbol] = kind
		return nil

	// 变量赋值
	case *ast.AssignStatement:
		symbol, ok := c.resolver.BindingOf(n.Name)
		if !ok {
			return fmt.Errorf(
				"missing symbol binding for assignment %q",
				n.Name.Name,
			)
		}

		if symbol.Kind != resolver.SymbolVariable {
			return fmt.Errorf(
				"cannot assign to %s %q",
				symbol.Kind,
				n.Name.Name,
			)
		}

		expected, ok := c.types[symbol]
		if !ok {
			return fmt.Errorf(
				"type of assignment target %q is not available",
				n.Name.Name,
			)
		}

		actual, err := c.checkExpression(n.Value)
		if err != nil {
			return err
		}

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

	// 表达式语句
	case *ast.ExpressionStatement:
		_, err := c.checkExpression(n.Expr)
		return err

	// moshi / sore 条件语句
	case *ast.IfStatement:
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

		if err := c.checkBlock(n.Consequence); err != nil {
			return err
		}

		if n.Alternative != nil {
			return c.checkBlock(n.Alternative)
		}

		return nil

	// 函数定义
	case *ast.FunctionStatement:
		return c.checkFunction(n)

		// kaesu 返回语句
	case *ast.ReturnStatement:
		if len(c.returnTypes) == 0 {
			return fmt.Errorf("kaesu outside function")
		}

		// 获取当前函数的返回类型
		expected := c.returnTypes[len(c.returnTypes)-1]

		// 没有返回表达式时视为 unit
		actual := KindUnit

		if n.Value != nil {
			kind, err := c.checkExpression(n.Value)
			if err != nil {
				return err
			}

			actual = kind
		}

		// 检查声明类型与实际返回类型
		if actual != expected {
			pos := n.Span().Start

			return fmt.Errorf(
				"return type mismatch: expected %s, got %s at %d:%d",
				expected,
				actual,
				pos.Line,
				pos.Column,
			)
		}

		// unit 函数不允许返回具体表达式
		if expected == KindUnit && n.Value != nil {
			pos := n.Span().Start

			return fmt.Errorf(
				"unit function cannot return a value at %d:%d",
				pos.Line,
				pos.Column,
			)
		}

		return nil

	default:
		return fmt.Errorf(
			"unsupported statement %T",
			stmt,
		)
	}
}

// checkFunction 检查函数形参、函数体与返回类型
func (c *Checker) checkFunction(fn *ast.FunctionStatement) error {
	// 获取预注册的函数签名
	signature, ok := c.SignatureOf(fn.Name)
	if !ok {
		return fmt.Errorf(
			"missing function signature for %q",
			fn.Name.Name,
		)
	}

	if len(signature.ParameterTypes) != len(fn.Parameters) {
		return fmt.Errorf(
			"incorrect parameter count in signature for %q",
			fn.Name.Name,
		)
	}

	// 将形参符号与声明类型绑定
	for i, param := range fn.Parameters {
		symbol, ok := c.resolver.BindingOf(param)
		if !ok {
			return fmt.Errorf(
				"missing symbol binding for parameter %q",
				param.Name,
			)
		}

		if symbol.Kind != resolver.SymbolParameter {
			return fmt.Errorf(
				"%q is not a parameter symbol",
				param.Name,
			)
		}

		c.types[symbol] = signature.ParameterTypes[i]
	}

	// 进入函数时保存预期返回类型
	c.returnTypes = append(c.returnTypes, signature.ReturnType)

	// 离开函数时恢复外层返回类型
	defer func() {
		c.returnTypes = c.returnTypes[:len(c.returnTypes)-1]
	}()

	// 检查函数体中的全部语句
	if err := c.checkBlock(fn.Body); err != nil {
		return err
	}

	// 非 unit 函数必须在所有路径上返回
	if signature.ReturnType != KindUnit &&
		!blockAlwaysReturns(fn.Body) {

		return fmt.Errorf(
			"function %q may exit without returning %s",
			fn.Name.Name,
			signature.ReturnType,
		)
	}

	return nil
}

// checkBlock 检查代码块中的所有语句
func (c *Checker) checkBlock(block *ast.BlockStatement) error {
	if block == nil {
		return fmt.Errorf("missing block statement")
	}

	return c.checkStatements(block.Statements)
}

// blockAlwaysReturns 保守判断代码块是否在所有路径上返回
func blockAlwaysReturns(block *ast.BlockStatement) bool {
	if block == nil {
		return false
	}

	for _, stmt := range block.Statements {
		switch n := stmt.(type) {

		case *ast.ReturnStatement:
			return true

		case *ast.IfStatement:
			// 两个分支都返回，条件语句才保证返回
			if n.Alternative != nil &&
				blockAlwaysReturns(n.Consequence) &&
				blockAlwaysReturns(n.Alternative) {
				return true
			}
		}
	}

	return false
}
