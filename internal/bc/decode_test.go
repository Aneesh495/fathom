package bc

import "testing"

func TestRoundTripJump(t *testing.T) {
	var b Buf
	b.Op(OpUnit)
	at := b.Len()
	b.I16(OpJump, 0)
	b.Op(OpHalt)
	b.PatchI16(at, b.Len())
	in, err := Decode(b.Code, at)
	if err != nil {
		t.Fatal(err)
	}
	if in.Op != OpJump {
		t.Fatalf("op %s", Lookup(in.Op).Name)
	}
	if in.Next+in.A != b.Len() {
		t.Fatalf("target %d want %d", in.Next+in.A, b.Len())
	}
	if err := Walk(b.Code, func(Inst) {}); err != nil {
		t.Fatal(err)
	}
}
