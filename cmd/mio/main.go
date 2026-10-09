package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello Mio Language~")
	fmt.Println("Interpreter V0.1.0-dev")

	if len(os.Args) > 1 {
		fmt.Println("Arguments:", os.Args[0])
	}
}
