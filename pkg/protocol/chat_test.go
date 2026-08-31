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
