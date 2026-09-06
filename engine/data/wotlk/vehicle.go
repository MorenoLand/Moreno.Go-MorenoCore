package wotlk

import "fmt"

const (
	MaxVehicleSeats = 8

	// Power types for vehicles (TrinityCore VehicleDefines.h:27-35)
	PowerSteam  uint32 = 61
	PowerPyrite uint32 = 41
	PowerHeat   uint32 = 101
	PowerOoze   uint32 = 121
	PowerBlood  uint32 = 141
	PowerWrath  uint32 = 142

	// VehicleFlags (TrinityCore VehicleDefines.h:37-48)
	VehicleFlagNoStrafe          uint32 = 0x00000001 // Sets MOVEFLAG2_NO_STRAFE
	VehicleFlagNoJumping         uint32 = 0x00000002 // Sets MOVEFLAG2_NO_JUMPING
	VehicleFlagFullSpeedTurning  uint32 = 0x00000004 // Sets MOVEFLAG2_FULLSPEEDTURNING
	VehicleFlagAllowPitching     uint32 = 0x00000010 // Sets MOVEFLAG2_ALLOW_PITCHING
	VehicleFlagFullSpeedPitching uint32 = 0x00000020 // Sets MOVEFLAG2_FULLSPEEDPITCHING
	VehicleFlagCustomPitch       uint32 = 0x00000040 // If set use pitchMin and pitchMax from DBC
	VehicleFlagAdjustAimAngle    uint32 = 0x00000400 // Lua_IsVehicleAimAngleAdjustable
	VehicleFlagAdjustAimPower    uint32 = 0x00000800 // Lua_IsVehicleAimPowerAdjustable
	VehicleFlagFixedPosition     uint32 = 0x00200000 // Used for cannons, when rooted

	// VehicleSpells (TrinityCore VehicleDefines.h:50-54)
	VehicleSpellRideHardcoded uint32 = 46598
	VehicleSpellParachute     uint32 = 45472

	// VehicleSeatFlags (TrinityCore DBCEnums.h:452-484)
	VehicleSeatFlagHasLowerAnimForEnter                     uint32 = 0x00000001
	VehicleSeatFlagHasLowerAnimForRide                      uint32 = 0x00000002
	VehicleSeatFlagUnk3                                     uint32 = 0x00000004
	VehicleSeatFlagShouldUseVehSeatExitAnimOnVoluntaryExit   uint32 = 0x00000008
	VehicleSeatFlagUnk5                                     uint32 = 0x00000010
	VehicleSeatFlagUnk6                                     uint32 = 0x00000020
	VehicleSeatFlagUnk7                                     uint32 = 0x00000040
	VehicleSeatFlagUnk8                                     uint32 = 0x00000080
	VehicleSeatFlagUnk9                                     uint32 = 0x00000100
	VehicleSeatFlagHidePassenger                            uint32 = 0x00000200
	VehicleSeatFlagAllowTurning                             uint32 = 0x00000400
	VehicleSeatFlagCanControl                               uint32 = 0x00000800
	VehicleSeatFlagCanCastMountSpell                        uint32 = 0x00001000
	VehicleSeatFlagUncontrolled                             uint32 = 0x00002000
	VehicleSeatFlagCanAttack                                uint32 = 0x00004000
	VehicleSeatFlagShouldUseVehSeatExitAnimOnForcedExit      uint32 = 0x00008000
	VehicleSeatFlagUnk17                                    uint32 = 0x00010000
	VehicleSeatFlagUnk18                                    uint32 = 0x00020000
	VehicleSeatFlagHasVehExitAnimVoluntaryExit              uint32 = 0x00040000
	VehicleSeatFlagHasVehExitAnimForcedExit                 uint32 = 0x00080000
	VehicleSeatFlagPassengerNotSelectable                   uint32 = 0x00100000
	VehicleSeatFlagUnk22                                    uint32 = 0x00200000
	VehicleSeatFlagRecHasVehicleEnterAnim                   uint32 = 0x00400000
	VehicleSeatFlagIsUsingVehicleControls                   uint32 = 0x00800000
	VehicleSeatFlagEnableVehicleZoom                        uint32 = 0x01000000
	VehicleSeatFlagCanEnterOrExit                           uint32 = 0x02000000
	VehicleSeatFlagCanSwitch                                uint32 = 0x04000000
	VehicleSeatFlagHasStartWaitingForVehTransitionAnimEnter uint32 = 0x08000000
	VehicleSeatFlagHasStartWaitingForVehTransitionAnimExit  uint32 = 0x10000000
	VehicleSeatFlagCanCast                                  uint32 = 0x20000000
	VehicleSeatFlagUnk2                                     uint32 = 0x40000000
	VehicleSeatFlagAllowsInteraction                        uint32 = 0x80000000

	// VehicleSeatFlagsB (TrinityCore DBCEnums.h:488-498)
	VehicleSeatFlagBNone                 uint32 = 0x00000000
	VehicleSeatFlagBUsableForced         uint32 = 0x00000002
	VehicleSeatFlagBTargetsInRaidUI      uint32 = 0x00000008
	VehicleSeatFlagBEjectable            uint32 = 0x00000020
	VehicleSeatFlagBUsableForced2        uint32 = 0x00000040
	VehicleSeatFlagBUsableForced3        uint32 = 0x00000100
	VehicleSeatFlagBKeepPet              uint32 = 0x00020000
	VehicleSeatFlagBUsableForced4        uint32 = 0x02000000
	VehicleSeatFlagBCanSwitch            uint32 = 0x04000000
	VehicleSeatFlagBVehiclePlayerFrameUI uint32 = 0x80000000

	// VehicleExitParameters (TrinityCore VehicleDefines.h:56-62)
	VehicleExitParamNone   uint8 = 0
	VehicleExitParamOffset uint8 = 1
	VehicleExitParamDest   uint8 = 2
)

