package vm

import (
	"fmt"

	"Mio/internal/code"
	"Mio/internal/compiler"
	"Mio/internal/value"
)

// VM 执行 Mio 字节码
type VM struct {
	bytecode *compiler.Bytecode
	stack    []value.Value
	globals  []value.Value

	lastPopped    value.Value
	hasLastPopped bool
}

// New 创建字节码虚拟机
func New(bytecode *compiler.Bytecode) *VM {
	return &VM{
		bytecode: bytecode,
		stack:    make([]value.Value, 0),
	}
}

// Run 解码并执行字节码
func (v *VM) Run() error {
	if v.bytecode == nil {
		return fmt.Errorf("nil bytecode")
	}

	// 检查全局变量槽位数量
	if v.bytecode.GlobalsCount < 0 ||
		v.bytecode.GlobalsCount > 1<<16 {
		return fmt.Errorf(
			"invalid global slot count %d",
			v.bytecode.GlobalsCount,
		)
	}

	// 每次执行时初始化全局变量存储区
	v.globals = make(
		[]value.Value,
		v.bytecode.GlobalsCount,
	)

	// 每次执行前重置运行状态
	v.stack = v.stack[:0]
	v.lastPopped = value.Value{}
	v.hasLastPopped = false

	ins := v.bytecode.Instructions

	// ip 表示当前指令的字节偏移
	for ip := 0; ip < len(ins); {
		op := code.Opcode(ins[ip])

		// 查询指令定义
		def, err := code.Lookup(op)
		if err != nil {
			return fmt.Errorf(
				"at offset %d: %w",
				ip,
				err,
			)
		}

		// 解码操作数
		operands, read, err := code.ReadOperands(
			def,
			ins[ip+1:],
		)
		if err != nil {
			return fmt.Errorf(
				"at offset %d: %w",
				ip,
				err,
			)
		}

		switch op {

		// 将整数常量包装为 Value 并压栈
		case code.OpConstant:
			index := operands[0]

			if index < 0 || index >= len(v.bytecode.Constants) {
				return fmt.Errorf(
					"constant index %d out of range at %d",
					index,
					ip,
				)
			}

			number := v.bytecode.Constants[index]

			v.stack = append(
				v.stack,
				value.NewInt(number),
			)

		// 整数二元算术运算
		case code.OpAdd,
			code.OpSub,
			code.OpMul,
			code.OpDiv:

			size := len(v.stack)
			if size < 2 {
				return fmt.Errorf(
					"stack underflow for %s at %d",
					def.Name,
					ip,
				)
			}

			// 栈顶为右操作数
			rightValue := v.stack[size-1]
			leftValue := v.stack[size-2]

			// 检查右操作数类型
			right, ok := rightValue.Int64()
			if !ok {
				return fmt.Errorf(
					"%s requires int64 right operand, got %s at %d",
					def.Name,
					rightValue.Kind(),
					ip,
				)
			}

			// 检查左操作数类型
			left, ok := leftValue.Int64()
			if !ok {
				return fmt.Errorf(
					"%s requires int64 left operand, got %s at %d",
					def.Name,
					leftValue.Kind(),
					ip,
				)
			}

			var result int64

			// 执行对应的整数运算
			switch op {
			case code.OpAdd:
				result = left + right

			case code.OpSub:
				result = left - right

			case code.OpMul:
				result = left * right

			case code.OpDiv:
				if right == 0 {
					return fmt.Errorf(
						"division by zero at %d",
						ip,
					)
				}

				result = left / right
			}

			// 弹出操作数并压入整数结果
			v.stack = v.stack[:size-2]
			v.stack = append(
				v.stack,
				value.NewInt(result),
			)

		// 整数一元取负
		case code.OpNeg:
			size := len(v.stack)
			if size == 0 {
				return fmt.Errorf(
					"stack underflow for OpNeg at %d",
					ip,
				)
			}

			number, ok := v.stack[size-1].Int64()
			if !ok {
				return fmt.Errorf(
					"OpNeg requires int64, got %s at %d",
					v.stack[size-1].Kind(),
					ip,
				)
			}

			// 替换栈顶值
			v.stack[size-1] = value.NewInt(-number)

		// 弹出表达式结果
		case code.OpPop:
			size := len(v.stack)
			if size == 0 {
				return fmt.Errorf(
					"stack underflow for OpPop at %d",
					ip,
				)
			}

			// 记录最后一次弹出的运行时值
			v.lastPopped = v.stack[size-1]
			v.hasLastPopped = true

			v.stack = v.stack[:size-1]

			// 布尔真值入栈
		case code.OpTrue:
			v.stack = append(
				v.stack,
				value.NewBool(true),
			)

			// 布尔假值入栈
		case code.OpFalse:
			v.stack = append(
				v.stack,
				value.NewBool(false),
			)

			// 执行布尔逻辑非
		case code.OpNot:
			size := len(v.stack)

			// 检查栈是否为空
			if size == 0 {
				return fmt.Errorf(
					"stack underflow for OpNot at %d",
					ip,
				)
			}

			// 读取栈顶布尔值
			boolean, ok := v.stack[size-1].Bool()
			if !ok {
				return fmt.Errorf(
					"OpNot requires bool, got %s at %d",
					v.stack[size-1].Kind(),
					ip,
				)
			}

			// 将取反结果写回栈顶
			v.stack[size-1] = value.NewBool(!boolean)

			// 执行比较运算
		case code.OpEqual,
			code.OpNotEqual,
			code.OpLess,
			code.OpLessEqual,
			code.OpGreater,
			code.OpGreaterEqual:

			if err := v.executeComparison(op, def.Name); err != nil {
				return fmt.Errorf(
					"at offset %d: %w",
					ip,
					err,
				)
			}

			// 无条件跳转
		case code.OpJump:
			target := operands[0]

			// 跳转目标不能超出字节码范围
			if target < 0 || target > len(ins) {
				return fmt.Errorf(
					"jump target %d out of range at %d",
					target,
					ip,
				)
			}

			// 跳转地址使用绝对字节偏移
			ip = target
			continue

		// 条件为假时跳转
		case code.OpJumpIfFalse:
			target := operands[0]

			if target < 0 || target > len(ins) {
				return fmt.Errorf(
					"jump target %d out of range at %d",
					target,
					ip,
				)
			}

			size := len(v.stack)

			if size == 0 {
				return fmt.Errorf(
					"stack underflow for OpJumpIfFalse at %d",
					ip,
				)
			}

			// 条件必须是 bool
			condition, ok := v.stack[size-1].Bool()
			if !ok {
				return fmt.Errorf(
					"OpJumpIfFalse requires bool, got %s at %d",
					v.stack[size-1].Kind(),
					ip,
				)
			}

			// 无论条件真假，都要弹出条件值
			v.stack = v.stack[:size-1]

			// 条件为假时跳转
			if !condition {
				ip = target
				continue
			}

		// 将栈顶值保存到全局变量槽位
		case code.OpSetGlobal:
			index := operands[0]

			// 检查槽位编号
			if index < 0 || index >= len(v.globals) {
				return fmt.Errorf(
					"global slot %d out of range at %d",
					index,
					ip,
				)
			}

			size := len(v.stack)

			// 保存变量需要一个栈顶值
			if size == 0 {
				return fmt.Errorf(
					"stack underflow for OpSetGlobal at %d",
					ip,
				)
			}

			// 将栈顶值写入指定槽位
			v.globals[index] = v.stack[size-1]

			// 保存完成后弹出栈顶值
			v.stack = v.stack[:size-1]

			// 读取全局变量并压栈
		case code.OpGetGlobal:
			index := operands[0]

			// 检查槽位编号
			if index < 0 || index >= len(v.globals) {
				return fmt.Errorf(
					"global slot %d out of range at %d",
					index,
					ip,
				)
			}

			stored := v.globals[index]

			// 禁止读取尚未初始化的槽位
			if stored.Kind() == value.KindInvalid {
				return fmt.Errorf(
					"global slot %d is uninitialized at %d",
					index,
					ip,
				)
			}

			// 将存储的值压入运行时栈
			v.stack = append(v.stack, stored)

		default:
			return fmt.Errorf(
				"unsupported opcode %s at %d",
				def.Name,
				ip,
			)
		}

		// 移动到下一条指令
		ip += 1 + read
	}

	return nil
}

// LastPoppedValue 获取最后弹出的运行时值
func (v *VM) LastPoppedValue() (value.Value, bool) {
	return v.lastPopped, v.hasLastPopped
}

// LastPoppedStackElem 兼容原有的整数结果接口
func (v *VM) LastPoppedStackElem() (int64, bool) {
	if !v.hasLastPopped {
		return 0, false
	}

	return v.lastPopped.Int64()
}
