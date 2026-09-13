package scripting

import (
	"context"
	"strings"
	"testing"

	"github.com/Shopify/go-lua"
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
