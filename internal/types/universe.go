package types

import "strconv"

// InitUniverse installs the value classes, Any, AnyRef, Nothing, and Null,
// plus the primitive methods the compiler lowers without a call.
func (c *Context) InitUniverse() *Scope {
	c.Any = c.class("Any", SymClass, FlagAbstract)
	c.AnyVal = c.class("AnyVal", SymClass, FlagAbstract)
	c.AnyRef = c.class("AnyRef", SymClass, FlagAbstract)
	c.Int = c.class("Int", SymClass, FlagFinal)
	c.Double = c.class("Double", SymClass, FlagFinal)
	c.Boolean = c.class("Boolean", SymClass, FlagFinal)
	c.Char = c.class("Char", SymClass, FlagFinal)
	c.Unit = c.class("Unit", SymClass, FlagFinal)
	c.String = c.class("String", SymClass, FlagFinal)
	c.Nothing = c.class("Nothing", SymClass, FlagFinal|FlagAbstract)
	c.Null = c.class("Null", SymClass, FlagFinal|FlagAbstract)

	c.Any.Type = c.Top
	c.Bot.Sym = c.Nothing
	c.Nothing.Type = c.Bot

	anyRef := c.Named(c.AnyRef, nil)
	anyVal := c.Named(c.AnyVal, nil)
	c.AnyRef.Type = anyRef
	c.AnyVal.Type = anyVal
	c.AnyVal.Parents = []*Type{c.Top}
	c.AnyRef.Parents = []*Type{c.Top}

	c.intT = c.Named(c.Int, nil)
	c.doubleT = c.Named(c.Double, nil)
	c.booleanT = c.Named(c.Boolean, nil)
	c.charT = c.Named(c.Char, nil)
	c.unitT = c.Named(c.Unit, nil)
	c.stringT = c.Named(c.String, nil)
	c.nullT = c.Named(c.Null, nil)

	c.Int.Type = c.intT
	c.Double.Type = c.doubleT
	c.Boolean.Type = c.booleanT
	c.Char.Type = c.charT
	c.Unit.Type = c.unitT
	c.String.Type = c.stringT
	c.Null.Type = c.nullT

	c.Int.Parents = []*Type{anyVal}
	c.Double.Parents = []*Type{anyVal}
	c.Boolean.Parents = []*Type{anyVal}
	c.Char.Parents = []*Type{anyVal}
	c.Unit.Parents = []*Type{anyVal}
	c.String.Parents = []*Type{anyRef}
	c.Null.Parents = []*Type{anyRef}

	c.installAnyMethods()
	c.installIntMethods()
	c.installDoubleMethods()
	c.installBooleanMethods()
	c.installStringMethods()
	c.installCharMethods()

	root := NewScope(nil, nil)
	for _, s := range []*Symbol{
		c.Any, c.AnyVal, c.AnyRef, c.Int, c.Double, c.Boolean, c.Char, c.Unit, c.String, c.Nothing, c.Null,
	} {
		root.Bind(s)
		c.installThis(s)
	}
	// println and print live in the root scope, not on a class.
	root.Bind(c.polyPrint("println", BuiltinPrintln))
	root.Bind(c.polyPrint("print", BuiltinPrint))
	c.Tuple2 = c.tupleClass(2)
	c.Tuple3 = c.tupleClass(3)
	c.Tuple4 = c.tupleClass(4)
	root.Bind(c.Tuple2)
	root.Bind(c.Tuple3)
	root.Bind(c.Tuple4)
	return root
}

