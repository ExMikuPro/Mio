package value

// Kind 表示运行时值的种类
type Kind uint8

const (
	KindInvalid Kind = iota
	KindInt
	KindBool
	KindString
	KindNil
)

// String 返回运行时类型名称
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
	default:
		return "invalid"
	}
}

// Value 表示 Mio 的运行时值
type Value struct {
	kind    Kind
	integer int64
	boolean bool
	text    string
}

// NewInt 创建整数值
func NewInt(n int64) Value {
	return Value{
		kind:    KindInt,
		integer: n,
	}
}

// NewBool 创建布尔值
func NewBool(b bool) Value {
	return Value{
		kind:    KindBool,
		boolean: b,
	}
}

// NewString 创建字符串值
func NewString(s string) Value {
	return Value{
		kind: KindString,
		text: s,
	}
}

// NewNil 创建空值
func NewNil() Value {
	return Value{
		kind: KindNil,
	}
}

// Kind 返回值的运行时类型
func (v Value) Kind() Kind {
	return v.kind
}

// Int64 读取整数值
func (v Value) Int64() (int64, bool) {
	if v.kind != KindInt {
		return 0, false
	}

	return v.integer, true
}

// Bool 读取布尔值
func (v Value) Bool() (bool, bool) {
	if v.kind != KindBool {
		return false, false
	}

	return v.boolean, true
}

// Text 读取字符串值
func (v Value) Text() (string, bool) {
	if v.kind != KindString {
		return "", false
	}

	return v.text, true
}

// IsNil 判断是否为空值
func (v Value) IsNil() bool {
	return v.kind == KindNil
}
