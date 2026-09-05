package protocol

// HitInfo bitmask constants.
// Reference: TrinityCore UnitDefines.h:291 (enum HitInfo).
const (
	HitInfoNormalSwing     uint32 = 0x00000000
	HitInfoUnk1            uint32 = 0x00000001
	HitInfoAffectsVictim   uint32 = 0x00000002
	HitInfoOffHand         uint32 = 0x00000004
	HitInfoUnk2            uint32 = 0x00000008
	HitInfoMiss            uint32 = 0x00000010
	HitInfoFullAbsorb      uint32 = 0x00000020
	HitInfoPartialAbsorb   uint32 = 0x00000040
	HitInfoFullResist      uint32 = 0x00000080
	HitInfoPartialResist   uint32 = 0x00000100
	HitInfoCriticalHit     uint32 = 0x00000200
	HitInfoBlock           uint32 = 0x00002000
	HitInfoGlancing        uint32 = 0x00010000
	HitInfoCrushing        uint32 = 0x00020000
	HitInfoNoAnimation     uint32 = 0x00040000
	HitInfoSwingNoHitSound uint32 = 0x00200000
	HitInfoRageGain        uint32 = 0x00800000
	HitInfoFakeDamage      uint32 = 0x01000000
)

// VictimState enum constants.
// Reference: TrinityCore Unit.h:43 (enum VictimState).
const (
	VictimStateIntact    uint8 = 0 // Set when attacker misses
	VictimStateHit       uint8 = 1 // Victim took damage / partial block
	VictimStateDodge     uint8 = 2
	VictimStateParry     uint8 = 3
	VictimStateInterrupt uint8 = 4
	VictimStateBlocks    uint8 = 5 // Full block
	VictimStateEvades    uint8 = 6
	VictimStateIsImmune  uint8 = 7
	VictimStateDeflects  uint8 = 8
)

// MeleeHitOutcome enum constants.
// Reference: TrinityCore Unit.h:356 (enum MeleeHitOutcome).
type MeleeHitOutcome uint8

const (
	MeleeHitEvade    MeleeHitOutcome = 0
	MeleeHitMiss     MeleeHitOutcome = 1
	MeleeHitDodge    MeleeHitOutcome = 2
	MeleeHitBlock    MeleeHitOutcome = 3
	MeleeHitParry    MeleeHitOutcome = 4
	MeleeHitGlancing MeleeHitOutcome = 5
	MeleeHitCrit     MeleeHitOutcome = 6
	MeleeHitCrushing MeleeHitOutcome = 7
	MeleeHitNormal   MeleeHitOutcome = 8
)

// BuildAttackerStateUpdate builds SMSG_ATTACKERSTATEUPDATE (0x14A) packet.
// Reference: TrinityCore Unit::SendAttackStateUpdate (Unit.cpp:5442).
func BuildAttackerStateUpdate(attacker, victim uint64, damage, overkill uint32, hitInfo uint32, targetState uint8, blocked uint32) []byte {
	packet := NewBuffer(64)
	packet.WriteU32(hitInfo)
	packet.WritePackedGUID(attacker)
	packet.WritePackedGUID(victim)
	packet.WriteU32(damage)
	packet.WriteU32(overkill)
	packet.WriteU8(1) // Sub damage count
	packet.WriteU32(1) // Damage school: Physical (1)
	packet.WriteF32(float32(damage))
	packet.WriteU32(damage)
	packet.WriteU8(targetState)
	packet.WriteU32(0) // Unknown attackerstate
	packet.WriteU32(0) // Melee spell ID
	if hitInfo&HitInfoBlock != 0 {
		packet.WriteU32(blocked)
	}
	return packet.Bytes()
}
