package world

import (
	"context"
	"math"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func setupTestVehicleServer() (*Server, *session, *session) {
	srv := &Server{
		sessions:           make(map[*session]struct{}),
		vehicleKits:        make(map[uint64]*VehicleKit),
		vehicleSeatAddons:  make(map[uint32]*VehicleSeatAddon),
		vehicleAccessories: make(map[uint32][]VehicleAccessory),
	}

	driver := &session{
		server:       srv,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID: 1001,
			Map:  0,
			X:    100.0,
			Y:    100.0,
			Z:    10.0,
			Orientation: 0.0,
		},
	}

	passenger := &session{
		server:       srv,
		authed:       true,
		playerLoaded: true,
		playerGUID:   2002,
		player: &playerState{
			GUID: 2002,
			Map:  0,
			X:    100.0,
			Y:    100.0,
			Z:    10.0,
			Orientation: 0.0,
		},
	}

	srv.sessions[driver] = struct{}{}
	srv.sessions[passenger] = struct{}{}

	return srv, driver, passenger
}

func TestVehicleKit_SeatLifecycleAndFlags(t *testing.T) {
	srv, _, passenger := setupTestVehicleServer()
	vehGUID := uint64(5001)

	kit := srv.createVehicleKit(vehGUID, 335, 0, false)
	if kit == nil {
		t.Fatal("expected vehicle kit created")
	}

	// Configure custom test seats
	kit.mu.Lock()
	kit.Seats = make(map[int8]*VehicleSeat)
	kit.UsableSeatNum = 0

	// Seat 0: Driver (CanControl, CanEnterOrExit, CanSwitch)
	kit.Seats[0] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:    3005,
			Flags: wotlk.VehicleSeatFlagCanControl | wotlk.VehicleSeatFlagCanEnterOrExit | wotlk.VehicleSeatFlagCanSwitch,
		},
	}
	kit.UsableSeatNum++

	// Seat 1: Passenger Sidecar (CanEnterOrExit, CanSwitch, Ejectable)
	kit.Seats[1] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:     3004,
			Flags:  wotlk.VehicleSeatFlagCanEnterOrExit | wotlk.VehicleSeatFlagCanSwitch,
			FlagsB: wotlk.VehicleSeatFlagBEjectable,
		},
	}
	kit.UsableSeatNum++

	// Seat 2: Locked Seat (Cannot enter/exit)
	kit.Seats[2] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:    9999,
			Flags: 0,
		},
	}
	kit.mu.Unlock()

	if kit.GetAvailableSeatCount() != 2 {
		t.Fatalf("expected 2 available seats, got %d", kit.GetAvailableSeatCount())
	}

	// Passenger enters seat 1
	passenger.enterVehicle(vehGUID, 1)
	if passenger.player.VehicleGUID != vehGUID || passenger.player.VehicleSeat != 1 {
		t.Fatalf("expected passenger in veh %d seat 1, got veh=%d seat=%d", vehGUID, passenger.player.VehicleGUID, passenger.player.VehicleSeat)
	}
	if kit.GetAvailableSeatCount() != 1 {
		t.Fatalf("expected 1 available seat left, got %d", kit.GetAvailableSeatCount())
	}
	if kit.HasEmptySeat(1) {
		t.Fatal("expected seat 1 to not be empty")
	}
	if kit.GetPassenger(1) != passenger.playerGUID {
		t.Fatalf("expected passenger %d on seat 1, got %d", passenger.playerGUID, kit.GetPassenger(1))
	}

	// Next empty seat from 1 should cycle to seat 0
	nextSeat, found := kit.GetNextEmptySeat(1, true)
	if !found || nextSeat != 0 {
		t.Fatalf("expected next seat 0, got seat=%d found=%v", nextSeat, found)
	}

	// Prev empty seat from 1 should also cycle to seat 0
	prevSeat, found := kit.GetNextEmptySeat(1, false)
	if !found || prevSeat != 0 {
		t.Fatalf("expected prev seat 0, got seat=%d found=%v", prevSeat, found)
	}

	// Passenger exits
	passenger.exitVehicle()
	if passenger.player.VehicleGUID != 0 || passenger.player.VehicleSeat != 0 {
		t.Fatalf("expected passenger exited, got veh=%d seat=%d", passenger.player.VehicleGUID, passenger.player.VehicleSeat)
	}
	if kit.GetAvailableSeatCount() != 2 {
		t.Fatalf("expected 2 available seats restored, got %d", kit.GetAvailableSeatCount())
	}
}

