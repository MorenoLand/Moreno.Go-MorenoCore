package world

import "context"

const unitNPCFlagVendor uint32 = 0x00000080

func (s *session) canInteractWithNPC(ctx context.Context, guid, requiredFlags uint64) bool {
	if s == nil || s.player == nil || s.server == nil || s.server.WorldStore == nil || s.server.WorldStore.DB == nil || uint16(guid>>48) != 0xF130 {
		return false
	}
	low := uint32(guid & 0x00FFFFFF)
	entry := uint32((guid >> 24) & 0x00FFFFFF)
	var mapID, npcFlags int64
	var x, y, z float32
	err := s.server.WorldStore.DB.QueryRowContext(ctx, `SELECT c.map, c.position_x, c.position_y, c.position_z, t.npcflag
		FROM creature AS c JOIN creature_template AS t ON t.entry = c.id WHERE c.guid = ? AND c.id = ?`, low, entry).Scan(&mapID, &x, &y, &z, &npcFlags)
	if err != nil {
		if missingTable(err) || isMissingColumn(err) {
			return true
		}
		return false
	}
	if uint32(mapID) != s.player.Map || distance3D(s.player.X, s.player.Y, s.player.Z, x, y, z) > 5.0 {
		return false
	}
	return requiredFlags == 0 || uint32(npcFlags)&uint32(requiredFlags) != 0
}
