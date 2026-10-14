package typecheck

// Kind Mio 类型种类
type Kind uint8

const (
	KindInvalid Kind = iota
	KindInt
	KindBool
	KindString
	KindNil
	KindUnit
)

// String 返回类型名称
func (k Kind) String() string {
	switch k {
	case KindInt:
		return "int64"

	case KindBool:
		return "bool"

	case KindString:
		return "string"

	case KindNil:
		return "nil"

	case KindUnit:
		return "unit"

	default:
		return "invalid"
	}
}
