package world

import (
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	globalAccountDataMask    uint32 = 0x15
	characterAccountDataMask uint32 = 0xEA
)

func buildLoginVerifyWorld(state playerState) []byte {
	packet := protocol.NewBuffer(20)
	packet.WriteI32(int32(state.Map))
	packet.WriteF32(state.X)
	packet.WriteF32(state.Y)
	packet.WriteF32(state.Z)
	packet.WriteF32(state.Orientation)
	return packet.Bytes()
}

func buildAccountDataTimes(now time.Time, mask uint32) []byte {
	packet := protocol.NewBuffer(29)
	packet.WriteU32(uint32(now.Unix()))
	packet.WriteU8(1)
	packet.WriteU32(mask)
	for index := uint32(0); index < 8; index++ {
		if mask&(1<<index) != 0 {
			packet.WriteU32(0)
		}
	}
	return packet.Bytes()
}

func buildRealmSplit(payload []byte) ([]byte, error) {
	reader := protocol.NewReader(payload)
	value, err := reader.ReadU32()
	if err != nil {
		return nil, err
	}
	packet := protocol.NewBuffer(17)
	packet.WriteU32(value)
	packet.WriteU32(0)
	packet.WriteCString("01/01/01")
	return packet.Bytes(), nil
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

func buildInitWorldStates(state playerState) []byte {
	worldStates := [][2]int32{{2264, 0}, {2263, 0}, {2262, 0}, {2261, 0}, {2260, 0}, {2259, 0}, {3191, 0}, {3901, 0}}
	packet := protocol.NewBuffer(16 + len(worldStates)*8)
	packet.WriteI32(int32(state.Map))
	packet.WriteI32(int32(state.Zone))
	packet.WriteI32(0)
	packet.WriteU16(uint16(len(worldStates)))
	for _, worldState := range worldStates {
		packet.WriteI32(worldState[0])
		packet.WriteI32(worldState[1])
	}
	return packet.Bytes()
}

func buildInstanceDifficulty() []byte {
	packet := protocol.NewBuffer(8)
	packet.WriteU32(0)
	packet.WriteU32(0)
	return packet.Bytes()
}

func buildTimeSyncRequest(counter uint32) []byte {
	packet := protocol.NewBuffer(4)
	packet.WriteU32(counter)
	return packet.Bytes()
}

func buildTutorialFlags(tutorials [8]uint32) []byte {
	packet := protocol.NewBuffer(32)
	for _, tut := range tutorials {
		packet.WriteU32(tut)
	}
	return packet.Bytes()
}

