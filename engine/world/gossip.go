package world

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type gossipMenuItem struct {
	Icon       uint8
	Coded      bool
	BoxMoney   uint32
	Message    string
	BoxMessage string
	Sender     uint32
	Action     uint32
}

type gossipMenuState struct {
	SenderGUID uint64
	MenuID     uint32
	TitleID    uint32
	Items      map[uint32]gossipMenuItem
}

func (s *session) handleGossipHello(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		s.debug("gossip hello rejected", "account", s.accountName, "error", err)
		return false
	}
	creature := s.luaCreature(ctx, guid)
	if creature == nil {
		s.debug("gossip hello unknown", "account", s.accountName, "guid", guid)
		return true
	}
	entry, ok := objectUint32Field(creature, "Entry")
	if !ok {
		return true
	}
	s.gossip = nil
	s.gossipClosed = false
	if s.server.Features != nil && s.server.Features.Scripts != nil {
		if _, err := s.server.Features.Scripts.Trigger(ctx, "creature_gossip:"+strconv.FormatUint(uint64(entry), 10), 1, uint32(1), s.luaPlayer(), creature); err != nil {
			s.debug("gossip hello hook failed", "account", s.accountName, "entry", entry, "error", err)
		}
	}
	if s.gossip == nil && !s.gossipClosed {
		s.gossip = &gossipMenuState{SenderGUID: guid, Items: make(map[uint32]gossipMenuItem)}
		if err := s.sendGossipMenu(); err != nil {
			s.debug("gossip hello response failed", "account", s.accountName, "entry", entry, "error", err)
			return false
		}
	}
	s.debug("gossip hello handled", "account", s.accountName, "entry", entry)
	return true
}

func (s *session) handleGossipSelectOption(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	menuID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	listID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if s.gossip == nil || s.gossip.SenderGUID != guid || s.gossip.MenuID != menuID {
		s.debug("gossip selection rejected", "account", s.accountName, "guid", guid, "menu", menuID, "list", listID)
		return true
	}
	item, ok := s.gossip.Items[listID]
	if !ok {
		return true
	}
	code := ""
	if item.Coded {
		code, err = reader.ReadCString()
		if err != nil {
			return false
		}
	}
	creature := s.luaCreature(ctx, guid)
	if creature == nil {
		return true
	}
	entry, ok := objectUint32Field(creature, "Entry")
	if !ok {
		return true
	}
	s.gossip = nil
	s.gossipClosed = false
	args := []any{uint32(2), s.luaPlayer(), creature, item.Sender, item.Action}
	if item.Coded {
		args = append(args, code)
	}
	if s.server.Features != nil && s.server.Features.Scripts != nil {
		if _, err := s.server.Features.Scripts.Trigger(ctx, "creature_gossip:"+strconv.FormatUint(uint64(entry), 10), 2, args...); err != nil {
			s.debug("gossip selection hook failed", "account", s.accountName, "entry", entry, "error", err)
		}
	}
	if s.gossip == nil && !s.gossipClosed {
		s.gossipClosed = true
		if err := s.write(uint16(protocol.OpcodeSMSG_GOSSIP_COMPLETE), nil, true); err != nil {
			return false
		}
	}
	s.debug("gossip selection handled", "account", s.accountName, "entry", entry, "list", listID)
	return true
}

func (s *session) luaGossipComplete(_ context.Context, _ []any) ([]any, error) {
	s.gossip = nil
	s.gossipClosed = true
	return nil, s.write(uint16(protocol.OpcodeSMSG_GOSSIP_COMPLETE), nil, true)
}

func (s *session) luaGossipClearMenu(_ context.Context, _ []any) ([]any, error) {
	if s.gossip != nil {
		s.gossip.Items = make(map[uint32]gossipMenuItem)
	}
	return nil, nil
}