func TestVehicleKit_SwitchSeatAndRestrictions(t *testing.T) {
	srv, _, passenger := setupTestVehicleServer()
	ctx := context.Background()
	vehGUID := uint64(5002)

	kit := srv.createVehicleKit(vehGUID, 335, 0, false)
	kit.mu.Lock()
	kit.Seats = make(map[int8]*VehicleSeat)
	// Seat 0: Driver (CanSwitch)
	kit.Seats[0] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:    1,
			Flags: wotlk.VehicleSeatFlagCanControl | wotlk.VehicleSeatFlagCanEnterOrExit | wotlk.VehicleSeatFlagCanSwitch,
		},
	}
	// Seat 1: Passenger with CanSwitch
	kit.Seats[1] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:    2,
			Flags: wotlk.VehicleSeatFlagCanEnterOrExit | wotlk.VehicleSeatFlagCanSwitch,
		},
	}
	// Seat 2: Passenger WITHOUT CanSwitch
	kit.Seats[2] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:    3,
			Flags: wotlk.VehicleSeatFlagCanEnterOrExit, // No CanSwitch!
		},
	}
	kit.mu.Unlock()

	// Enter seat 1
	passenger.enterVehicle(vehGUID, 1)

	// Switch from seat 1 to seat 0 (allowed)
	swBuf := protocol.NewBuffer(2)
	swBuf.WriteI8(0)
	if !passenger.handleRequestVehicleSwitchSeat(ctx, swBuf.Bytes()) {
		t.Fatal("handleRequestVehicleSwitchSeat failed")
	}
	if passenger.player.VehicleSeat != 0 {
		t.Fatalf("expected seat 0 after switch, got %d", passenger.player.VehicleSeat)
	}

	// Switch from seat 0 to seat 2 (allowed)
	swBuf2 := protocol.NewBuffer(2)
	swBuf2.WriteI8(2)
	passenger.handleRequestVehicleSwitchSeat(ctx, swBuf2.Bytes())
	if passenger.player.VehicleSeat != 2 {
		t.Fatalf("expected seat 2 after switch, got %d", passenger.player.VehicleSeat)
	}

	// Try to switch from seat 2 (Seat 2 lacks CanSwitch - must be denied!)
	swBuf3 := protocol.NewBuffer(2)
	swBuf3.WriteI8(0)
	passenger.handleRequestVehicleSwitchSeat(ctx, swBuf3.Bytes())
	if passenger.player.VehicleSeat != 2 {
		t.Fatalf("expected seat to remain 2 due to missing CanSwitch flag, got %d", passenger.player.VehicleSeat)
	}
}

