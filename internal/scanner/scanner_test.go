package scanner

import (
	"strings"
	"testing"

	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/token"
)

func dump(src string) (string, *token.Token, int) {
	file := span.NewFile("t.scala", src)
	toks, diags := Scan(file)
	var b strings.Builder
	var first *token.Token
	for i := range toks {
		t := toks[i]
		if first == nil && t.Kind != token.EOF {
			cp := t
			first = &cp
		}
		if t.Kind == token.EOF {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		if t.Inserted {
			b.WriteString("SEMI")
			continue
		}
		switch t.Kind {
		case token.Ident, token.Op, token.Int, token.Float, token.String, token.Char, token.StrPart, token.InterpStart:
			b.WriteString(t.Kind.String())
			b.WriteByte(':')
			b.WriteString(t.Lit)
		default:
			b.WriteString(t.Kind.String())
		}
	}
	nerr := 0
	if diags != nil {
		nerr = diags.ErrorCount()
	}
	return b.String(), first, nerr
}

func TestBasics(t *testing.T) {
	got, first, nerr := dump("def + :: => <- <: >: = @ ? #")
	if nerr != 0 {
		t.Fatalf("errors %d", nerr)
	}
	want := "def operator:+ operator::: => <- <: >: = @ ? #"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if first == nil || first.Span.Position().Line != 1 || first.Span.Position().Col != 1 {
		t.Fatalf("pos %+v", first)
	}
}

func TestNumbers(t *testing.T) {
	got, _, nerr := dump("1 1_000 0xFF 0b1010 1.5 1e-3 1.foo")
	if nerr != 0 {
		t.Fatalf("errors %d", nerr)
	}
	want := "integer:1 integer:1_000 integer:0xFF integer:0b1010 floating point number:1.5 floating point number:1e-3 integer:1 . identifier:foo"
	if got != want {
		t.Fatalf("got %q", got)
	}
	n, err := token.ParseInt("1_000")
	if err != nil || n != 1000 {
		t.Fatal(err, n)
	}
	n, err = token.ParseInt("0xFF")
	if err != nil || n != 255 {
		t.Fatal(n, err)
	}
	n, err = token.ParseInt("0b1010")
	if err != nil || n != 10 {
		t.Fatal(n, err)
	}
	f, err := token.ParseFloat("1e-3")
	if err != nil || f != 0.001 {
		t.Fatal(f, err)
	}
}

func TestStringsAndChars(t *testing.T) {
	got, _, nerr := dump(`"hi\n" 'A' '\u0041' """a"b"""`)
	if nerr != 0 {
		t.Fatalf("errors %d in %s", nerr, got)
	}
	if !strings.Contains(got, "string:hi\n") {
		t.Fatalf("string %q", got)
	}
	if !strings.Contains(got, "character:A") {
		t.Fatalf("char %q", got)
	}
	// Both 'A' and '\u0041' decode to A, and the triple string keeps the inner quote.
	if strings.Count(got, "character:A") != 2 {
		t.Fatalf("chars %q", got)
	}
	if !strings.Contains(got, `string:a"b`) {
		t.Fatalf("triple %q", got)
	}
}

func TestEmptyStringIsNotTriple(t *testing.T) {
	got, _, nerr := dump(`""`)
	if nerr != 0 {
		t.Fatalf("errors %d (%s)", nerr, got)
	}
	if got != "string:" {
		t.Fatalf("got %q", got)
	}
}

func TestCommentsAndIdent(t *testing.T) {
	got, _, nerr := dump("foo /* nested /* x */ y */ `class` // tail\nbar")
	if nerr != 0 {
		t.Fatalf("errors %d", nerr)
	}
	if got != "identifier:foo identifier:class SEMI identifier:bar" {
		t.Fatalf("got %q", got)
	}
}

func TestSemicolonInsertion(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"val x = 1\nval y = 2", "val identifier:x = integer:1 SEMI val identifier:y = integer:2"},
		{"f(1,\n2)", "identifier:f ( integer:1 , integer:2 )"},
		{"obj\n  .m", "identifier:obj . identifier:m"},
		{"f(1)\n(2)", "identifier:f ( integer:1 ) ( integer:2 )"},
		{"class A\nextends B", "class identifier:A extends identifier:B"},
		{"if c then\n  a\nelse\n  b", "if identifier:c then identifier:a else identifier:b"},
		{"1\n+ 2", "integer:1 operator:+ integer:2"},
		{"return\n1", "return SEMI integer:1"},
		{"do\n  body\nwhile cond", "do identifier:body SEMI while identifier:cond"},
	}
	for _, tc := range cases {
		got, _, nerr := dump(tc.src)
		if nerr != 0 {
			t.Fatalf("%q errors", tc.src)
		}
		if got != tc.want {
			t.Fatalf("src %q\ngot  %q\nwant %q", tc.src, got, tc.want)
		}
	}
}

func TestInterpolation(t *testing.T) {
	got, _, nerr := dump(`s"hi $name!"`)
	if nerr != 0 {
		t.Fatalf("err %s", got)
	}
	want := "interpolation:s string part:hi  ( identifier:name ) string part:! interpolation"
	if got != want {
		t.Fatalf("got %q", got)
	}

	got, _, nerr = dump(`s"n=${1 + 2}"`)
	if nerr != 0 {
		t.Fatal(got)
	}
	if !strings.Contains(got, "integer:1 operator:+ integer:2") {
		t.Fatalf("hole %q", got)
	}

	got, _, nerr = dump("raw\"a\\n$x\"")
	if nerr != 0 {
		t.Fatal(got)
	}
	// raw keeps the backslash and the letter n, then interpolates x.
	if !strings.Contains(got, `string part:a\n`) || !strings.Contains(got, "identifier:x") {
		t.Fatalf("raw %q", got)
	}

	got, _, nerr = dump(`s"$$"`)
	if nerr != 0 || !strings.Contains(got, "string part:$") {
		t.Fatalf("dollar %q err %d", got, nerr)
	}

	got, _, nerr = dump(`s"a ${ s"b" } c"`)
	if nerr != 0 {
		t.Fatal(got)
	}
	if !strings.Contains(got, "interpolation:s") || strings.Count(got, "interpolation:s") != 2 {
		t.Fatalf("nested %q", got)
	}
}

func TestUnterminated(t *testing.T) {
	_, _, nerr := dump(`"abc`)
	if nerr == 0 {
		t.Fatal("wanted an error")
	}
	_, _, nerr = dump("/* nope")
	if nerr == 0 {
		t.Fatal("wanted comment error")
	}
	_, _, nerr = dump("`nope")
	if nerr == 0 {
		t.Fatal("wanted backtick error")
	}
}

func TestLineColumnAfterNewline(t *testing.T) {
	file := span.NewFile("t.scala", "ab\ncd")
	toks, diags := Scan(file)
	if diags.ErrorCount() != 0 {
		t.Fatal(diags.Render())
	}
	var idents []token.Token
	for _, t := range toks {
		if t.Kind == token.Ident {
			idents = append(idents, t)
		}
	}
	if len(idents) != 2 {
		t.Fatalf("idents %d", len(idents))
	}
	p := idents[1].Span.Position()
	if p.Line != 2 || p.Col != 1 {
		t.Fatalf("pos %s", p)
	}
}

func TestUnicodeIdentAndIllegal(t *testing.T) {
	got, _, nerr := dump("π ¢")
	if nerr != 1 {
		t.Fatalf("errors %d dump %s", nerr, got)
	}
	if !strings.HasPrefix(got, "identifier:π") {
		t.Fatalf("got %q", got)
	}
}
