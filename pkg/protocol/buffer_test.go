package protocol

import (
	"math"
	"testing"
)

func TestBufferRoundTrip(t *testing.T) {
	b := NewBuffer(32)
	b.WriteBool(true)
	b.WriteU16(0x1234)
	b.WriteU32(0x12345678)
	b.WriteU64(0x123456789abcdef0)
	b.WriteI32(-42)
	b.WriteF32(1.5)
	b.WriteF64(math.Pi)
	b.WriteCString("hello")
	b.ResetRead()
	if value, err := b.ReadBool(); err != nil || !value {
		t.Fatalf("bool: %v %v", value, err)
	}
	if value, err := b.ReadU16(); err != nil || value != 0x1234 {
		t.Fatalf("u16: %x %v", value, err)
	}
	if value, err := b.ReadU32(); err != nil || value != 0x12345678 {
		t.Fatalf("u32: %x %v", value, err)
	}
	if value, err := b.ReadU64(); err != nil || value != 0x123456789abcdef0 {
		t.Fatalf("u64: %x %v", value, err)
	}
	if value, err := b.ReadI32(); err != nil || value != -42 {
		t.Fatalf("i32: %d %v", value, err)
	}
	if value, err := b.ReadF32(); err != nil || value != float32(1.5) {
		t.Fatalf("f32: %v %v", value, err)
	}
	if value, err := b.ReadF64(); err != nil || value != math.Pi {
		t.Fatalf("f64: %v %v", value, err)
	}
	if value, err := b.ReadCString(); err != nil || value != "hello" {
		t.Fatalf("string: %q %v", value, err)
	}
	if b.Remaining() != 0 {
		t.Fatalf("remaining bytes: %d", b.Remaining())
	}
}

func TestPackedGUIDRoundTrip(t *testing.T) {
	b := NewBuffer(16)
	b.WritePackedGUID(0x0011223300445500)
	b.ResetRead()
	value, err := b.ReadPackedGUID()
	if err != nil || value != 0x0011223300445500 {
		t.Fatalf("value=%x err=%v", value, err)
	}
}
