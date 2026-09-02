package world

import (
	"context"
)

// handleChannelPassword processes CMSG_CHANNEL_PASSWORD (0x09C).
func (s *session) handleChannelPassword(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelSetOwner processes CMSG_CHANNEL_SET_OWNER (0x09D).
func (s *session) handleChannelSetOwner(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelOwner processes CMSG_CHANNEL_OWNER (0x09E).
func (s *session) handleChannelOwner(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelModerator processes CMSG_CHANNEL_MODERATOR (0x09F).
func (s *session) handleChannelModerator(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelUnmoderator processes CMSG_CHANNEL_UNMODERATOR (0x0A0).
func (s *session) handleChannelUnmoderator(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelMute processes CMSG_CHANNEL_MUTE (0x0A1).
func (s *session) handleChannelMute(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelUnmute processes CMSG_CHANNEL_UNMUTE (0x0A2).
func (s *session) handleChannelUnmute(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelInvite processes CMSG_CHANNEL_INVITE (0x0A3).
func (s *session) handleChannelInvite(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelKick processes CMSG_CHANNEL_KICK (0x0A4).
func (s *session) handleChannelKick(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelBan processes CMSG_CHANNEL_BAN (0x0A5).
func (s *session) handleChannelBan(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelUnban processes CMSG_CHANNEL_UNBAN (0x0A6).
func (s *session) handleChannelUnban(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelAnnouncements processes CMSG_CHANNEL_ANNOUNCEMENTS (0x0A7).
func (s *session) handleChannelAnnouncements(ctx context.Context, payload []byte) bool {
	return true
}

// handleChannelVoiceOn processes CMSG_CHANNEL_VOICE_ON (0x3D6).
func (s *session) handleChannelVoiceOn(ctx context.Context, payload []byte) bool {
	return true
}

// handleDeclineChannelInvite processes CMSG_DECLINE_CHANNEL_INVITE (0x264).
func (s *session) handleDeclineChannelInvite(ctx context.Context, payload []byte) bool {
	return true
}
