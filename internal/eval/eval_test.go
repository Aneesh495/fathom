package eval

import (
	"strings"
	"testing"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/check"
	"github.com/Aneesh495/fathom/internal/parser"
	"github.com/Aneesh495/fathom/internal/span"
)

func runSrc(t *testing.T, src string) (Value, string) {
	t.Helper()
	file, diags := parser.Parse(span.NewFile("t.scala", src))
	if diags.HasErrors() {
		t.Fatalf("parse:\n%s", diags.Render())
	}
	info := check.Check([]*ast.File{file})
	if info.Diags.HasErrors() {
		t.Fatalf("check:\n%s", info.Diags.Render())
	}
	v, out, err := Run(info)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return v, out
}

func TestFact(t *testing.T) {
	v, _ := runSrc(t, `
def fact(n: Int): Int = if n == 0 then 1 else n * fact(n - 1)
def main(): Int = fact(6)
`)
	if v.Tag != VInt || v.I != 720 {
		t.Fatalf("got %s", v)
	}
}

func TestClassAndPrint(t *testing.T) {
	v, out := runSrc(t, `
class Box[T](val value: T) {
  def map[U](f: T => U): Box[U] = Box(f(value))
}
def main(): Int = {
  val b = Box(3)
  println(b.map((x: Int) => x + 10).value)
  b.value
}
`)
	if v.I != 3 {
		t.Fatalf("value %s", v)
	}
	if strings.TrimSpace(out) != "13" {
		t.Fatalf("out %q", out)
	}
}

func TestEnumMatch(t *testing.T) {
	v, _ := runSrc(t, `
enum Color {
  case Red
  case Blue
}
def name(c: Color): String = c match {
  case Red => "red"
  case Blue => "blue"
}
def main(): String = name(Blue)
`)
	if v.S != "blue" {
		t.Fatalf("got %s", v)
	}
}

func TestGivenMax(t *testing.T) {
	v, _ := runSrc(t, `
trait Ord[A] { def compare(x: A, y: A): Int }
given intOrd: Ord[Int] with {
  def compare(x: Int, y: Int): Int = if x < y then -1 else if x > y then 1 else 0
}
def max(x: Int, y: Int)(using ord: Ord[Int]): Int =
  if ord.compare(x, y) < 0 then y else x
def main(): Int = max(3, 9)
`)
	if v.I != 9 {
		t.Fatalf("got %s", v)
	}
}

func TestListMap(t *testing.T) {
	v, _ := runSrc(t, `
enum List[+A] {
  case Nil
  case Cons(head: A, tail: List[A])
  def map[B](f: A => B): List[B] = this match {
    case Nil => Nil
    case Cons(h, t) => Cons(f(h), t.map(f))
  }
}
def main(): Int = Cons(1, Cons(2, Nil)).map((x: Int) => x + 10) match {
  case Cons(h, _) => h
  case Nil => 0
}
`)
	if v.I != 11 {
		t.Fatalf("got %s", v)
	}
}

func TestDivZero(t *testing.T) {
	file, diags := parser.Parse(span.NewFile("t.scala", "def main(): Int = 1 / 0\n"))
	if diags.HasErrors() {
		t.Fatal(diags.Render())
	}
	info := check.Check([]*ast.File{file})
	if info.Diags.HasErrors() {
		t.Fatal(info.Diags.Render())
	}
	_, _, err := Run(info)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("err %v", err)
	}
}
