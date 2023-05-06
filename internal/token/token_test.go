package token

import "testing"

func TestKeywordsRoundTrip(t *testing.T) {
	for lit, kind := range keywords {
		got, ok := Keyword(lit)
		if !ok || got != kind {
			t.Fatalf("keyword %s", lit)
		}
		if kind.String() != lit {
			t.Fatalf("string %s -> %s", lit, kind.String())
		}
	}
	if _, ok := Keyword("foo"); ok {
		t.Fatal("foo is not a keyword")
	}
	if _, ok := Keyword("Int"); ok {
		t.Fatal("Int is a type name, not a keyword")
	}
}

func TestOpPrecScala(t *testing.T) {
	cases := []struct {
		lit   string
		prec  Prec
		right bool
	}{
		{"*", PrecMul, false},
		{"/", PrecMul, false},
		{"+", PrecAdd, false},
		{"-", PrecAdd, false},
		{"::", PrecCons, true},
		{":+", PrecCons, false},
		{"+:", PrecAdd, true},
		{"==", PrecRel, false},
		{"!=", PrecRel, false},
		{"<", PrecRel, false},
		{">", PrecRel, false},
		{"&&", PrecAmp, false},
		{"||", PrecPipe, false},
		{"^", PrecCaret, false},
		{"eq", PrecLetter, false},
	}
	for _, tc := range cases {
		p, right := OpPrec(tc.lit)
		if p != tc.prec || right != tc.right {
			t.Fatalf("%s: got %d right=%v", tc.lit, p, right)
		}
	}
	// * binds tighter than +, and + binds tighter than ::.
	star, _ := OpPrec("*")
	plus, _ := OpPrec("+")
	cons, _ := OpPrec("::")
	if !(star > plus && plus > cons) {
		t.Fatalf("ladder %d %d %d", star, plus, cons)
	}
}

func TestAssignmentOp(t *testing.T) {
	if !IsAssignmentOp("+=") || AssignBase("+=") != "+" {
		t.Fatal("plus assign")
	}
	if IsAssignmentOp("==") || IsAssignmentOp("=") || IsAssignmentOp("<=") {
		t.Fatal("comparisons are not assignments")
	}
	if !Ident.CanEndStmt() || Dot.CanEndStmt() {
		t.Fatal("end of statement")
	}
	if !Else.ContinuesLine() || !Match.ContinuesLine() || Ident.ContinuesLine() {
		t.Fatal("continuation")
	}
}

func TestTokenString(t *testing.T) {
	tok := Token{Kind: Op, Lit: "::"}
	if tok.String() != "::" {
		t.Fatal(tok.String())
	}
	if (Token{Kind: Arrow}).String() != "=>" {
		t.Fatal("arrow")
	}
}
