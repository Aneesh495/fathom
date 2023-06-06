package bc

import "encoding/binary"

// Buf is a growing instruction buffer. Jumps are patched once their targets exist.
type Buf struct {
	Code []byte
}

// Len is the current program counter.
func (b *Buf) Len() int { return len(b.Code) }

// Op emits an instruction with no immediate.
func (b *Buf) Op(op Op) { b.Code = append(b.Code, byte(op)) }

// U8 emits an instruction and one byte.
func (b *Buf) U8(op Op, v byte) {
	b.Code = append(b.Code, byte(op), v)
}

// U16 emits an instruction and a little-endian unsigned 16-bit immediate.
func (b *Buf) U16(op Op, v int) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(v))
	b.Code = append(b.Code, byte(op))
	b.Code = append(b.Code, buf[:]...)
}

// I16 emits a signed 16-bit immediate.
func (b *Buf) I16(op Op, v int) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(int16(v)))
	b.Code = append(b.Code, byte(op))
	b.Code = append(b.Code, buf[:]...)
}

// U16U8 emits two immediates.
func (b *Buf) U16U8(op Op, a int, c byte) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(a))
	b.Code = append(b.Code, byte(op))
	b.Code = append(b.Code, buf[:]...)
	b.Code = append(b.Code, c)
}

// PatchI16 writes a signed relative offset at the immediate of an instruction
// that begins at at. The offset is computed from the instruction's end.
func (b *Buf) PatchI16(at, target int) {
	// at points at the opcode. The immediate starts at at+1. The next
	// instruction starts at at+3. Relative offset is target - (at+3).
	rel := target - (at + 3)
	binary.LittleEndian.PutUint16(b.Code[at+1:], uint16(int16(rel)))
}

// ReadU16 reads a little-endian u16 at i.
func ReadU16(code []byte, i int) int {
	return int(binary.LittleEndian.Uint16(code[i:]))
}

// ReadI16 reads a little-endian i16 at i.
func ReadI16(code []byte, i int) int {
	return int(int16(binary.LittleEndian.Uint16(code[i:])))
}
