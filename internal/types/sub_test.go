package types

import "testing"

func univ() *Context {
	c := NewContext()
	c.InitUniverse()
	return c
}

func TestLinearization(t *testing.T) {
	c := univ()
	a := c.NewSymbol("A", SymClass)
	b := c.NewSymbol("B", SymTrait)
	cc := c.NewSymbol("C", SymTrait)
	d := c.NewSymbol("D", SymClass)
	a.Parents = []*Type{c.AnyRef.Type}
	a.Type = c.Named(a, nil)
	b.Parents = []*Type{a.Type}
	b.Type = c.Named(b, nil)
	cc.Parents = []*Type{a.Type}
	cc.Type = c.Named(cc, nil)
	d.Parents = []*Type{c.Named(b, nil), c.Named(cc, nil)}
	d.Type = c.Named(d, nil)
	got := names(c.Linearize(d))
	want := []string{"D", "C", "B", "A", "AnyRef", "Any"}
	if stringsJoin(got) != stringsJoin(want) {
		t.Fatalf("lin %v", got)
	}
	intLin := names(c.Linearize(c.Int))
	if stringsJoin(intLin) != "Int,AnyVal,Any" {
		t.Fatalf("int %v", intLin)
	}
}

func TestSubtypeLattice(t *testing.T) {
	c := univ()
	intT := c.IntType()
	str := c.StringType()
	boolT := c.BooleanType()
	top := c.Top
	bot := c.Bot
	fun := c.Fun([]*Type{intT}, intT)
	wide := c.Fun([]*Type{top}, bot)
	union := c.Union(intT, str)
	inter := c.Inter(intT, str)

	if !c.Subtype(intT, top) || !c.Subtype(bot, intT) || !c.Subtype(intT, intT) {
		t.Fatal("basic bounds")
	}
	if c.Subtype(intT, str) || c.Subtype(str, intT) {
		t.Fatal("int and string are unrelated")
	}
	if !c.Subtype(intT, union) || !c.Subtype(str, union) || c.Subtype(boolT, union) {
		t.Fatal("union arms")
	}
	if !c.Subtype(union, top) || c.Subtype(union, intT) {
		t.Fatal("union bound")
	}
	if !c.Subtype(inter, intT) || !c.Subtype(inter, str) || c.Subtype(intT, inter) {
		t.Fatal("intersection")
	}
	// Functions are contravariant in the argument and covariant in the result.
	// Any => Nothing  <:  Int => Int, because Int <: Any and Nothing <: Int.
	if !c.Subtype(wide, fun) {
		t.Fatal("function variance")
	}
	if c.Subtype(fun, wide) {
		t.Fatal("function variance is not symmetric")
	}
	if !c.Subtype(c.LiteralInt(1), intT) || c.Subtype(c.LiteralInt(1), str) {
		t.Fatal("literal int")
	}
	if !c.Subtype(c.LiteralInt(1), c.LiteralInt(1)) || c.Subtype(c.LiteralInt(1), c.LiteralInt(2)) {
		t.Fatal("literal identity")
	}
	if !c.Subtype(c.NullType(), str) || c.Subtype(c.NullType(), intT) {
		t.Fatal("null")
	}
	if !c.Subtype(c.StringType(), c.AnyRef.Type) || c.Subtype(intT, c.AnyRef.Type) {
		t.Fatal("anyref")
	}

	// Transitivity on a finite ground set.
	set := []*Type{bot, intT, str, boolT, top, fun, wide, union, inter, c.LiteralInt(1), c.NullType(), c.AnyRef.Type, c.AnyVal.Type}
	for _, a := range set {
		for _, b := range set {
			for _, d := range set {
				if c.Subtype(a, b) && c.Subtype(b, d) && !c.Subtype(a, d) {
					t.Fatalf("transitivity %s <: %s <: %s", c.Format(a), c.Format(b), c.Format(d))
				}
			}
		}
	}
}

