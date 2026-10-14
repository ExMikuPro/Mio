package main

import (
	"fmt"
	"io"
	"os"

	"Mio/internal/ast"
	"Mio/internal/compiler"
	"Mio/internal/lexer"
	"Mio/internal/parser"
	"Mio/internal/resolver"
	"Mio/internal/token"
	"Mio/internal/typecheck"
	"Mio/internal/value"
	"Mio/internal/vm"
)

const version = "0.1.0-dev"

// main 是 Mio 命令行程序的入口
func main() {
	os.Exit(runCLI(
		os.Args[1:],
		os.Stdout,
		os.Stderr,
	))
}

// runCLI 解析并执行命令行指令
func runCLI(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	if len(args) == 1 {
		switch args[0] {
		case "version":
			fmt.Fprintf(stdout, "Mio %s\n", version)
			return 0

		case "help", "-h", "--help":
			printUsage(stdout)
			return 0
		}
	}

	// check 和 run 都需要一个源码路径
	if len(args) != 2 ||
		(args[0] != "check" && args[0] != "run") {
		printUsage(stderr)
		return 2
	}

	// 读取、解析并检查 Mio 源码
	program, r, err := parseAndCheckFile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "mio:", err)
		return 1
	}

	// 静态检查通过后直接退出
	if args[0] == "check" {
		fmt.Fprintln(stdout, "check passed")
		return 0
	}

	// 将经过检查的 AST 编译为字节码
	c := compiler.NewWithResolver(r)

	if err := c.Compile(program); err != nil {
		fmt.Fprintln(stderr, "mio: compile:", err)
		return 1
	}

	// 创建并执行虚拟机
	machine := vm.New(c.Bytecode())

	if err := machine.Run(); err != nil {
		fmt.Fprintln(stderr, "mio: runtime:", err)
		return 1
	}

	// 显示最后一条表达式的计算结果
	result, ok := machine.LastPoppedValue()
	if !ok {
		return 0
	}

	if err := printValue(stdout, result); err != nil {
		fmt.Fprintln(stderr, "mio:", err)
		return 1
	}

	return 0
}

// parseAndCheckFile 完成源码读取与静态语义检查
func parseAndCheckFile(
	path string,
) (*ast.Program, *resolver.Resolver, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"read %q: %w",
			path,
			err,
		)
	}

	// 词法分析
	l, err := lexer.NewChecked(
		string(source),
		token.V01,
	)
	if err != nil {
		return nil, nil, err
	}

	// 语法分析
	p := parser.New(l)

	program, err := p.ParseProgram()
	if err != nil {
		return nil, nil, err
	}

	if err := l.Err(); err != nil {
		return nil, nil, err
	}

	// 名称解析
	r := resolver.New()

	if err := r.ResolveProgram(program); err != nil {
		return nil, nil, err
	}

	// 静态类型检查
	checker := typecheck.NewWithResolver(r)

	if err := checker.CheckProgram(program); err != nil {
		return nil, nil, err
	}

	return program, r, nil
}

// printValue 显示 Mio 运行时值
func printValue(out io.Writer, v value.Value) error {
	switch v.Kind() {
	case value.KindInt:
		number, _ := v.Int64()
		_, err := fmt.Fprintln(out, number)
		return err

	case value.KindBool:
		boolean, _ := v.Bool()

		if boolean {
			_, err := fmt.Fprintln(out, "hai")
			return err
		}

		_, err := fmt.Fprintln(out, "iie")
		return err

	case value.KindString:
		text, _ := v.Text()
		_, err := fmt.Fprintln(out, text)
		return err

	case value.KindNil:
		_, err := fmt.Fprintln(out, "mu")
		return err

	default:
		return fmt.Errorf(
			"cannot display runtime value of type %s",
			v.Kind(),
		)
	}
}

// printUsage 显示命令行用法
func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  mio check <file.mio>")
	fmt.Fprintln(out, "  mio run <file.mio>")
	fmt.Fprintln(out, "  mio version")
	fmt.Fprintln(out, "  mio help")
}