// VehicleEntry represents a record from Vehicle.dbc (TrinityCore DBCStructure.h:1761-1793).
type VehicleEntry struct {
	ID                   uint32
	Flags                uint32
	TurnSpeed            float32
	PitchSpeed           float32
	PitchMin             float32
	PitchMax             float32
	SeatIDs              [MaxVehicleSeats]uint32
	UILocomotionType     uint32
	VehicleUIIndicatorID uint32
	PowerDisplayID       uint32
}

// VehicleSeatEntry represents a record from VehicleSeat.dbc (TrinityCore DBCStructure.h:1795-1861).
type VehicleSeatEntry struct {
	ID                    uint32
	Flags                 uint32
	AttachmentID          int32
	AttachmentOffsetX     float32
	AttachmentOffsetY     float32
	AttachmentOffsetZ     float32
	EnterSpeed            float32
	EnterGravity          float32
	ExitSpeed             float32
	ExitGravity           float32
	PassengerYaw          float32
	PassengerPitch        float32
	PassengerRoll         float32
	PassengerAttachmentID int32
	VehicleEnterAnim      int32
	VehicleExitAnim       int32
	VehicleRideAnimLoop   int32
	VehicleAbilityDisplay uint32
	EnterUISoundID        uint32
	ExitUISoundID         uint32
	UiSkin                int32
	FlagsB                uint32
}

// HasFlag checks whether a flag is set in Flags.
func (s *VehicleSeatEntry) HasFlag(flag uint32) bool {
	return (s.Flags & flag) != 0
}

// HasFlagB checks whether a flag is set in FlagsB.
func (s *VehicleSeatEntry) HasFlagB(flag uint32) bool {
	return (s.FlagsB & flag) != 0
}

// CanEnterOrExit mirrors VehicleSeatEntry::CanEnterOrExit (DBCStructure.h:1855).
func (s *VehicleSeatEntry) CanEnterOrExit() bool {
	return s.HasFlag(VehicleSeatFlagCanEnterOrExit | VehicleSeatFlagCanControl | VehicleSeatFlagShouldUseVehSeatExitAnimOnVoluntaryExit)
}

// CanSwitchFromSeat mirrors VehicleSeatEntry::CanSwitchFromSeat (DBCStructure.h:1856).
func (s *VehicleSeatEntry) CanSwitchFromSeat() bool {
	return s.HasFlag(VehicleSeatFlagCanSwitch)
}

// IsUsableByOverride mirrors VehicleSeatEntry::IsUsableByOverride (DBCStructure.h:1857-1859).
func (s *VehicleSeatEntry) IsUsableByOverride() bool {
	return s.HasFlag(VehicleSeatFlagUncontrolled|VehicleSeatFlagUnk18) ||
		s.HasFlagB(VehicleSeatFlagBUsableForced|VehicleSeatFlagBUsableForced2|VehicleSeatFlagBUsableForced3|VehicleSeatFlagBUsableForced4)
}

// CanControl mirrors VehicleSeatEntry::CanControl (DBCEnums.h:463).
func (s *VehicleSeatEntry) CanControl() bool {
	return s.HasFlag(VehicleSeatFlagCanControl)
}

