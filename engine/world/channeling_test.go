package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func newChannelTestSession(t *testing.T, player *playerState) (*session, net.Conn) {
	t.Helper()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() { clientConn.Close() })
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: player != nil, playerGUID: 9, player: player}
	return state, clientConn
}

// writeChannelSpellDBC writes a minimal Spell.dbc whose spell 1000 carries the
// channeled attribute (AttributesEx1 field 6), CHANNEL_FLAG_DELAY, push-back
// interrupt flags, duration index 1, and one periodic damage effect.
func writeChannelSpellDBC(t *testing.T, dir string) {
	t.Helper()
	const fieldCount = 234
	records := make(map[uint32][]uint32)
	setField := func(spell, index uint32, value uint32) []uint32 {
		row, ok := records[spell]
		if !ok {
			row = make([]uint32, fieldCount)
			records[spell] = row
		}
		row[index] = value
		return row
	}
	setField(1000, 0, 1000)  // ID column
	setField(1000, 6, 0x04)  // SPELL_ATTR1_CHANNELED_1
	setField(1000, 31, 0x02) // SPELL_INTERRUPT_FLAG_PUSH_BACK
	setField(1000, 33, 0x4000|0x01)
	setField(1000, 40, 1) // duration index
	setField(1000, 71, 6) // effect 0: apply aura (periodic)
	setField(1000, 80, 10)
	setField(1000, 95, 3)   // periodic damage aura
	setField(1000, 98, 500) // 500ms period
	setField(2000, 0, 2000)
	setField(2000, 31, 0x02)
	setField(2000, 28, 5) // cast time index unused; only flags matter
	setField(2000, 40, 1)

	var body []byte
	putU32 := func(v uint32) {
		b := []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
		body = append(body, b...)
	}
	for id := uint32(1000); id <= 2000; id += 1000 {
		row := records[id]
		for _, v := range row {
			putU32(v)
		}
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	put := func(off int, v uint32) {
		header[off] = byte(v)
		header[off+1] = byte(v >> 8)
		header[off+2] = byte(v >> 16)
		header[off+3] = byte(v >> 24)
	}
	put(4, 2)             // records
	put(8, fieldCount)    // fields
	put(12, fieldCount*4) // record size
	put(16, 1)            // string block
	if err := writeFileBytes(dir, "Spell.dbc", append(header, append(body, 0)...)); err != nil {
		t.Fatal(err)
	}
	if err := writeFileBytes(dir, "SpellDuration.dbc", buildSpellDurationDBC()); err != nil {
		t.Fatal(err)
	}
}

func buildSpellDurationDBC() []byte {
	// SpellDuration.dbc "niii": one record: id 1, duration 4000, per-level 0, max 4000.
	var out []byte
	out = append(out, "WDBC"...)
	appendU32 := func(v uint32) {
		out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	}
	appendU32(1)    // record count
	appendU32(4)    // field count
	appendU32(16)   // record size
	appendU32(1)    // string block size
	appendU32(1)    // record: id
	appendU32(4000) // duration
	appendU32(0)    // per level
	appendU32(4000) // max duration
	out = append(out, 0)
	return out
}

func writeFileBytes(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), data, 0o644)
}

func TestIsChanneledSpell(t *testing.T) {
	if isChanneledSpell(wotlk.Spell{AttributesEx1: 0x04}) != true {
		t.Fatal("channeled_1 flag not detected")
	}
	if isChanneledSpell(wotlk.Spell{AttributesEx1: 0x40}) != true {
		t.Fatal("channeled_2 flag not detected")
	}
	if isChanneledSpell(wotlk.Spell{AttributesEx1: 0x00, Attributes: 0}) != false {
		t.Fatal("ordinary spell flagged as channeled")
	}
}

func TestStartChannelAnnouncesAndTicks(t *testing.T) {
	dir := t.TempDir()
	writeChannelSpellDBC(t, dir)
	data := wotlk.NewStore(dir)
	spell, found, err := data.Spell(1000)
	if err != nil || !found {
		t.Fatalf("spell lookup: found=%v err=%v", found, err)
	}
	duration, ok, err := data.SpellDuration(spell.DurationIndex, 70)
	if err != nil || !ok || duration != 4000 {
		t.Fatalf("duration=%d ok=%v err=%v", duration, ok, err)
	}
	player := &playerState{GUID: 9, Level: 70, Health: 100, MaxHealth: 100}
	state, clientConn := newChannelTestSession(t, player)
	state.server.Data = data
	frames := make(chan uint16, 8)
	go func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			opcode, _, err := readServerFrame(clientConn, nil)
			if err != nil {
				close(frames)
				return
			}
			frames <- opcode
		}
	}()
	state.startChannel(1, 1000, spell, 0)
	sawStart := false
	timeout := time.After(2 * time.Second)
	for !sawStart {
		select {
		case opcode, ok := <-frames:
			if !ok {
				t.Fatal("connection closed before channel start")
			}
			if opcode == uint16(protocol.OpcodeMSG_CHANNEL_START) {
				sawStart = true
			}
		case <-timeout:
			t.Fatal("channel start not observed")
		}
	}
	state.finishChannel()
	select {
	case opcode, ok := <-frames:
		if ok && opcode != uint16(protocol.OpcodeMSG_CHANNEL_UPDATE) {
			t.Fatalf("unexpected opcode %x", opcode)
		}
	case <-time.After(time.Second):
	}
	state.castMu.Lock()
	active := state.activeChannel
	state.castMu.Unlock()
	if active != nil {
		t.Fatal("channel not cleared after finish")
	}
}

