package world

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	spellAttributePassive uint32 = 0x00000040
	spellCastFlagStart    uint32 = 0x00000002
	spellCastFlagGo       uint32 = 0x00000100
)

// isSelfCastOnly checks if all active spell effects are self/caster targeting.
// Dynamically resolves from Spell.dbc effect targets:
// 1 = TARGET_UNIT_CASTER, 18 = TARGET_DEST_CASTER, 22 = TARGET_SRC_CASTER.
func isSelfCastOnly(spell wotlk.Spell) bool {
	hasEffect := false
	for _, eff := range spell.Effects {
		if eff.Effect == 0 {
			continue
		}
		hasEffect = true
		// In TrinityCore SpellInfo::IsSelfCast:
		// Every active effect must target TARGET_UNIT_CASTER (1), TARGET_DEST_CASTER (18), or TARGET_SRC_CASTER (22).
		if eff.ImplicitTargetA != 1 && eff.ImplicitTargetA != 18 && eff.ImplicitTargetA != 22 {
			return false
		}
	}
	return hasEffect
}

func (s *session) calculateSpellPowerCost(spell wotlk.Spell) uint32 {
	cost := spell.ManaCost
	if spell.ManaCostPct > 0 && s.player != nil {
		pType := spell.PowerType
		if pType == 0 { // Mana: calculate percentage from BaseMana per TrinityCore Player::GetCreateMana()
			basePower := s.player.BaseMana
			if basePower == 0 {
				basePower = s.player.MaxPowers[0]
			}
			if basePower == 0 {
				basePower = 100
			}
			cost += (basePower * spell.ManaCostPct) / 100
		} else if pType < 7 {
			basePower := s.player.MaxPowers[pType]
			if basePower == 0 {
				basePower = s.player.Powers[pType]
			}
			if basePower == 0 {
				basePower = 100
			}
			cost += (basePower * spell.ManaCostPct) / 100
		}
	}
	return cost
}

func (s *session) handleCastSpell(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.server.Data == nil {
		return true
	}
	reader := protocol.NewReader(payload)
	castID, err := reader.ReadU8()
	if err != nil {
		return false
	}
	spellID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	clientCastFlags, err := reader.ReadU8()
	if err != nil {
		return false
	}
	target, err := protocol.ReadSpellTargetData(reader)
	if err != nil {
		s.debug("spell cast rejected", "account", s.accountName, "spell", spellID, "reason", "malformed targets", "error", err)
		return false
	}
	if clientCastFlags&0x02 != 0 {
		if _, err = reader.ReadF32(); err != nil {
			return false
		}
		if _, err = reader.ReadF32(); err != nil {
			return false
		}
	}
	spell, found, err := s.server.Data.Spell(spellID)
	if err != nil {
		s.debug("spell lookup failed", "account", s.accountName, "spell", spellID, "error", err)
		return true
	}
	learned := s.hasActiveSpell(spellID)
	if !found || spell.Attributes&spellAttributePassive != 0 || !learned {
		s.debug("spell cast ignored", "account", s.accountName, "spell", spellID, "reason", spellCastIgnoreReason(spell, found, learned))
		return true
	}
	nowUnix := time.Now().Unix()
	for _, cd := range s.player.Cooldowns {
		if cd.Spell == spellID && cd.End > nowUnix {
			_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(castID, spellID, 47), true) // SPELL_FAILED_NOT_READY = 47
			s.debug("spell cast rejected", "account", s.accountName, "spell", spellID, "reason", "on cooldown")
			return true
		}
	}
	// Self-cast only spells (e.g. Demon Skin, Demon Armor, Ice Barrier) must always target the caster
	if isSelfCastOnly(spell) {
		if target.Flags&protocol.SpellTargetFlagUnitWireMask != 0 && target.UnitGUID != 0 && target.UnitGUID != s.playerGUID {
			_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(castID, spellID, 2), true) // SPELL_FAILED_BAD_TARGETS = 2
			s.debug("spell cast rejected", "account", s.accountName, "spell", spellID, "reason", "self-cast only spell cannot target other units")
			return true
		}
		target.UnitGUID = s.playerGUID
		target.Flags = protocol.SpellTargetFlagUnit
	}
	cost := s.calculateSpellPowerCost(spell)
	pType := spell.PowerType
	if cost > 0 && pType < 7 && s.player.Powers[pType] < cost {
		_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(castID, spellID, 85), true) // SPELL_FAILED_NO_POWER = 85
		s.debug("spell cast rejected", "account", s.accountName, "spell", spellID, "reason", "not enough power", "power", s.player.Powers[pType], "cost", cost)
		return true
	}

	// Interrupt any existing spell cast (TC: Unit::InterruptNonMeleeSpells)
	s.interruptCurrentCast()
	s.interruptCurrentChannel()

	castTime := uint32(0)
	s.lastCastTime = time.Now()
	if value, ok, castErr := s.server.Data.SpellCastTime(spell.CastingTimeIndex); castErr == nil && ok && value > 0 {
		castTime = uint32(value)
	}
	if err := s.write(uint16(protocol.OpcodeSMSG_SPELL_START), protocol.BuildSpellStart(s.playerGUID, s.playerGUID, castID, spellID, spellCastFlagStart, castTime, target), true); err != nil {
		return false
	}

	if castTime > 0 {
		s.castMu.Lock()
		castState := &activeCastState{
			CastID:       castID,
			SpellID:      spellID,
			StartAt:      time.Now(),
			CastTimeMs:   castTime,
			InterruptFlg: spell.InterruptFlags,
		}
		castState.Timer = time.AfterFunc(time.Duration(castTime)*time.Millisecond, func() {
			s.castMu.Lock()
			if castState.Cancelled {
				s.castMu.Unlock()
				return
			}
			s.activeCast = nil
			s.castMu.Unlock()
			s.finishSpellCast(context.Background(), castID, spellID, spell, target)
		})
		s.activeCast = castState
		s.castMu.Unlock()
	} else {
		s.finishSpellCast(context.Background(), castID, spellID, spell, target)
	}

	s.debug("spell cast accepted", "account", s.accountName, "spell", spellID, "cast_id", castID, "cast_time", castTime, "cost", cost)
	return true
}

