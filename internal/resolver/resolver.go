package resolver

import (
	"Mio/internal/ast"
	"fmt"
)

// 名称解析器
type Resolver struct {
	scope         *Scope
	functionDepth int

	bindings map[*ast.Identifier]Symbol
}

// 创建名称解析器
func New() *Resolver {
	// 创建内置符号作用域
	builtins := NewScope()
	// 注册内置函数
	if err := builtins.DefineSymbol(Symbol{
		Name: "chant",
		Kind: SymbolBuiltin,
	}); err != nil {
		panic(err)
	}

	// 创建用户程序全局作用域
	globals := NewEnclosedScope(builtins)

	return &Resolver{
		scope:    globals,
		bindings: make(map[*ast.Identifier]Symbol),
	}
}

// 解析程序中的名称引用
func (r *Resolver) ResolveProgram(program *ast.Program) error {
	return r.resolveStatements(program.Statements)
}

// 解析语句中的符号定义与引用
func (r *Resolver) resolveStatement(stmt ast.Statement) error {
	switch n := stmt.(type) {

	// kaku 变量声明
	case *ast.LetStatement:
		// 先检查初始化表达式
		if err := r.resolveExpression(n.Value); err != nil {
			return err
		}

		// 构造变量符号
		symbol := Symbol{
			Name: n.Name.Name,
			Kind: SymbolVariable,
			Decl: n.Name.Span().Start,
		}

		// 定义到当前作用域
		if err := r.scope.DefineSymbol(symbol); err != nil {
			pos := n.Name.Span().Start

			return fmt.Errorf(
				"%d:%d: %w",
				pos.Line,
				pos.Column,
				err,
			)
		}

		// 保存声明节点的符号绑定
		r.bindings[n.Name] = symbol

		return nil

	case *ast.AssignStatement:
		// 检查赋值目标是否已经声明
		symbol, ok := r.scope.Resolve(n.Name.Name)

		if !ok {
			pos := n.Name.Span().Start

			return fmt.Errorf(
				"undefined assignment target %q at %d:%d",
				n.Name.Name,
				pos.Line,
				pos.Column,
			)
		}

		// 保存赋值目标的符号绑定
		r.bindings[n.Name] = symbol

		return r.resolveExpression(n.Value)

	case *ast.IfStatement:
		// 条件表达式在当前作用域中解析
		if err := r.resolveExpression(n.Condition); err != nil {
			return err
		}

		// 解析 moshi 分支
		if err := r.resolveBlock(n.Consequence); err != nil {
			return err
		}

		// 解析可选的 sore 分支
		if n.Alternative != nil {
			return r.resolveBlock(n.Alternative)
		}
		return nil

	case *ast.ExpressionStatement:
		return r.resolveExpression(n.Expr)

	// kaesu 返回语句
	case *ast.ReturnStatement:
		// 检查是否在函数内部
		if r.functionDepth == 0 {
			pos := n.Span().Start

			return fmt.Errorf(
				"kaesu outside function at %d:%d",
				pos.Line,
				pos.Column,
			)
		}

		// 检查返回表达式中的名称
		return r.resolveExpression(n.Value)

	// waza 函数定义
	case *ast.FunctionStatement:
		return r.resolveFunctionStatement(n)

	default:
		return fmt.Errorf(
			"unsupported statement %T",
			stmt,
		)
	}
}

func (r *Resolver) resolveExpression(expr ast.Expression) error {
	switch n := expr.(type) {

	// 标识符
	// 标识符引用
	case *ast.Identifier:
		// 查找当前作用域及其父作用域
		symbol, ok := r.scope.Resolve(n.Name)

		if !ok {
			pos := n.Span().Start

			return fmt.Errorf(
				"undefined identifier %q at %d:%d",
				n.Name,
				pos.Line,
				pos.Column,
			)
		}

		// 记录标识符对应的符号
		r.bindings[n] = symbol

		return nil
	// 中缀表达式
	case *ast.InfixExpression:
		if err := r.resolveExpression(n.Left); err != nil {
			return err
		}
		return r.resolveExpression(n.Right)

	// 一元表达式
	case *ast.PrefixExpression:
		return r.resolveExpression(n.Right)

	// 括号分组表达式
	case *ast.GroupedExpression:
		return r.resolveExpression(n.Expr)

	// 函数调用表达式
	case *ast.CallExpression:
		// 检查被调用的名称或表达式
		if err := r.resolveExpression(n.Function); err != nil {
			return err
		}

		// 递归检查每个实参
		for _, arg := range n.Arguments {
			if err := r.resolveExpression(arg); err != nil {
				return err
			}
		}
		return nil

	// 基础字面量
	case *ast.IntegerLiteral,
		*ast.StringLiteral,
		*ast.BooleanLiteral,
		*ast.NilLiteral:
		return nil

	default:
		return fmt.Errorf(
			"unsupported expression %T",
			expr,
		)
	}
}

// 解析代码块中的局部作用域
func (r *Resolver) resolveBlock(block *ast.BlockStatement) error {
	// 保存当前作用域
	parent := r.scope

	// 创建代码作用域
	r.scope = NewEnclosedScope(parent)

	// 离开代码块恢复作用域
	defer func() {
		r.scope = parent
	}()

	return r.resolveStatements(block.Statements)
}

func (r *Resolver) resolveFunctionStatement(fn *ast.FunctionStatement) error {
	// 函数名称已经在 resolveStatements 中预声明

	// 保存外层作用域
	parent := r.scope

	// 创建函数作用域
	r.scope = NewEnclosedScope(parent)

	// 进入函数上下文
	r.functionDepth++

	// 离开函数时恢复作用域与上下文
	defer func() {
		r.functionDepth--
		r.scope = parent
	}()

	// 定义函数形参
	// 定义函数形参
	for _, param := range fn.Parameters {
		symbol := Symbol{
			Name: param.Name,
			Kind: SymbolParameter,
			Decl: param.Span().Start,
		}

		if err := r.scope.DefineSymbol(symbol); err != nil {
			pos := param.Span().Start

			return fmt.Errorf(
				"%d:%d: %w",
				pos.Line,
				pos.Column,
				err,
			)
		}

		// 保存形参声明节点的绑定
		r.bindings[param] = symbol
	}

	// 解析函数体中的语句
	return r.resolveStatements(fn.Body.Statements)
}

// 解析统一作用域中的语句
func (r *Resolver) resolveStatements(statements []ast.Statement) error {
	// 预声明函数名称
	// 预声明当前作用域中的函数
	for _, stmt := range statements {
		fn, ok := stmt.(*ast.FunctionStatement)
		if !ok {
			continue
		}

		// 构造函数符号
		symbol := Symbol{
			Name: fn.Name.Name,
			Kind: SymbolFunction,
			Decl: fn.Name.Span().Start,
		}

		// 注册函数名称
		if err := r.scope.DefineSymbol(symbol); err != nil {
			pos := fn.Name.Span().Start

			return fmt.Errorf(
				"%d:%d: %w",
				pos.Line,
				pos.Column,
				err,
			)
		}

		// 保存函数声明节点的绑定
		r.bindings[fn.Name] = symbol
	}

	// 按顺序解析所有语句
	for _, stmt := range statements {
		if err := r.resolveStatement(stmt); err != nil {
			return err
		}
	}

	return nil
}

// 获取标识符绑定的符号
func (r *Resolver) BindingOf(ident *ast.Identifier) (Symbol, bool) {
	symbol, ok := r.bindings[ident]
	return symbol, ok
}
