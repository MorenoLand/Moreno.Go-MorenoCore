package world

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	logoutDelay       = 20 * time.Second
	playerFlagResting = uint32(0x00000020)
)

func (s *session) handleLogoutRequest(ctx context.Context) bool {
	if !s.playerLoaded {
		return true
	}
	instant := s.player != nil && s.player.PlayerFlags&playerFlagResting != 0
	response := protocol.NewBuffer(5)
	response.WriteU32(0)
	if instant {
		response.WriteU8(1)
	} else {
		response.WriteU8(0)
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_LOGOUT_RESPONSE), response.Bytes(), true); err != nil {
		return false
	}
	if instant {
		if err := s.completeLogout(ctx); err != nil {
			s.debug("player logout failed", "account", s.accountName, "error", err)
		}
		return true
	}
	s.logoutAt = time.Now().Add(logoutDelay)
	s.debug("player logout pending", "account", s.accountName, "delay_seconds", int(logoutDelay/time.Second))
	return true
}

func (s *session) handleLogoutCancel() bool {
	if !s.playerLoaded {
		return true
	}
	s.logoutAt = time.Time{}
	s.debug("player logout cancelled", "account", s.accountName)
	return s.write(uint16(protocol.OpcodeSMSG_LOGOUT_CANCEL_ACK), nil, true) == nil
}

func (s *session) completeLogout(ctx context.Context) error {
	if !s.playerLoaded {
		return nil
	}
	s.triggerLogout(ctx)
	var firstErr error
	if err := s.savePlayerPosition(ctx); err != nil {
		firstErr = err
	}
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_ACCOUNT_ONLINE", s.accountID); err != nil && firstErr == nil {
		firstErr = err
	}
	if _, err := s.server.AuthStore.DB.ExecContext(ctx, "UPDATE account SET online = 0 WHERE id = ?", s.accountID); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_LOGOUT_COMPLETE), nil, true); err != nil {
		return err
	}
	s.playerLoaded = false
	s.player = nil
	s.logoutAt = time.Time{}
	s.debug("player logged out", "account", s.accountName, "guid", s.playerGUID)
	return firstErr
}

func (s *session) triggerLogout(ctx context.Context) {
	if !s.playerLoaded || s.logoutHook {
		return
	}
	s.logoutHook = true
	if s.server.Features != nil && s.server.Features.LFG != nil {
		s.server.Features.LFG.Leave(s.playerGUID)
	}
	s.triggerPlayerEvent(ctx, scripting.PlayerEventLogout, s.luaPlayer())
}

func (s *session) savePlayerPosition(ctx context.Context) error {
	if !s.playerLoaded || s.player == nil {
		return nil
	}
	_, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_CHARACTER_POSITION", s.player.X, s.player.Y, s.player.Z, s.player.Orientation, s.player.Map, s.player.Zone, s.player.GUID)
	return err
}

func isReadTimeout(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}
