package protocoltrace

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecorderRoundTripAndReplay(t *testing.T) {
	recorder := NewRecorder("fixture")
	if recorder.Record(ClientToServer, 0x1234, []byte{1, 2, 3}, "world-entry") != 1 {
		t.Fatal("expected first sequence to be one")
	}
	recorder.Record(ServerToClient, 0x5678, []byte{4, 5}, "character-list")
	var encoded bytes.Buffer
	if err := recorder.Snapshot().Write(&encoded); err != nil {
		t.Fatal(err)
	}
	trace, err := Load(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	if trace.Header.Source != "fixture" || len(trace.Events) != 2 {
		t.Fatalf("unexpected trace: %+v", trace)
	}
	if payload, err := trace.Payload(trace.Events[1]); err != nil || !bytes.Equal(payload, []byte{4, 5}) {
		t.Fatalf("unexpected payload: %x %v", payload, err)
	}
	sink := &recordingSink{}
	if err := trace.Replay(sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 2 || sink.events[0].opcode != 0x1234 || !bytes.Equal(sink.events[1].payload, []byte{4, 5}) {
		t.Fatalf("unexpected replay: %+v", sink.events)
	}
}

func TestDiffReportsPacketAndStateChanges(t *testing.T) {
	expected := Trace{Header: Header{Format: Format, Version: Version}, Events: []Event{{Sequence: 1, TimeNS: 10, Direction: ServerToClient, Opcode: 1, Payload: "AQI=", State: "ready"}}}
	actual := Trace{Header: Header{Format: Format, Version: Version}, Events: []Event{{Sequence: 1, TimeNS: 30, Direction: ServerToClient, Opcode: 2, Payload: "AwQ=", State: "loading"}}}
	differences, err := Diff(expected, actual, CompareOptions{CompareTiming: true, TimingTolerance: time.Nanosecond})
	if err != nil {
		t.Fatal(err)
	}
	fields := make(map[string]bool)
	for _, difference := range differences {
		fields[difference.Field] = true
	}
	for _, field := range []string{"opcode", "payload_base64", "state", "time_ns"} {
		if !fields[field] {
			t.Fatalf("missing diff field %q: %+v", field, differences)
		}
	}
}

func TestLoadRejectsInvalidTrace(t *testing.T) {
	if _, err := Load(strings.NewReader("{}\n")); err == nil {
		t.Fatal("expected invalid header error")
	}
	if _, err := Load(nil); err == nil {
		t.Fatal("expected nil reader error")
	}
}

func TestReplayStopsOnSinkError(t *testing.T) {
	trace := Trace{Header: Header{Format: Format, Version: Version}, Events: []Event{{Sequence: 1, Direction: ClientToServer, Payload: "", Opcode: 1}}}
	want := errors.New("stop")
	err := trace.Replay(&recordingSink{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("expected sink error, got %v", err)
	}
}

type replayEvent struct {
	direction Direction
	opcode    uint32
	payload   []byte
	state     string
}

type recordingSink struct {
	events []replayEvent
	err    error
}

func (s *recordingSink) Handle(direction Direction, opcode uint32, payload []byte, state string) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, replayEvent{direction: direction, opcode: opcode, payload: payload, state: state})
	return nil
}