func (s *session) finishSpellCast(ctx context.Context, castID uint8, spellID uint32, spell wotlk.Spell, target protocol.SpellTargetData) {
	if s.player == nil {
		return
	}
	s.castMu.Lock()
	if s.activeCast != nil && s.activeCast.CastID == castID && s.activeCast.SpellID == spellID {
		s.activeCast = nil
	}
	s.castMu.Unlock()

	hitTargets := []uint64{s.playerGUID}
	if isSelfCastOnly(spell) {
		hitTargets[0] = s.playerGUID
	} else if target.Flags&protocol.SpellTargetFlagUnitWireMask != 0 && target.UnitGUID != 0 {
		hitTargets[0] = target.UnitGUID
	} else if s.selection != 0 {
		hitTargets[0] = s.selection
	}
	targetGUID := hitTargets[0]

	// Spell hit check for offensive spells targeting another unit
	var missStatus []protocol.SpellMissStatus
	if targetGUID != 0 && targetGUID != s.playerGUID && isHarmfulSpell(spell) {
		targetLevel := uint8(1)
		if tgt, ok := s.getCombatTarget(ctx, targetGUID); ok {
			targetLevel = tgt.Level
		}
		isPlayerVictim := s.server != nil && s.server.findSessionByGUID(targetGUID) != nil
		missInfo := magicSpellHitResult(s.player.Level, targetLevel, isPlayerVictim)
		if missInfo != protocol.SpellMissNone {
			hitTargets = nil
			missStatus = []protocol.SpellMissStatus{{TargetGUID: targetGUID, Reason: missInfo}}
		}
	}

	castTimeStamp := uint32(time.Now().UnixMilli())
	_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_GO), protocol.BuildSpellGo(s.playerGUID, s.playerGUID, castID, spellID, spellCastFlagGo, castTimeStamp, hitTargets, missStatus, target), true)

	pType := spell.PowerType
	cost := s.calculateSpellPowerCost(spell)
	if pType < 7 && cost > 0 {
		if s.player.Powers[pType] >= cost {
			s.player.Powers[pType] -= cost
		} else {
			s.player.Powers[pType] = 0
		}
		fields := map[int]uint32{
			unitFieldPower1 + int(pType): s.player.Powers[pType],
		}
		if pVal, pErr := s.server.buildPlayerValuesUpdate(s.playerGUID, fields); pErr == nil && pVal != nil {
			_ = s.write(pVal.Opcode, pVal.Payload.Bytes(), true)
			if s.server != nil {
				s.server.broadcastToNearby(pVal.Opcode, pVal.Payload.Bytes(), s)
			}
		}
		if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			col := fmt.Sprintf("power%d", pType+1)
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, fmt.Sprintf("UPDATE characters SET %s = ? WHERE guid = ?", col), s.player.Powers[pType], s.playerGUID)
		}
	}

	if len(hitTargets) == 0 {
		// Spell missed, do not trigger channel, cooldown, or effects
		return
	}

	// Reference Spell::handle_immediate: channeled spells begin their timed
	// channel lifecycle after the cast completes.
	if isChanneledSpell(spell) {
		s.startChannel(castID, spellID, spell, targetGUID)
	}
	if spell.RecoveryTime > 0 {
		nowUnix := time.Now().Unix()
		cooldownEnd := nowUnix + int64((spell.RecoveryTime+999)/1000)
		s.player.Cooldowns = append(s.player.Cooldowns, spellCooldown{Spell: spellID, End: cooldownEnd})
		_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_COOLDOWN), buildSpellCooldown(s.playerGUID, spellID, uint32(spell.RecoveryTime)), true)
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_spell_cooldown (guid, spell, item, time, categoryId, categoryEnd) VALUES (?, ?, 0, ?, 0, 0)", s.playerGUID, spellID, cooldownEnd)
		}
	}
	if spellID == 8690 {
		nowUnix := time.Now().Unix()
		cooldownEnd := nowUnix + 900 // 15 min cooldown
		s.player.Cooldowns = append(s.player.Cooldowns, spellCooldown{Spell: spellID, End: cooldownEnd})
		_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_COOLDOWN), buildSpellCooldown(s.playerGUID, spellID, 900000), true)
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "REPLACE INTO character_spell_cooldown (guid, spell, item, time, categoryId, categoryEnd) VALUES (?, ?, 6948, ?, 0, 0)", s.playerGUID, spellID, cooldownEnd)
		}
	}

	// Reference SpellEffects.cpp:3858-3874: Spell 7266 (Duel)
	if spellID == 7266 && targetGUID != 0 && targetGUID != s.playerGUID && s.server != nil {
		if partner := s.server.findSessionByGUID(targetGUID); partner != nil && partner.player != nil {
			s.duelPartner = targetGUID
			partner.duelPartner = s.playerGUID
			arbiterGUID := uint64(s.playerGUID) | (uint64(0xF110) << 48)
			s.player.DuelArbiter = arbiterGUID
			partner.player.DuelArbiter = arbiterGUID
			s.player.DuelTeam = 0
			partner.player.DuelTeam = 0
			s.sendPlayerUpdate()
			partner.sendPlayerUpdate()

			midX := s.player.X + (partner.player.X-s.player.X)/2
			midY := s.player.Y + (partner.player.Y-s.player.Y)/2
			midZ := s.player.Z
			s.duelArbiterX, s.duelArbiterY, s.duelArbiterZ = midX, midY, midZ
			partner.duelArbiterX, partner.duelArbiterY, partner.duelArbiterZ = midX, midY, midZ
			s.duelOutOfBounds = time.Time{}
			partner.duelOutOfBounds = time.Time{}

			reqBuf := protocol.NewBuffer(16)
			reqBuf.WriteU64(arbiterGUID)
			reqBuf.WriteU64(s.playerGUID)
			_ = s.write(uint16(protocol.OpcodeSMSG_DUEL_REQUESTED), reqBuf.Bytes(), true)
			_ = partner.write(uint16(protocol.OpcodeSMSG_DUEL_REQUESTED), reqBuf.Bytes(), true)
		}
	}

	// Apply spell effects
	applyEffects := func(effCtx context.Context) {
		for _, eff := range spell.Effects {
			if eff.Effect == 0 {
				continue
			}
			switch eff.Effect {
			case 2, 87, 108, 17: // Damage effects (School damage, Weapon damage, etc.)
				damage := uint32(eff.BasePoints + 1)
				if damage <= 1 {
					damage = uint32(20 + int(s.player.Level)*10)
				}
				if targetGUID != 0 && targetGUID != s.playerGUID {
					s.executeSpellDamage(effCtx, targetGUID, spellID, damage)
				}
			case 10, 136, 105: // Heal effects
				heal := uint32(eff.BasePoints + 1)
				if heal == 0 {
					heal = uint32(30 + int(s.player.Level)*15)
				}
				s.executeSpellHeal(effCtx, targetGUID, spellID, heal)
			case 6, 27, 35: // Apply Aura
				durationMs := uint32(0)
				if spell.DurationIndex > 0 && s.server != nil && s.server.Data != nil {
					if val, ok, err := s.server.Data.SpellDuration(spell.DurationIndex, uint32(s.player.Level)); err == nil && ok && val > 0 {
						durationMs = uint32(val)
					}
				}
				if durationMs == 0 && eff.AuraPeriod > 0 {
					durationMs = eff.AuraPeriod * 5
				}
				if durationMs == 0 && eff.Effect == 6 && !isHarmfulAura(eff.Aura) {
					durationMs = 1800000 // 30 min default for passive buffs
				}
				periodMs := eff.AuraPeriod
				if periodMs == 0 && (eff.Aura == 3 || eff.Aura == 8 || eff.Aura == 24 || eff.Aura == 89) {
					periodMs = 3000
				}
				amount := uint32(eff.BasePoints + 1)
				if amount == 0 {
					if eff.Aura == 3 || eff.Aura == 89 {
						amount = uint32(10 + int(s.player.Level)*2)
					} else if eff.Aura == 8 || eff.Aura == 20 {
						amount = uint32(15 + int(s.player.Level)*3)
					}
				}
				// Spell power bonus for periodic effects (TrinityCore Unit::SpellDamageBonusDone / SpellHealingBonusDone)
				if s.player != nil && s.player.SpellPower > 0 && periodMs > 0 {
					if eff.Aura == 3 || eff.Aura == 89 || eff.Aura == 8 || eff.Aura == 20 {
						tickBonus := uint32(math.Round(float64(s.player.SpellPower) * (float64(periodMs) / 15000.0)))
						amount += tickBonus
					}
				}
				schoolMask := spell.SchoolMask
				if schoolMask == 0 {
					schoolMask = 1
				}

				auraTarget := s.playerGUID
				if eff.ImplicitTargetA == 1 || isSelfCastOnly(spell) {
					auraTarget = s.playerGUID
				} else if eff.ImplicitTargetA == 6 || isHarmfulAura(eff.Aura) {
					if targetGUID != 0 && targetGUID != s.playerGUID {
						auraTarget = targetGUID
					}
				} else if eff.ImplicitTargetA == 21 {
					if targetGUID != 0 {
						auraTarget = targetGUID
					}
				} else {
					if targetGUID != 0 && targetGUID != s.playerGUID && isHarmfulSpell(spell) {
						auraTarget = targetGUID
					}
				}
				s.applyAuraToTarget(effCtx, auraTarget, spell, eff, durationMs, periodMs, amount, schoolMask)
			case spellEffectResurrectNew: // SPELL_EFFECT_RESURRECT_NEW: self resurrect chain
				s.applySelfResurrectEffect(spell)
			case 5: // SPELL_EFFECT_TELEPORT_UNITS (e.g. Hearthstone 8690, Astral Recall 556)
				if (spellID == 8690 || spellID == 556) && s.player != nil {
					hbMap, hbX, hbY, hbZ := s.player.HomebindMap, s.player.HomebindX, s.player.HomebindY, s.player.HomebindZ
					if hbX == 0 && hbY == 0 && hbZ == 0 && s.server != nil && s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
						_ = s.server.WorldStore.DB.QueryRowContext(effCtx, "SELECT map, position_x, position_y, position_z FROM playercreateinfo WHERE race = ? AND class = ? LIMIT 1", s.player.Race, s.player.Class).Scan(&hbMap, &hbX, &hbY, &hbZ)
					}
					s.teleportTo(hbMap, hbX, hbY, hbZ, 0)
				}
			case 162: // SPELL_EFFECT_TALENT_SPEC_SELECT
				targetSpec := uint8(0)
				if eff.BasePoints+1 >= 2 {
					targetSpec = 1
				}
				s.activateSpec(effCtx, targetSpec)
			case 74: // SPELL_EFFECT_APPLY_GLYPH
				glyphPropID := uint16(eff.MiscValue)
				s.applyGlyph(effCtx, s.targetGlyphSlot, glyphPropID)
			case 56: // SPELL_EFFECT_SUMMON_PET
				s.handleSummonPet(effCtx, spellID, uint32(eff.MiscValue))
			case 28: // SPELL_EFFECT_SUMMON
				s.handleSummonPet(effCtx, spellID, uint32(eff.MiscValue))
			case 101: // SPELL_EFFECT_FEED_PET
				s.handleFeedPet(effCtx, spellID)
			case 102: // SPELL_EFFECT_DISMISS_PET
				s.handleDismissPet(effCtx)
			case 109: // SPELL_EFFECT_RESURRECT_PET
				s.handleResurrectPet(effCtx, spellID)
			case 54: // SPELL_EFFECT_TAMECREATURE
				s.handleTameCreature(effCtx, spellID, targetGUID)
			case 135: // SPELL_EFFECT_CALL_PET
				s.handleSummonPet(effCtx, spellID, 0)
			}
		}
		if spellID == 2641 { // Dismiss Pet
			s.handleDismissPet(effCtx)
		} else if spellID == 883 { // Call Pet
			s.handleSummonPet(effCtx, spellID, 0)
		} else if spellID == 31687 { // Summon Water Elemental
			s.handleSummonPet(effCtx, spellID, 510)
		} else if spellID == 46584 { // Raise Dead
			s.handleSummonPet(effCtx, spellID, 26125)
		} else if spellID == 63645 {
			s.activateSpec(effCtx, 0)
		} else if spellID == 63644 {
			s.activateSpec(effCtx, 1)
		} else if (spellID == 63624 || spellID == 63680) && s.player != nil && s.player.TalentGroupsCount < 2 {
			s.player.TalentGroupsCount = 2
			if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
				_, _ = s.server.CharactersStore.DB.ExecContext(effCtx, "UPDATE characters SET talentGroupsCount = 2 WHERE guid = ?", s.playerGUID)
			}
			_ = s.sendTalentsInfo(false)
		}
	}

	// Reference Spell.cpp:2156-2169:
	// If spell has projectile speed and target is not self, delay effect execution until missile arrival
	if spell.Speed > 0 && targetGUID != 0 && targetGUID != s.playerGUID {
		dist := float32(20.0) // default 20 yards if positions unknown
		if target, ok := s.getCombatTarget(ctx, targetGUID); ok {
			dx := target.X - s.player.X
			dy := target.Y - s.player.Y
			dz := target.Z - s.player.Z
			computedDist := float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
			if computedDist > 5.0 {
				dist = computedDist
			} else {
				dist = 5.0
			}
		}
		timeDelayMs := int(math.Floor(float64(dist) / float64(spell.Speed) * 1000.0))
		if timeDelayMs > 0 {
			if timeDelayMs > 4000 {
				timeDelayMs = 4000
			}
			time.AfterFunc(time.Duration(timeDelayMs)*time.Millisecond, func() {
				applyEffects(context.Background())
			})
			return
		}
	}

	applyEffects(ctx)
}