func TestVariance(t *testing.T) {
	c := univ()
	list := c.NewSymbol("List", SymTrait)
	list.Flags = FlagAbstract
	a := c.NewSymbol("A", SymTypeParam)
	a.Variance = Covariant
	a.Skolem = c.Skolem("A", c.Bot, c.Top)
	list.AddMember(a)
	list.Parents = []*Type{c.AnyRef.Type}
	listInt := c.Named(list, []*Type{c.IntType()})
	listAny := c.Named(list, []*Type{c.Top})
	listStr := c.Named(list, []*Type{c.StringType()})
	if !c.Subtype(listInt, listAny) {
		t.Fatal("covariant List[Int] <: List[Any]")
	}
	if c.Subtype(listAny, listInt) || c.Subtype(listInt, listStr) {
		t.Fatal("variance rejected a bad list conversion")
	}
}

func TestConstrainInstantiate(t *testing.T) {
	c := univ()
	a := c.Fresh("A", 1)
	if !c.Constrain(c.IntType(), a) {
		t.Fatal("constrain")
	}
	got := c.Instantiate(a, true)
	if got != c.IntType() {
		t.Fatalf("instantiate %s", c.Format(got))
	}
	free := c.Fresh("B", 1)
	if c.Instantiate(free, true) != c.Error {
		t.Fatal("unconstrained variable should not escape")
	}
}

func TestPathSelection(t *testing.T) {
	c := univ()
	cell := c.NewSymbol("Cell", SymTrait)
	cell.Flags = FlagAbstract
	cell.Parents = []*Type{c.AnyRef.Type}
	cell.Type = c.Named(cell, nil)
	c.installThis(cell)
	elem := c.NewSymbol("Elem", SymType)
	elem.Flags = FlagAbstract
	elem.Type = c.Bot
	elem.Result = c.Top
	elem.Skolem = c.Skolem("Elem", c.Bot, c.Top)
	cell.AddMember(elem)
	param := c.NewSymbol("c", SymTerm)
	param.Flags = FlagParam | FlagStable
	param.Type = cell.Type
	sel := c.Select(&Path{Head: param}, "Elem")
	if !c.Subtype(sel, c.Top) {
		t.Fatal("abstract selection <: Any")
	}
	if c.Subtype(c.IntType(), sel) || c.Subtype(sel, c.IntType()) {
		t.Fatal("abstract selection is not Int")
	}
	// A refined alias Elem = Int makes the selection equal to Int in both directions.
	box := c.NewSymbol("Box", SymClass)
	box.Parents = []*Type{c.AnyRef.Type}
	box.Type = c.Named(box, nil)
	c.installThis(box)
	alias := c.NewSymbol("Elem", SymType)
	alias.Type = c.IntType()
	alias.Result = c.IntType()
	box.AddMember(alias)
	b := c.NewSymbol("b", SymTerm)
	b.Flags = FlagStable | FlagParam
	b.Type = box.Type
	got := c.Select(&Path{Head: b}, "Elem")
	if !c.Subtype(got, c.IntType()) || !c.Subtype(c.IntType(), got) {
		t.Fatalf("alias selection is Int, sub %v super %v (%s)", c.Subtype(got, c.IntType()), c.Subtype(c.IntType(), got), c.Format(got))
	}
}

func TestFuelTerminates(t *testing.T) {
	c := univ()
	c.Fuel = 8
	// A skolem bounded by itself still answers, and a deep union tower finishes.
	v := c.Skolem("A", c.Bot, nil)
	v.Hi = v
	if c.Subtype(v, c.StringType()) {
		t.Fatal("self-bounded skolem is not String")
	}
	tpe := c.IntType()
	for i := 0; i < 40; i++ {
		tpe = c.Union(tpe, c.StringType())
	}
	if !c.Subtype(c.IntType(), tpe) {
		t.Fatal("deep union")
	}
}

func names(syms []*Symbol) []string {
	var out []string
	for _, s := range syms {
		out = append(out, s.Name)
	}
	return out
}

func stringsJoin(xs []string) string {
	s := ""
	for i, x := range xs {
		if i > 0 {
			s += ","
		}
		s += x
	}
	return s
}
