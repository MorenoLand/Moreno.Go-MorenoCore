package wotlk

const MaxItemExtendedCostRequirements = 5

type ItemExtendedCostEntry struct {
	ID                  uint32
	HonorPoints         uint32
	ArenaPoints         uint32
	ArenaBracket        uint32
	ItemIDs             [MaxItemExtendedCostRequirements]uint32
	ItemCounts          [MaxItemExtendedCostRequirements]uint32
	RequiredArenaRating uint32
}

func (s *Store) ItemExtendedCost(id uint32) (ItemExtendedCostEntry, bool, error) {
	file, err := s.File("ItemExtendedCost")
	if err != nil {
		return ItemExtendedCostEntry{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return ItemExtendedCostEntry{}, false, nil
	}
	entry := ItemExtendedCostEntry{ID: id}
	values := []*uint32{&entry.ID, &entry.HonorPoints, &entry.ArenaPoints, &entry.ArenaBracket}
	for i := range entry.ItemIDs {
		values = append(values, &entry.ItemIDs[i])
	}
	for i := range entry.ItemCounts {
		values = append(values, &entry.ItemCounts[i])
	}
	values = append(values, &entry.RequiredArenaRating)
	for field, value := range values {
		if *value, err = record.Uint32(field); err != nil {
			return ItemExtendedCostEntry{}, false, err
		}
	}
	return entry, true, nil
}
