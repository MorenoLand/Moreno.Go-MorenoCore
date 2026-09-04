package world

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// CreatureText matches the schema of world database `creature_text`.
// Reference: TrinityCore CreatureTextMgr.h / CreatureTextMgr.cpp.
type CreatureText struct {
	CreatureID  uint32
	GroupID     uint8
	ID          uint8
	Text        string
	Type        uint8
	Language    uint8
	Probability float32
	Emote       uint32
	Duration    uint32
	Sound       uint32
}

type creatureTextMgr struct {
	mu    sync.RWMutex
	texts map[uint32]map[uint8][]CreatureText
}

func newCreatureTextMgr() *creatureTextMgr {
	return &creatureTextMgr{
		texts: make(map[uint32]map[uint8][]CreatureText),
	}
}

func (s *Server) loadCreatureTexts(ctx context.Context, creatureID uint32) map[uint8][]CreatureText {
	if s == nil || s.WorldStore == nil || s.WorldStore.DB == nil || creatureID == 0 {
		return nil
	}
	rows, err := s.WorldStore.DB.QueryContext(ctx, "SELECT CreatureID, GroupID, ID, Text, Type, Language, Probability, Emote, Duration, Sound FROM creature_text WHERE CreatureID = ? ORDER BY GroupID, ID", creatureID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	groups := make(map[uint8][]CreatureText)
	for rows.Next() {
		var t CreatureText
		var p float64
		if err := rows.Scan(&t.CreatureID, &t.GroupID, &t.ID, &t.Text, &t.Type, &t.Language, &p, &t.Emote, &t.Duration, &t.Sound); err == nil {
			t.Probability = float32(p)
			groups[t.GroupID] = append(groups[t.GroupID], t)
		}
	}
	return groups
}

// broadcastCreatureTalk emits sound, emote, and chat message matching TrinityCore CreatureTextMgr::SendChat.
func (s *Server) broadcastCreatureTalk(ctx context.Context, mapID uint32, creatureGUID uint64, entry uint32, creatureName string, groupID uint8, whisperTarget uint64) {
	if s == nil || entry == 0 {
		return
	}
	if s.creatureTextMgr == nil {
		s.creatureTextMgr = newCreatureTextMgr()
	}

	s.creatureTextMgr.mu.RLock()
	groups, found := s.creatureTextMgr.texts[entry]
	s.creatureTextMgr.mu.RUnlock()

	if !found {
		loaded := s.loadCreatureTexts(ctx, entry)
		s.creatureTextMgr.mu.Lock()
		if s.creatureTextMgr.texts == nil {
			s.creatureTextMgr.texts = make(map[uint32]map[uint8][]CreatureText)
		}
		s.creatureTextMgr.texts[entry] = loaded
		groups = loaded
		s.creatureTextMgr.mu.Unlock()
	}

	if groups == nil {
		return
	}
	list := groups[groupID]
	if len(list) == 0 {
		return
	}

	// Select text by probability or random element
	chosen := list[0]
	if len(list) > 1 {
		chosen = list[rand.Intn(len(list))]
	}

	msg := chosen.Text
	if strings.Contains(msg, "%s") {
		msg = fmt.Sprintf(msg, creatureName)
	}

	// 1. Play Sound
	if chosen.Sound > 0 {
		soundPkt := protocol.NewBuffer(4)
		soundPkt.WriteU32(chosen.Sound)
		s.broadcastToNearby(uint16(protocol.OpcodeSMSG_PLAY_SOUND), soundPkt.Bytes(), nil)
	}

	// 2. Play Emote
	if chosen.Emote > 0 {
		emotePkt := protocol.NewBuffer(12)
		emotePkt.WriteU32(chosen.Emote)
		emotePkt.WriteU64(creatureGUID)
		s.broadcastToNearby(uint16(protocol.OpcodeSMSG_EMOTE), emotePkt.Bytes(), nil)
	}

	// 3. Broadcast chat message
	chatType := chosen.Type
	if chatType == 0 {
		chatType = 12 // CHAT_MSG_MONSTER_SAY
	}
	chatPkt := protocol.BuildMonsterChatMessage(chatType, uint32(chosen.Language), creatureGUID, creatureName, msg)
	s.broadcastToNearby(uint16(protocol.OpcodeSMSG_MESSAGECHAT), chatPkt, nil)
}
