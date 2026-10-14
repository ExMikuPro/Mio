package compiler

import (
	"fmt"

	"Mio/internal/ast"
	"Mio/internal/code"
	"Mio/internal/resolver"
)

// compileExpressionStatement 丢弃普通表达式的结果；unit 调用没有栈值可弹出。
func (c *Compiler) compileExpressionStatement(expr ast.Expression) error {
	switch n := expr.(type) {
	case *ast.GroupedExpression:
		return c.compileExpressionStatement(n.Expr)
	case *ast.CallExpression:
		// 当前唯一可编译的调用是返回 unit 的内置 chant。
		// 增加其他调用时，必须按其返回类型决定是否生成 OpPop。
		return c.compileCall(n)
	default:
		if err := c.Compile(expr); err != nil {
			return err
		}
		return c.emit(code.OpPop)
	}
}

// compileCall 只编译 Resolver 绑定到内置符号的调用。
// 与其他表达式一样，实参类型由编译前的语义检查保证。
func (c *Compiler) compileCall(call *ast.CallExpression) error {
	callee, ok := call.Function.(*ast.Identifier)
	if !ok {
		return fmt.Errorf("call target must be a function name, got %T", call.Function)
	}
	if c.resolver == nil {
		return fmt.Errorf("function call compilation requires a name resolver")
	}
	symbol, ok := c.resolver.BindingOf(callee)
	if !ok {
		return fmt.Errorf("missing symbol binding for function %q", callee.Name)
	}
	if symbol.Kind == resolver.SymbolFunction {
		return fmt.Errorf("user function call %q is not supported yet", symbol.Name)
	}
	if symbol.Kind != resolver.SymbolBuiltin {
		return fmt.Errorf("cannot call %s %q", symbol.Kind, symbol.Name)
	}
	if symbol.Name != "chant" {
		return fmt.Errorf("unsupported builtin function %q", symbol.Name)
	}
	if len(call.Arguments) != 1 {
		return fmt.Errorf("builtin %q expects 1 argument, got %d", symbol.Name, len(call.Arguments))
	}
	if err := c.Compile(call.Arguments[0]); err != nil {
		return err
	}
	return c.emit(code.OpChant)
}
