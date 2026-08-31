package scripting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Shopify/go-lua"
)

const (
	PlayerEventLogin = 3
	PlayerEventChat  = 18
)

type Hook struct {
	Kind  string
	Event int
}

type ObjectMethod func(context.Context, []any) ([]any, error)

type Object struct {
	Type    string
	Methods map[string]ObjectMethod
	Fields  map[string]any
}

type Config struct {
	Enabled       bool
	ScriptPath    string
	CoreExpansion uint32
	AuthDatabase  *sql.DB
	CharacterDB   *sql.DB
	WorldDatabase *sql.DB
	Logger        *slog.Logger
}

type Runtime struct {
	config       Config
	mu           sync.Mutex
	state        *lua.State
	nextRef      int
	hooks        []registeredHook
	timers       []timer
	loadedFiles  []string
	loadFailures []error
}

type registeredHook struct {
	Hook
	ref int
}

type timer struct {
	id      int
	ref     int
	delay   int64
	repeats int
	elapsed int64
}

func NewRuntime(c Config) *Runtime {
	return &Runtime{config: c, nextRef: 3}
}

func (r *Runtime) Load(ctx context.Context) error {
	if !r.config.Enabled || strings.TrimSpace(r.config.ScriptPath) == "" {
		return nil
	}
	if _, err := os.Stat(r.config.ScriptPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.initializeLocked()
	files := make([]string, 0)
	err := filepath.WalkDir(r.config.ScriptPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".lua" || extension == ".ext" {
			if strings.HasSuffix(strings.ToLower(path), string(filepath.Separator)+"stacktraceplus"+string(filepath.Separator)+"stacktraceplus.ext") {
				return nil
			}
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool {
		iExt := strings.ToLower(filepath.Ext(files[i]))
		jExt := strings.ToLower(filepath.Ext(files[j]))
		if iExt != jExt {
			return iExt == ".ext"
		}
		return files[i] < files[j]
	})
	for _, path := range files {
		r.state.SetTop(0)
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			r.state.SetTop(0)
			r.loadFailures = append(r.loadFailures, fmt.Errorf("%s: %w", path, readErr))
			if r.config.Logger != nil {
				r.config.Logger.Error("lua script failed", "path", path, "error", readErr)
			}
			continue
		}
		loadErr := r.state.Load(strings.NewReader(string(source)), "@"+path, "")
		if loadErr == nil {
			loadErr = r.state.ProtectedCall(0, lua.MultipleReturns, 0)
		}
		if loadErr != nil {
			r.state.SetTop(0)
			r.loadFailures = append(r.loadFailures, fmt.Errorf("%s: %w", path, loadErr))
			if r.config.Logger != nil {
				r.config.Logger.Error("lua script failed", "path", path, "error", loadErr)
			}
			continue
		}
		r.state.SetTop(0)
		r.loadedFiles = append(r.loadedFiles, path)
	}
	_ = ctx
	return nil
}

func (r *Runtime) LoadString(source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.initializeLocked()
	return lua.DoString(r.state, source)
}

func (r *Runtime) LoadedFiles() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.loadedFiles...)
}

func (r *Runtime) LoadFailures() []error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]error(nil), r.loadFailures...)
}

func (r *Runtime) Hooks() []Hook {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Hook, 0, len(r.hooks))
	for _, hook := range r.hooks {
		result = append(result, hook.Hook)
	}
	return result
}

func (r *Runtime) Trigger(ctx context.Context, kind string, event int, args ...any) ([]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		return nil, nil
	}
	result := make([]any, 0)
	for _, hook := range r.hooks {
		if hook.Kind != kind || hook.Event != event {
			continue
		}
		r.state.SetTop(0)
		r.state.RawGetInt(lua.RegistryIndex, hook.ref)
		for _, arg := range args {
			if err := pushValue(r.state, arg); err != nil {
				return result, err
			}
		}
		if err := r.state.ProtectedCall(len(args), 1, 0); err != nil {
			if r.config.Logger != nil {
				r.config.Logger.Error("lua hook failed", "kind", kind, "event", event, "error", err)
			}
			continue
		}
		if r.state.Top() != 0 {
			result = append(result, luaValue(r.state, -1))
		}
	}
	_ = ctx
	return result, nil
}

