package typecheck

import (
	"fmt"

	"Mio/internal/ast"
	"Mio/internal/resolver"
)

// FunctionSignature 保存函数参数与返回类型
type FunctionSignature struct {
	ParameterTypes []Kind
	ReturnType     Kind
}

// resolveTypeRef 将源码类型名称转换为内部类型
func resolveTypeRef(ref *ast.TypeRef) (Kind, error) {
	if ref == nil {
		return KindInvalid, fmt.Errorf(
			"missing type annotation",
		)
	}

	switch ref.Name {
	case "int64":
		return KindInt, nil

	case "bool":
		return KindBool, nil

	case "string":
		return KindString, nil

	case "unit":
		return KindUnit, nil

	default:
		pos := ref.Span().Start

		return KindInvalid, fmt.Errorf(
			"unknown type %q at %d:%d",
			ref.Name,
			pos.Line,
			pos.Column,
		)
	}
}

// registerFunctionSignature 检查并登记函数签名
func (c *Checker) registerFunctionSignature(
	fn *ast.FunctionStatement,
) error {
	if c.resolver == nil {
		return fmt.Errorf(
			"function signature requires a name resolver",
		)
	}

	if fn == nil || fn.Name == nil {
		return fmt.Errorf("invalid function declaration")
	}

	// 获取函数声明对应的符号
	symbol, ok := c.resolver.BindingOf(fn.Name)
	if !ok {
		return fmt.Errorf(
			"missing symbol binding for function %q",
			fn.Name.Name,
		)
	}

	if symbol.Kind != resolver.SymbolFunction {
		return fmt.Errorf(
			"%q is not a function symbol",
			fn.Name.Name,
		)
	}

	// 函数必须声明返回类型
	if fn.ReturnType == nil {
		return fmt.Errorf(
			"function %q requires a return type",
			fn.Name.Name,
		)
	}

	returnType, err := resolveTypeRef(fn.ReturnType)
	if err != nil {
		return err
	}

	signature := FunctionSignature{
		ParameterTypes: make([]Kind, 0, len(fn.Parameters)),
		ReturnType:     returnType,
	}

	// 检查每个形参的类型标注
	for _, param := range fn.Parameters {
		if param.DeclaredType == nil {
			return fmt.Errorf(
				"parameter %q requires a type annotation",
				param.Name,
			)
		}

		kind, err := resolveTypeRef(param.DeclaredType)
		if err != nil {
			return err
		}

		signature.ParameterTypes = append(
			signature.ParameterTypes,
			kind,
		)
	}

	// 登记经过验证的完整函数签名
	c.signatures[symbol] = signature

	return nil
}

// SignatureOf 查询标识符绑定的函数签名
func (c *Checker) SignatureOf(
	ident *ast.Identifier,
) (FunctionSignature, bool) {
	if c.resolver == nil || ident == nil {
		return FunctionSignature{}, false
	}

	symbol, ok := c.resolver.BindingOf(ident)
	if !ok {
		return FunctionSignature{}, false
	}

	signature, ok := c.signatures[symbol]
	return signature, ok
}