func (s *session) executeSpellDamage(ctx context.Context, targetGUID uint64, spellID, damage uint32) {
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	target, ok := s.getCombatTarget(ctx, targetGUID)
	if !ok {
		return
	}
	if target.Health == 0 {
		target.Health = 100
	}

	// Apply Spell Power bonus (TrinityCore Unit::SpellDamageBonusDone)
	if s.player != nil && s.player.SpellPower > 0 {
		coeff := 0.857 // standard 3.0s cast (~85.7%)
		if s.server.Data != nil {
			if spell, found, err := s.server.Data.Spell(spellID); err == nil && found {
				if spell.CastingTimeIndex > 0 {
					if ct, ok, _ := s.server.Data.SpellCastTime(spell.CastingTimeIndex); ok && ct > 0 {
						coeff = float64(ct) / 3500.0
						if coeff > 1.0 {
							coeff = 1.0
						}
					}
				} else {
					coeff = 1.5 / 3.5 // instant cast coefficient ~0.4286
				}
			}
		}
		damage += uint32(math.Round(float64(s.player.SpellPower) * coeff))
	}

	// Spell crit roll (5% base crit)
	crit := rand.Float64() < 0.05
	hitInfo := uint32(0)
	if crit {
		damage = uint32(float64(damage) * 1.5)
		hitInfo = 0x02 // SPELL_HIT_TYPE_CRIT
	}

	// Use the school mask from the Spell DBC (field 17). Fallback to physical (1).
	schoolMask := uint8(1)
	if s.server.Data != nil {
		if spell, found, err := s.server.Data.Spell(spellID); err == nil && found && spell.SchoolMask != 0 {
			schoolMask = uint8(spell.SchoolMask)
		}
	}

	resisted := uint32(0)
	if schoolMask > 1 && s.player != nil && s.player.Level > 0 {
		resIdx := schoolMaskToResistanceIndex(schoolMask)
		victimRes := target.Resistances[resIdx]
		resisted, damage = calcMagicSpellResistance(damage, schoolMask, victimRes, s.player.Level, target.Level)
	}

	overkill := uint32(0)
	if damage >= target.Health && target.Health > 0 {
		overkill = damage - target.Health
	}

	_ = s.write(uint16(protocol.OpcodeSMSG_SPELLNONMELEEDAMAGELOG), buildSpellNonMeleeDamageLog(target.GUID, s.playerGUID, spellID, damage, overkill, schoolMask, 0, resisted, hitInfo), true)

	s.lastCombatTime = time.Now()
	if s.player != nil && s.player.UnitFlags&unitFlagInCombat == 0 {
		s.player.UnitFlags |= unitFlagInCombat
		s.sendPlayerUpdate()
	}

	// If target is an online player (e.g. duel opponent or PvP)
	if s.server != nil {
		if playerSess := s.server.findSessionByGUID(target.GUID); playerSess != nil && playerSess.player != nil {
			playerSess.lastCombatTime = time.Now()
			if playerSess.player.UnitFlags&unitFlagInCombat == 0 {
				playerSess.player.UnitFlags |= unitFlagInCombat
			}
			_ = playerSess.write(uint16(protocol.OpcodeSMSG_SPELLNONMELEEDAMAGELOG), buildSpellNonMeleeDamageLog(target.GUID, s.playerGUID, spellID, damage, overkill, schoolMask, 0, resisted, hitInfo), true)
			if damage >= playerSess.player.Health {
				if s.duelPartner == target.GUID && s.player.DuelTeam != 0 {
					// Duel defeat: loser drops to 1 HP and duel completes
					playerSess.player.Health = 1
					playerSess.sendPlayerUpdate()
					s.endDuel(true, s.playerGUID, false)
				} else {
					playerSess.player.Health = 0
					playerSess.sendPlayerUpdate()
					playerSess.killPlayer(ctx)
				}
			} else {
				playerSess.player.Health -= damage
				playerSess.sendPlayerUpdate()
			}
			return
		}
	}

	low := uint32(target.GUID & 0x00FFFFFF)
	entry := uint32((target.GUID >> 24) & 0x00FFFFFF)
	stdKey := creatureWorldGUID(low, entry)

	if damage >= target.Health {
		// Target dies
		s.server.motionMu.Lock()
		motion := s.server.creatureMotion[target.GUID]
		if motion == nil {
			motion = s.server.creatureMotion[stdKey]
		}
		if motion != nil {
			motion.Health = 0
			motion.InCombat = false
			motion.TargetGUID = 0
			motion.Moving = false
			if motion.ThreatMgr != nil {
				motion.ThreatMgr.ClearThreat()
			}
		}
		s.server.motionMu.Unlock()

		s.server.stopCreatureMotion(target.Map, target.GUID, target.X, target.Y, target.Z)
		s.server.broadcastCreatureValuesUpdate(target.Map, target.GUID, map[int]uint32{
			unitFieldHealth:       0,
			unitFieldDynamicFlags: 1, // UNIT_DYNFLAG_LOOTABLE
		})
		s.server.broadcastThreatClear(target.Map, target.GUID)
		_ = s.sendAttackStop(target.GUID, true)
		s.attackTarget = 0
		s.onCreatureKilled(ctx, target)
		s.debug("target slain by spell", "account", s.accountName, "spell", spellID, "guid", target.GUID)
	} else {
		newHealth := target.Health - damage
		s.server.motionMu.Lock()
		motion := s.server.creatureMotion[target.GUID]
		if motion == nil {
			motion = s.server.creatureMotion[stdKey]
		}
		if motion != nil {
			motion.Health = newHealth
			motion.InCombat = true
			if motion.ThreatMgr == nil {
				motion.ThreatMgr = NewThreatManager(target.GUID)
			}
			if motion.BossAI == nil {
				motion.BossAI = getBossAIForCreature(motion, motion.ScriptName)
			}
			dist := distance3D(s.player.X, s.player.Y, s.player.Z, motion.X, motion.Y, motion.Z)
			inMelee := dist <= meleeAttackRange
			switched, newVictim := motion.ThreatMgr.AddThreat(s.playerGUID, float32(damage), inMelee)
			if switched && newVictim != motion.TargetGUID {
				motion.TargetGUID = newVictim
				entries := motion.ThreatMgr.SortedEntries()
				s.server.broadcastHighestThreatUpdate(motion.Map, motion.GUID, newVictim, entries)
			} else {
				motion.TargetGUID = motion.ThreatMgr.GetCurrentVictim()
			}
			if motion.BossAI != nil {
				motion.BossAI.OnDamageTaken(ctx, s.server, motion, s.playerGUID, damage)
			}
			motion.Moving = true
		}
		s.server.motionMu.Unlock()
		s.server.broadcastCreatureValuesUpdate(target.Map, target.GUID, map[int]uint32{unitFieldHealth: newHealth})
		s.server.triggerCreatureAggro(ctx, target.GUID, s.playerGUID)
	}
}

func (s *session) executeSpellHeal(ctx context.Context, targetGUID uint64, spellID, heal uint32) {
	if s.player == nil {
		return
	}
	if targetGUID == 0 {
		targetGUID = s.playerGUID
	}

	targetSess := s
	if targetGUID != s.playerGUID && s.server != nil {
		if other := s.server.findSessionByGUID(targetGUID); other != nil && other.player != nil {
			targetSess = other
		}
	}

	// Apply Spell Power bonus to healing (TrinityCore Unit::SpellHealingBonusDone)
	if s.player != nil && s.player.SpellPower > 0 {
		coeff := 0.857
		if s.server.Data != nil {
			if spell, found, err := s.server.Data.Spell(spellID); err == nil && found {
				if spell.CastingTimeIndex > 0 {
					if ct, ok, _ := s.server.Data.SpellCastTime(spell.CastingTimeIndex); ok && ct > 0 {
						coeff = float64(ct) / 3500.0
						if coeff > 1.0 {
							coeff = 1.0
						}
					}
				} else {
					coeff = 1.5 / 3.5
				}
			}
		}
		heal += uint32(math.Round(float64(s.player.SpellPower) * coeff))
	}

	effectiveHeal := heal
	if targetSess.player.Health+heal > targetSess.player.MaxHealth {
		effectiveHeal = targetSess.player.MaxHealth - targetSess.player.Health
		targetSess.player.Health = targetSess.player.MaxHealth
	} else {
		targetSess.player.Health += heal
	}
	overheal := heal - effectiveHeal

	// TC Unit.cpp:6550: packet = packed(target), packed(healer), spellID, heal, overheal, absorb, crit, unused
	healPkt := buildSpellHealLog(targetGUID, s.playerGUID, spellID, heal, overheal, 0, false)
	_ = s.write(uint16(protocol.OpcodeSMSG_SPELLHEALLOG), healPkt, true)
	if targetSess != s {
		_ = targetSess.write(uint16(protocol.OpcodeSMSG_SPELLHEALLOG), healPkt, true)
	}
	targetSess.sendPlayerUpdate()
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET health = ? WHERE guid = ?", targetSess.player.Health, targetSess.playerGUID)
	}
}

func buildSpellNonMeleeDamageLog(targetGUID, attackerGUID uint64, spellID, damage, overkill uint32, schoolMask uint8, extra ...uint32) []byte {
	// Layout matches TrinityCore Unit::SendSpellNonMeleeDamageLog (Unit.cpp:5302)
	// packed target, packed attacker, spellID, damage, overkill, schoolMask,
	// absorbed, resist, periodicLog, unused, blocked, HitInfo, HitInfo&debugMask
	absorb := uint32(0)
	resist := uint32(0)
	hitInfo := uint32(0)
	if len(extra) > 0 {
		absorb = extra[0]
	}
	if len(extra) > 1 {
		resist = extra[1]
	}
	if len(extra) > 2 {
		hitInfo = extra[2]
	}
	buf := protocol.NewBuffer(64)
	buf.WritePackedGUID(targetGUID)
	buf.WritePackedGUID(attackerGUID)
	buf.WriteU32(spellID)
	buf.WriteU32(damage)
	buf.WriteU32(overkill)
	buf.WriteU8(schoolMask)
	buf.WriteU32(absorb) // Absorbed
	buf.WriteU32(resist) // Resist
	buf.WriteU8(0)       // periodicLog (0 = show spell name prefix)
	buf.WriteU8(0)       // unused
	buf.WriteU32(0)       // blocked
	buf.WriteU32(hitInfo) // HitInfo flags (0 = normal hit, 2 = SPELL_HIT_TYPE_CRIT)
	buf.WriteU8(0)       // HitInfo & debugMask (always 0, no crit/hit debug)
	return buf.Bytes()
}

func buildSpellHealLog(targetGUID, healerGUID uint64, spellID, healAmount, overheal, absorb uint32, crit bool) []byte {
	buf := protocol.NewBuffer(32)
	buf.WritePackedGUID(targetGUID)
	buf.WritePackedGUID(healerGUID)
	buf.WriteU32(spellID)
	buf.WriteU32(healAmount)
	buf.WriteU32(overheal)
	buf.WriteU32(absorb)
	if crit {
		buf.WriteU8(1)
	} else {
		buf.WriteU8(0)
	}
	buf.WriteU8(0)
	return buf.Bytes()
}

func (s *session) hasActiveSpell(spellID uint32) bool {
	for _, spell := range s.player.Spells {
		if spell.ID == spellID {
			return spell.Active && !spell.Disabled
		}
	}
	return false
}

func spellCastIgnoreReason(spell wotlk.Spell, found, learned bool) string {
	if !found {
		return "unknown spell"
	}
	if spell.Attributes&spellAttributePassive != 0 {
		return "passive spell"
	}
	if !learned {
		return "spell not learned"
	}
	return "unavailable"
}

func (s *session) interruptCurrentCast() {
	s.castMu.Lock()
	if s.activeCast != nil {
		if s.activeCast.Timer != nil {
			s.activeCast.Timer.Stop()
		}
		s.activeCast.Cancelled = true
		castID := s.activeCast.CastID
		spellID := s.activeCast.SpellID
		s.activeCast = nil
		s.castMu.Unlock()

		_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(castID, spellID, 24), true) // SPELL_FAILED_INTERRUPTED = 24
		return
	}
	s.castMu.Unlock()
}

func (s *session) handleCancelCast(payload []byte) bool {
	reader := protocol.NewReader(payload)
	castID, _ := reader.ReadU8()
	spellID, _ := reader.ReadU32()

	s.castMu.Lock()
	if s.activeCast != nil {
		if s.activeCast.Timer != nil {
			s.activeCast.Timer.Stop()
		}
		s.activeCast.Cancelled = true
		curCastID := s.activeCast.CastID
		curSpellID := s.activeCast.SpellID
		s.activeCast = nil
		s.castMu.Unlock()

		_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(curCastID, curSpellID, 24), true) // SPELL_FAILED_INTERRUPTED = 24
		return true
	}
	s.castMu.Unlock()

	_ = s.write(uint16(protocol.OpcodeSMSG_CAST_FAILED), buildCastFailed(castID, spellID, 24), true)
	return true
}

func (s *session) handleCancelChanneling(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	// Reference: cancel clears the running channel and its bar.
	s.interruptCurrentChannel()
	return true
}

func (s *session) handleCancelAura(payload []byte) bool {
	reader := protocol.NewReader(payload)
	spellID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	s.removeAura(spellID)
	return true
}

// activeAura tracks an applied periodic or timed aura on a unit (player or creature).
type activeAura struct {
	SpellID     uint32
	AuraType    uint32
	CasterGUID  uint64
	TargetGUID  uint64
	SchoolMask  uint32
	Amount      uint32
	DurationMs  uint32
	PeriodMs    uint32
	RemainingMs uint32
	Slot        uint8
	Positive    bool
	CasterLevel uint8
	Timer       *time.Timer
	TickTimer   *time.Timer
	Stopped     bool
}

