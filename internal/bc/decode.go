package bc

import "fmt"

// Inst is one decoded instruction.
type Inst struct {
	Op   Op
	PC   int
	A    int
	B    int
	Next int
}

// Decode reads the instruction at pc.
func Decode(code []byte, pc int) (Inst, error) {
	if pc < 0 || pc >= len(code) {
		return Inst{}, fmt.Errorf("pc %d out of range", pc)
	}
	op := Op(code[pc])
	info := Lookup(op)
	in := Inst{Op: op, PC: pc, Next: pc + 1 + ImmSize(info.Imm)}
	if in.Next > len(code) {
		return Inst{}, fmt.Errorf("truncated instruction %s at %d", info.Name, pc)
	}
	switch info.Imm {
	case ImmNone:
	case ImmU8:
		in.A = int(code[pc+1])
	case ImmU16:
		in.A = ReadU16(code, pc+1)
	case ImmI16:
		in.A = ReadI16(code, pc+1)
	case ImmU16U8:
		in.A = ReadU16(code, pc+1)
		in.B = int(code[pc+3])
	case ImmU8U8:
		in.A = int(code[pc+1])
		in.B = int(code[pc+2])
	}
	return in, nil
}

// Walk calls fn for every instruction. It returns the first decode error.
func Walk(code []byte, fn func(Inst)) error {
	for pc := 0; pc < len(code); {
		in, err := Decode(code, pc)
		if err != nil {
			return err
		}
		fn(in)
		if in.Next <= pc {
			return fmt.Errorf("instruction did not advance at %d", pc)
		}
		pc = in.Next
	}
	return nil
}