func (s *session) luaGossipMenuAddItem(_ context.Context, args []any) ([]any, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("GossipMenuAddItem requires icon, message, sender, and action")
	}
	icon, err := luaUint32Arg(args, 0)
	if err != nil {
		return nil, err
	}
	message, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("gossip message must be a string")
	}
	sender, err := luaUint32Arg(args, 2)
	if err != nil {
		return nil, err
	}
	action, err := luaUint32Arg(args, 3)
	if err != nil {
		return nil, err
	}
	coded := false
	if len(args) > 4 && args[4] != nil {
		var ok bool
		coded, ok = args[4].(bool)
		if !ok {
			return nil, fmt.Errorf("gossip coded flag must be a boolean")
		}
	}
	boxMessage := ""
	if len(args) > 5 && args[5] != nil {
		var ok bool
		boxMessage, ok = args[5].(string)
		if !ok {
			return nil, fmt.Errorf("gossip box message must be a string")
		}
	}
	boxMoney := uint32(0)
	if len(args) > 6 && args[6] != nil {
		boxMoney, err = luaUint32Arg(args, 6)
		if err != nil {
			return nil, err
		}
	}
	if s.gossip == nil {
		s.gossip = &gossipMenuState{Items: make(map[uint32]gossipMenuItem)}
	}
	if len(s.gossip.Items) >= 32 {
		return nil, fmt.Errorf("gossip menu item limit reached")
	}
	itemID := uint32(0)
	for {
		if _, exists := s.gossip.Items[itemID]; !exists {
			break
		}
		itemID++
	}
	s.gossip.Items[itemID] = gossipMenuItem{Icon: uint8(icon), Coded: coded, BoxMoney: boxMoney, Message: message, BoxMessage: boxMessage, Sender: sender, Action: action}
	return nil, nil
}

func (s *session) luaGossipSendMenu(_ context.Context, args []any) ([]any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("GossipSendMenu requires a title and source object")
	}
	title, err := luaUint32Arg(args, 0)
	if err != nil {
		return nil, err
	}
	object, ok := args[1].(*scripting.Object)
	if !ok {
		return nil, fmt.Errorf("gossip source must be an object")
	}
	guid, ok := objectUint64Field(object, "GUID")
	if !ok {
		return nil, fmt.Errorf("gossip source has no guid")
	}
	menuID := uint32(0)
	if len(args) > 2 && args[2] != nil {
		menuID, err = luaUint32Arg(args, 2)
		if err != nil {
			return nil, err
		}
	}
	if s.gossip == nil {
		s.gossip = &gossipMenuState{Items: make(map[uint32]gossipMenuItem)}
	}
	s.gossip.SenderGUID, s.gossip.MenuID, s.gossip.TitleID = guid, menuID, title
	s.gossipClosed = false
	return nil, s.sendGossipMenu()
}

func (s *session) sendGossipMenu() error {
	if s.gossip == nil {
		return nil
	}
	return s.write(uint16(protocol.OpcodeSMSG_GOSSIP_MESSAGE), buildGossipMessage(*s.gossip), true)
}

func buildGossipMessage(menu gossipMenuState) []byte {
	packet := protocol.NewBuffer(128)
	packet.WriteU64(menu.SenderGUID)
	packet.WriteU32(menu.MenuID)
	packet.WriteU32(menu.TitleID)
	ids := make([]uint32, 0, len(menu.Items))
	for id := range menu.Items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	packet.WriteU32(uint32(len(ids)))
	for _, id := range ids {
		item := menu.Items[id]
		packet.WriteU32(id)
		packet.WriteU8(item.Icon)
		if item.Coded {
			packet.WriteU8(1)
		} else {
			packet.WriteU8(0)
		}
		packet.WriteU32(item.BoxMoney)
		packet.WriteCString(item.Message)
		packet.WriteCString(item.BoxMessage)
	}
	packet.WriteU32(0)
	return packet.Bytes()
}

func objectUint32Field(object *scripting.Object, name string) (uint32, bool) {
	value, ok := object.Fields[name]
	if !ok {
		return 0, false
	}
	switch value := value.(type) {
	case uint8:
		return uint32(value), true
	case uint16:
		return uint32(value), true
	case uint32:
		return value, true
	case uint64:
		return uint32(value), true
	case int:
		return uint32(value), true
	case int64:
		return uint32(value), true
	case float64:
		return uint32(value), true
	default:
		return 0, false
	}
}

func objectUint64Field(object *scripting.Object, name string) (uint64, bool) {
	value, ok := object.Fields[name]
	if !ok {
		return 0, false
	}
	switch value := value.(type) {
	case uint64:
		return value, true
	case uint32:
		return uint64(value), true
	case int64:
		return uint64(value), true
	case float64:
		return uint64(value), true
	default:
		return 0, false
	}
}