func isHarmfulAura(auraType uint32) bool {
	switch auraType {
	case 3: // SPELL_AURA_PERIODIC_DAMAGE
		return true
	case 5: // SPELL_AURA_MOD_CONFUSE
		return true
	case 6: // SPELL_AURA_MOD_CHARM
		return true
	case 7: // SPELL_AURA_MOD_FEAR
		return true
	case 11: // SPELL_AURA_MOD_TAUNT
		return true
	case 12: // SPELL_AURA_MOD_STUN
		return true
	case 26: // SPELL_AURA_MOD_ROOT
		return true
	case 27: // SPELL_AURA_MOD_SILENCE
		return true
	case 33: // SPELL_AURA_MOD_DECREASE_SPEED
		return true
	case 53: // SPELL_AURA_PERIODIC_LEECH
		return true
	case 89: // SPELL_AURA_PERIODIC_DAMAGE_PERCENT
		return true
	default:
		return false
	}
}

func isHarmfulSpell(spell wotlk.Spell) bool {
	for _, eff := range spell.Effects {
		if eff.Effect == 0 {
			continue
		}
		if eff.Effect == 2 || eff.Effect == 87 || eff.Effect == 108 || eff.Effect == 17 {
			return true
		}
		if eff.Effect == 6 && isHarmfulAura(eff.Aura) {
			return true
		}
		if eff.ImplicitTargetA == 6 || eff.ImplicitTargetA == 15 || eff.ImplicitTargetA == 16 {
			return true
		}
	}
	return false
}

func magicSpellHitResult(casterLevel, victimLevel uint8, isPlayerVictim bool) uint8 {
	lchance := int32(11)
	if isPlayerVictim {
		lchance = 7
	}
	leveldif := int32(victimLevel) - int32(casterLevel)
	var modHitChance int32
	if leveldif < 3 {
		modHitChance = 96 - leveldif
	} else {
		modHitChance = 94 - (leveldif-2)*lchance
	}
	if modHitChance < 1 {
		modHitChance = 1
	} else if modHitChance > 100 {
		modHitChance = 100
	}
	roll := rand.IntN(100)
	if roll >= int(modHitChance) {
		return protocol.SpellMissMiss
	}
	return protocol.SpellMissNone
}

// schoolMaskToResistanceIndex converts SpellSchoolMask to resistance index:
// 0: Physical (Armor), 1: Holy, 2: Fire, 3: Nature, 4: Frost, 5: Shadow, 6: Arcane.
func schoolMaskToResistanceIndex(schoolMask uint8) uint8 {
	switch {
	case schoolMask&1 != 0:
		return 0
	case schoolMask&2 != 0:
		return 1
	case schoolMask&4 != 0:
		return 2
	case schoolMask&8 != 0:
		return 3
	case schoolMask&16 != 0:
		return 4
	case schoolMask&32 != 0:
		return 5
	case schoolMask&64 != 0:
		return 6
	default:
		return 0
	}
}

// calcMagicSpellResistance computes magic damage resisted based on target resistance and caster/victim levels,
// matching TrinityCore Unit::CalculateAverageResistReduction and Unit::CalcSpellResistance (Unit.cpp:1721-1775).
func calcMagicSpellResistance(damage uint32, schoolMask uint8, victimResistance uint32, casterLevel, victimLevel uint8) (resisted uint32, remainingDamage uint32) {
	if damage == 0 || schoolMask == 0 || schoolMask&1 != 0 {
		return 0, damage
	}
	if schoolMask&2 != 0 {
		// Holy damage cannot be resisted in WotLK
		return 0, damage
	}

	res := float64(victimResistance)
	// Level-based resistance: 5 resistance per level difference if victim is higher level
	if victimLevel > casterLevel {
		res += float64(victimLevel-casterLevel) * 5.0
	}

	const bossLevel = 83
	const bossResistanceConstant = 510.0
	resConstant := float64(victimLevel) * 5.0
	if victimLevel >= bossLevel {
		resConstant = bossResistanceConstant
	}
	if resConstant < 5.0 {
		resConstant = 5.0
	}

	averageResist := res / (res + resConstant)
	if averageResist <= 0.0 {
		return 0, damage
	}
	if averageResist > 1.0 {
		averageResist = 1.0
	}

	var discreteProb [11]float64
	if averageResist <= 0.1 {
		discreteProb[0] = 1.0 - 7.5*averageResist
		discreteProb[1] = 5.0 * averageResist
		discreteProb[2] = 2.5 * averageResist
	} else {
		for i := 0; i < 11; i++ {
			p := 0.5 - 2.5*math.Abs(0.1*float64(i)-averageResist)
			if p > 0 {
				discreteProb[i] = p
			}
		}
	}

	roll := rand.Float64()
	probSum := 0.0
	step := 0
	for i := 0; i < 11; i++ {
		probSum += discreteProb[i]
		if roll < probSum {
			step = i
			break
		}
	}

	resPercent := float64(step) * 0.1
	resisted = uint32(math.Round(float64(damage) * resPercent))
	if resisted > damage {
		resisted = damage
	}
	remainingDamage = damage - resisted
	return resisted, remainingDamage
}

func (s *Server) clearCreatureAuras(guid uint64) {
	if s == nil {
		return
	}
	s.auraMu.Lock()
	defer s.auraMu.Unlock()
	if s.activeCreatureAuras != nil {
		if auras, ok := s.activeCreatureAuras[guid]; ok {
			for _, aura := range auras {
				if aura != nil {
					aura.Stopped = true
					if aura.Timer != nil {
						aura.Timer.Stop()
					}
					if aura.TickTimer != nil {
						aura.TickTimer.Stop()
					}
				}
			}
			delete(s.activeCreatureAuras, guid)
		}
	}
	if s.creatureAuras != nil {
		delete(s.creatureAuras, guid)
	}
}

func (s *session) clearActiveAuras() {
	if s == nil {
		return
	}
	s.castMu.Lock()
	defer s.castMu.Unlock()
	if s.activeAuras != nil {
		for _, aura := range s.activeAuras {
			if aura != nil {
				aura.Stopped = true
				if aura.Timer != nil {
					aura.Timer.Stop()
				}
				if aura.TickTimer != nil {
					aura.TickTimer.Stop()
				}
			}
		}
		s.activeAuras = make(map[uint32]*activeAura)
	}
}

func (s *session) sendAuraUpdate(slot uint8, spellID uint32, remove bool, maxDurationMs, durationMs uint32) {
	level := uint8(1)
	if s.player != nil && s.player.Level > 0 {
		level = s.player.Level
	}
	pkt := protocol.BuildAuraUpdate(s.playerGUID, s.playerGUID, slot, spellID, remove, true, maxDurationMs, durationMs, level)
	_ = s.write(uint16(protocol.OpcodeSMSG_AURA_UPDATE), pkt, true)
}

func (s *session) applyAura(spellID uint32) {
	s.applyAuraWithDuration(spellID, 1800000)
}

func (s *session) applyAuraWithDuration(spellID uint32, durationMs uint32) {
	if s.auras == nil {
		s.auras = make(map[uint32]struct{})
	}
	if s.auraSlots == nil {
		s.auraSlots = make(map[uint32]uint8)
	}
	s.auras[spellID] = struct{}{}

	slot, ok := s.auraSlots[spellID]
	if !ok {
		used := make(map[uint8]bool)
		for _, sl := range s.auraSlots {
			used[sl] = true
		}
		var freeSlot uint8
		for sl := uint8(0); sl < 64; sl++ {
			if !used[sl] {
				freeSlot = sl
				break
			}
		}
		slot = freeSlot
		s.auraSlots[spellID] = slot
	}

	s.castMu.Lock()
	if s.activeAuras == nil {
		s.activeAuras = make(map[uint32]*activeAura)
	}
	if existing, exists := s.activeAuras[spellID]; exists && existing != nil {
		existing.Stopped = true
		if existing.Timer != nil {
			existing.Timer.Stop()
		}
		if existing.TickTimer != nil {
			existing.TickTimer.Stop()
		}
	}
	aura := &activeAura{
		SpellID:     spellID,
		CasterGUID:  s.playerGUID,
		TargetGUID:  s.playerGUID,
		DurationMs:  durationMs,
		RemainingMs: durationMs,
		Slot:        slot,
		Positive:    true,
	}
	if durationMs > 0 && durationMs < 18000000 {
		aura.Timer = time.AfterFunc(time.Duration(durationMs)*time.Millisecond, func() {
			s.removeAura(spellID)
		})
	}
	s.activeAuras[spellID] = aura
	s.castMu.Unlock()

	s.sendAuraUpdate(slot, spellID, false, durationMs, durationMs)
	s.sendPlayerUpdate()
}

func (s *session) removeAura(spellID uint32) {
	s.castMu.Lock()
	if s.activeAuras != nil {
		if aura, ok := s.activeAuras[spellID]; ok && aura != nil {
			aura.Stopped = true
			if aura.Timer != nil {
				aura.Timer.Stop()
			}
			if aura.TickTimer != nil {
				aura.TickTimer.Stop()
			}
			delete(s.activeAuras, spellID)
		}
	}
	s.castMu.Unlock()

	if s.auras != nil {
		delete(s.auras, spellID)
	}
	if s.auraSlots != nil {
		if slot, ok := s.auraSlots[spellID]; ok {
			s.sendAuraUpdate(slot, 0, true, 0, 0)
			delete(s.auraSlots, spellID)
		}
	}
	s.sendPlayerUpdate()
}

func (s *session) hasAura(spellID uint32) bool {
	if s.auras == nil {
		return false
	}
	_, ok := s.auras[spellID]
	return ok
}