func (r *Runtime) TriggerPlayerEvent(ctx context.Context, event int, args ...any) ([]any, error) {
	return r.Trigger(ctx, "player", event, args...)
}

func (r *Runtime) Tick(ctx context.Context, elapsed int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil || elapsed < 0 {
		return nil
	}
	remaining := r.timers[:0]
	for _, current := range r.timers {
		current.elapsed += elapsed
		for current.elapsed >= current.delay {
			current.elapsed -= current.delay
			r.state.SetTop(0)
			r.state.RawGetInt(lua.RegistryIndex, current.ref)
			if err := r.state.ProtectedCall(0, 0, 0); err != nil && r.config.Logger != nil {
				r.config.Logger.Error("lua timer failed", "id", current.id, "error", err)
			}
			if current.repeats > 0 {
				current.repeats--
			}
			if current.repeats == 0 {
				break
			}
		}
		if current.repeats != 0 {
			remaining = append(remaining, current)
		}
	}
	r.timers = remaining
	_ = ctx
	return nil
}

func (r *Runtime) initializeLocked() {
	if r.state != nil {
		return
	}
	r.state = lua.NewState()
	lua.OpenLibraries(r.state)
	for _, name := range []string{"Map", "Player", "Creature", "GameObject"} {
		r.state.NewTable()
		r.state.SetGlobal(name)
	}
	r.state.Register("RegisterPlayerEvent", r.registerPlayerEvent)
	r.state.Register("RegisterCreatureGossipEvent", r.registerCreatureGossipEvent)
	r.state.Register("RegisterPlayerGossipEvent", r.registerPlayerGossipEvent)
	r.state.Register("RegisterServerEvent", r.registerServerEvent)
	r.state.Register("RegisterGlobalEvent", r.registerServerEvent)
	r.state.Register("GetCoreExpansion", r.getCoreExpansion)
	r.state.Register("GetPlayersInWorld", r.getPlayersInWorld)
	r.state.Register("CreateLuaEvent", r.createLuaEvent)
	r.state.Register("RemoveEventById", r.removeEvent)
	r.state.Register("CharDBQuery", r.charDBQuery)
	r.state.Register("WorldDBQuery", r.worldDBQuery)
	r.state.Register("AuthDBQuery", r.authDBQuery)
	r.state.Register("CharDBExecute", r.charDBExecute)
	r.state.Register("WorldDBExecute", r.worldDBExecute)
	r.state.Register("AuthDBExecute", r.authDBExecute)
	lua.NewMetaTable(r.state, queryMetaTable)
	lua.SetFunctions(r.state, []lua.RegistryFunction{{Name: "__index", Function: queryIndex}}, 0)
	r.state.Pop(1)
	lua.NewMetaTable(r.state, objectMetaTable)
	lua.SetFunctions(r.state, []lua.RegistryFunction{{Name: "__index", Function: objectIndex}}, 0)
	r.state.Pop(1)
	setPackagePath(r.state, r.config.ScriptPath)
}

func (r *Runtime) getCoreExpansion(state *lua.State) int {
	state.PushUnsigned(uint(r.config.CoreExpansion))
	return 1
}

func (r *Runtime) getPlayersInWorld(state *lua.State) int {
	state.NewTable()
	return 1
}

func setPackagePath(state *lua.State, scriptPath string) {
	state.Global("package")
	if state.IsNil(-1) {
		state.Pop(1)
		return
	}
	state.PushString(filepath.Join(scriptPath, "?.lua") + ";" + filepath.Join(scriptPath, "?.ext") + ";" + filepath.Join(scriptPath, "extensions", "?.ext") + ";" + filepath.Join(scriptPath, "extensions", "?", "?.ext"))
	state.SetField(-2, "path")
	state.Pop(1)
}

func (r *Runtime) registerPlayerEvent(state *lua.State) int {
	r.registerHook(state, "player", lua.CheckInteger(state, 1), 2)
	return 0
}

