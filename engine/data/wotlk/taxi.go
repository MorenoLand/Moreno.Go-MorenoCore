package wotlk

import (
	"fmt"
)

// TaxiNode mirrors TrinityCore TaxiNodesEntry (3.3.5): ID, ContinentID,
// position and MountCreatureID[4] (index 0 = Horde, 1 = Alliance).
type TaxiNode struct {
	ID               uint32
	ContinentID      int32
	X                float32
	Y                float32
	Z                float32
	MountCreatureID  [4]uint32
	HasOutgoingPath  bool
}

const (
	taxiDKSpecialMount = 32981 // Ebon Hold flight, valid for both factions
	taxiMaskSize       = 8
)

type taxiNetwork struct {
	nodes  []TaxiNode
	mask   [taxiMaskSize]uint32
	byID   map[uint32]int
}

func (s *Store) taxiNetwork() (*taxiNetwork, error) {
	s.taxiOnce.Do(func() {
		network := &taxiNetwork{byID: make(map[uint32]int)}
		nodesFile, err := s.File("TaxiNodes")
		if err != nil {
			s.taxiErr = err
			return
		}
		// TaxiPath.dbc: ID, FromTaxiNode, ToTaxiNode, price. Nodes carrying a
		// priced outgoing path belong to the visible flight network, matching
		// ObjectMgr's sTaxiNodesMask build (spell paths are price 0 and skip).
		pathSources := make(map[uint32]struct{})
		pathsFile, err := s.File("TaxiPath")
		if err != nil {
			s.taxiErr = err
			return
		}
		for i := 0; i < pathsFile.Records(); i++ {
			record, err := pathsFile.Record(i)
			if err != nil {
				s.taxiErr = err
				return
			}
			price, err := record.Uint32(3)
			if err != nil {
				s.taxiErr = err
				return
			}
			if price == 0 {
				continue
			}
			from, err := record.Uint32(1)
			if err != nil {
				s.taxiErr = err
				return
			}
			pathSources[from] = struct{}{}
		}
		for i := 0; i < nodesFile.Records(); i++ {
			record, err := nodesFile.Record(i)
			if err != nil {
				s.taxiErr = err
				return
			}
			node := TaxiNode{}
			if node.ID, err = record.Uint32(0); err != nil {
				s.taxiErr = err
				return
			}
			continent, err := record.Int32(1)
			if err != nil {
				s.taxiErr = err
				return
			}
			node.ContinentID = continent
			if node.X, err = record.Float32(2); err != nil {
				s.taxiErr = err
				return
			}
			if node.Y, err = record.Float32(3); err != nil {
				s.taxiErr = err
				return
			}
			if node.Z, err = record.Float32(4); err != nil {
				s.taxiErr = err
				return
			}
			for mount := 0; mount < 4; mount++ {
				if node.MountCreatureID[mount], err = record.Uint32(5 + mount); err != nil {
					s.taxiErr = err
					return
				}
			}
			if _, ok := pathSources[node.ID]; ok {
				node.HasOutgoingPath = true
			}
			if node.ID != 0 && node.HasOutgoingPath {
				field := (node.ID - 1) / 32
				if field < taxiMaskSize {
					network.mask[field] |= 1 << ((node.ID - 1) % 32)
				}
			}
			network.byID[node.ID] = len(network.nodes)
			network.nodes = append(network.nodes, node)
		}
		s.taxi = network
	})
	if s.taxiErr != nil {
		return nil, s.taxiErr
	}
	if s.taxi == nil {
		return nil, fmt.Errorf("taxi network unavailable")
	}
	return s.taxi, nil
}

// TaxiNetworkMask returns the mask of nodes belonging to the flight network.
func (s *Store) TaxiNetworkMask() ([taxiMaskSize]uint32, error) {
	network, err := s.taxiNetwork()
	if err != nil {
		return [taxiMaskSize]uint32{}, err
	}
	return network.mask, nil
}

// NearestTaxiNode mirrors ObjectMgr::GetNearestTaxiNode: closest network node
// on the map whose mount list serves the team (0 = Horde, 1 = Alliance; the
// 32981 Ebon Hold mount serves everyone).
func (s *Store) NearestTaxiNode(x, y, z float32, mapID uint32, teamAlliance bool) (uint32, error) {
	network, err := s.taxiNetwork()
	if err != nil {
		return 0, err
	}
	teamIndex := 0
	if teamAlliance {
		teamIndex = 1
	}
	found := false
	best := float64(0)
	id := uint32(0)
	for _, node := range network.nodes {
		if node.ContinentID != int32(mapID) {
			continue
		}
		if node.MountCreatureID[teamIndex] == 0 && node.MountCreatureID[0] != taxiDKSpecialMount {
			continue
		}
		field := (node.ID - 1) / 32
		if node.ID == 0 || field >= taxiMaskSize || network.mask[field]&(1<<((node.ID-1)%32)) == 0 {
			continue
		}
		dx := float64(node.X - x)
		dy := float64(node.Y - y)
		dz := float64(node.Z - z)
		dist2 := dx*dx + dy*dy + dz*dz
		if !found || dist2 < best {
			found = true
			best = dist2
			id = node.ID
		}
	}
	return id, nil
}