func (c *Context) tupleClass(n int) *Symbol {
	s := c.class("Tuple"+strconv.Itoa(n), SymClass, FlagFinal|FlagCase)
	skolems := make([]*Type, n)
	var ctor []*Symbol
	for i := 0; i < n; i++ {
		name := string(rune('A' + i))
		tp := c.NewSymbol(name, SymTypeParam)
		tp.Variance = Covariant
		tp.Skolem = c.Skolem(name, c.Bot, c.Top)
		s.AddMember(tp)
		skolems[i] = tp.Skolem
		field := c.NewSymbol("_"+strconv.Itoa(i+1), SymTerm)
		field.Flags = FlagStable | FlagParam | FlagCase
		field.Type = tp.Skolem
		field.Index = i
		s.AddMember(field)
		ctor = append(ctor, field)
	}
	s.Ctor = ctor
	s.Type = c.Named(s, skolems)
	s.Parents = []*Type{c.Named(c.AnyRef, nil)}
	c.installThis(s)
	return s
}

func (c *Context) class(name string, kind SymKind, flags Flags) *Symbol {
	s := c.NewSymbol(name, kind)
	s.Flags = flags
	s.Scope = NewScope(nil, s)
	return s
}

func (c *Context) installThis(class *Symbol) {
	if class == nil || class.Type == nil {
		return
	}
	if class.Get("this") != nil {
		return
	}
	th := c.NewSymbol("this", SymTerm)
	th.Flags = FlagSynthetic | FlagStable | FlagParam
	th.Type = class.Type
	class.AddMember(th)
}

func (c *Context) installAnyMethods() {
	eq := c.method(c.Any, "==", BuiltinEq, []ParamList{{Params: []*Symbol{c.term("x", c.Top)}}}, c.booleanT)
	ne := c.method(c.Any, "!=", BuiltinNe, []ParamList{{Params: []*Symbol{c.term("x", c.Top)}}}, c.booleanT)
	c.method(c.Any, "toString", BuiltinToString, nil, c.stringT)
	_ = eq
	_ = ne
	c.method(c.Any, "eq", BuiltinEq, []ParamList{{Params: []*Symbol{c.term("x", c.Top)}}}, c.booleanT)
	c.method(c.Any, "ne", BuiltinNe, []ParamList{{Params: []*Symbol{c.term("x", c.Top)}}}, c.booleanT)
}

func (c *Context) installIntMethods() {
	bin := []struct {
		name string
		b    Builtin
	}{
		{"+", BuiltinAdd},
		{"-", BuiltinSub},
		{"*", BuiltinMul},
		{"/", BuiltinDiv},
		{"%", BuiltinMod},
		{"&", BuiltinBitAnd},
		{"|", BuiltinBitOr},
		{"^", BuiltinBitXor},
		{"<<", BuiltinShl},
		{">>", BuiltinShr},
	}
	for _, op := range bin {
		c.method(c.Int, op.name, op.b, []ParamList{{Params: []*Symbol{c.term("x", c.intT)}}}, c.intT)
	}
	rel := []struct {
		name string
		b    Builtin
	}{
		{"==", BuiltinEq}, {"!=", BuiltinNe},
		{"<", BuiltinLt}, {"<=", BuiltinLe},
		{">", BuiltinGt}, {">=", BuiltinGe},
	}
	for _, op := range rel {
		c.method(c.Int, op.name, op.b, []ParamList{{Params: []*Symbol{c.term("x", c.intT)}}}, c.booleanT)
	}
	c.method(c.Int, "unary_-", BuiltinNeg, nil, c.intT)
	c.method(c.Int, "unary_~", BuiltinBitNot, nil, c.intT)
	c.method(c.Int, "unary_+", BuiltinNone, nil, c.intT)
	c.method(c.Int, "toString", BuiltinToString, nil, c.stringT)
}

func (c *Context) installDoubleMethods() {
	for _, op := range []string{"+", "-", "*", "/", "%"} {
		b := BuiltinAdd
		switch op {
		case "-":
			b = BuiltinSub
		case "*":
			b = BuiltinMul
		case "/":
			b = BuiltinDiv
		case "%":
			b = BuiltinMod
		}
		c.method(c.Double, op, b, []ParamList{{Params: []*Symbol{c.term("x", c.doubleT)}}}, c.doubleT)
	}
	for _, op := range []struct {
		n string
		b Builtin
	}{
		{"==", BuiltinEq}, {"!=", BuiltinNe}, {"<", BuiltinLt}, {"<=", BuiltinLe}, {">", BuiltinGt}, {">=", BuiltinGe},
	} {
		c.method(c.Double, op.n, op.b, []ParamList{{Params: []*Symbol{c.term("x", c.doubleT)}}}, c.booleanT)
	}
	c.method(c.Double, "unary_-", BuiltinNeg, nil, c.doubleT)
	c.method(c.Double, "toString", BuiltinToString, nil, c.stringT)
}

