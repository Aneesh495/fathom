package check

import (
	"strings"
	"testing"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/parser"
	"github.com/Aneesh495/fathom/internal/span"
	"github.com/Aneesh495/fathom/internal/types"
)

func checkSrc(t *testing.T, src string) *Info {
	t.Helper()
	file, diags := parser.Parse(span.NewFile("t.scala", src))
	if diags.HasErrors() {
		t.Fatalf("parse:\n%s", diags.Render())
	}
	info := Check([]*ast.File{file})
	return info
}

func mustCheck(t *testing.T, src string) *Info {
	t.Helper()
	info := checkSrc(t, src)
	if info.Diags.HasErrors() {
		t.Fatalf("check:\n%s\nsource:\n%s", info.Diags.Render(), src)
	}
	return info
}

func findDef(info *Info, name string) *types.Symbol {
	for n, s := range info.Syms {
		if s != nil && s.Name == name && s.Kind == types.SymMethod {
			if _, ok := n.(*ast.DefDecl); ok {
				return s
			}
		}
	}
	return nil
}

func TestArithmetic(t *testing.T) {
	info := mustCheck(t, "def main(): Int = 1 + 2 * 3\n")
	m := findDef(info, "main")
	if m == nil || info.Ctx.Format(m.Result) != "Int" {
		t.Fatalf("main result %v", m)
	}
}

func TestFactAndMismatch(t *testing.T) {
	mustCheck(t, "def fact(n: Int): Int = if n == 0 then 1 else n * fact(n - 1)\n")
	info := checkSrc(t, "val x: Int = \"hi\"\n")
	if !info.Diags.ByMessage("type mismatch") {
		t.Fatalf("expected mismatch, got:\n%s", info.Diags.Render())
	}
}

func TestClassAndGeneric(t *testing.T) {
	src := "" +
		"class Box[T](val value: T) {\n" +
		"  def map[U](f: T => U): Box[U] = Box(f(value))\n" +
		"}\n" +
		"def id[A](x: A): A = x\n" +
		"def use(): Int = id(1)\n" +
		"def box(): Box[Int] = Box(1)\n"
	info := mustCheck(t, src)
	if findDef(info, "id") == nil || findDef(info, "use") == nil {
		t.Fatal("missing defs")
	}
	box := findDef(info, "box")
	if box == nil || info.Ctx.Format(box.Result) != "Box[Int]" {
		t.Fatalf("box %s", info.Ctx.Format(box.Result))
	}
}

func TestTuplesAndNull(t *testing.T) {
	mustCheck(t, "def pair(): Tuple2[Int, String] = (1, \"a\")\n")
	info := checkSrc(t, "val x: Int = null\n")
	if !info.Diags.HasErrors() {
		t.Fatal("null should not inhabit Int")
	}
	mustCheck(t, "val s: String = null\n")
}

func TestLambdaAndBlock(t *testing.T) {
	mustCheck(t, "def apply(f: Int => Int): Int = f(1)\nval g: Int => Int = (x: Int) => x + 1\n")
	mustCheck(t, "def blk(): Int = { val x = 1\n val y = x + 2\n y }\n")
}

func TestSealedMatch(t *testing.T) {
	src := "" +
		"enum Color {\n" +
		"  case Red\n" +
		"  case Blue\n" +
		"  def name(c: Color): String = c match {\n" +
		"    case Red => \"red\"\n" +
		"    case Blue => \"blue\"\n" +
		"  }\n" +
		"}\n"
	mustCheck(t, src)
	bad := "" +
		"enum Color {\n" +
		"  case Red\n" +
		"  case Blue\n" +
		"  def name(c: Color): String = c match {\n" +
		"    case Red => \"red\"\n" +
		"  }\n" +
		"}\n"
	info := checkSrc(t, bad)
	if !info.Diags.ByMessage("exhaustive") {
		t.Fatalf("exhaust:\n%s", info.Diags.Render())
	}
}

func TestConsInstantiation(t *testing.T) {
	mustCheck(t, `
enum List[+A] {
  case Nil
  case Cons(head: A, tail: List[A])
  def map[B](f: A => B): List[B] = this match {
    case Nil => Nil
    case Cons(h, t) => Cons(f(h), t.map(f))
  }
}
def main(): List[Int] = Cons(1, Nil).map((x: Int) => x)
`)
}

func TestGiven(t *testing.T) {
	src := "" +
		"trait Ord[A] { def compare(x: A, y: A): Int }\n" +
		"given intOrd: Ord[Int] with {\n" +
		"  def compare(x: Int, y: Int): Int = if x < y then -1 else if x > y then 1 else 0\n" +
		"}\n" +
		"def max(x: Int, y: Int)(using ord: Ord[Int]): Int = if ord.compare(x, y) < 0 then y else x\n" +
		"def use(): Int = max(1, 2)\n"
	info := mustCheck(t, src)
	if findDef(info, "use") == nil {
		t.Fatal("use")
	}
	if !strings.Contains(info.Ctx.Format(findDef(info, "use").Result), "Int") {
		t.Fatal(info.Ctx.Format(findDef(info, "use").Result))
	}
}