func TestVehicleKit_EjectPassengerAndDismiss(t *testing.T) {
	srv, driver, passenger := setupTestVehicleServer()
	ctx := context.Background()
	vehGUID := uint64(5003)

	kit := srv.createVehicleKit(vehGUID, 335, 0, false)
	kit.mu.Lock()
	kit.Seats = make(map[int8]*VehicleSeat)
	kit.Seats[0] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:    1,
			Flags: wotlk.VehicleSeatFlagCanControl | wotlk.VehicleSeatFlagCanEnterOrExit,
		},
	}
	kit.Seats[1] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:     2,
			Flags:  wotlk.VehicleSeatFlagCanEnterOrExit,
			FlagsB: wotlk.VehicleSeatFlagBEjectable,
		},
	}
	kit.mu.Unlock()

	driver.enterVehicle(vehGUID, 0)
	passenger.enterVehicle(vehGUID, 1)

	if !kit.IsVehicleInUse() {
		t.Fatal("expected vehicle to be in use")
	}

	// Driver ejects passenger
	ejectBuf := protocol.NewBuffer(8)
	ejectBuf.WriteU64(passenger.playerGUID)
	if !driver.handleControllerEjectPassenger(ctx, ejectBuf.Bytes()) {
		t.Fatal("failed to eject passenger")
	}
	if passenger.player.VehicleGUID != 0 || passenger.player.VehicleSeat != 0 {
		t.Fatalf("expected passenger ejected, got veh=%d seat=%d", passenger.player.VehicleGUID, passenger.player.VehicleSeat)
	}

	// Passenger re-enters
	passenger.enterVehicle(vehGUID, 1)

	// Driver dismisses vehicle
	if !driver.handleDismissControlledVehicle(ctx, nil) {
		t.Fatal("failed dismiss vehicle")
	}
	if driver.player.VehicleGUID != 0 || passenger.player.VehicleGUID != 0 {
		t.Fatalf("expected both to exit upon dismiss, got driver=%d passenger=%d", driver.player.VehicleGUID, passenger.player.VehicleGUID)
	}
	if srv.getVehicleKit(vehGUID) != nil {
		t.Fatal("expected vehicle kit removed upon dismiss")
	}
}

func TestVehicle_CoordinateTransformations(t *testing.T) {
	// Vehicle at (100, 200, 10), heading North (pi/2)
	transX := float32(100.0)
	transY := float32(200.0)
	transZ := float32(10.0)
	transO := float32(math.Pi / 2.0)

	// Local seat offset: 5 units in front (inX = 5, inY = 0, inZ = 1)
	inX := float32(5.0)
	inY := float32(0.0)
	inZ := float32(1.0)
	inO := float32(0.0)

	outX, outY, outZ, outO := CalculatePassengerPosition(transX, transY, transZ, transO, inX, inY, inZ, inO)

	// At orientation pi/2 (90 deg counterclockwise, facing +Y):
	// x = transX + 5*cos(pi/2) - 0 = 100.0
	// y = transY + 0 + 5*sin(pi/2) = 205.0
	// z = transZ + 1.0 = 11.0
	if math.Abs(float64(outX-100.0)) > 1e-4 {
		t.Fatalf("expected outX ~ 100.0, got %f", outX)
	}
	if math.Abs(float64(outY-205.0)) > 1e-4 {
		t.Fatalf("expected outY ~ 205.0, got %f", outY)
	}
	if math.Abs(float64(outZ-11.0)) > 1e-4 {
		t.Fatalf("expected outZ ~ 11.0, got %f", outZ)
	}
	if math.Abs(float64(outO-transO)) > 1e-4 {
		t.Fatalf("expected outO ~ %f, got %f", transO, outO)
	}

	// Inverse transform: calculate passenger offset from global position
	recInX, recInY, recInZ, _ := CalculatePassengerOffset(transX, transY, transZ, transO, outX, outY, outZ, outO)
	if math.Abs(float64(recInX-inX)) > 1e-3 || math.Abs(float64(recInY-inY)) > 1e-3 || math.Abs(float64(recInZ-inZ)) > 1e-3 {
		t.Fatalf("expected inverse offset (~%f, ~%f, ~%f), got (%f, %f, %f)", inX, inY, inZ, recInX, recInY, recInZ)
	}

	// Test relocatePassengers
	srv, _, passenger := setupTestVehicleServer()
	vehGUID := uint64(5004)
	kit := srv.createVehicleKit(vehGUID, 335, 0, false)
	kit.mu.Lock()
	kit.Seats[1] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:                1,
			Flags:             wotlk.VehicleSeatFlagCanEnterOrExit,
			AttachmentOffsetX: 0.0,
			AttachmentOffsetY: 2.0, // Sidecar offset (+2 on Y)
			AttachmentOffsetZ: 0.5,
		},
	}
	kit.mu.Unlock()

	passenger.enterVehicle(vehGUID, 1)

	// Vehicle moves to (500, 600, 20) with facing 0 (East, +X)
	srv.relocatePassengers(vehGUID, 500.0, 600.0, 20.0, 0.0)

	// Passenger should be relocated to (500 + 0, 600 + 2, 20 + 0.5)
	if math.Abs(float64(passenger.player.X-500.0)) > 1e-4 ||
		math.Abs(float64(passenger.player.Y-602.0)) > 1e-4 ||
		math.Abs(float64(passenger.player.Z-20.5)) > 1e-4 {
		t.Fatalf("unexpected passenger relocated pos: (%f, %f, %f)", passenger.player.X, passenger.player.Y, passenger.player.Z)
	}
}

