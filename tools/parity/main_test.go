package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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

func TestSessionHandlerAuditExcludesReferenceNoOps(t *testing.T) {
	root := t.TempDir()
	source := `package world
type session struct{}
func (s *session) handleKeepAlive() bool { return true }
func (s *session) handlePlayerLogout() bool { return true }
func (s *session) handleMissingBehavior() bool { return true }
`
	if err := os.WriteFile(filepath.Join(root, "audit.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	total, trivial, err := goSessionHandlerAudit(root)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(trivial) != 1 || !strings.Contains(trivial[0], "handleMissingBehavior") {
		t.Fatalf("total=%d trivial=%v", total, trivial)
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

func TestGeneratedHandleNullAuditHasEvidenceAndGoDispatchCoverage(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate parity package")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	report, err := os.ReadFile(filepath.Join(root, "docs", "PARITY_COVERAGE.md"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := os.ReadFile(filepath.Join(root, "engine", "world", "server.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(report)
	start := strings.Index(text, "## Handle_NULL status audit")
	end := strings.Index(text[start:], "## Missing prepared statements")
	if start < 0 || end < 0 {
		t.Fatal("generated Handle_NULL audit section is missing")
	}
	section := text[start : start+end]
	rowPattern := regexp.MustCompile(`(?m)^\| \x60((?:CMSG|MSG)_[A-Z0-9_]+)\x60 \| \x60(STATUS_[A-Z0-9_]+)\x60 \| ([^|]+) \| ([^|]+) \|$`)
	rows := rowPattern.FindAllStringSubmatch(section, -1)
	if len(rows) < 290 {
		t.Fatalf("Handle_NULL audit rows=%d, want at least 290", len(rows))
	}
	for _, row := range rows {
		if strings.TrimSpace(row[3]) == "" || strings.TrimSpace(row[4]) == "" {
			t.Fatalf("Handle_NULL row lacks classification/evidence: %v", row)
		}
		if !strings.Contains(string(server), "protocol.Opcode"+row[1]) {
			t.Fatalf("Handle_NULL opcode %s has no Go dispatch reference", row[1])
		}
	}
}
