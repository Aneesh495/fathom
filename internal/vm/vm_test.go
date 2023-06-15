package vm

import (
	"testing"

	"github.com/Aneesh495/fathom/internal/ast"
	"github.com/Aneesh495/fathom/internal/check"
	"github.com/Aneesh495/fathom/internal/compile"
	"github.com/Aneesh495/fathom/internal/eval"
	"github.com/Aneesh495/fathom/internal/parser"
	"github.com/Aneesh495/fathom/internal/span"
)

func agree(t *testing.T, src string) {
	t.Helper()
	file, diags := parser.Parse(span.NewFile("t.scala", src))
	if diags.HasErrors() {
		t.Fatalf("parse:\n%s", diags.Render())
	}
	info := check.Check([]*ast.File{file})
	if info.Diags.HasErrors() {
		t.Fatalf("check:\n%s", info.Diags.Render())
	}
	ev, out, err := eval.Run(info)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	prog, err := compile.Compile(info)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got, err := Run(prog)
	if err != nil {
		main := prog.Funcs[prog.Main]
		t.Fatalf("vm: %v\n%s", err, compile.Disasm(main))
	}
	if got.Value != ev.String() || got.Output != out {
		t.Fatalf("eval %q out %q\nvm  %q out %q\n%s", ev.String(), out, got.Value, got.Output, compile.Disasm(prog.Funcs[prog.Main]))
	}
}

func TestVMAgrees(t *testing.T) {
	agree(t, `
def fact(n: Int): Int = if n == 0 then 1 else n * fact(n - 1)
def main(): Int = fact(6)
`)
	agree(t, `
class Box[T](val value: T) {
  def map[U](f: T => U): Box[U] = Box(f(value))
}
def main(): Int = {
  val b = Box(3)
  println(b.map((x: Int) => x + 10).value)
  b.value
}
`)
	agree(t, `
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
	agree(t, `
trait Ord[A] { def compare(x: A, y: A): Int }
given intOrd: Ord[Int] with {
  def compare(x: Int, y: Int): Int = if x < y then -1 else if x > y then 1 else 0
}
def max(x: Int, y: Int)(using ord: Ord[Int]): Int =
  if ord.compare(x, y) < 0 then y else x
def main(): Int = max(3, 9)
`)
	agree(t, `
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
}
