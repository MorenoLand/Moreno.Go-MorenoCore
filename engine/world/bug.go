package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleBug processes CMSG_BUG (0x1CA).
// Reference: WorldSession::HandleBugOpcode (MiscHandler.cpp:551).
func (s *session) handleBug(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	suggestion, err := r.ReadU32()
	if err != nil {
		return false
	}
	contentLen, err := r.ReadU32()
	if err != nil {
		return false
	}
	content, err := r.ReadString(int(contentLen))
	if err != nil {
		return false
	}
	typeLen, err := r.ReadU32()
	if err != nil {
		return false
	}
	typeStr, err := r.ReadString(int(typeLen))
	if err != nil {
		return false
	}

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		// CHAR_INS_BUG_REPORT: INSERT INTO bugreport (type, content) VALUES(?, ?)
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "INSERT INTO bugreport (type, content) VALUES (?, ?)", typeStr, content)
	}

	s.debug("bug report received", "account", s.accountName, "suggestion", suggestion, "type", typeStr, "len", len(content))
	return true
}
