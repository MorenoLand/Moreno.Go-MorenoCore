package world

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	unitNPCFlagFlightmaster uint32 = 0x00002000
	playerExtraTaxiCheat     uint32 = 0x00000008
	taxiMaskSize                   = 8
)

// TrinityCore Player.h extra flags and transient PLAYER_FLAGS bits.
const (
	playerExtraGMOn          uint32 = 0x00000001
	playerExtraGMInvisible   uint32 = 0x00000010
	playerExtraGMChat        uint32 = 0x00000020
	playerFlagAFK              uint32 = 0x00000002
	playerFlagDND              uint32 = 0x00000004
	playerFlagGM               uint32 = 0x00000008
	playerFlagAllowOnlyAbility uint32 = 0x00000001
)

// setTaxiMaskNode mirrors PlayerTaxi::SetTaximaskNode; returns true when the
// bit transitioned from unknown to known.
func (s *session) setTaxiMaskNode(node uint32) bool {
	if s.player == nil || node == 0 {
		return false
	}
	field := (node - 1) / 32
	if field >= taxiMaskSize {
		return false
	}
	bit := uint32(1) << ((node - 1) % 32)
	if s.player.TaxiMask[field]&bit != 0 {
		return false
	}
	s.player.TaxiMask[field] |= bit
	return true
}

func (s *session) isTaxiMaskNodeKnown(node uint32) bool {
	if s.player == nil || node == 0 {
		return false
	}
	field := (node - 1) / 32
	if field >= taxiMaskSize {
		return false
	}
	return s.player.TaxiMask[field]&(1<<((node-1)%32)) != 0
}

// isTaxiCheater mirrors Player::isTaxiCheater (PLAYER_EXTRA_TAXICHEAT).
func (s *session) isTaxiCheater() bool {
	return s.player != nil && s.player.ExtraFlags&playerExtraTaxiCheat != 0
}

// saveTaxiMask persists the mask in TrinityCore's space separated text form.
func (s *session) saveTaxiMask(ctx context.Context) {
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil || s.player == nil {
		return
	}
	parts := make([]string, taxiMaskSize)
	for i, v := range s.player.TaxiMask {
		parts[i] = strconv.FormatUint(uint64(v), 10)
	}
	_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET taximask = ? WHERE guid = ?", strings.Join(parts, " "), s.playerGUID)
}

// loadTaxiMask mirrors PlayerTaxi::LoadTaxiMask (space separated words).
func (s *session) loadTaxiMask(raw sql.NullString) {
	for i := range s.player.TaxiMask {
		s.player.TaxiMask[i] = 0
	}
	if !raw.Valid {
		return
	}
	tokens := strings.Fields(raw.String)
	for i := 0; i < taxiMaskSize && i < len(tokens); i++ {
		if v, err := strconv.ParseUint(tokens[i], 10, 32); err == nil {
			s.player.TaxiMask[i] = uint32(v)
		}
	}
}

// nearestCreatureTaxiNode finds the flight node nearest to the creature, as
// ObjectMgr::GetNearestTaxiNode does using the unit's world position.
func (s *session) nearestCreatureTaxiNode(ctx context.Context, guid uint64) (uint32, bool) {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return 0, false
	}
	creature := s.luaCreature(ctx, guid)
	if creature == nil {
		return 0, false
	}
	if objectUint32OrZero(creature, "NPCFlags")&unitNPCFlagFlightmaster == 0 {
		return 0, false
	}
	mapID := objectUint32OrZero(creature, "Map")
	x, okX := objectFloat32Field(creature, "X")
	y, okY := objectFloat32Field(creature, "Y")
	z, okZ := objectFloat32Field(creature, "Z")
	if !okX || !okY || !okZ || s.server.Data == nil {
		return 0, false
	}
	node, err := s.server.Data.NearestTaxiNode(x, y, z, mapID, s.playerAlliance())
	if err != nil {
		return 0, false
	}
	return node, node != 0
}

// playerAlliance reports the player team like Player::GetTeam.
func (s *session) playerAlliance() bool {
	if s.player == nil {
		return false
	}
	switch s.player.Race {
	case 1, 3, 4, 7, 11: // Human, Dwarf, NightElf, Gnome, Draenei
		return true
	}
	return false
}

func (s *session) handleTaxiNodeStatusQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	// TrinityCore SendTaxiStatus: ignore non-flightmasters and units with no
	// resolvable node instead of answering "known" for everything.
	node, ok := s.nearestCreatureTaxiNode(ctx, guid)
	if !ok {
		return true
	}
	known := uint8(0)
	if s.isTaxiMaskNodeKnown(node) || s.isTaxiCheater() {
		known = 1
	}
	packet := protocol.NewBuffer(9)
	packet.WriteU64(guid)
	packet.WriteU8(known)
	_ = s.write(uint16(protocol.OpcodeSMSG_TAXINODE_STATUS), packet.Bytes(), true)
	return true
}

func (s *session) handleTaxiQueryAvailableNodes(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	// HandleTaxiQueryAvailableNodes: learn unknown node, otherwise show the
	// flight map for known nodes.
	if s.learnNewTaxiNode(ctx, guid) {
		return true
	}
	return s.sendTaxiMenu(ctx, guid)
}