func (s *session) applyAuraToTarget(ctx context.Context, targetGUID uint64, spell wotlk.Spell, eff wotlk.SpellEffect, durationMs, periodMs, amount, schoolMask uint32) {
	if s.player == nil {
		return
	}
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}

	positive := !isHarmfulAura(eff.Aura) && eff.ImplicitTargetA != 6

	// Target is a player (self or other online player)
	var targetSess *session
	if targetGUID == s.playerGUID || targetGUID == 0 {
		targetSess = s
		targetGUID = s.playerGUID
	} else if s.server != nil {
		targetSess = s.server.findSessionByGUID(targetGUID)
	}

	if targetSess != nil && targetSess.player != nil {
		if targetSess.player.Health == 0 {
			return
		}
		targetSess.castMu.Lock()
		if targetSess.auras == nil {
			targetSess.auras = make(map[uint32]struct{})
		}
		if targetSess.auraSlots == nil {
			targetSess.auraSlots = make(map[uint32]uint8)
		}
		if targetSess.activeAuras == nil {
			targetSess.activeAuras = make(map[uint32]*activeAura)
		}

		if existing, exists := targetSess.activeAuras[spell.ID]; exists && existing != nil {
			existing.Stopped = true
			if existing.Timer != nil {
				existing.Timer.Stop()
			}
			if existing.TickTimer != nil {
				existing.TickTimer.Stop()
			}
		}

		targetSess.auras[spell.ID] = struct{}{}
		slot, ok := targetSess.auraSlots[spell.ID]
		if !ok {
			used := make(map[uint8]bool)
			for _, sl := range targetSess.auraSlots {
				used[sl] = true
			}
			var freeSlot uint8
			for sl := uint8(0); sl < 64; sl++ {
				if !used[sl] {
					freeSlot = sl
					break
				}
			}
			slot = freeSlot
			targetSess.auraSlots[spell.ID] = slot
		}

		aura := &activeAura{
			SpellID:     spell.ID,
			AuraType:    eff.Aura,
			CasterGUID:  s.playerGUID,
			TargetGUID:  targetGUID,
			SchoolMask:  schoolMask,
			Amount:      amount,
			DurationMs:  durationMs,
			PeriodMs:    periodMs,
			RemainingMs: durationMs,
			Slot:        slot,
			Positive:    positive,
			CasterLevel: s.player.Level,
		}
		targetSess.activeAuras[spell.ID] = aura
		targetSess.castMu.Unlock()

		updatePkt := protocol.BuildAuraUpdate(targetGUID, s.playerGUID, slot, spell.ID, false, positive, durationMs, durationMs, s.player.Level)
		_ = targetSess.write(uint16(protocol.OpcodeSMSG_AURA_UPDATE), updatePkt, true)
		if s.server != nil {
			s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_AURA_UPDATE), updatePkt, targetSess)
		}
		targetSess.sendPlayerUpdate()

		if periodMs > 0 {
			targetSess.schedulePlayerPeriodicTick(aura, periodMs)
		}
		if durationMs > 0 && durationMs < 18000000 {
			aura.Timer = time.AfterFunc(time.Duration(durationMs)*time.Millisecond, func() {
				targetSess.expirePlayerAura(spell.ID)
			})
		}
		return
	}

	// Target is a creature in the world
	target, ok := s.getCombatTarget(ctx, targetGUID)
	if !ok || target.Health == 0 {
		return
	}

	if s.server == nil {
		return
	}
	s.server.auraMu.Lock()
	if s.server.creatureAuras == nil {
		s.server.creatureAuras = make(map[uint64]map[uint32]struct{})
	}
	if s.server.creatureAuras[targetGUID] == nil {
		s.server.creatureAuras[targetGUID] = make(map[uint32]struct{})
	}
	s.server.creatureAuras[targetGUID][spell.ID] = struct{}{}

	if s.server.activeCreatureAuras == nil {
		s.server.activeCreatureAuras = make(map[uint64]map[uint32]*activeAura)
	}
	if s.server.activeCreatureAuras[targetGUID] == nil {
		s.server.activeCreatureAuras[targetGUID] = make(map[uint32]*activeAura)
	}
	if existing, exists := s.server.activeCreatureAuras[targetGUID][spell.ID]; exists && existing != nil {
		existing.Stopped = true
		if existing.Timer != nil {
			existing.Timer.Stop()
		}
		if existing.TickTimer != nil {
			existing.TickTimer.Stop()
		}
	}
	slot := uint8(len(s.server.activeCreatureAuras[targetGUID]) % 64)
	aura := &activeAura{
		SpellID:     spell.ID,
		AuraType:    eff.Aura,
		CasterGUID:  s.playerGUID,
		TargetGUID:  targetGUID,
		SchoolMask:  schoolMask,
		Amount:      amount,
		DurationMs:  durationMs,
		PeriodMs:    periodMs,
		RemainingMs: durationMs,
		Slot:        slot,
		Positive:    positive,
		CasterLevel: s.player.Level,
	}
	s.server.activeCreatureAuras[targetGUID][spell.ID] = aura
	s.server.auraMu.Unlock()

	updatePkt := protocol.BuildAuraUpdate(targetGUID, s.playerGUID, slot, spell.ID, false, positive, durationMs, durationMs, s.player.Level)
	_ = s.write(uint16(protocol.OpcodeSMSG_AURA_UPDATE), updatePkt, true)
	s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_AURA_UPDATE), updatePkt, s)

	if periodMs > 0 {
		s.scheduleCreaturePeriodicTick(aura, periodMs)
	}
	if durationMs > 0 && durationMs < 18000000 {
		aura.Timer = time.AfterFunc(time.Duration(durationMs)*time.Millisecond, func() {
			s.expireCreatureAura(targetGUID, spell.ID, slot)
		})
	}
}

func (ts *session) schedulePlayerPeriodicTick(aura *activeAura, periodMs uint32) {
	aura.TickTimer = time.AfterFunc(time.Duration(periodMs)*time.Millisecond, func() {
		ts.castMu.Lock()
		if aura.Stopped || ts.player == nil || ts.player.Health == 0 {
			ts.castMu.Unlock()
			return
		}
		if aura.RemainingMs >= periodMs {
			aura.RemainingMs -= periodMs
		} else {
			aura.RemainingMs = 0
		}
		stillRunning := aura.RemainingMs > 0 || aura.DurationMs == 0
		ts.castMu.Unlock()

		ts.executePeriodicTickOnPlayer(aura)

		if stillRunning {
			ts.castMu.Lock()
			if !aura.Stopped {
				ts.schedulePlayerPeriodicTick(aura, periodMs)
			}
			ts.castMu.Unlock()
		}
	})
}

func (ts *session) executePeriodicTickOnPlayer(aura *activeAura) {
	if ts.player == nil || ts.player.Health == 0 {
		return
	}

	switch aura.AuraType {
	case 3, 89: // SPELL_AURA_PERIODIC_DAMAGE, SPELL_AURA_PERIODIC_DAMAGE_PERCENT
		dmg := aura.Amount
		resisted := uint32(0)
		if aura.SchoolMask&1 != 0 && ts.player.Armor > 0 {
			dmg = calcArmorReducedDamage(float64(ts.player.Armor), aura.CasterLevel, dmg)
		} else if aura.SchoolMask > 1 && aura.CasterLevel > 0 {
			resIdx := schoolMaskToResistanceIndex(uint8(aura.SchoolMask))
			vRes := ts.player.Resistances[resIdx]
			resisted, dmg = calcMagicSpellResistance(dmg, uint8(aura.SchoolMask), vRes, aura.CasterLevel, ts.player.Level)
		}
		if dmg < 1 && resisted == 0 {
			dmg = 1
		}
		targetHealth := ts.player.Health
		overkill := uint32(0)
		if dmg >= targetHealth && targetHealth > 0 {
			overkill = dmg - targetHealth
		}

		logPkt := protocol.BuildPeriodicAuraLogDamage(aura.TargetGUID, aura.CasterGUID, aura.SpellID, aura.AuraType, dmg, overkill, aura.SchoolMask, 0, resisted, false)
		_ = ts.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
		if ts.server != nil {
			if casterSess := ts.server.findSessionByGUID(aura.CasterGUID); casterSess != nil && casterSess != ts {
				_ = casterSess.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
			}
			ts.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, ts)
		}

		if dmg >= targetHealth {
			if ts.duelPartner != 0 && ts.player.DuelTeam != 0 {
				ts.player.Health = 1
				ts.sendPlayerUpdate()
				if ts.server != nil {
					if casterSess := ts.server.findSessionByGUID(aura.CasterGUID); casterSess != nil {
						casterSess.endDuel(true, casterSess.playerGUID, false)
					}
				}
			} else {
				ts.player.Health = 0
				ts.sendPlayerUpdate()
				ts.killPlayer(context.Background())
			}
			ts.clearActiveAuras()
		} else {
			ts.player.Health -= dmg
			ts.sendPlayerUpdate()
		}

	case 8, 20: // SPELL_AURA_PERIODIC_HEAL, SPELL_AURA_OBS_MOD_HEALTH
		heal := aura.Amount
		curHP := ts.player.Health
		maxHP := ts.player.MaxHealth
		newHP := curHP + heal
		overheal := uint32(0)
		if newHP > maxHP {
			overheal = newHP - maxHP
			newHP = maxHP
		}

		logPkt := protocol.BuildPeriodicAuraLogHeal(aura.TargetGUID, aura.CasterGUID, aura.SpellID, aura.AuraType, heal, overheal, 0, false)
		_ = ts.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
		if ts.server != nil {
			if casterSess := ts.server.findSessionByGUID(aura.CasterGUID); casterSess != nil && casterSess != ts {
				_ = casterSess.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
			}
			ts.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, ts)
		}

		ts.player.Health = newHP
		ts.sendPlayerUpdate()

	case 24: // SPELL_AURA_PERIODIC_ENERGIZE
		logPkt := protocol.BuildPeriodicAuraLogEnergize(aura.TargetGUID, aura.CasterGUID, aura.SpellID, aura.AuraType, 0, aura.Amount)
		_ = ts.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
		if ts.server != nil {
			if casterSess := ts.server.findSessionByGUID(aura.CasterGUID); casterSess != nil && casterSess != ts {
				_ = casterSess.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
			}
			ts.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, ts)
		}
	}
}

func (ts *session) expirePlayerAura(spellID uint32) {
	ts.castMu.Lock()
	if ts.activeAuras != nil {
		if aura, ok := ts.activeAuras[spellID]; ok && aura != nil {
			aura.Stopped = true
			if aura.TickTimer != nil {
				aura.TickTimer.Stop()
			}
			delete(ts.activeAuras, spellID)
		}
	}
	ts.castMu.Unlock()
	ts.removeAura(spellID)
}

func (s *session) scheduleCreaturePeriodicTick(aura *activeAura, periodMs uint32) {
	aura.TickTimer = time.AfterFunc(time.Duration(periodMs)*time.Millisecond, func() {
		if s.server == nil {
			return
		}
		s.server.auraMu.Lock()
		if aura.Stopped {
			s.server.auraMu.Unlock()
			return
		}
		if aura.RemainingMs >= periodMs {
			aura.RemainingMs -= periodMs
		} else {
			aura.RemainingMs = 0
		}
		stillRunning := aura.RemainingMs > 0 || aura.DurationMs == 0
		s.server.auraMu.Unlock()

		targetAlive := s.executePeriodicTickOnCreature(aura)
		if !targetAlive {
			return
		}

		if stillRunning {
			s.server.auraMu.Lock()
			if !aura.Stopped {
				s.scheduleCreaturePeriodicTick(aura, periodMs)
			}
			s.server.auraMu.Unlock()
		}
	})
}

