package resolver

import (
	"Mio/internal/token"
	"testing"
)

// 测试符号定义与查找
func TestScopeDefineAndResolve(t *testing.T) {
	scope := NewScope()

	// 定义 age
	if err := scope.Define("age"); err != nil {
		t.Fatal(err)
	}

	// 查找已定义符号
	symbol, ok := scope.Resolve("age")
	if !ok {
		t.Fatal("expected symbol age")
	}

	if symbol.Name != "age" {
		t.Fatalf(
			"expected age, got %s",
			symbol.Name,
		)
	}

	// 查找未定义符号
	if _, ok := scope.Resolve("score"); ok {
		t.Fatal("unexpected symbol score")
	}
}

// 测试同一作用域重复定义
func TestScopeDuplicateDefinition(t *testing.T) {
	scope := NewScope()

	if err := scope.Define("age"); err != nil {
		t.Fatal(err)
	}

	// 同名符号不能重复定义
	if err := scope.Define("age"); err == nil {
		t.Fatal("expected duplicate definition error")
	}
}

// 测试子作用域沿父作用域查找符号
func TestEnclosedScopeResolve(t *testing.T) {
	global := NewScope()

	if err := global.Define("global"); err != nil {
		t.Fatal(err)
	}

	function := NewEnclosedScope(global)

	if err := function.Define("param"); err != nil {
		t.Fatal(err)
	}

	block := NewEnclosedScope(function)

	if err := block.Define("local"); err != nil {
		t.Fatal(err)
	}

	// 最内层应该能查找全部三个符号
	names := []string{
		"global",
		"param",
		"local",
	}

	for _, name := range names {
		symbol, ok := block.Resolve(name)

		if !ok {
			t.Fatalf("expected symbol %q", name)
		}

		if symbol.Name != name {
			t.Fatalf(
				"got %q, want %q",
				symbol.Name,
				name,
			)
		}
	}
}

// 测试父作用域不能访问子作用域中的符号
func TestScopeIsolation(t *testing.T) {
	global := NewScope()
	local := NewEnclosedScope(global)

	if err := local.Define("value"); err != nil {
		t.Fatal(err)
	}

	// 子作用域可以访问自己的符号
	if _, ok := local.Resolve("value"); !ok {
		t.Fatal("expected local symbol")
	}

	// 全局作用域不能访问子作用域的符号
	if _, ok := global.Resolve("value"); ok {
		t.Fatal("global scope should not resolve local symbol")
	}
}

// 测试符号元数据与遮蔽关系
func TestScopeSymbolMetadata(t *testing.T) {
	global := NewScope()

	outer := Symbol{
		Name: "value",
		Kind: SymbolVariable,
		Decl: token.Position{
			Offset: 5,
			Line:   1,
			Column: 6,
		},
	}

	if err := global.DefineSymbol(outer); err != nil {
		t.Fatal(err)
	}

	// 创建子作用域中的同名符号
	local := NewEnclosedScope(global)

	inner := Symbol{
		Name: "value",
		Kind: SymbolParameter,
		Decl: token.Position{
			Offset: 25,
			Line:   3,
			Column: 10,
		},
	}

	if err := local.DefineSymbol(inner); err != nil {
		t.Fatal(err)
	}

	// 子作用域优先找到自己的符号
	got, ok := local.Resolve("value")
	if !ok {
		t.Fatal("expected local symbol")
	}

	if got != inner {
		t.Fatalf("got %+v, want %+v", got, inner)
	}

	// 父作用域仍然保存原来的符号
	got, ok = global.Resolve("value")
	if !ok {
		t.Fatal("expected global symbol")
	}

	if got != outer {
		t.Fatalf("got %+v, want %+v", got, outer)
	}
}
