package compile

import (
	"fmt"
	"strings"

	"github.com/Aneesh495/fathom/internal/bc"
)

// Disasm renders a function for diagnostics.
func Disasm(fn *Func) string {
	if fn == nil {
		return "<nil>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "func %s locals=%d\n", fn.Name, fn.Locals)
	_ = bc.Walk(fn.Code, func(in bc.Inst) {
		info := bc.Lookup(in.Op)
		fmt.Fprintf(&b, "  %04d  %s", in.PC, info.Name)
		switch info.Imm {
		case bc.ImmNone:
		case bc.ImmU8, bc.ImmU16, bc.ImmI16:
			fmt.Fprintf(&b, " %d", in.A)
		case bc.ImmU16U8, bc.ImmU8U8:
			fmt.Fprintf(&b, " %d %d", in.A, in.B)
		}
		if in.Op == bc.OpConst && in.A < len(fn.Consts) {
			fmt.Fprintf(&b, "  ; %s", constString(fn.Consts[in.A]))
		}
		b.WriteByte('\n')
	})
	return b.String()
}

func constString(k Const) string {
	switch k.Tag {
	case ConstInt:
		return fmt.Sprintf("%d", k.I)
	case ConstFloat:
		return fmt.Sprintf("%g", k.F)
	case ConstBool:
		return fmt.Sprintf("%v", k.B)
	default:
		return fmt.Sprintf("%q", k.S)
	}
}
