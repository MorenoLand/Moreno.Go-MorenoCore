package scripting

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/Shopify/go-lua"
	_ "modernc.org/sqlite"
)

func TestPlayerEventRegistrationAndInvocation(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`RegisterPlayerEvent(18, function(event, player, message) return message == "allowed" end)`); err != nil {
		t.Fatal(err)
	}
	values, err := runtime.TriggerPlayerEvent(context.Background(), 18, 18, nil, "allowed")
	if err != nil || len(values) != 1 || values[0] != true {
		t.Fatalf("values=%v err=%v", values, err)
	}
}

func TestLuaObjectMethodsAndTimers(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	object := &Object{Type: "Player", Methods: map[string]ObjectMethod{"GetValue": func(context.Context, []any) ([]any, error) { return []any{uint32(42)}, nil }}}
	if err := runtime.LoadString(`CreateLuaEvent(function() end, 10, 1); RegisterPlayerEvent(3, function(event, player) return player:GetValue() end)`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tick(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	values, err := runtime.TriggerPlayerEvent(context.Background(), 3, 3, object)
	if err != nil || len(values) != 1 || values[0].(float64) != 42 {
		t.Fatalf("values=%v err=%v", values, err)
	}
}

func TestLuaTimerArgumentsAndInfiniteRepeats(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`calls = 0; lastEvent = 0; lastDelay = 0; lastRepeats = 0; CreateLuaEvent(function(event, delay, repeats) calls = calls + 1; lastEvent = event; lastDelay = delay; lastRepeats = repeats end, 10, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tick(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tick(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`assert(calls == 2); assert(lastEvent > 0); assert(lastDelay == 10); assert(lastRepeats == 0)`); err != nil {
		t.Fatal(err)
	}
}

func TestLuaTimerAcceptsDelayRange(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`lastDelay = 0; CreateLuaEvent(function(event, delay, repeats) lastDelay = delay end, {10, 20}, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tick(context.Background(), 20); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`assert(lastDelay >= 10 and lastDelay <= 20)`); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveEventsCancelsGlobalLuaTimers(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`count = 0; CreateLuaEvent(function() count = count + 1 end, 10, 0); RemoveEvents()`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tick(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`assert(count == 0)`); err != nil {
		t.Fatal(err)
	}
}

func TestFileChunkEnvironment(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.initializeLocked()
	if err := runtime.state.Load(strings.NewReader(`return type(_ENV), type(_G), type(string)`), "@test", ""); err != nil {
		t.Fatal(err)
	}
	if err := runtime.state.ProtectedCall(0, lua.MultipleReturns, 0); err != nil {
		t.Fatal(err)
	}
	if runtime.state.Top() != 3 {
		t.Fatalf("results=%d", runtime.state.Top())
	}
	for index, expected := range []string{"table", "table", "table"} {
		value, ok := runtime.state.ToString(index + 1)
		if !ok || value != expected {
			t.Fatalf("result %d=%q", index, value)
		}
	}
}

func TestMultipleHookEnvironment(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`local f = function() end; RegisterPlayerEvent(4, f); RegisterServerEvent(31, f); RegisterServerEvent(32, f); RegisterServerEvent(17, f); RegisterServerEvent(18, f)`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`assert(type(_G) == "table"); assert(type(RegisterPlayerEvent) == "function")`); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeServerEventAndLuaTimer(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`count = 0; CreateLuaEvent(function() count = count + 1 end, 10, 1); RegisterServerEvent(13, function(event, diff) count = count + diff; return count end)`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Tick(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	values, err := runtime.TriggerServerEvent(context.Background(), 13, uint32(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != float64(101) {
		t.Fatalf("server event values=%v", values)
	}
}

func TestElunaGlobalFunctions(t *testing.T) {
	players := []*Object{
		{Fields: map[string]any{"GUID": uint64(7), "Name": "Alliance", "Team": uint32(0), "IsGM": false}},
		{Fields: map[string]any{"GUID": uint64(8), "Name": "HordeGM", "Team": uint32(1), "IsGM": true}},
	}
	runtime := NewRuntime(Config{Enabled: true, CoreName: "MorenoCore", CoreVersion: "1.2.3+gabc", RealmID: 7, CoreExpansion: 2, PlayerProvider: func() []*Object { return players }})
	source := `
		assert(GetLuaEngine() == "ElunaEngine")
		assert(GetCoreName() == "MorenoCore")
		assert(GetCoreVersion() == "1.2.3+gabc")
		assert(GetGameTime() > 0)
		assert(GetRealmID() == 7)
		assert(GetCoreExpansion() == 2)
		assert(GetPlayerCount() == 2)
		assert(GetPlayerByGUID(GetPlayerGUID(7)).Name == "Alliance")
		assert(GetGUIDLow(GetPlayerByName("hOrDeGm").GUID) == 8)
		assert(#GetPlayersInWorld() == 2)
		assert(#GetPlayersInWorld(1) == 1)
		assert(#GetPlayersInWorld(1, true) == 1)
		assert(#GetPlayersInWorld(0, true) == 0)
		assert(GetGUIDLow(GetPlayerGUID(7)) == 7)
		assert(GetGUIDLow(GetObjectGUID(7, 68)) == 7)
		assert(GetGUIDEntry(GetObjectGUID(7, 68)) == 68)
		assert(GetGUIDType(GetObjectGUID(7, 68)) == 0xF110)
		assert(GetGUIDEntry(GetUnitGUID(7, 68)) == 68)
		assert(GetGUIDType(GetUnitGUID(7, 68)) == 0xF130)
		assert(bit_and(0xF0, 0x0F) == 0)
		assert(bit_or(0xF0, 0x0F) == 0xFF)
		assert(bit_xor(0xFF, 0x0F) == 0xF0)
		assert(bit_lshift(1, 8) == 256)
		assert(bit_rshift(256, 8) == 1)
		assert(bit_not(0) == 0xFFFFFFFF)
		local now = GetCurrTime()
		assert(GetTimeDiff(now) < 1000)
	`
	if err := runtime.LoadString(source); err != nil {
		t.Fatal(err)
	}
}

func TestElunaRegistrationFamiliesShotsAndClear(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`
		count = 0
		RegisterPlayerEvent(90, function() count = count + 1 end, 1)
		RegisterCreatureEvent(68, 4, function() count = count + 10 end)
		RegisterCreatureGossipEvent(68, 1, function() count = count + 100 end)
		RegisterUniqueCreatureEvent(GetUnitGUID(7, 68), 3, 2, function() count = count + 1000 end)
		RegisterGameObjectEvent(9001, 14, function() count = count + 10000 end)
		RegisterGameObjectGossipEvent(9001, 1, function() count = count + 100000 end)
		RegisterItemEvent(6948, 2, function() count = count + 1000000 end)
		RegisterItemGossipEvent(6948, 1, function() count = count + 10000000 end)
		RegisterPacketEvent(0x123, 5, function() count = count + 100000000 end)
		RegisterMapEvent(0, 17, function() count = count + 1000000000 end)
		RegisterInstanceEvent(33, 17, function() count = count + 10000000000 end)
		RegisterGuildEvent(1, function() count = count + 100000000000 end)
		RegisterGroupEvent(1, function() count = count + 1000000000000 end)
		RegisterBGEvent(1, function() count = count + 10000000000000 end)
		RegisterServerEvent(31, function() count = count + 100000000000000 end)
		cancel = RegisterPlayerEvent(91, function() count = count + 1000000000000000 end)
		cancel()
	`); err != nil {
		t.Fatal(err)
	}
	if len(runtime.Hooks()) != 15 {
		t.Fatalf("hooks after cancel=%d", len(runtime.Hooks()))
	}
	if _, err := runtime.TriggerPlayerEvent(context.Background(), 90); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.TriggerPlayerEvent(context.Background(), 90); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`assert(count == 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Trigger(context.Background(), "creature:68", 4); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Trigger(context.Background(), "creature_gossip:68", 1); err != nil {
		t.Fatal(err)
	}
	uniqueKind := fmt.Sprintf("creature_unique:%d:3", uint64(0xF130)<<48|uint64(68)<<24|7)
	if _, err := runtime.Trigger(context.Background(), uniqueKind, 2); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`assert(count == 1111)`); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`ClearCreatureEvents(68, 4); ClearCreatureGossipEvents(68); ClearUniqueCreatureEvents(GetUnitGUID(7, 68), 3)`); err != nil {
		t.Fatal(err)
	}
	if len(runtime.Hooks()) != 11 {
		t.Fatalf("hooks after targeted clear=%d", len(runtime.Hooks()))
	}
	if _, err := runtime.Trigger(context.Background(), "creature:68", 4); err != nil {
		t.Fatal(err)
	}
	if err := runtime.LoadString(`assert(count == 1111); ClearServerEvents(31)`); err != nil {
		t.Fatal(err)
	}
	if len(runtime.Hooks()) != 10 {
		t.Fatalf("hooks after server clear=%d", len(runtime.Hooks()))
	}
}

func TestElunaGenericObjectMethods(t *testing.T) {
	var logs bytes.Buffer
	runtime := NewRuntime(Config{Enabled: true, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	creature := &Object{Type: "Creature", Fields: map[string]any{"GUID": uint64(7) | uint64(68)<<24 | uint64(0xF130)<<48, "Entry": uint32(68), "MapId": uint32(0), "X": float32(10), "Y": float32(20), "Z": float32(30), "Orientation": float32(1), "Health": uint32(50), "MaxHealth": uint32(100), "Level": uint32(10), "Power": uint32(25), "MaxPower": uint32(50), "InWorld": true}}
	other := &Object{Type: "GameObject", Fields: map[string]any{"GUID": uint64(8) | uint64(9001)<<24 | uint64(0xF110)<<48, "Entry": uint32(9001), "MapId": uint32(0), "X": float32(13), "Y": float32(24), "Z": float32(30), "InWorld": true}}
	if err := runtime.LoadString(`RegisterPlayerEvent(88, function(event, object, other)
		assert(object:GetEntry() == 68)
		assert(GetGUIDLow(object:GetGUID()) == 7)
		assert(object:GetTypeId() == 3)
		assert(object:IsInWorld())
		assert(object:GetMapId() == 0)
		assert(object:GetX() == 10 and object:GetY() == 20 and object:GetZ() == 30 and object:GetO() == 1)
		local x, y, z, o = object:GetLocation()
		assert(x == 10 and y == 20 and z == 30 and o == 1)
		assert(object:GetHealth() == 50 and object:GetMaxHealth() == 100)
		assert(object:GetHealthPct() == 50 and object:GetPowerPct() == 50)
		assert(object:IsAlive() and not object:IsDead() and not object:IsFullHealth())
		assert(object:IsInMap(other) and object:GetDistance(other) == 5)
		assert(object:IsWithinDist3d(other, 5.1) and object:IsWithinDist2d(other, 5.1))
		assert(object:ToCreature() == object and object:ToUnit() == object and object:ToPlayer() == nil)
		return true
	end)`); err != nil {
		t.Fatal(err)
	}
	values, err := runtime.TriggerPlayerEvent(context.Background(), 88, 88, creature, other)
	if err != nil || len(values) != 1 || values[0] != true {
		t.Fatalf("values=%v err=%v logs=%s", values, err, logs.String())
	}
}

func TestElunaInt64Constructors(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`
		signed = CreateInt64("-9223372036854775808")
		assert(tostring(signed) == "-9223372036854775808")
		assert(tostring(signed + CreateInt64(2)) == "-9223372036854775806")
		unsigned = CreateUint64("18446744073709551615")
		assert(tostring(unsigned) == "18446744073709551615")
		assert(tostring(unsigned + CreateUint64(1)) == "0")
		assert(CreateUint64("7") == CreateUint64(7))
	`); err != nil {
		t.Fatal(err)
	}
}

func TestElunaQueryMethods(t *testing.T) {
	var logs bytes.Buffer
	runtime := NewRuntime(Config{Enabled: true, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	query := &Query{columns: []string{"signed", "unsigned", "nullable", "text", "decimal", "flag"}, rows: [][]any{{int64(-8), uint64(18446744073709551615), nil, []byte("hello"), float64(1.5), int64(1)}, {int64(4), uint64(9), "not-null", "next", float64(2.5), int64(0)}}}
	if err := runtime.LoadString(`RegisterPlayerEvent(89, function(event, query)
		assert(query:GetColumnCount() == 6 and query:GetRowCount() == 2)
		assert(query:GetInt8(0) == -8 and query:GetInt16(0) == -8 and query:GetInt32(0) == -8)
		assert(tostring(query:GetInt64(0)) == "-8")
		assert(tostring(query:GetUInt64(1)) == "18446744073709551615")
		assert(query:IsNull(2) and query:GetString(3) == "hello")
		assert(query:GetFloat(4) == 1.5 and query:GetDouble(4) == 1.5 and query:GetBool(5))
		local row = query:GetRow()
		assert(row.signed == -8 and row.text == "hello")
		assert(query:NextRow())
		assert(not query:IsNull(2) and tostring(query:GetInt64(0)) == "4" and query:GetUInt32(1) == 9 and not query:GetBool(5))
		assert(not query:NextRow())
		return true
	end)`); err != nil {
		t.Fatal(err)
	}
	values, err := runtime.TriggerPlayerEvent(context.Background(), 89, 89, query)
	if err != nil || len(values) != 1 || values[0] != true {
		t.Fatalf("values=%v err=%v logs=%s", values, err, logs.String())
	}
}

func TestElunaPacketMethods(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`
		packet = CreatePacket(0x123)
		assert(packet:GetOpcode() == 0x123 and packet:GetSize() == 0)
		packet:WriteByte(-2); packet:WriteUByte(254); packet:WriteShort(-3); packet:WriteUShort(65530)
		packet:WriteLong(-4); packet:WriteULong(4294967292); packet:WriteFloat(1.5); packet:WriteDouble(2.5)
		packet:WriteGUID(CreateUint64("18446744073709551615")); packet:WriteString("hello")
		assert(packet:GetSize() > 0)
		assert(packet:ReadByte() == -2 and packet:ReadUByte() == 254)
		assert(packet:ReadShort() == -3 and packet:ReadUShort() == 65530)
		assert(packet:ReadLong() == -4 and packet:ReadULong() == 4294967292)
		assert(packet:ReadFloat() == 1.5 and packet:ReadDouble() == 2.5)
		assert(tostring(packet:ReadGUID()) == "18446744073709551615" and packet:ReadString() == "hello")
		packet:SetOpcode(7)
		assert(packet:GetOpcode() == 7)
	`); err != nil {
		t.Fatal(err)
	}
}

func TestElunaPacketEventDispatch(t *testing.T) {
	runtime := NewRuntime(Config{Enabled: true})
	if err := runtime.LoadString(`RegisterPacketEvent(0x123, 5, function(event, packet, player) assert(event == 5 and packet:GetOpcode() == 0x123 and player == nil); return false end)`); err != nil {
		t.Fatal(err)
	}
	values, err := runtime.TriggerPacketEvent(context.Background(), 0x123, 5, &Packet{Opcode: 0x123}, nil)
	if err != nil || len(values) != 1 || values[0] != false {
		t.Fatalf("values=%v err=%v", values, err)
	}
}

func TestElunaQuestLookup(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE quest_template (ID INTEGER PRIMARY KEY, LogTitle TEXT, QuestLevel INTEGER, MinLevel INTEGER, Flags INTEGER, RewardNextQuest INTEGER, PrevQuestId INTEGER, Type INTEGER); INSERT INTO quest_template VALUES (42, 'A Test Quest', 10, 5, 4096, 43, 41, 1)"); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(Config{Enabled: true, WorldDatabase: db})
	if err := runtime.LoadString(`
		quest = GetQuest(42)
		assert(quest:GetId() == 42 and quest:GetLevel() == 10 and quest:GetMinLevel() == 5)
		assert(quest:GetFlags() == 4096 and quest:HasFlag(4096))
		assert(quest:IsDaily() and quest:IsRepeatable())
		assert(quest:GetNextQuestId() == 43 and quest:GetPrevQuestId() == 41 and quest:GetType() == 1)
		assert(GetQuest(999) == nil)
	`); err != nil {
		t.Fatal(err)
	}
}

func TestElunaGuildLookup(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE guild (guildid INTEGER PRIMARY KEY, name TEXT, leaderguid INTEGER); CREATE TABLE guild_member (guildid INTEGER, guid INTEGER); INSERT INTO guild VALUES (7, 'Moreno Land', 99); INSERT INTO guild_member VALUES (7, 99), (7, 100)"); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(Config{Enabled: true, CharacterDB: db})
	if err := runtime.LoadString(`
		guild = GetGuildByName("Moreno Land")
		assert(guild:GetId() == 7 and guild:GetName() == "Moreno Land")
		assert(GetGUIDLow(guild:GetLeaderGUID()) == 99 and guild:GetMemberCount() == 2)
		assert(GetGuildByLeaderGUID(CreateUint64(99)):GetId() == 7)
		assert(GetGuildByName("Missing") == nil)
	`); err != nil {
		t.Fatal(err)
	}
}
