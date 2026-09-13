package scripting

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/version"
	"github.com/Shopify/go-lua"
)

const (
	guidHighItem        uint16 = 0x4000
	guidHighPlayer      uint16 = 0x0000
	guidHighGameObject  uint16 = 0xF110
	guidHighTransport   uint16 = 0xF120
	guidHighUnit        uint16 = 0xF130
	guidHighPet         uint16 = 0xF140
	guidHighVehicle     uint16 = 0xF150
	guidHighDynamic     uint16 = 0xF100
	guidHighCorpse      uint16 = 0xF101
	guidHighMOTransport uint16 = 0x1FC0
	guidHighInstance    uint16 = 0x1F40
	guidHighGroup       uint16 = 0x1F50
)

const uint64MetaTable = "MorenoCore.UInt64"

type UInt64 uint64

func (r *Runtime) getLuaEngine(state *lua.State) int {
	state.PushString("ElunaEngine")
	return 1
}

func (r *Runtime) getCoreName(state *lua.State) int {
	name := r.config.CoreName
	if name == "" {
		name = version.Product
	}
	state.PushString(name)
	return 1
}

func (r *Runtime) getRealmID(state *lua.State) int {
	realmID := r.config.RealmID
	if realmID == 0 {
		realmID = 1
	}
	state.PushUnsigned(uint(realmID))
	return 1
}

func (r *Runtime) getCoreVersion(state *lua.State) int {
	value := r.config.CoreVersion
	if value == "" {
		value = version.String()
	}
	state.PushString(value)
	return 1
}

func (r *Runtime) getPlayerByGUID(state *lua.State) int {
	guid := checkLuaUint64(state, 1)
	if r.config.PlayerProvider != nil {
		for _, player := range r.config.PlayerProvider() {
			if value, ok := objectUint64(player, "GUID"); ok && value == guid {
				PushObject(state, player)
				return 1
			}
		}
	}
	state.PushNil()
	return 1
}

func (r *Runtime) getPlayerCount(state *lua.State) int {
	count := 0
	if r.config.PlayerProvider != nil {
		for _, player := range r.config.PlayerProvider() {
			if player != nil {
				count++
			}
		}
	}
	state.PushUnsigned(uint(count))
	return 1
}

func (r *Runtime) getPlayerGUID(state *lua.State) int {
	pushUInt64(state, makeGlobalGUID(guidHighPlayer, uint32(checkLuaUint64(state, 1)), 0))
	return 1
}

func (r *Runtime) getItemGUID(state *lua.State) int {
	pushUInt64(state, makeGlobalGUID(guidHighItem, uint32(checkLuaUint64(state, 1)), 0))
	return 1
}

func (r *Runtime) getObjectGUID(state *lua.State) int {
	pushUInt64(state, makeMapGUID(guidHighGameObject, uint32(checkLuaUint64(state, 1)), uint32(checkLuaUint64(state, 2))))
	return 1
}

func (r *Runtime) getUnitGUID(state *lua.State) int {
	pushUInt64(state, makeMapGUID(guidHighUnit, uint32(checkLuaUint64(state, 1)), uint32(checkLuaUint64(state, 2))))
	return 1
}

func (r *Runtime) getGUIDLow(state *lua.State) int {
	guid := checkLuaUint64(state, 1)
	state.PushUnsigned(uint(guidLow(guid)))
	return 1
}

func (r *Runtime) getGUIDType(state *lua.State) int {
	guid := checkLuaUint64(state, 1)
	state.PushUnsigned(uint(guid >> 48))
	return 1
}

func (r *Runtime) getGUIDEntry(state *lua.State) int {
	guid := checkLuaUint64(state, 1)
	if guidHasEntry(uint16(guid >> 48)) {
		state.PushUnsigned(uint((guid >> 24) & 0x00FFFFFF))
		return 1
	}
	state.PushUnsigned(0)
	return 1
}

func (r *Runtime) getCurrTime(state *lua.State) int {
	state.PushUnsigned(uint(uint32(time.Now().UnixMilli())))
	return 1
}

func (r *Runtime) getTimeDiff(state *lua.State) int {
	old := uint32(lua.CheckUnsigned(state, 1))
	now := uint32(time.Now().UnixMilli())
	state.PushUnsigned(uint(now - old))
	return 1
}

func (r *Runtime) printInfo(state *lua.State) int {
	r.printLua(state, "info")
	return 0
}

func (r *Runtime) printError(state *lua.State) int {
	r.printLua(state, "error")
	return 0
}

func (r *Runtime) printDebug(state *lua.State) int {
	r.printLua(state, "debug")
	return 0
}

func (r *Runtime) printLua(state *lua.State, level string) {
	if r.config.Logger == nil {
		return
	}
	parts := make([]string, 0, state.Top())
	for index := 1; index <= state.Top(); index++ {
		parts = append(parts, fmt.Sprint(luaValue(state, index)))
	}
	r.config.Logger.Log(context.Background(), slogLevel(level), strings.Join(parts, "\t"))
}

func installUInt64MetaTable(state *lua.State) {
	lua.NewMetaTable(state, uint64MetaTable)
	lua.SetFunctions(state, []lua.RegistryFunction{
		{Name: "__tostring", Function: uint64String},
		{Name: "__eq", Function: uint64Equal},
		{Name: "__lt", Function: uint64Less},
		{Name: "__le", Function: uint64LessEqual},
		{Name: "__add", Function: uint64Add},
		{Name: "__sub", Function: uint64Sub},
	}, 0)
	state.Pop(1)
}