func (s *session) executePeriodicTickOnCreature(aura *activeAura) bool {
	ctx := context.Background()
	target, ok := s.getCombatTarget(ctx, aura.TargetGUID)
	if !ok || target.Health == 0 {
		if s.server != nil {
			s.server.clearCreatureAuras(aura.TargetGUID)
		}
		return false
	}

	switch aura.AuraType {
	case 3, 89: // SPELL_AURA_PERIODIC_DAMAGE, SPELL_AURA_PERIODIC_DAMAGE_PERCENT
		dmg := aura.Amount
		resisted := uint32(0)
		if aura.SchoolMask&1 != 0 && target.Armor > 0 {
			dmg = calcArmorReducedDamage(float64(target.Armor), aura.CasterLevel, dmg)
		} else if aura.SchoolMask > 1 && aura.CasterLevel > 0 {
			resIdx := schoolMaskToResistanceIndex(uint8(aura.SchoolMask))
			vRes := target.Resistances[resIdx]
			resisted, dmg = calcMagicSpellResistance(dmg, uint8(aura.SchoolMask), vRes, aura.CasterLevel, target.Level)
		}
		if dmg < 1 && resisted == 0 {
			dmg = 1
		}
		targetHealth := target.Health
		overkill := uint32(0)
		if dmg >= targetHealth && targetHealth > 0 {
			overkill = dmg - targetHealth
		}

		logPkt := protocol.BuildPeriodicAuraLogDamage(aura.TargetGUID, aura.CasterGUID, aura.SpellID, aura.AuraType, dmg, overkill, aura.SchoolMask, 0, resisted, false)
		_ = s.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
		if s.server != nil {
			s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, s)
		}

		if dmg >= targetHealth {
			// Target slain by DoT
			if s.server != nil {
				s.server.motionMu.Lock()
				motion := s.server.creatureMotion[aura.TargetGUID]
				if motion == nil {
					low := uint32(aura.TargetGUID & 0x00FFFFFF)
					entry := uint32((aura.TargetGUID >> 24) & 0x00FFFFFF)
					motion = s.server.creatureMotion[creatureWorldGUID(low, entry)]
				}
				if motion != nil {
					motion.Health = 0
					motion.InCombat = false
					motion.TargetGUID = 0
					motion.Moving = false
					if motion.ThreatMgr != nil {
						motion.ThreatMgr.ClearThreat()
					}
				}
				s.server.motionMu.Unlock()

				s.server.stopCreatureMotion(target.Map, target.GUID, target.X, target.Y, target.Z)
				s.server.broadcastCreatureValuesUpdate(target.Map, target.GUID, map[int]uint32{
					unitFieldHealth:       0,
					unitFieldDynamicFlags: 1, // UNIT_DYNFLAG_LOOTABLE
				})
				s.server.broadcastThreatClear(target.Map, target.GUID)
				s.server.clearCreatureAuras(aura.TargetGUID)
			}
			_ = s.sendAttackStop(target.GUID, true)
			s.attackTarget = 0
			s.onCreatureKilled(ctx, target)
			return false
		} else {
			newHealth := targetHealth - dmg
			if s.server != nil {
				s.server.motionMu.Lock()
				motion := s.server.creatureMotion[aura.TargetGUID]
				if motion == nil {
					low := uint32(aura.TargetGUID & 0x00FFFFFF)
					entry := uint32((aura.TargetGUID >> 24) & 0x00FFFFFF)
					motion = s.server.creatureMotion[creatureWorldGUID(low, entry)]
				}
				if motion != nil {
					motion.Health = newHealth
					motion.InCombat = true
					if motion.ThreatMgr == nil {
						motion.ThreatMgr = NewThreatManager(target.GUID)
					}
					dist := distance3D(s.player.X, s.player.Y, s.player.Z, motion.X, motion.Y, motion.Z)
					inMelee := dist <= meleeAttackRange
					motion.ThreatMgr.AddThreat(s.playerGUID, float32(dmg), inMelee)
					motion.Moving = true
				}
				s.server.motionMu.Unlock()
				s.server.broadcastCreatureValuesUpdate(target.Map, target.GUID, map[int]uint32{unitFieldHealth: newHealth})
				s.server.triggerCreatureAggro(ctx, target.GUID, s.playerGUID)
			}
			return true
		}

	case 8, 20: // SPELL_AURA_PERIODIC_HEAL, SPELL_AURA_OBS_MOD_HEALTH
		heal := aura.Amount
		curHP := target.Health
		maxHP := target.MaxHealth
		newHP := curHP + heal
		overheal := uint32(0)
		if newHP > maxHP {
			overheal = newHP - maxHP
			newHP = maxHP
		}

		logPkt := protocol.BuildPeriodicAuraLogHeal(aura.TargetGUID, aura.CasterGUID, aura.SpellID, aura.AuraType, heal, overheal, 0, false)
		_ = s.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
		if s.server != nil {
			s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, s)
			s.server.motionMu.Lock()
			motion := s.server.creatureMotion[aura.TargetGUID]
			if motion != nil {
				motion.Health = newHP
			}
			s.server.motionMu.Unlock()
			s.server.broadcastCreatureValuesUpdate(target.Map, target.GUID, map[int]uint32{unitFieldHealth: newHP})
		}
		return true

	case 24: // SPELL_AURA_PERIODIC_ENERGIZE
		logPkt := protocol.BuildPeriodicAuraLogEnergize(aura.TargetGUID, aura.CasterGUID, aura.SpellID, aura.AuraType, 0, aura.Amount)
		_ = s.write(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, true)
		if s.server != nil {
			s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_PERIODICAURALOG), logPkt, s)
		}
		return true
	}
	return true
}

func (s *session) expireCreatureAura(creatureGUID uint64, spellID uint32, slot uint8) {
	if s.server == nil {
		return
	}
	s.server.auraMu.Lock()
	if s.server.activeCreatureAuras != nil {
		if auras, ok := s.server.activeCreatureAuras[creatureGUID]; ok {
			if aura, exists := auras[spellID]; exists && aura != nil {
				aura.Stopped = true
				if aura.TickTimer != nil {
					aura.TickTimer.Stop()
				}
				delete(auras, spellID)
			}
		}
	}
	if s.server.creatureAuras != nil {
		if auras, ok := s.server.creatureAuras[creatureGUID]; ok {
			delete(auras, spellID)
		}
	}
	s.server.auraMu.Unlock()

	removePkt := protocol.BuildAuraUpdate(creatureGUID, s.playerGUID, slot, 0, true, false, 0, 0, 1)
	_ = s.write(uint16(protocol.OpcodeSMSG_AURA_UPDATE), removePkt, true)
	s.server.broadcastToNearby(uint16(protocol.OpcodeSMSG_AURA_UPDATE), removePkt, s)
}

// handleCancelMountAura processes CMSG_CANCEL_MOUNT_AURA (0x375).
// Reference: WorldSession::HandleCancelMountAuraOpcode (SpellHandler.cpp:544).
func (s *session) handleCancelMountAura(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	if s.player.MountDisplayID != 0 {
		s.player.MountDisplayID = 0
		s.sendPlayerUpdate()
	}
	return true
}

// handleCancelGrowthAura processes CMSG_CANCEL_GROWTH_AURA (0x29B).
// Reference: WorldSession::HandleCancelGrowthAuraOpcode (SpellHandler.cpp:535).
func (s *session) handleCancelGrowthAura(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	if s.scale != 1.0 {
		s.scale = 1.0
		s.sendPlayerUpdate()
	}
	return true
}

// handleCancelAutoRepeatSpell processes CMSG_CANCEL_AUTO_REPEAT_SPELL (0x26D).
// Reference: WorldSession::HandleCancelAutoRepeatSpellOpcode (SpellHandler.cpp:553) and CombatPackets.h:115.
func (s *session) handleCancelAutoRepeatSpell(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	buf := protocol.NewBuffer(9)
	buf.WritePackedGUID(s.playerGUID)
	_ = s.write(uint16(protocol.OpcodeSMSG_CANCEL_AUTO_REPEAT), buf.Bytes(), true)
	return true
}

// handleCancelTempEnchantment processes CMSG_CANCEL_TEMP_ENCHANTMENT (0x379).
// Reference: WorldSession::HandleCancelTempEnchantmentOpcode (ItemHandler.cpp:1145).
func (s *session) handleCancelTempEnchantment(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	r := protocol.NewReader(payload)
	slot, err := r.ReadU32()
	if err != nil {
		return false
	}
	_ = slot
	s.sendPlayerUpdate()
	return true
}

// handleCorpseMapPositionQuery processes CMSG_CORPSE_MAP_POSITION_QUERY (0x4B6).
// Reference: WorldSession::HandleCorpseMapPositionQuery (QueryHandler.cpp:317).
func (s *session) handleCorpseMapPositionQuery(payload []byte) bool {
	r := protocol.NewReader(payload)
	_, _ = r.ReadU32() // unk

	buf := protocol.NewBuffer(16)
	buf.WriteF32(0)
	buf.WriteF32(0)
	buf.WriteF32(0)
	buf.WriteF32(0)
	return s.write(uint16(protocol.OpcodeSMSG_CORPSE_MAP_POSITION_QUERY_RESPONSE), buf.Bytes(), true) == nil
}

func buildCastFailed(castID uint8, spellID uint32, result uint8) []byte {
	buf := protocol.NewBuffer(6)
	buf.WriteU8(castID)
	buf.WriteU32(spellID)
	buf.WriteU8(result)
	return buf.Bytes()
}

func buildSpellCooldown(playerGUID uint64, spellID uint32, cooldownDurationMs uint32) []byte {
	buf := protocol.NewBuffer(8 + 1 + 4 + 4)
	buf.WriteU64(playerGUID)
	buf.WriteU8(0) // flags = 0
	buf.WriteU32(spellID)
	buf.WriteU32(cooldownDurationMs)
	return buf.Bytes()
}

// handleFarSight processes CMSG_FAR_SIGHT (0x27A).
// Reference: WorldSession::HandleFarSightOpcode (SpellHandler.cpp).
func (s *session) handleFarSight(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 1 {
		return true
	}
	op := payload[0]
	s.debug("far sight opcode", "account", s.accountName, "op", op)
	return true
}

// handleGetMirrorImageData processes CMSG_GET_MIRRORIMAGE_DATA (0x401).
// Reference: WorldSession::HandleMirrorImageDataRequest (SpellHandler.cpp:635).
func (s *session) handleGetMirrorImageData(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	guid, _ := r.ReadU64()

	buf := protocol.NewBuffer(68)
	buf.WriteU64(guid)
	buf.WriteU32(0) // displayId
	buf.WriteU8(s.player.Race)
	buf.WriteU8(s.player.Gender)
	buf.WriteU8(s.player.Class)
	buf.WriteU8(s.player.Skin)
	buf.WriteU8(s.player.Face)
	buf.WriteU8(s.player.HairStyle)
	buf.WriteU8(s.player.HairColor)
	buf.WriteU8(s.player.FacialStyle)
	buf.WriteU32(0) // guildId
	for i := 0; i < 11; i++ {
		buf.WriteU32(0) // outfit item displays
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_MIRRORIMAGE_DATA), buf.Bytes(), true)
	return true
}

// handleTotemDestroyed processes CMSG_TOTEM_DESTROYED (0x413).
// Reference: WorldSession::HandleTotemDestroyed (SpellHandler.cpp:582).
func (s *session) handleTotemDestroyed(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 1 {
		return true
	}
	slotID := payload[0]
	if slotID >= 4 {
		return true
	}
	s.player.TotemSlots[slotID] = 0

	// Broadcast SMSG_TOTEM_CREATED with duration 0 and empty GUID to clear client totem frame
	resp := protocol.NewBuffer(17)
	resp.WriteU8(slotID)
	resp.WriteU64(0)
	resp.WriteU32(0)
	resp.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_TOTEM_CREATED), resp.Bytes(), true)
	s.debug("totem destroyed", "account", s.accountName, "slot", slotID)
	return true
}

// handleSpellClick processes CMSG_SPELLCLICK (0x410).
// Reference: WorldSession::HandleSpellClick (SpellHandler.cpp:616) -> Unit::HandleSpellClick (Unit.cpp:12982).
func (s *session) handleSpellClick(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	targetGUID, err := r.ReadU64()
	if err != nil || targetGUID == 0 {
		return true
	}

	npcEntry := uint32((targetGUID >> 24) & 0x00FFFFFF)
	s.debug("spell click", "account", s.accountName, "target", targetGUID, "entry", npcEntry)

	if s.server == nil || s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		return true
	}

	rows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT spell_id, cast_flags, user_type FROM npc_spellclick_spells WHERE npc_entry = ?", npcEntry)
	if err != nil {
		return true
	}
	defer rows.Close()

	type spellClick struct {
		spellID   uint32
		castFlags uint8
		userType  uint8
	}
	var clicks []spellClick
	for rows.Next() {
		var sp, cf, ut uint32
		if err := rows.Scan(&sp, &cf, &ut); err == nil && sp > 0 {
			clicks = append(clicks, spellClick{
				spellID:   sp,
				castFlags: uint8(cf),
				userType:  uint8(ut),
			})
		}
	}

	for _, click := range clicks {
		targetUnit := targetGUID
		if click.castFlags&0x02 != 0 { // NPC_CLICK_CAST_TARGET_CLICKER
			targetUnit = s.playerGUID
		}

		if s.server.Data != nil {
			if spell, found, err := s.server.Data.Spell(click.spellID); err == nil && found {
				targetData := protocol.SpellTargetData{
					Flags:    protocol.SpellTargetFlagUnitWireMask,
					UnitGUID: targetUnit,
				}
				s.finishSpellCast(ctx, 0, click.spellID, spell, targetData)
			}
		}
	}
	return true
}

