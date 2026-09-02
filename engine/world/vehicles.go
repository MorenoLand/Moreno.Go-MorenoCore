package world

import (
	"context"
)

// handleChangeSeatsOnControlledVehicle processes CMSG_CHANGE_SEATS_ON_CONTROLLED_VEHICLE (0x49B).
// Reference: WorldSession::HandleChangeSeatsOnControlledVehicle (VehicleHandler.cpp:52).
func (s *session) handleChangeSeatsOnControlledVehicle(ctx context.Context, payload []byte) bool {
	return true
}

// handleControllerEjectPassenger processes CMSG_CONTROLLER_EJECT_PASSENGER (0x4A9).
// Reference: WorldSession::HandleEjectPassenger (VehicleHandler.cpp:151).
func (s *session) handleControllerEjectPassenger(ctx context.Context, payload []byte) bool {
	return true
}

// handleDismissControlledVehicle processes CMSG_DISMISS_CONTROLLED_VEHICLE (0x46D).
// Reference: WorldSession::HandleDismissControlledVehicle (VehicleHandler.cpp:27).
func (s *session) handleDismissControlledVehicle(ctx context.Context, payload []byte) bool {
	return true
}
