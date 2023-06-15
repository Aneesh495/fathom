package vm

// gc is a precise mark-sweep collector.
//
// Roots are the globals and every live frame's locals and operand stack.
// Heap slots are integer references, so the collector never inspects Go
// pointers inside a value except the string bytes, which live on the host heap.
// Collection runs before a fresh allocation once the allocation counter crosses
// the threshold. The object being allocated is not yet a root, so it is created
// after the collection returns.
func (m *Machine) gc() {
	for _, o := range m.heap {
		if o != nil {
			o.mark = false
		}
	}
	for _, g := range m.globals {
		m.markVal(g)
	}
	for _, fr := range m.frames {
		if fr == nil {
			continue
		}
		for _, v := range fr.locals {
			m.markVal(v)
		}
		for _, v := range fr.stack {
			m.markVal(v)
		}
	}
	var free []int
	for i, o := range m.heap {
		if i == 0 || o == nil {
			continue
		}
		if !o.mark {
			m.heap[i] = nil
			free = append(free, i)
		}
	}
	m.free = free
}

func (m *Machine) markVal(v val) {
	if v.tag != tObj && v.tag != tClos {
		return
	}
	o := m.obj(v.ref)
	if o == nil || o.mark {
		return
	}
	o.mark = true
	for _, f := range o.fields {
		m.markVal(f)
	}
	for _, c := range o.caps {
		m.markVal(c)
	}
}

// LiveObjects reports how many heap slots are occupied. Tests use it to see
// that a collection actually reclaimed something.
func (m *Machine) LiveObjects() int {
	n := 0
	for i, o := range m.heap {
		if i > 0 && o != nil {
			n++
		}
	}
	return n
}