// handleTalentWipeConfirm processes MSG_TALENT_WIPE_CONFIRM (0x2AA).
// Reference: WorldSession::HandleTalentWipeConfirmOpcode (SpellHandler.cpp:732).
func (s *session) handleTalentWipeConfirm(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	wipeGUID, _ := r.ReadU64()

	// Clear player talents and unlearn all talent spells
	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		rows, err := cdb.QueryContext(ctx, "SELECT spell FROM character_talent WHERE guid = ? AND talentGroup = ?", s.playerGUID, s.player.ActiveTalentGroup)
		if err == nil {
			var unlearnSpells []uint32
			for rows.Next() {
				var sp int64
				if rows.Scan(&sp) == nil && sp > 0 {
					unlearnSpells = append(unlearnSpells, uint32(sp))
				}
			}
			rows.Close()
			for _, sp := range unlearnSpells {
				_, _ = cdb.ExecContext(ctx, "DELETE FROM character_spell WHERE guid = ? AND spell = ?", s.playerGUID, sp)
				unlearnBuf := protocol.NewBuffer(4)
				unlearnBuf.WriteU32(sp)
				_ = s.write(uint16(protocol.OpcodeSMSG_REMOVED_SPELL), unlearnBuf.Bytes(), true)
			}
		}
		_, _ = cdb.ExecContext(ctx, "DELETE FROM character_talent WHERE guid = ? AND talentGroup = ?", s.playerGUID, s.player.ActiveTalentGroup)
	}
	s.player.Talents = make(map[uint32]uint8)

	buf := protocol.NewBuffer(12)
	buf.WriteU64(wipeGUID)
	buf.WriteU32(0) // free or cost
	_ = s.write(uint16(protocol.OpcodeMSG_TALENT_WIPE_CONFIRM), buf.Bytes(), true)

	// Cast visual untalent effect 14867 from trainer to player
	castPkt := protocol.NewBuffer(16)
	castPkt.WritePackedGUID(wipeGUID)
	castPkt.WritePackedGUID(s.playerGUID)
	castPkt.WriteU8(1)
	castPkt.WriteU32(14867)
	castPkt.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_GO), castPkt.Bytes(), true)
	_ = s.sendTalentsInfo(false)
	s.sendPlayerUpdate()
	return true
}

const PlayerExtraHas310Flyer uint32 = 0x0040

type LearnedMountSpell struct {
	ID                 uint32
	MountedFlightSpeed int
}

type MountState struct {
	extraFlags uint32
	spells     map[uint32]LearnedMountSpell
}

func NewMountState(extraFlags uint32, spells []LearnedMountSpell) *MountState {
	state := &MountState{extraFlags: extraFlags, spells: make(map[uint32]LearnedMountSpell, len(spells))}
	for _, spell := range spells {
		state.spells[spell.ID] = spell
	}
	return state
}

func (s *MountState) ExtraFlags() uint32 {
	return s.extraFlags
}

func (s *MountState) Has310Flyer(checkAllSpells bool, excludeSpellID uint32) bool {
	if !checkAllSpells {
		return s.extraFlags&PlayerExtraHas310Flyer != 0
	}
	s.extraFlags &^= PlayerExtraHas310Flyer
	for _, spell := range s.spells {
		if spell.ID != excludeSpellID && spell.MountedFlightSpeed == 310 {
			s.extraFlags |= PlayerExtraHas310Flyer
			return true
		}
	}
	return false
}

func (s *MountState) SetHas310Flyer(enabled bool) {
	if enabled {
		s.extraFlags |= PlayerExtraHas310Flyer
	} else {
		s.extraFlags &^= PlayerExtraHas310Flyer
	}
}

func (s *MountState) LearnSpell(spell LearnedMountSpell) {
	s.spells[spell.ID] = spell
	if spell.MountedFlightSpeed == 310 {
		s.SetHas310Flyer(true)
	}
}

func (s *MountState) UnlearnSpell(id uint32) bool {
	if _, ok := s.spells[id]; !ok {
		return s.Has310Flyer(false, 0)
	}
	delete(s.spells, id)
	return s.Has310Flyer(true, 0)
}

func (s *MountState) PreferredFlightSpeed(canFly bool) int {
	if !canFly {
		return 0
	}
	if s.Has310Flyer(false, 0) {
		return 310
	}
	return 280
}

// handleRemoveGlyph processes CMSG_REMOVE_GLYPH (0x48A).
// Reference: WorldSession::HandleRemoveGlyph (SpellHandler.cpp:840): read the
// slot index, and when a glyph is socketed there clear it, persist the change,
// and resend the talent panel so the client drops the glyph. The reference
// also removes the glyph's granted aura; the Go server has no aura engine yet.
func (s *session) handleRemoveGlyph(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	slot, err := r.ReadU32()
	if err != nil {
		return false
	}
	if slot >= 6 {
		return true
	}
	spec := s.player.ActiveTalentGroup
	if spec >= 2 {
		spec = 0
	}
	s.player.Glyphs[spec][slot] = 0

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		col := fmt.Sprintf("glyph%d", slot+1)
		_, _ = cdb.ExecContext(ctx, fmt.Sprintf("UPDATE character_glyphs SET %s = 0 WHERE guid = ? AND talentGroup = ?", col), s.playerGUID, spec)
	}
	_ = s.sendTalentsInfo(false)
	return true
}

// applyGlyph sockets a glyph into the specified slot for the active talent group.
// Reference: Spell::EffectApplyGlyph (SpellEffects.cpp:4018).
func (s *session) applyGlyph(ctx context.Context, slot uint8, glyphPropID uint16) {
	if s.player == nil || slot >= 6 || glyphPropID == 0 {
		return
	}
	spec := s.player.ActiveTalentGroup
	if spec >= 2 {
		spec = 0
	}
	s.player.Glyphs[spec][slot] = glyphPropID

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		col := fmt.Sprintf("glyph%d", slot+1)
		_, _ = cdb.ExecContext(ctx, "INSERT OR IGNORE INTO character_glyphs (guid, talentGroup, glyph1, glyph2, glyph3, glyph4, glyph5, glyph6) VALUES (?, ?, 0, 0, 0, 0, 0, 0)", s.playerGUID, spec)
		_, _ = cdb.ExecContext(ctx, fmt.Sprintf("UPDATE character_glyphs SET %s = ? WHERE guid = ? AND talentGroup = ?", col), glyphPropID, s.playerGUID, spec)
	}
	_ = s.sendTalentsInfo(false)
}

// handleUpdateMissileTrajectory processes CMSG_UPDATE_MISSILE_TRAJECTORY (0x462).
// Reference: WorldSession::HandleUpdateMissileTrajectory (MiscHandler.cpp:1545).
func (s *session) handleUpdateMissileTrajectory(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 45 {
		return true
	}
	r := protocol.NewReader(payload)
	guid, _ := r.ReadU64()
	spellID, _ := r.ReadU32()
	elevation, _ := r.ReadF32()
	speed, _ := r.ReadF32()
	fireX, _ := r.ReadF32()
	fireY, _ := r.ReadF32()
	fireZ, _ := r.ReadF32()
	impactX, _ := r.ReadF32()
	impactY, _ := r.ReadF32()
	impactZ, _ := r.ReadF32()
	moveStop, _ := r.ReadU8()

	s.debug("update missile trajectory", "account", s.accountName, "guid", guid, "spell", spellID, "elevation", elevation, "speed", speed, "fire", []float32{fireX, fireY, fireZ}, "impact", []float32{impactX, impactY, impactZ}, "moveStop", moveStop)

	if moveStop != 0 && r.Remaining() >= 4 {
		opcode, _ := r.ReadU32()
		s.handleMovement(ctx, opcode, payload[r.Position():])
	}
	return true
}

// handleUpdateProjectilePosition processes CMSG_UPDATE_PROJECTILE_POSITION (0x4BE).
// Reference: WorldSession::HandleUpdateProjectilePosition (SpellHandler.cpp:816).
func (s *session) handleUpdateProjectilePosition(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 25 {
		return true
	}
	r := protocol.NewReader(payload)
	casterGuid, _ := r.ReadU64()
	spellID, _ := r.ReadU32()
	castCount, _ := r.ReadU8()
	hitX, _ := r.ReadF32()
	hitY, _ := r.ReadF32()
	hitZ, _ := r.ReadF32()

	s.debug("update projectile position", "account", s.accountName, "guid", casterGuid, "spell", spellID, "castCount", castCount, "hit", []float32{hitX, hitY, hitZ})
	return true
}

func (s *session) sendActionButtons() {
	if s.player == nil {
		return
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_ACTION_BUTTONS), buildActionButtons(s.player.Actions), true)
}

// activateSpec mirrors Player::ActivateSpec (Player.cpp:26292).
func (s *session) activateSpec(ctx context.Context, targetSpec uint8) {
	if s.player == nil || targetSpec >= s.player.TalentGroupsCount || targetSpec == s.player.ActiveTalentGroup {
		return
	}
	if s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return
	}
	cdb := s.server.CharactersStore.DB

	// 1. Unlearn talents of current spec
	rows, err := cdb.QueryContext(ctx, "SELECT spell FROM character_talent WHERE guid = ? AND talentGroup = ?", s.playerGUID, s.player.ActiveTalentGroup)
	if err == nil {
		var oldSpells []uint32
		for rows.Next() {
			var sp int64
			if rows.Scan(&sp) == nil && sp > 0 {
				oldSpells = append(oldSpells, uint32(sp))
			}
		}
		rows.Close()
		for _, sp := range oldSpells {
			_, _ = cdb.ExecContext(ctx, "DELETE FROM character_spell WHERE guid = ? AND spell = ?", s.playerGUID, sp)
			unlearnBuf := protocol.NewBuffer(4)
			unlearnBuf.WriteU32(sp)
			_ = s.write(uint16(protocol.OpcodeSMSG_REMOVED_SPELL), unlearnBuf.Bytes(), true)
		}
	}

	// 2. Set new active talent group
	s.player.ActiveTalentGroup = targetSpec
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET activeTalentGroup = ? WHERE guid = ?", targetSpec, s.playerGUID)

	// 3. Load talents for new spec
	s.loadPlayerTalents(ctx, s.player)

	// 4. Teach spells for new spec
	var newSpells []uint32
	tRows, err := cdb.QueryContext(ctx, "SELECT spell FROM character_talent WHERE guid = ? AND talentGroup = ?", s.playerGUID, targetSpec)
	if err == nil {
		for tRows.Next() {
			var sp int64
			if tRows.Scan(&sp) == nil && sp > 0 {
				newSpells = append(newSpells, uint32(sp))
			}
		}
		tRows.Close()
		for _, sp := range newSpells {
			_, _ = cdb.ExecContext(ctx, "REPLACE INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)", s.playerGUID, sp)
			learnBuf := protocol.NewBuffer(6)
			learnBuf.WriteU32(sp)
			learnBuf.WriteU16(0)
			_ = s.write(uint16(protocol.OpcodeSMSG_LEARNED_SPELL), learnBuf.Bytes(), true)
		}
	}

	// 5. Load and send action buttons for new spec
	if actions, err := s.loadActionButtons(ctx, s.playerGUID, s.player.Race, s.player.Class); err == nil {
		s.player.Actions = actions
		s.sendActionButtons()
	}

	// 6. Send talents info update
	_ = s.sendTalentsInfo(false)
}

