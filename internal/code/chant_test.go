package code

import "testing"

func TestChantInstruction(t *testing.T) {
	ins, err := Make(OpChant)
	if err != nil || len(ins) != 1 || ins[0] != 22 {
		t.Fatalf("encoding: %v, error: %v", ins, err)
	}
	def, err := Lookup(OpChant)
	if err != nil {
		t.Fatal(err)
	}
	operands, read, err := ReadOperands(def, nil)
	if err != nil || read != 0 || len(operands) != 0 {
		t.Fatalf("operands: %v, read=%d, error=%v", operands, read, err)
	}
	if got := ins.String(); got != "0000 OpChant\n" {
		t.Fatalf("disassembly: %q", got)
	}
	if _, err := Make(OpChant, 1); err == nil {
		t.Fatal("expected operand count error")
	}
}