// IsUsingVehicleControls mirrors Lua_IsUsingVehicleControls (DBCEnums.h:475).
func (s *VehicleSeatEntry) IsUsingVehicleControls() bool {
	return s.HasFlag(VehicleSeatFlagIsUsingVehicleControls)
}

// IsEjectable mirrors VehicleSeatEntry::IsEjectable (DBCStructure.h:1860).
func (s *VehicleSeatEntry) IsEjectable() bool {
	return s.HasFlagB(VehicleSeatFlagBEjectable)
}


// Vehicle loads a record by ID from Vehicle.dbc.
func (s *Store) Vehicle(id uint32) (VehicleEntry, bool, error) {
	file, err := s.File("Vehicle")
	if err != nil {
		return VehicleEntry{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return VehicleEntry{}, false, nil
	}
	flags, err := record.Uint32(1)
	if err != nil {
		return VehicleEntry{}, false, fmt.Errorf("read flags: %w", err)
	}
	turnSpeed, _ := record.Float32(2)
	pitchSpeed, _ := record.Float32(3)
	pitchMin, _ := record.Float32(4)
	pitchMax, _ := record.Float32(5)
	var seatIDs [MaxVehicleSeats]uint32
	for i := 0; i < MaxVehicleSeats; i++ {
		seatIDs[i], _ = record.Uint32(6 + i)
	}
	uiLocomotion, _ := record.Uint32(34)
	uiIndicator, _ := record.Uint32(36)
	powerDisplay, _ := record.Uint32(37)

	return VehicleEntry{
		ID:                   id,
		Flags:                flags,
		TurnSpeed:            turnSpeed,
		PitchSpeed:           pitchSpeed,
		PitchMin:             pitchMin,
		PitchMax:             pitchMax,
		SeatIDs:              seatIDs,
		UILocomotionType:     uiLocomotion,
		VehicleUIIndicatorID: uiIndicator,
		PowerDisplayID:       powerDisplay,
	}, true, nil
}

// VehicleSeat loads a record by ID from VehicleSeat.dbc.
func (s *Store) VehicleSeat(id uint32) (VehicleSeatEntry, bool, error) {
	file, err := s.File("VehicleSeat")
	if err != nil {
		return VehicleSeatEntry{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return VehicleSeatEntry{}, false, nil
	}
	flags, err := record.Uint32(1)
	if err != nil {
		return VehicleSeatEntry{}, false, fmt.Errorf("read flags: %w", err)
	}
	attID, _ := record.Int32(2)
	attX, _ := record.Float32(3)
	attY, _ := record.Float32(4)
	attZ, _ := record.Float32(5)
	enterSpeed, _ := record.Float32(7)
	enterGravity, _ := record.Float32(8)
	exitSpeed, _ := record.Float32(20)
	exitGravity, _ := record.Float32(21)
	yaw, _ := record.Float32(29)
	pitch, _ := record.Float32(30)
	roll, _ := record.Float32(31)
	passAttID, _ := record.Int32(32)
	vehEnterAnim, _ := record.Int32(33)
	vehExitAnim, _ := record.Int32(34)
	vehRideLoop, _ := record.Int32(35)
	abilityDisplay, _ := record.Uint32(41)
	enterSound, _ := record.Uint32(42)
	exitSound, _ := record.Uint32(43)
	uiSkin, _ := record.Int32(44)
	flagsB, _ := record.Uint32(45)

	return VehicleSeatEntry{
		ID:                    id,
		Flags:                 flags,
		AttachmentID:          attID,
		AttachmentOffsetX:     attX,
		AttachmentOffsetY:     attY,
		AttachmentOffsetZ:     attZ,
		EnterSpeed:            enterSpeed,
		EnterGravity:          enterGravity,
		ExitSpeed:             exitSpeed,
		ExitGravity:           exitGravity,
		PassengerYaw:          yaw,
		PassengerPitch:        pitch,
		PassengerRoll:         roll,
		PassengerAttachmentID: passAttID,
		VehicleEnterAnim:      vehEnterAnim,
		VehicleExitAnim:       vehExitAnim,
		VehicleRideAnimLoop:   vehRideLoop,
		VehicleAbilityDisplay: abilityDisplay,
		EnterUISoundID:        enterSound,
		ExitUISoundID:         exitSound,
		UiSkin:                uiSkin,
		FlagsB:                flagsB,
	}, true, nil
}
