package compile

import (
	"fmt"

	"github.com/Aneesh495/fathom/internal/bc"
)

// verifyFunc checks that every jump lands on an instruction boundary.
func verifyFunc(fn *Func) error {
	if fn == nil {
		return fmt.Errorf("nil function")
	}
	starts := map[int]bool{}
	var insts []bc.Inst
	err := bc.Walk(fn.Code, func(in bc.Inst) {
		starts[in.PC] = true
		insts = append(insts, in)
	})
	if err != nil {
		return err
	}
	for _, in := range insts {
		switch in.Op {
		case bc.OpJump, bc.OpJumpIf, bc.OpJumpIfNot:
			target := in.Next + in.A
			if !starts[target] && target != len(fn.Code) {
				return fmt.Errorf("jump at %d lands at %d", in.PC, target)
			}
		case bc.OpConst, bc.OpTrap:
			if in.A < 0 || in.A >= len(fn.Consts) {
				return fmt.Errorf("const %d out of range", in.A)
			}
		}
	}
	return nil
}
