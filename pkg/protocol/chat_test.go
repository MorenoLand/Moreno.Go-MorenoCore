package protocol

import "testing"

func TestBuildSystemChatMessage(t *testing.T) {
	data := BuildSystemChatMessage("hello")
	reader := NewReader(data)
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("chat type=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("language=%d err=%v", value, err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 0 {
		t.Fatalf("sender=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("flags=%d err=%v", value, err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 0 {
		t.Fatalf("receiver=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 6 {
		t.Fatalf("message length=%d err=%v", value, err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "hello" {
		t.Fatalf("message=%q err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("chat tag=%d err=%v", value, err)
	}
}

func TestBuildChatMessage(t *testing.T) {
	data := BuildChatMessage(0x11, 1, 99, 99, "hello", "General")
	reader := NewReader(data)
	if value, err := reader.ReadU8(); err != nil || value != 0x11 {
		t.Fatalf("chat type=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 1 {
		t.Fatalf("language=%d err=%v", value, err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("sender=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "General" {
		t.Fatalf("channel=%q err=%v", value, err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("receiver=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 6 {
		t.Fatalf("message length=%d err=%v", value, err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "hello" {
		t.Fatalf("message=%q err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("tag=%d err=%v", value, err)
	}
}