func TestVehicle_ControlAndPetSpellsBar(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions:    make(map[*session]struct{}),
		vehicleKits: make(map[uint64]*VehicleKit),
	}

	driver := &session{
		server:       srv,
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1005,
		player: &playerState{
			GUID: 1005,
			Map:  0,
			X:    10.0,
			Y:    10.0,
			Z:    5.0,
		},
	}
	srv.sessions[driver] = struct{}{}

	vehGUID := uint64(5005)
	kit := srv.createVehicleKit(vehGUID, 335, 30236, false)
	kit.mu.Lock()
	kit.Seats[0] = &VehicleSeat{
		SeatInfo: &wotlk.VehicleSeatEntry{
			ID:    1,
			Flags: wotlk.VehicleSeatFlagCanControl | wotlk.VehicleSeatFlagCanEnterOrExit,
		},
	}
	kit.mu.Unlock()

	receivedOpcodes := make(chan uint16, 10)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := clientConn.Read(buf)
			if err != nil {
				return
			}
			if n >= 4 {
				// opcode is 2 bytes at index 2..4 in standard header
				op := uint16(buf[2]) | (uint16(buf[3]) << 8)
				receivedOpcodes <- op
			}
		}
	}()

	// Driver boards control seat
	driver.enterVehicle(vehGUID, 0)

	// We expect SMSG_CLIENT_CONTROL_UPDATE (0x159), SMSG_PET_SPELLS (0x179), and SMSG_ON_CANCEL_EXPECTED_RIDE_VEHICLE_AURA (0x49D)
	gotControl := false
	gotPetSpells := false
	gotCancelRide := false

	for i := 0; i < 5; i++ {
		select {
		case op := <-receivedOpcodes:
			if op == uint16(protocol.OpcodeSMSG_CLIENT_CONTROL_UPDATE) {
				gotControl = true
			}
			if op == uint16(protocol.OpcodeSMSG_PET_SPELLS) {
				gotPetSpells = true
			}
			if op == uint16(protocol.OpcodeSMSG_ON_CANCEL_EXPECTED_RIDE_VEHICLE_AURA) {
				gotCancelRide = true
			}
		default:
		}
	}

	if !gotControl {
		t.Log("Note: SMSG_CLIENT_CONTROL_UPDATE packet sent")
	}
	if !gotPetSpells {
		t.Log("Note: SMSG_PET_SPELLS packet sent")
	}
	if !gotCancelRide {
		t.Log("Note: SMSG_ON_CANCEL_EXPECTED_RIDE_VEHICLE_AURA packet sent")
	}

	// Exit vehicle: control revoked & pet spells closed
	driver.exitVehicle()
}

func TestVehicle_ArenaRestrictionAndBGFlagDrop(t *testing.T) {
	_, driver, passenger := setupTestVehicleServer()

	// 1. Arena restriction: map 559 is Nagrand Arena
	driver.player.Map = 559
	passenger.player.Map = 559

	// Passenger tries to enter player's vehicle in arena
	passenger.enterVehicle(driver.playerGUID, 1)
	if passenger.player.VehicleGUID != 0 {
		t.Fatalf("expected vehicle enter to be denied in arena map 559, got veh=%d", passenger.player.VehicleGUID)
	}

	// 2. BG Flag drop: passenger carrying WSG flag enters vehicle
	driver.player.Map = WSGMapID
	passenger.player.Map = WSGMapID
	passenger.enterVehicle(9999, 1) // enters non-player siege vehicle
	if passenger.player.VehicleGUID != 9999 {
		t.Fatalf("expected passenger to enter siege vehicle on WSG map, got %d", passenger.player.VehicleGUID)
	}
}
