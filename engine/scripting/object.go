package scripting

import (
	"context"
	"fmt"

	"github.com/Shopify/go-lua"
)

const objectMetaTable = "MorenoCore.Object"

func PushObject(state *lua.State, object *Object) {
	state.PushUserData(object)
	lua.SetMetaTableNamed(state, objectMetaTable)
}

func objectIndex(state *lua.State) int {
	object := lua.CheckUserData(state, 1, objectMetaTable).(*Object)
	name := lua.CheckString(state, 2)
	if method, ok := object.Methods[name]; ok {
		state.PushGoFunction(func(call *lua.State) int {
			args := make([]any, 0, call.Top()-1)
			for index := 2; index <= call.Top(); index++ {
				args = append(args, luaValue(call, index))
			}
			values, err := method(context.Background(), args)
			if err != nil {
				lua.Errorf(call, "%s: %s", name, err)
			}
			for _, value := range values {
				pushValue(call, value)
			}
			return len(values)
		})
		return 1
	}
	if value, ok := object.Fields[name]; ok {
		pushValue(state, value)
		return 1
	}
	state.PushNil()
	return 1
}

func pushValue(state *lua.State, value any) error {
	switch value := value.(type) {
	case nil:
		state.PushNil()
	case bool:
		state.PushBoolean(value)
	case string:
		state.PushString(value)
	case []byte:
		state.PushString(string(value))
	case int:
		state.PushInteger(value)
	case int8:
		state.PushInteger(int(value))
	case int16:
		state.PushInteger(int(value))
	case int32:
		state.PushInteger(int(value))
	case int64:
		state.PushNumber(float64(value))
	case uint:
		state.PushUnsigned(value)
	case uint8:
		state.PushUnsigned(uint(value))
	case uint16:
		state.PushUnsigned(uint(value))
	case uint32:
		state.PushUnsigned(uint(value))
	case uint64:
		state.PushNumber(float64(value))
	case float32:
		state.PushNumber(float64(value))
	case float64:
		state.PushNumber(value)
	case *Query:
		pushQuery(state, value)
	case *Object:
		PushObject(state, value)
	default:
		return fmt.Errorf("unsupported Lua value %T", value)
	}
	return nil
}

func luaValue(state *lua.State, index int) any {
	if state.IsNil(index) {
		return nil
	}
	if state.IsBoolean(index) {
		return state.ToBoolean(index)
	}
	if state.IsNumber(index) {
		value, _ := state.ToNumber(index)
		return value
	}
	if state.IsString(index) {
		value, _ := state.ToString(index)
		return value
	}
	if value := state.ToUserData(index); value != nil {
		return value
	}
	return state.ToValue(index)
}