func (c *Context) installBooleanMethods() {
	c.method(c.Boolean, "unary_!", BuiltinNot, nil, c.booleanT)
	c.method(c.Boolean, "==", BuiltinEq, []ParamList{{Params: []*Symbol{c.term("x", c.booleanT)}}}, c.booleanT)
	c.method(c.Boolean, "!=", BuiltinNe, []ParamList{{Params: []*Symbol{c.term("x", c.booleanT)}}}, c.booleanT)
	c.method(c.Boolean, "toString", BuiltinToString, nil, c.stringT)
}

func (c *Context) installStringMethods() {
	c.method(c.String, "+", BuiltinConcat, []ParamList{{Params: []*Symbol{c.term("x", c.Top)}}}, c.stringT)
	c.method(c.String, "length", BuiltinLength, nil, c.intT)
	c.method(c.String, "charAt", BuiltinCharAt, []ParamList{{Params: []*Symbol{c.term("i", c.intT)}}}, c.charT)
	c.method(c.String, "substring", BuiltinSubstring, []ParamList{{Params: []*Symbol{c.term("from", c.intT), c.term("until", c.intT)}}}, c.stringT)
	c.method(c.String, "==", BuiltinEq, []ParamList{{Params: []*Symbol{c.term("x", c.stringT)}}}, c.booleanT)
	c.method(c.String, "!=", BuiltinNe, []ParamList{{Params: []*Symbol{c.term("x", c.stringT)}}}, c.booleanT)
	c.method(c.String, "toString", BuiltinToString, nil, c.stringT)
}

func (c *Context) installCharMethods() {
	c.method(c.Char, "toString", BuiltinToString, nil, c.stringT)
	c.method(c.Char, "==", BuiltinEq, []ParamList{{Params: []*Symbol{c.term("x", c.charT)}}}, c.booleanT)
	c.method(c.Char, "!=", BuiltinNe, []ParamList{{Params: []*Symbol{c.term("x", c.charT)}}}, c.booleanT)
}

func (c *Context) method(owner *Symbol, name string, b Builtin, lists []ParamList, result *Type) *Symbol {
	m := c.NewSymbol(name, SymMethod)
	m.Flags = FlagFinal | FlagBuiltin
	m.Builtin = b
	m.Lists = lists
	m.Result = result
	if len(lists) == 1 && !lists[0].Using {
		params := make([]*Type, len(lists[0].Params))
		for i, p := range lists[0].Params {
			params[i] = p.Type
		}
		m.Type = c.Fun(params, result)
	} else if len(lists) == 0 {
		m.Type = result
	}
	owner.AddMember(m)
	return m
}

func (c *Context) term(name string, t *Type) *Symbol {
	s := c.NewSymbol(name, SymTerm)
	s.Flags = FlagParam | FlagStable
	s.Type = t
	return s
}

func (c *Context) polyPrint(name string, b Builtin) *Symbol {
	tp := c.NewSymbol("A", SymTypeParam)
	sk := c.Skolem("A", c.Bot, c.Top)
	tp.Skolem = sk
	tp.Variance = Invariant
	m := c.NewSymbol(name, SymMethod)
	m.Flags = FlagBuiltin
	m.Builtin = b
	m.TParams = []*Symbol{tp}
	arg := c.term("x", sk)
	m.Lists = []ParamList{{Params: []*Symbol{arg}}}
	m.Result = c.unitT
	m.Type = c.Fun([]*Type{sk}, c.unitT)
	return m
}
