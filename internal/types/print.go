package types

import (
	"strconv"
	"strings"
)

// Format prints a type the way a diagnostic should show it.
func (c *Context) Format(t *Type) string {
	return c.str(t, map[*Type]bool{})
}

func (c *Context) str(t *Type, seen map[*Type]bool) string {
	t = Repr(t)
	if t == nil {
		return "<nil>"
	}
	if seen[t] {
		if t.Name != "" {
			return t.Name
		}
		return "..."
	}
	switch t.Kind {
	case TyTop:
		return "Any"
	case TyBot:
		return "Nothing"
	case TyError:
		return "<error>"
	case TyNamed:
		name := "?"
		if t.Sym != nil {
			name = t.Sym.Name
		}
		if len(t.Args) == 0 {
			return name
		}
		var b strings.Builder
		b.WriteString(name)
		b.WriteByte('[')
		for i, a := range t.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(c.str(a, seen))
		}
		b.WriteByte(']')
		return b.String()
	case TyVar:
		if t.Name != "" {
			return t.Name
		}
		return "T" + strconv.Itoa(t.VarID)
	case TyFun:
		var b strings.Builder
		if len(t.Params) == 1 {
			b.WriteString(c.str(t.Params[0], seen))
		} else {
			b.WriteByte('(')
			for i, p := range t.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(c.str(p, seen))
			}
			b.WriteByte(')')
		}
		b.WriteString(" => ")
		b.WriteString(c.str(t.Res, seen))
		return b.String()
	case TyUnion:
		return c.str(t.L, seen) + " | " + c.str(t.R, seen)
	case TyInter:
		return c.str(t.L, seen) + " & " + c.str(t.R, seen)
	case TySingleton:
		if t.Path == nil {
			return "<singleton>"
		}
		return t.Path.String() + ".type"
	case TySelect:
		if t.Path == nil {
			return t.Sel
		}
		return t.Path.String() + "." + t.Sel
	case TyLiteral:
		switch t.Lit {
		case LitInt:
			return strconv.FormatInt(t.Int, 10)
		case LitBool:
			if t.Bool {
				return "true"
			}
			return "false"
		case LitString:
			return strconv.Quote(t.String)
		case LitChar:
			return strconv.QuoteRune(t.Char)
		case LitUnit:
			return "()"
		case LitNull:
			return "null"
		default:
			return "<literal>"
		}
	default:
		return t.Kind.String()
	}
}
