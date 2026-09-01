package world

import (
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const characterAccountDataMask uint32 = 0xEA

func buildLoginVerifyWorld(state playerState) []byte {
	packet := protocol.NewBuffer(20)
	packet.WriteI32(int32(state.Map))
	packet.WriteF32(state.X)
	packet.WriteF32(state.Y)
	packet.WriteF32(state.Z)
	packet.WriteF32(state.Orientation)
	return packet.Bytes()
}

func buildAccountDataTimes(now time.Time) []byte {
	packet := protocol.NewBuffer(29)
	packet.WriteU32(uint32(now.Unix()))
	packet.WriteU8(1)
	packet.WriteU32(characterAccountDataMask)
	for index := uint32(0); index < 8; index++ {
		if characterAccountDataMask&(1<<index) != 0 {
			packet.WriteU32(0)
		}
	}
	return packet.Bytes()
}

func buildFeatureSystemStatus() []byte { return []byte{2, 0} }

func buildMotd(message string) []byte {
	lines := strings.Split(message, "@")
	packet := protocol.NewBuffer(4 + len(message) + len(lines))
	packet.WriteU32(uint32(len(lines)))
	for _, line := range lines {
		packet.WriteCString(line)
	}
	return packet.Bytes()
}

func buildLearnedDanceMoves() []byte {
	packet := protocol.NewBuffer(8)
	packet.WriteU32(0)
	packet.WriteU32(0)
	return packet.Bytes()
}