func pushUInt64(state *lua.State, value uint64) {
	state.PushUserData(UInt64(value))
	lua.SetMetaTableNamed(state, uint64MetaTable)
}

func checkLuaUint64(state *lua.State, index int) uint64 {
	if value, ok := luaUint64Value(state, index); ok {
		return value
	}
	lua.ArgumentError(state, index, "unsigned integer expected")
	return 0
}

func luaUint64Value(state *lua.State, index int) (uint64, bool) {
	if value := state.ToUserData(index); value != nil {
		switch value := value.(type) {
		case UInt64:
			return uint64(value), true
		case *UInt64:
			return uint64(*value), true
		}
	}
	if number, ok := state.ToNumber(index); ok && number >= 0 && number <= math.MaxUint64 && number == math.Trunc(number) {
		return uint64(number), true
	}
	return 0, false
}

func uint64Operand(state *lua.State, index int) uint64 { return checkLuaUint64(state, index) }

func uint64String(state *lua.State) int {
	state.PushString(strconv.FormatUint(uint64Operand(state, 1), 10))
	return 1
}

func uint64Equal(state *lua.State) int {
	state.PushBoolean(uint64Operand(state, 1) == uint64Operand(state, 2))
	return 1
}

func uint64Less(state *lua.State) int {
	state.PushBoolean(uint64Operand(state, 1) < uint64Operand(state, 2))
	return 1
}

func uint64LessEqual(state *lua.State) int {
	state.PushBoolean(uint64Operand(state, 1) <= uint64Operand(state, 2))
	return 1
}

func uint64Add(state *lua.State) int {
	pushUInt64(state, uint64Operand(state, 1)+uint64Operand(state, 2))
	return 1
}

func uint64Sub(state *lua.State) int {
	pushUInt64(state, uint64Operand(state, 1)-uint64Operand(state, 2))
	return 1
}

func luaBitAnd(state *lua.State) int {
	state.PushUnsigned(uint(lua.CheckUnsigned(state, 1) & lua.CheckUnsigned(state, 2)))
	return 1
}
func luaBitOr(state *lua.State) int {
	state.PushUnsigned(uint(lua.CheckUnsigned(state, 1) | lua.CheckUnsigned(state, 2)))
	return 1
}
func luaBitLShift(state *lua.State) int {
	state.PushUnsigned(uint(uint32(lua.CheckUnsigned(state, 1)) << uint32(lua.CheckUnsigned(state, 2))))
	return 1
}
func luaBitRShift(state *lua.State) int {
	state.PushUnsigned(uint(uint32(lua.CheckUnsigned(state, 1)) >> uint32(lua.CheckUnsigned(state, 2))))
	return 1
}
func luaBitXor(state *lua.State) int {
	state.PushUnsigned(uint(lua.CheckUnsigned(state, 1) ^ lua.CheckUnsigned(state, 2)))
	return 1
}
func luaBitNot(state *lua.State) int {
	state.PushUnsigned(uint(^uint32(lua.CheckUnsigned(state, 1))))
	return 1
}

func objectUint64(object *Object, field string) (uint64, bool) {
	if object == nil {
		return 0, false
	}
	value, ok := object.Fields[field]
	if !ok || value == nil {
		return 0, false
	}
	switch value := value.(type) {
	case UInt64:
		return uint64(value), true
	case uint64:
		return value, true
	case uint32:
		return uint64(value), true
	case uint16:
		return uint64(value), true
	case uint8:
		return uint64(value), true
	case uint:
		return uint64(value), true
	case int:
		if value >= 0 {
			return uint64(value), true
		}
	case int8:
		if value >= 0 {
			return uint64(value), true
		}
	case int16:
		if value >= 0 {
			return uint64(value), true
		}
	case int32:
		if value >= 0 {
			return uint64(value), true
		}
	case int64:
		if value >= 0 {
			return uint64(value), true
		}
	case float64:
		if value >= 0 && value <= math.MaxUint64 && value == math.Trunc(value) {
			return uint64(value), true
		}
	}
	return 0, false
}

func objectUint32(object *Object, field string) (uint32, bool) {
	value, ok := objectUint64(object, field)
	return uint32(value), ok && value <= math.MaxUint32
}

func makeGlobalGUID(high uint16, low, _ uint32) uint64 {
	if low == 0 {
		return 0
	}
	return uint64(low) | uint64(high)<<48
}

func makeMapGUID(high uint16, low, entry uint32) uint64 {
	if low == 0 {
		return 0
	}
	return uint64(low&0x00FFFFFF) | uint64(entry&0x00FFFFFF)<<24 | uint64(high)<<48
}

func guidHasEntry(high uint16) bool {
	switch high {
	case guidHighGameObject, guidHighTransport, guidHighUnit, guidHighPet, guidHighVehicle:
		return true
	default:
		return false
	}
}

func guidLow(guid uint64) uint32 {
	if guidHasEntry(uint16(guid >> 48)) {
		return uint32(guid & 0x00FFFFFF)
	}
	return uint32(guid)
}

func slogLevel(level string) slog.Level {
	switch level {
	case "error":
		return slog.LevelError
	case "debug":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}
