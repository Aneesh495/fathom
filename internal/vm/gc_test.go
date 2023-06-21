package vm

import (
	"testing"

	"github.com/Aneesh495/fathom/internal/compile"
)

func TestGCKeepsRoots(t *testing.T) {
	m := &Machine{
		prog:   testProg(),
		nextGC: 1,
		heap:   []*object{nil},
	}
	root := m.alloc(1, false, 0, nil)
	m.frames = []*frame{{
		locals: []val{{tag: tObj, ref: root}},
	}}
	// The rooted object points at a child. Everything else is garbage.
	child := m.alloc(1, false, 0, nil)
	m.heap[root].fields[0] = val{tag: tObj, ref: child}
	for i := 0; i < 40; i++ {
		m.alloc(1, false, 0, nil)
	}
	if m.LiveObjects() < 10 {
		t.Fatalf("expected garbage, live %d", m.LiveObjects())
	}
	m.gc()
	if got := m.LiveObjects(); got != 2 {
		t.Fatalf("live %d, want the root and its child", got)
	}
	if m.obj(root) == nil || m.obj(child) == nil {
		t.Fatal("collection dropped a root")
	}
	// A second collection is idempotent.
	m.gc()
	if m.LiveObjects() != 2 {
		t.Fatalf("second gc live %d", m.LiveObjects())
	}
}

func testProg() *compile.Program {
	return &compile.Program{Classes: []*compile.Class{{}, {ID: 1, Name: "Cell", NFields: 1}}}
}
