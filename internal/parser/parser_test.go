package parser

import (
	"strings"
	"testing"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/span"
)

func parseOK(t *testing.T, src string) *ast.File {
	t.Helper()
	f, diags := Parse(span.NewFile("t.scala", src))
	if diags.HasErrors() {
		t.Fatalf("parse errors:\n%s\nsource:\n%s", diags.Render(), src)
	}
	return f
}

func TestPrecedence(t *testing.T) {
	f := parseOK(t, "1 + 2 * 3\n")
	got := ast.Sprint(f.Stats[0])
	if got != "(infix + 1 (infix * 2 3))" {
		t.Fatalf("got %s", got)
	}
	f = parseOK(t, "1 :: 2 :: Nil\n")
	got = ast.Sprint(f.Stats[0])
	if got != "(infix :: 1 (infix :: 2 Nil))" {
		t.Fatalf("cons %s", got)
	}
	f = parseOK(t, "1 + 2 eq 3\n")
	got = ast.Sprint(f.Stats[0])
	if got != "(infix eq (infix + 1 2) 3)" {
		t.Fatalf("eq %s", got)
	}
	f = parseOK(t, "1: Int\n")
	got = ast.Sprint(f.Stats[0])
	if got != "(ascribe 1 Int)" {
		t.Fatalf("ascribe %s", got)
	}
}

func TestControlAndLambda(t *testing.T) {
	src := "if a < b then a else b\n"
	f := parseOK(t, src)
	got := ast.Sprint(f.Stats[0])
	if got != "(if (infix < a b) a b)" {
		t.Fatalf("if %s", got)
	}
	f = parseOK(t, "if c then\n  a\nelse\n  b\n")
	got = ast.Sprint(f.Stats[0])
	if got != "(if c a b)" {
		t.Fatalf("if indent %s", got)
	}
	f = parseOK(t, "(x: Int) => x + 1\n")
	got = ast.Sprint(f.Stats[0])
	if got != "(lam [x: Int] (infix + x 1))" {
		t.Fatalf("lam %s", got)
	}
	f = parseOK(t, "x => x\n")
	if ast.Sprint(f.Stats[0]) != "(lam [x] x)" {
		t.Fatalf("short lam %s", ast.Sprint(f.Stats[0]))
	}
}

func TestDecls(t *testing.T) {
	src := "" +
		"package demo.core\n" +
		"import demo.core.{A, B => C, _}\n" +
		"class Box[T](val value: T) extends Any {\n" +
		"  def map[U](f: T => U): Box[U] = Box(f(value))\n" +
		"}\n" +
		"def fact(n: Int): Int =\n" +
		"  if n == 0 then 1 else n * fact(n - 1)\n"
	f := parseOK(t, src)
	got := ast.Sprint(f)
	if !strings.Contains(got, "(qual demo.core)") {
		t.Fatalf("pkg %s", got)
	}
	if !strings.Contains(got, "(import demo.core A (B => C) _)") {
		t.Fatalf("import %s", got)
	}
	if !strings.Contains(got, "(class Box") || !strings.Contains(got, "(def map") {
		t.Fatalf("class %s", got)
	}
	if !strings.Contains(got, "(def fact") || !strings.Contains(got, "(if (infix == n 0)") {
		t.Fatalf("fact %s", got)
	}
}

func TestMatchForEnumGiven(t *testing.T) {
	src := "" +
		"enum List[+A] {\n" +
		"  case Nil\n" +
		"  case Cons(head: A, tail: List[A])\n" +
		"}\n" +
		"xs match {\n" +
		"  case Nil => 0\n" +
		"  case Cons(h, t) if h > 0 => h\n" +
		"  case _ => 1\n" +
		"}\n" +
		"for { x <- xs; if x > 0 } yield x\n" +
		"given intOrd: Ord[Int] with {\n" +
		"  def compare(x: Int, y: Int): Int = 0\n" +
		"}\n" +
		"extension (x: Int)\n" +
		"  def abs: Int = if x < 0 then -x else x\n"
	f := parseOK(t, src)
	got := ast.Sprint(f)
	if !strings.Contains(got, "(enum List") || !strings.Contains(got, "(case Nil)") || !strings.Contains(got, "(case Cons") {
		t.Fatalf("enum %s", got)
	}
	if !strings.Contains(got, "(match xs") || !strings.Contains(got, "(case (ctor Cons h t) if") {
		t.Fatalf("match %s", got)
	}
	if !strings.Contains(got, "(for-yield") {
		t.Fatalf("for %s", got)
	}
	if !strings.Contains(got, "(given intOrd") {
		t.Fatalf("given %s", got)
	}
	if !strings.Contains(got, "(extension") || !strings.Contains(got, "(def abs") {
		t.Fatalf("ext %s", got)
	}
}

func TestInterpAndStrings(t *testing.T) {
	f := parseOK(t, "s\"hi $name!\"\n")
	got := ast.Sprint(f.Stats[0])
	if got != "(interp s \"hi \" name \"!\")" {
		t.Fatalf("interp %s", got)
	}
	f = parseOK(t, "s\"n = ${1 + 2}\"\n")
	got = ast.Sprint(f.Stats[0])
	if !strings.Contains(got, "(infix + 1 2)") {
		t.Fatalf("hole %s", got)
	}
}

func TestIndentedClass(t *testing.T) {
	src := "" +
		"class A:\n" +
		"  def f: Int = 1\n" +
		"  def g: Int = f + 1\n"
	f := parseOK(t, src)
	got := ast.Sprint(f)
	if !strings.Contains(got, "(def f") || !strings.Contains(got, "(def g") {
		t.Fatalf("indent class %s", got)
	}
}

func TestDoesNotHang(t *testing.T) {
	samples := []string{
		"",
		"(",
		"class",
		"def",
		"{{{{",
		"if if if",
		"1 + + +",
		"match {",
		"\"unterminated",
		"case class",
	}
	for _, src := range samples {
		_, diags := Parse(span.NewFile("bad.scala", src))
		if diags == nil {
			t.Fatal("nil diags")
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add("def f(x: Int): Int = x + 1\n")
	f.Add("class A { def m: Int = 1 }\n")
	f.Add("if a then b else c\n")
	f.Fuzz(func(t *testing.T, src string) {
		_, _ = Parse(span.NewFile("fuzz.scala", src))
	})
}
