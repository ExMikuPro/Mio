package typecheck

import (
	"fmt"

	"Mio/internal/ast"
)

// checkBuiltinCall 检查内置函数调用
func (c *Checker) checkBuiltinCall(
	name string,
	args []ast.Expression,
) (Kind, error) {
	switch name {

	// chant 输出一个基础类型的值
	case "chant":
		if len(args) != 1 {
			return KindInvalid, fmt.Errorf(
				"builtin %q expects 1 argument, got %d",
				name,
				len(args),
			)
		}

		// 检查实参表达式的类型
		kind, err := c.checkExpression(args[0])
		if err != nil {
			return KindInvalid, err
		}

		// 仅允许可输出的基础类型
		switch kind {
		case KindInt, KindBool, KindString, KindNil:
			return KindUnit, nil

		default:
			return KindInvalid, fmt.Errorf(
				"builtin %q cannot accept %s",
				name,
				kind,
			)
		}

	default:
		return KindInvalid, fmt.Errorf(
			"unsupported builtin function %q",
			name,
		)
	}
}