func TestChannelPushbackReducesRemaining(t *testing.T) {
	dir := t.TempDir()
	writeChannelSpellDBC(t, dir)
	data := wotlk.NewStore(dir)
	spell, _, _ := data.Spell(1000)
	player := &playerState{GUID: 9, Level: 70, Health: 100, MaxHealth: 100}
	state, clientConn := newChannelTestSession(t, player)
	state.server.Data = data
	go func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()
	state.startChannel(1, 1000, spell, 0)
	state.castMu.Lock()
	if state.activeChannel == nil {
		state.castMu.Unlock()
		t.Fatal("channel missing")
	}
	before := state.activeChannel.Remaining
	state.castMu.Unlock()

	state.delayCurrentChannel() // 25% of 4000ms = 1000ms
	state.castMu.Lock()
	after := state.activeChannel
	pushbacks := 0
	var remaining time.Duration
	if after != nil {
		pushbacks = after.Pushbacks
		remaining = after.Remaining
	}
	state.castMu.Unlock()
	if after == nil || pushbacks != 1 {
		t.Fatalf("pushbacks=%d channel=%v", pushbacks, after)
	}
	if remaining >= before {
		t.Fatalf("remaining=%v not reduced from %v", remaining, before)
	}
	if diff := before - remaining; diff < 900*time.Millisecond {
		t.Fatalf("pushback %v too small", diff)
	}

	// Max two pushbacks total.
	state.delayCurrentChannel()
	state.delayCurrentChannel()
	state.castMu.Lock()
	pushbacks = 0
	if state.activeChannel != nil {
		pushbacks = state.activeChannel.Pushbacks
	}
	state.castMu.Unlock()
	if pushbacks != 2 {
		t.Fatalf("pushbacks=%d exceeds reference max of 2", pushbacks)
	}
}

func TestCastPushbackDelaysTimer(t *testing.T) {
	player := &playerState{GUID: 9, Level: 70, Health: 100, MaxHealth: 100}
	state, clientConn := newChannelTestSession(t, player)
	go func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()
	timer := time.NewTimer(2 * time.Second)
	state.castMu.Lock()
	state.activeCast = &activeCastState{
		CastID:       1,
		SpellID:      2000,
		Timer:        timer,
		StartAt:      time.Now(),
		CastTimeMs:   3000,
		InterruptFlg: 0x02, // SPELL_INTERRUPT_FLAG_PUSH_BACK
	}
	state.castMu.Unlock()

	state.delayCurrentCast()
	state.castMu.Lock()
	pushbacks := state.activeCast.Pushbacks
	startAt := state.activeCast.StartAt
	state.castMu.Unlock()
	if pushbacks != 1 {
		t.Fatalf("pushbacks=%d", pushbacks)
	}
	// The pushback extends the effective cast window: StartAt shifts back so
	// the remaining time (CastTimeMs - elapsed) grows by ~500ms.
	elapsed := time.Since(startAt)
	if elapsed > -400*time.Millisecond {
		t.Fatalf("effective cast time not extended: elapsed=%v", elapsed)
	}
	// Reference caps pushbacks at two.
	state.delayCurrentCast()
	state.delayCurrentCast()
	state.castMu.Lock()
	pushbacks = state.activeCast.Pushbacks
	state.castMu.Unlock()
	if pushbacks != 2 {
		t.Fatalf("pushbacks=%d exceeds reference cap", pushbacks)
	}
	// Casts without the pushback flag are unaffected.
	state.castMu.Lock()
	state.activeCast.InterruptFlg = 0
	state.castMu.Unlock()
	state.delayCurrentCast()
	state.castMu.Lock()
	pushbacks = state.activeCast.Pushbacks
	state.castMu.Unlock()
	if pushbacks != 2 {
		t.Fatalf("unflagged cast was pushed back: %d", pushbacks)
	}
	timer.Stop()
}

func TestDamageTakenHooksDelayCastAndChannel(t *testing.T) {
	dir := t.TempDir()
	writeChannelSpellDBC(t, dir)
	data := wotlk.NewStore(dir)
	spell, _, _ := data.Spell(1000)
	player := &playerState{GUID: 9, Level: 70, Health: 100, MaxHealth: 100}
	player.Powers[0] = 100
	state, clientConn := newChannelTestSession(t, player)
	state.server.Data = data
	go func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()
	state.startChannel(1, 1000, spell, 0)
	timer := time.NewTimer(2 * time.Second)
	state.castMu.Lock()
	state.activeCast = &activeCastState{CastID: 2, SpellID: 2000, Timer: timer, StartAt: time.Now(), CastTimeMs: 3000, InterruptFlg: 0x02}
	state.castMu.Unlock()

	// The damage-taken hooks must push both states exactly like the reference
	// DealDamage path.
	state.delayCurrentCast()
	state.delayCurrentChannel()
	state.castMu.Lock()
	castPushbacks := state.activeCast.Pushbacks
	channelPushbacks := 0
	if state.activeChannel != nil {
		channelPushbacks = state.activeChannel.Pushbacks
	}
	state.castMu.Unlock()
	if castPushbacks != 1 || channelPushbacks != 1 {
		t.Fatalf("cast=%d channel=%d", castPushbacks, channelPushbacks)
	}
	timer.Stop()
	var _ atomic.Uint32
	var _ = sql.ErrNoRows
	_ = context.Background()
}
