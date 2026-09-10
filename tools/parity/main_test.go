package main

import (
	"strings"
	"testing"
)

func TestDifference(t *testing.T) {
	left := []string{"alpha", "bravo", "charlie", "delta"}
	right := []string{"bravo", "delta"}
	diff := difference(left, right)
	if len(diff) != 2 || diff[0] != "alpha" || diff[1] != "charlie" {
		t.Fatalf("unexpected diff: %+v", diff)
	}
}

func TestKeys(t *testing.T) {
	m := map[string]string{
		"z": "1",
		"a": "2",
		"m": "3",
	}
	k := keys(m)
	if len(k) != 3 || k[0] != "a" || k[1] != "m" || k[2] != "z" {
		t.Fatalf("unexpected sorted keys: %+v", k)
	}
}

func TestListFormatting(t *testing.T) {
	items := []string{"foo", "bar"}
	res := list(items)
	expected := "- `foo`\n- `bar`\n"
	if res != expected {
		t.Fatalf("expected %q, got %q", expected, res)
	}

	empty := list(nil)
	if empty != "No missing symbols detected." {
		t.Fatalf("expected no symbols message, got %q", empty)
	}
}

func TestOpcodePatternMatching(t *testing.T) {
	sample := `
		DEFINE_HANDLER(CMSG_PING, STATUS_LOGGEDIN, &WorldSession::HandlePing);
		DEFINE_HANDLER(CMSG_MOVE_WATER_WALK_ACK, STATUS_LOGGEDIN, &WorldSession::Handle_NULL);
	`
	matches := opcodePattern.FindAllStringSubmatch(sample, -1)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0][1] != "CMSG_PING" || matches[1][1] != "CMSG_MOVE_WATER_WALK_ACK" {
		t.Fatalf("unexpected opcodes: %s, %s", matches[0][1], matches[1][1])
	}

	nullMatches := nullOpcodePattern.FindAllStringSubmatch(sample, -1)
	if len(nullMatches) != 1 || nullMatches[0][1] != "CMSG_MOVE_WATER_WALK_ACK" {
		t.Fatalf("expected null opcode match for CMSG_MOVE_WATER_WALK_ACK, got %+v", nullMatches)
	}
}

func TestClientOpcodesFilter(t *testing.T) {
	input := []string{"CMSG_TEST", "SMSG_TEST", "MSG_TEST", "NUM_MSG_TYPES"}
	filtered := clientOpcodes(input)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 client opcodes, got %d: %+v", len(filtered), filtered)
	}
	for _, op := range filtered {
		if !strings.HasPrefix(op, "CMSG_") && !strings.HasPrefix(op, "MSG_") {
			t.Fatalf("unexpected non-client opcode: %s", op)
		}
	}
}