func (r *Runtime) registerServerEvent(state *lua.State) int {
	r.registerHook(state, "server", lua.CheckInteger(state, 1), 2)
	return 0
}

func (r *Runtime) registerCreatureGossipEvent(state *lua.State) int {
	npcID := lua.CheckInteger(state, 1)
	event := lua.CheckInteger(state, 2)
	if !state.IsFunction(3) {
		lua.ArgumentError(state, 3, "function expected")
	}
	r.registerHook(state, "creature_gossip:"+strconv.Itoa(npcID), event, 3)
	return 0
}

func (r *Runtime) registerPlayerGossipEvent(state *lua.State) int {
	npcID := lua.CheckInteger(state, 1)
	event := lua.CheckInteger(state, 2)
	if !state.IsFunction(3) {
		lua.ArgumentError(state, 3, "function expected")
	}
	r.registerHook(state, "player_gossip:"+strconv.Itoa(npcID), event, 3)
	return 0
}

func (r *Runtime) registerHook(state *lua.State, kind string, event, functionIndex int) {
	if !state.IsFunction(functionIndex) {
		lua.ArgumentError(state, functionIndex, "function expected")
	}
	ref := r.storeFunction(state, functionIndex)
	r.hooks = append(r.hooks, registeredHook{Hook: Hook{Kind: kind, Event: event}, ref: ref})
}

func (r *Runtime) storeFunction(state *lua.State, index int) int {
	ref := r.nextRef
	r.nextRef++
	state.PushValue(index)
	state.RawSetInt(lua.RegistryIndex, ref)
	return ref
}

func (r *Runtime) createLuaEvent(state *lua.State) int {
	if !state.IsFunction(1) {
		lua.ArgumentError(state, 1, "function expected")
	}
	delay := int64(lua.CheckInteger(state, 2))
	if delay < 1 {
		delay = 1
	}
	repeats := -1
	if state.Top() >= 3 && state.IsNumber(3) {
		repeats = lua.CheckInteger(state, 3)
	}
	id := r.nextRef
	r.timers = append(r.timers, timer{id: id, ref: r.storeFunction(state, 1), delay: delay, repeats: repeats})
	state.PushInteger(id)
	return 1
}

func (r *Runtime) removeEvent(state *lua.State) int {
	id := lua.CheckInteger(state, 1)
	for index, current := range r.timers {
		if current.id == id {
			r.timers = append(r.timers[:index], r.timers[index+1:]...)
			break
		}
	}
	return 0
}

func (r *Runtime) charDBQuery(state *lua.State) int  { return r.dbQuery(state, r.config.CharacterDB) }
func (r *Runtime) worldDBQuery(state *lua.State) int { return r.dbQuery(state, r.config.WorldDatabase) }
func (r *Runtime) authDBQuery(state *lua.State) int  { return r.dbQuery(state, r.config.AuthDatabase) }
func (r *Runtime) charDBExecute(state *lua.State) int {
	return r.dbExecute(state, r.config.CharacterDB)
}
func (r *Runtime) worldDBExecute(state *lua.State) int {
	return r.dbExecute(state, r.config.WorldDatabase)
}
func (r *Runtime) authDBExecute(state *lua.State) int {
	return r.dbExecute(state, r.config.AuthDatabase)
}

func (r *Runtime) dbQuery(state *lua.State, db *sql.DB) int {
	sqlText := lua.CheckString(state, 1)
	query, err := queryDatabase(db, sqlText)
	if err != nil {
		if r.config.Logger != nil {
			r.config.Logger.Error("lua database query failed", "error", err)
		}
		state.PushNil()
		return 1
	}
	if query == nil {
		state.PushNil()
		return 1
	}
	pushQuery(state, query)
	return 1
}

func (r *Runtime) dbExecute(state *lua.State, db *sql.DB) int {
	sqlText := lua.CheckString(state, 1)
	if err := executeDatabase(db, sqlText); err != nil && r.config.Logger != nil {
		r.config.Logger.Error("lua database execute failed", "error", err)
	}
	return 0
}