// learnNewTaxiNode mirrors WorldSession::SendLearnNewTaxiNode: discovers the
// nearest node, stores it and notifies the client; true when learned.
func (s *session) learnNewTaxiNode(ctx context.Context, guid uint64) bool {
	node, ok := s.nearestCreatureTaxiNode(ctx, guid)
	if !ok {
		return true // avoid a second failing node search in SendTaxiMenu
	}
	if !s.isTaxiMaskNodeKnown(node) && s.setTaxiMaskNode(node) {
		s.saveTaxiMask(ctx)
		_ = s.write(uint16(protocol.OpcodeSMSG_NEW_TAXI_PATH), nil, true)
		status := protocol.NewBuffer(9)
		status.WriteU64(guid)
		status.WriteU8(1)
		_ = s.write(uint16(protocol.OpcodeSMSG_TAXINODE_STATUS), status.Bytes(), true)
		return true
	}
	return false
}

func (s *session) sendTaxiMenu(ctx context.Context, flightMasterGUID uint64) bool {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return true
	}
	creature := s.luaCreature(ctx, flightMasterGUID)
	if creature == nil {
		return true
	}
	if objectUint32OrZero(creature, "NPCFlags")&unitNPCFlagFlightmaster == 0 {
		return true
	}
	mapID := objectUint32OrZero(creature, "Map")
	x, _ := objectFloat32Field(creature, "X")
	y, _ := objectFloat32Field(creature, "Y")
	z, _ := objectFloat32Field(creature, "Z")
	curNode := uint32(0)
	if s.server.Data != nil {
		if node, err := s.server.Data.NearestTaxiNode(x, y, z, mapID, s.playerAlliance()); err == nil {
			curNode = node
		}
	}
	if curNode == 0 {
		return true
	}
	// SendTaxiMenu never announces SMSG_NEW_TAXI_PATH; the map carries the
	// player's known nodes (or the whole network for taxi cheaters).
	packet := protocol.NewBuffer(4 + 8 + 4 + 8*taxiMaskSize)
	packet.WriteU32(1)
	packet.WriteU64(flightMasterGUID)
	packet.WriteU32(curNode)
	cheater := s.isTaxiCheater()
	networkMask := [taxiMaskSize]uint32{}
	if s.server.Data != nil {
		networkMask, _ = s.server.Data.TaxiNetworkMask()
	}
	for i := 0; i < taxiMaskSize; i++ {
		if cheater {
			packet.WriteU32(networkMask[i])
		} else {
			packet.WriteU32(s.player.TaxiMask[i])
		}
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_SHOWTAXINODES), packet.Bytes(), true)
	s.debug("taxi menu sent", "account", s.accountName, "master", flightMasterGUID, "node", curNode)
	return true
}

func (s *session) handleActivateTaxi(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 16 {
		return true
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	sourceNode, err := reader.ReadU32()
	if err != nil {
		return false
	}
	destNode, err := reader.ReadU32()
	if err != nil {
		return false
	}
	packet := protocol.NewBuffer(4)
	packet.WriteU32(0) // ERR_TAXIOK = 0
	_ = s.write(uint16(protocol.OpcodeSMSG_ACTIVATETAXIREPLY), packet.Bytes(), true)
	s.debug("taxi flight activated", "account", s.accountName, "master", guid, "source", sourceNode, "dest", destNode)
	return true
}

// initTaxiNodesForLevel mirrors PlayerTaxi::InitTaxiNodesForLevel for the
// race/team starting nodes available without discovering them first.
func (s *session) initTaxiNodesForLevel() {
	if s.player == nil {
		return
	}
	// Race specific initial known nodes: capital and taxi hub.
	switch s.player.Race {
	case 1: // Human
		s.setTaxiMaskNode(2)
	case 3: // Dwarf
		s.setTaxiMaskNode(6)
	case 4: // Night Elf
		s.setTaxiMaskNode(26)
		s.setTaxiMaskNode(27)
	case 7: // Gnome
		s.setTaxiMaskNode(6)
	case 11: // Draenei
		s.setTaxiMaskNode(94)
	case 2: // Orc
		s.setTaxiMaskNode(23)
	case 5: // Undead
		s.setTaxiMaskNode(11)
	case 6: // Tauren
		s.setTaxiMaskNode(22)
	case 8: // Troll
		s.setTaxiMaskNode(23)
	case 10: // Blood Elf
		s.setTaxiMaskNode(82)
	}
	// New continent starting node (Dalaran area by team).
	if s.playerAlliance() {
		s.setTaxiMaskNode(100)
	} else {
		s.setTaxiMaskNode(99)
	}
}

// objectFloat32Field reads a float object field set by luaCreature.
func objectFloat32Field(object *scripting.Object, name string) (float32, bool) {
	value, ok := object.Fields[name]
	if !ok {
		return 0, false
	}
	switch value := value.(type) {
	case float32:
		return value, true
	case float64:
		return float32(value), true
	case uint32:
		return float32(value), true
	case int64:
		return float32(value), true
	default:
		return 0, false
	}
}