// Channel and pushback state machine, mirroring the reference:
//   - Spell::handle_immediate / SendChannelStart (Spell.cpp:3568/4661):
//     channeled spells (SPELL_ATTR1_CHANNELED_1 0x04 | CHANNELED_2 0x40 in
//     AttributesEx field 6) start a channel after SPELL_GO, announced with
//     MSG_CHANNEL_START (packed caster GUID, spell id, duration) and periodic
//     effect ticks every EffectAuraPeriod milliseconds.
//   - Spell::Delayed (Spell.cpp:7250): a player taking damage while casting a
//     spell with SPELL_INTERRUPT_FLAG_PUSH_BACK 0x02 loses 500ms per hit, at
//     most twice per cast, clamped to the remaining cast time, announced via
//     SMSG_SPELL_DELAYED (packed caster GUID, delay).
//   - Spell::DelayedChannel (Spell.cpp:7290): a channeling player taking
//     damage on a spell with CHANNEL_FLAG_DELAY 0x4000 loses 25% of the total
//     channel duration per hit, at most twice, announced via MSG_CHANNEL_UPDATE.
//   - Movement interrupts: movement flags cancel the active cast and channel
//     (movement.go), matching reference movement-cast interruption.

const (
	spellAttr1Channeled1   uint32 = 0x04
	spellAttr1Channeled2   uint32 = 0x40
	spellInterruptPushBack uint32 = 0x02
	channelFlagDelay       uint32 = 0x4000
	maxSpellPushbacks      int    = 2
	defaultCastPushbackMs  uint32 = 500
)

// activeChannelState tracks one running channeled spell.
type activeChannelState struct {
	CastID     uint8
	SpellID    uint32
	TargetGUID uint64
	Spell      wotlk.Spell
	DurationMs uint32
	Remaining  time.Duration
	PeriodMs   uint32
	Pushbacks  int
	Timer      *time.Timer
	TickTimer  *time.Timer
	Stopped    bool
}

func isChanneledSpell(spell wotlk.Spell) bool {
	return spell.AttributesEx1&(spellAttr1Channeled1|spellAttr1Channeled2) != 0
}

// sendChannelUpdate mirrors Spell::SendChannelUpdate: packed caster GUID plus
// remaining channel milliseconds; zero clears the client channel bar.
func (s *session) sendChannelUpdate(remainingMs uint32) {
	packet := protocol.NewBuffer(12)
	packet.WritePackedGUID(s.playerGUID)
	packet.WriteU32(remainingMs)
	_ = s.write(uint16(protocol.OpcodeMSG_CHANNEL_UPDATE), packet.Bytes(), true)
}

// startChannel begins the channeled phase of a finished cast: broadcast the
// channel start, schedule periodic ticks, and arm completion.
func (s *session) startChannel(castID uint8, spellID uint32, spell wotlk.Spell, targetGUID uint64) {
	if s.player == nil || s.server.Data == nil {
		return
	}
	var durationMs int32 = 0
	if value, ok, err := s.server.Data.SpellDuration(spell.DurationIndex, uint32(s.player.Level)); err == nil && ok {
		durationMs = value
	}
	if durationMs <= 0 {
		return // instant or infinite channels have no timed lifecycle here
	}
	period := uint32(0)
	for _, effect := range spell.Effects {
		if effect.Effect != 0 && effect.AuraPeriod > period {
			period = effect.AuraPeriod
		}
	}
	channel := &activeChannelState{
		CastID:     castID,
		SpellID:    spellID,
		TargetGUID: targetGUID,
		Spell:      spell,
		DurationMs: uint32(durationMs),
		Remaining:  time.Duration(durationMs) * time.Millisecond,
		PeriodMs:   period,
	}
	s.castMu.Lock()
	s.activeChannel = channel
	s.castMu.Unlock()

	packet := protocol.NewBuffer(16)
	packet.WritePackedGUID(s.playerGUID)
	packet.WriteU32(spellID)
	packet.WriteU32(uint32(durationMs))
	_ = s.write(uint16(protocol.OpcodeMSG_CHANNEL_START), packet.Bytes(), true)
	if s.server != nil {
		s.server.broadcastToNearby(uint16(protocol.OpcodeMSG_CHANNEL_START), packet.Bytes(), s)
	}

	channel.Timer = time.AfterFunc(channel.Remaining, func() { s.finishChannel() })
	if period > 0 && period <= uint32(durationMs) {
		channel.TickTimer = time.AfterFunc(time.Duration(period)*time.Millisecond, func() { s.channelTick() })
	}
	s.debug("channel started", "account", s.accountName, "spell", spellID, "duration_ms", durationMs, "period_ms", period)
}

// finishChannel completes the channel: clear state and zero the bar.
func (s *session) finishChannel() {
	s.castMu.Lock()
	channel := s.activeChannel
	if channel == nil || channel.Stopped {
		s.castMu.Unlock()
		return
	}
	s.activeChannel = nil
	if channel.Timer != nil {
		channel.Timer.Stop()
	}
	if channel.TickTimer != nil {
		channel.TickTimer.Stop()
	}
	channel.Stopped = true
	spellID := channel.SpellID
	s.castMu.Unlock()

	s.sendChannelUpdate(0)
	s.debug("channel finished", "account", s.accountName, "spell", spellID)
}

// interruptCurrentChannel stops the active channel without the completion
// path (movement, new cast, cancel).
func (s *session) interruptCurrentChannel() {
	s.castMu.Lock()
	channel := s.activeChannel
	if channel == nil || channel.Stopped {
		s.castMu.Unlock()
		return
	}
	s.activeChannel = nil
	if channel.Timer != nil {
		channel.Timer.Stop()
	}
	if channel.TickTimer != nil {
		channel.TickTimer.Stop()
	}
	channel.Stopped = true
	s.castMu.Unlock()

	s.sendChannelUpdate(0)
}

// channelTick applies one periodic effect tick of the channeled spell and
// schedules the next while the channel is alive.
func (s *session) channelTick() {
	s.castMu.Lock()
	channel := s.activeChannel
	if channel == nil || channel.Stopped {
		s.castMu.Unlock()
		return
	}
	next := time.Duration(channel.PeriodMs) * time.Millisecond
	spell := channel.Spell
	targetGUID := channel.TargetGUID
	remaining := channel.Remaining
	s.castMu.Unlock()

	ctx := context.Background()
	for _, effect := range spell.Effects {
		if effect.Effect == 0 {
			continue
		}
		amount := uint32(effect.BasePoints + 1)
		if amount == 0 {
			continue
		}
		switch effect.Effect {
		case 2, 87, 108, 17: // damage effects tick on the target
			if targetGUID != 0 && targetGUID != s.playerGUID {
				s.executeSpellDamage(ctx, targetGUID, spell.ID, amount)
			}
		case 6, 10, 136, 105: // auras and heals tick on the target or caster
			if targetGUID != 0 && targetGUID != s.playerGUID {
				s.executeSpellDamage(ctx, targetGUID, spell.ID, amount)
			} else {
				s.executeSpellHeal(ctx, s.playerGUID, spell.ID, amount)
			}
		}
	}

	s.castMu.Lock()
	channel = s.activeChannel
	if channel == nil || channel.Stopped {
		s.castMu.Unlock()
		return
	}
	remaining -= next
	if remaining > 0 {
		channel.TickTimer = time.AfterFunc(next, func() { s.channelTick() })
		s.castMu.Unlock()
		return
	}
	s.castMu.Unlock()
}

// delayCurrentCast mirrors Spell::Delayed: called when the player takes
// damage during a timed cast. Requires SPELL_INTERRUPT_FLAG_PUSH_BACK, at
// most two pushbacks per cast, 500ms each clamped to remaining time, and
// announces SMSG_SPELL_DELAYED.
func (s *session) delayCurrentCast() {
	if s.player == nil {
		return
	}
	s.castMu.Lock()
	cast := s.activeCast
	if cast == nil || cast.CastTimeMs == 0 || cast.InterruptFlg&spellInterruptPushBack == 0 || cast.Pushbacks >= maxSpellPushbacks {
		s.castMu.Unlock()
		return
	}
	elapsed := time.Since(cast.StartAt)
	remaining := time.Duration(cast.CastTimeMs)*time.Millisecond - elapsed
	if remaining <= 0 {
		s.castMu.Unlock()
		return
	}
	delay := time.Duration(defaultCastPushbackMs) * time.Millisecond
	if delay > remaining {
		delay = remaining
	}
	cast.Pushbacks++
	newRemaining := remaining + delay
	cast.StartAt = time.Now().Add(-(time.Duration(cast.CastTimeMs)*time.Millisecond - newRemaining))
	if cast.Timer != nil {
		cast.Timer.Reset(newRemaining)
	}
	pushbacks := cast.Pushbacks
	spellID := cast.SpellID
	s.castMu.Unlock()

	packet := protocol.NewBuffer(8)
	packet.WritePackedGUID(s.playerGUID)
	packet.WriteU32(uint32(delay.Milliseconds()))
	_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_DELAYED), packet.Bytes(), true)
	s.debug("cast pushed back", "account", s.accountName, "spell", spellID, "delay_ms", delay.Milliseconds(), "count", pushbacks)
}

// delayCurrentChannel mirrors Spell::DelayedChannel: called when the player
// takes damage while channeling. Requires CHANNEL_FLAG_DELAY, at most two
// pushbacks, 25% of the total channel duration per hit, announced via
// MSG_CHANNEL_UPDATE with the new remaining time.
func (s *session) delayCurrentChannel() {
	if s.player == nil {
		return
	}
	s.castMu.Lock()
	channel := s.activeChannel
	if channel == nil || channel.Stopped || channel.Spell.ChannelInterrupt&channelFlagDelay == 0 || channel.Pushbacks >= maxSpellPushbacks {
		s.castMu.Unlock()
		return
	}
	delayMs := channel.DurationMs / 4 // 25% of total duration per hit
	if delayMs == 0 {
		s.castMu.Unlock()
		return
	}
	if time.Duration(delayMs)*time.Millisecond >= channel.Remaining {
		delayMs = uint32(channel.Remaining.Milliseconds())
		channel.Remaining = 0
	} else {
		channel.Remaining -= time.Duration(delayMs) * time.Millisecond
	}
	channel.Pushbacks++
	remaining := channel.Remaining
	if channel.Timer != nil {
		channel.Timer.Reset(remaining)
	}
	spellID := channel.SpellID
	s.castMu.Unlock()

	s.sendChannelUpdate(uint32(remaining.Milliseconds()))
	s.debug("channel pushed back", "account", s.accountName, "spell", spellID, "delay_ms", delayMs, "count", channel.Pushbacks)
}
