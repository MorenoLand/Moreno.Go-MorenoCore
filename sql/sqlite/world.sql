-- schema-only template
CREATE TABLE IF NOT EXISTS `access_requirement` (
`mapId` INTEGER  NOT NULL,
`difficulty` INTEGER  NOT NULL DEFAULT '0',
`level_min` INTEGER  NOT NULL DEFAULT '0',
`level_max` INTEGER  NOT NULL DEFAULT '0',
`item_level` INTEGER  NOT NULL DEFAULT '0',
`item` INTEGER  NOT NULL DEFAULT '0',
`item2` INTEGER  NOT NULL DEFAULT '0',
`quest_done_A` INTEGER  NOT NULL DEFAULT '0',
`quest_done_H` INTEGER  NOT NULL DEFAULT '0',
`completed_achievement` INTEGER  NOT NULL DEFAULT '0',
`quest_failed_text` TEXT,
`comment` TEXT,
PRIMARY KEY (`mapId`,`difficulty`)
);
CREATE TABLE IF NOT EXISTS `achievement_criteria_data` (
`criteria_id` INTEGER NOT NULL,
`type` INTEGER  NOT NULL DEFAULT '0',
`value1` INTEGER  NOT NULL DEFAULT '0',
`value2` INTEGER  NOT NULL DEFAULT '0',
`ScriptName` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`criteria_id`,`type`)
);
CREATE TABLE IF NOT EXISTS `achievement_dbc` (
`ID` INTEGER  NOT NULL,
`requiredFaction` INTEGER NOT NULL DEFAULT '-1',
`mapID` INTEGER NOT NULL DEFAULT '-1',
`points` INTEGER  NOT NULL DEFAULT '0',
`flags` INTEGER  NOT NULL DEFAULT '0',
`count` INTEGER  NOT NULL DEFAULT '0',
`refAchievement` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `achievement_reward` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`TitleA` INTEGER  NOT NULL DEFAULT '0',
`TitleH` INTEGER  NOT NULL DEFAULT '0',
`ItemID` INTEGER  NOT NULL DEFAULT '0',
`Sender` INTEGER  NOT NULL DEFAULT '0',
`Subject` TEXT DEFAULT NULL,
`Body` TEXT,
`MailTemplateID` INTEGER  DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `achievement_reward_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Locale` TEXT NOT NULL,
`Subject` TEXT,
`Body` TEXT,
PRIMARY KEY (`ID`,`Locale`)
);
CREATE TABLE IF NOT EXISTS `areatrigger_involvedrelation` (
`id` INTEGER  NOT NULL DEFAULT '0',
`quest` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `areatrigger_scripts` (
`entry` INTEGER NOT NULL,
`ScriptName` TEXT NOT NULL,
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `areatrigger_tavern` (
`id` INTEGER  NOT NULL DEFAULT '0',
`name` TEXT,
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `areatrigger_teleport` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Name` TEXT,
`target_map` INTEGER  NOT NULL DEFAULT '0',
`target_position_x` REAL NOT NULL DEFAULT '0',
`target_position_y` REAL NOT NULL DEFAULT '0',
`target_position_z` REAL NOT NULL DEFAULT '0',
`target_orientation` REAL NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE INDEX IF NOT EXISTS `areatrigger_teleport__name` ON `areatrigger_teleport` (`Name`);
CREATE TABLE IF NOT EXISTS `battlefield_template` (
`TypeId` INTEGER  NOT NULL,
`ScriptName` TEXT NOT NULL DEFAULT '',
`comment` TEXT
);
CREATE TABLE IF NOT EXISTS `battleground_template` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`MinPlayersPerTeam` INTEGER  NOT NULL DEFAULT '0',
`MaxPlayersPerTeam` INTEGER  NOT NULL DEFAULT '0',
`MinLvl` INTEGER  NOT NULL DEFAULT '0',
`MaxLvl` INTEGER  NOT NULL DEFAULT '0',
`AllianceStartLoc` INTEGER  NOT NULL,
`AllianceStartO` REAL NOT NULL,
`HordeStartLoc` INTEGER  NOT NULL,
`HordeStartO` REAL NOT NULL,
`StartMaxDist` REAL NOT NULL DEFAULT '0',
`Weight` INTEGER  NOT NULL DEFAULT '1',
`ScriptName` TEXT NOT NULL DEFAULT '',
`Comment` TEXT NOT NULL,
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `battlemaster_entry` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`bg_template` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `broadcast_text` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`LanguageID` INTEGER  NOT NULL DEFAULT '0',
`Text` TEXT,
`Text1` TEXT,
`EmoteID1` INTEGER  NOT NULL DEFAULT '0',
`EmoteID2` INTEGER  NOT NULL DEFAULT '0',
`EmoteID3` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay2` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay3` INTEGER  NOT NULL DEFAULT '0',
`SoundEntriesID` INTEGER  NOT NULL DEFAULT '0',
`EmotesID` INTEGER  NOT NULL DEFAULT '0',
`Flags` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `broadcast_text_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Text` TEXT,
`Text1` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `command` (
`name` TEXT NOT NULL DEFAULT '',
`help` TEXT,
PRIMARY KEY (`name`)
);
CREATE TABLE IF NOT EXISTS `conditions` (
`SourceTypeOrReferenceId` INTEGER NOT NULL DEFAULT '0',
`SourceGroup` INTEGER  NOT NULL DEFAULT '0',
`SourceEntry` INTEGER NOT NULL DEFAULT '0',
`SourceId` INTEGER NOT NULL DEFAULT '0',
`ElseGroup` INTEGER  NOT NULL DEFAULT '0',
`ConditionTypeOrReference` INTEGER NOT NULL DEFAULT '0',
`ConditionTarget` INTEGER  NOT NULL DEFAULT '0',
`ConditionValue1` INTEGER  NOT NULL DEFAULT '0',
`ConditionValue2` INTEGER  NOT NULL DEFAULT '0',
`ConditionValue3` INTEGER  NOT NULL DEFAULT '0',
`NegativeCondition` INTEGER  NOT NULL DEFAULT '0',
`ErrorType` INTEGER  NOT NULL DEFAULT '0',
`ErrorTextId` INTEGER  NOT NULL DEFAULT '0',
`ScriptName` TEXT NOT NULL DEFAULT '',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`SourceTypeOrReferenceId`,`SourceGroup`,`SourceEntry`,`SourceId`,`ElseGroup`,`ConditionTypeOrReference`,`ConditionTarget`,`ConditionValue1`,`ConditionValue2`,`ConditionValue3`)
);
CREATE TABLE IF NOT EXISTS `creature` (
`guid` INTEGER  NOT NULL,
`id` INTEGER  NOT NULL DEFAULT '0',
`map` INTEGER  NOT NULL DEFAULT '0',
`zoneId` INTEGER  NOT NULL DEFAULT '0',
`areaId` INTEGER  NOT NULL DEFAULT '0',
`spawnMask` INTEGER  NOT NULL DEFAULT '1',
`phaseMask` INTEGER  NOT NULL DEFAULT '1',
`modelid` INTEGER  NOT NULL DEFAULT '0',
`equipment_id` INTEGER NOT NULL DEFAULT '0',
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`spawntimesecs` INTEGER  NOT NULL DEFAULT '120',
`wander_distance` REAL NOT NULL DEFAULT '0',
`currentwaypoint` INTEGER  NOT NULL DEFAULT '0',
`curhealth` INTEGER  NOT NULL DEFAULT '1',
`curmana` INTEGER  NOT NULL DEFAULT '0',
`MovementType` INTEGER  NOT NULL DEFAULT '0',
`npcflag` INTEGER  NOT NULL DEFAULT '0',
`unit_flags` INTEGER  NOT NULL DEFAULT '0',
`dynamicflags` INTEGER  NOT NULL DEFAULT '0',
`ScriptName` TEXT DEFAULT '',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`guid`)
);
CREATE INDEX IF NOT EXISTS `creature__idx_map` ON `creature` (`map`);
CREATE INDEX IF NOT EXISTS `creature__idx_id` ON `creature` (`id`);
CREATE INDEX IF NOT EXISTS `creature__idx_map_coords` ON `creature` (`map`, `position_x`, `position_y`);
CREATE TABLE IF NOT EXISTS `creature_addon` (
`guid` INTEGER  NOT NULL DEFAULT '0',
`path_id` INTEGER  NOT NULL DEFAULT '0',
`mount` INTEGER  NOT NULL DEFAULT '0',
`bytes1` INTEGER  NOT NULL DEFAULT '0',
`bytes2` INTEGER  NOT NULL DEFAULT '0',
`emote` INTEGER  NOT NULL DEFAULT '0',
`visibilityDistanceType` INTEGER  NOT NULL DEFAULT '0',
`auras` TEXT,
PRIMARY KEY (`guid`)
);
CREATE TABLE IF NOT EXISTS `creature_classlevelstats` (
`level` INTEGER  NOT NULL,
`class` INTEGER  NOT NULL,
`basehp0` INTEGER  NOT NULL DEFAULT '1',
`basehp1` INTEGER  NOT NULL DEFAULT '1',
`basehp2` INTEGER  NOT NULL DEFAULT '1',
`basemana` INTEGER  NOT NULL DEFAULT '0',
`basearmor` INTEGER  NOT NULL DEFAULT '1',
`attackpower` INTEGER  NOT NULL DEFAULT '0',
`rangedattackpower` INTEGER  NOT NULL DEFAULT '0',
`damage_base` REAL NOT NULL DEFAULT '0',
`damage_exp1` REAL NOT NULL DEFAULT '0',
`damage_exp2` REAL NOT NULL DEFAULT '0',
`comment` TEXT,
PRIMARY KEY (`level`,`class`)
);
CREATE TABLE IF NOT EXISTS `creature_default_trainer` (
`CreatureId` INTEGER  NOT NULL,
`TrainerId` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`CreatureId`)
);
CREATE TABLE IF NOT EXISTS `creature_equip_template` (
`CreatureID` INTEGER  NOT NULL DEFAULT '0',
`ID` INTEGER  NOT NULL DEFAULT '1',
`ItemID1` INTEGER  NOT NULL DEFAULT '0',
`ItemID2` INTEGER  NOT NULL DEFAULT '0',
`ItemID3` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`CreatureID`,`ID`)
);
CREATE TABLE IF NOT EXISTS `creature_formations` (
`leaderGUID` INTEGER  NOT NULL DEFAULT '0',
`memberGUID` INTEGER  NOT NULL DEFAULT '0',
`dist` REAL  NOT NULL,
`angle` REAL  NOT NULL,
`groupAI` INTEGER  NOT NULL,
`point_1` INTEGER  NOT NULL DEFAULT '0',
`point_2` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`memberGUID`)
);
CREATE TABLE IF NOT EXISTS `creature_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `creature_model_info` (
`DisplayID` INTEGER  NOT NULL DEFAULT '0',
`BoundingRadius` REAL NOT NULL DEFAULT '0',
`CombatReach` REAL NOT NULL DEFAULT '0',
`Gender` INTEGER  NOT NULL DEFAULT '2',
`DisplayID_Other_Gender` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`DisplayID`)
);
CREATE TABLE IF NOT EXISTS `creature_movement_override` (
`SpawnId` INTEGER  NOT NULL DEFAULT '0',
`Ground` INTEGER  NOT NULL DEFAULT '1',
`Swim` INTEGER  NOT NULL DEFAULT '1',
`Flight` INTEGER  NOT NULL DEFAULT '0',
`Rooted` INTEGER  NOT NULL DEFAULT '0',
`Chase` INTEGER  NOT NULL DEFAULT '0',
`Random` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`SpawnId`)
);
CREATE TABLE IF NOT EXISTS `creature_onkill_reputation` (
`creature_id` INTEGER  NOT NULL DEFAULT '0',
`RewOnKillRepFaction1` INTEGER NOT NULL DEFAULT '0',
`RewOnKillRepFaction2` INTEGER NOT NULL DEFAULT '0',
`MaxStanding1` INTEGER NOT NULL DEFAULT '0',
`IsTeamAward1` INTEGER NOT NULL DEFAULT '0',
`RewOnKillRepValue1` INTEGER NOT NULL DEFAULT '0',
`MaxStanding2` INTEGER NOT NULL DEFAULT '0',
`IsTeamAward2` INTEGER NOT NULL DEFAULT '0',
`RewOnKillRepValue2` INTEGER NOT NULL DEFAULT '0',
`TeamDependent` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`creature_id`)
);
CREATE TABLE IF NOT EXISTS `creature_questender` (
`id` INTEGER  NOT NULL DEFAULT '0',
`quest` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`quest`)
);
CREATE TABLE IF NOT EXISTS `creature_questitem` (
`CreatureEntry` INTEGER  NOT NULL DEFAULT '0',
`Idx` INTEGER  NOT NULL DEFAULT '0',
`ItemId` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`CreatureEntry`,`Idx`)
);
CREATE TABLE IF NOT EXISTS `creature_queststarter` (
`id` INTEGER  NOT NULL DEFAULT '0',
`quest` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`quest`)
);
CREATE TABLE IF NOT EXISTS `creature_summon_groups` (
`summonerId` INTEGER  NOT NULL DEFAULT '0',
`summonerType` INTEGER  NOT NULL DEFAULT '0',
`groupId` INTEGER  NOT NULL DEFAULT '0',
`entry` INTEGER  NOT NULL DEFAULT '0',
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`summonType` INTEGER  NOT NULL DEFAULT '0',
`summonTime` INTEGER  NOT NULL DEFAULT '0',
`Comment` TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS `creature_template` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`difficulty_entry_1` INTEGER  NOT NULL DEFAULT '0',
`difficulty_entry_2` INTEGER  NOT NULL DEFAULT '0',
`difficulty_entry_3` INTEGER  NOT NULL DEFAULT '0',
`KillCredit1` INTEGER  NOT NULL DEFAULT '0',
`KillCredit2` INTEGER  NOT NULL DEFAULT '0',
`modelid1` INTEGER  NOT NULL DEFAULT '0',
`modelid2` INTEGER  NOT NULL DEFAULT '0',
`modelid3` INTEGER  NOT NULL DEFAULT '0',
`modelid4` INTEGER  NOT NULL DEFAULT '0',
`name` TEXT NOT NULL DEFAULT '0',
`subname` TEXT DEFAULT NULL,
`IconName` TEXT DEFAULT NULL,
`gossip_menu_id` INTEGER  NOT NULL DEFAULT '0',
`minlevel` INTEGER  NOT NULL DEFAULT '1',
`maxlevel` INTEGER  NOT NULL DEFAULT '1',
`exp` INTEGER NOT NULL DEFAULT '0',
`faction` INTEGER  NOT NULL DEFAULT '0',
`npcflag` INTEGER  NOT NULL DEFAULT '0',
`speed_walk` REAL NOT NULL DEFAULT '1',
`speed_run` REAL NOT NULL DEFAULT '1.14286',
`scale` REAL NOT NULL DEFAULT '1',
`rank` INTEGER  NOT NULL DEFAULT '0',
`dmgschool` INTEGER NOT NULL DEFAULT '0',
`BaseAttackTime` INTEGER  NOT NULL DEFAULT '0',
`RangeAttackTime` INTEGER  NOT NULL DEFAULT '0',
`BaseVariance` REAL NOT NULL DEFAULT '1',
`RangeVariance` REAL NOT NULL DEFAULT '1',
`unit_class` INTEGER  NOT NULL DEFAULT '0',
`unit_flags` INTEGER  NOT NULL DEFAULT '0',
`unit_flags2` INTEGER  NOT NULL DEFAULT '0',
`dynamicflags` INTEGER  NOT NULL DEFAULT '0',
`family` INTEGER NOT NULL DEFAULT '0',
`type` INTEGER  NOT NULL DEFAULT '0',
`type_flags` INTEGER  NOT NULL DEFAULT '0',
`lootid` INTEGER  NOT NULL DEFAULT '0',
`pickpocketloot` INTEGER  NOT NULL DEFAULT '0',
`skinloot` INTEGER  NOT NULL DEFAULT '0',
`PetSpellDataId` INTEGER  NOT NULL DEFAULT '0',
`VehicleId` INTEGER  NOT NULL DEFAULT '0',
`mingold` INTEGER  NOT NULL DEFAULT '0',
`maxgold` INTEGER  NOT NULL DEFAULT '0',
`AIName` TEXT NOT NULL DEFAULT '',
`MovementType` INTEGER  NOT NULL DEFAULT '0',
`HoverHeight` REAL NOT NULL DEFAULT '1',
`HealthModifier` REAL NOT NULL DEFAULT '1',
`ManaModifier` REAL NOT NULL DEFAULT '1',
`ArmorModifier` REAL NOT NULL DEFAULT '1',
`DamageModifier` REAL NOT NULL DEFAULT '1',
`ExperienceModifier` REAL NOT NULL DEFAULT '1',
`RacialLeader` INTEGER  NOT NULL DEFAULT '0',
`movementId` INTEGER  NOT NULL DEFAULT '0',
`RegenHealth` INTEGER  NOT NULL DEFAULT '1',
`mechanic_immune_mask` INTEGER  NOT NULL DEFAULT '0',
`spell_school_immune_mask` INTEGER  NOT NULL DEFAULT '0',
`flags_extra` INTEGER  NOT NULL DEFAULT '0',
`ScriptName` TEXT NOT NULL DEFAULT '',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE INDEX IF NOT EXISTS `creature_template__idx_name` ON `creature_template` (`name`);
CREATE TABLE IF NOT EXISTS `creature_template_addon` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`path_id` INTEGER  NOT NULL DEFAULT '0',
`mount` INTEGER  NOT NULL DEFAULT '0',
`bytes1` INTEGER  NOT NULL DEFAULT '0',
`bytes2` INTEGER  NOT NULL DEFAULT '0',
`emote` INTEGER  NOT NULL DEFAULT '0',
`visibilityDistanceType` INTEGER  NOT NULL DEFAULT '0',
`auras` TEXT,
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `creature_template_locale` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Name` TEXT,
`Title` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`entry`,`locale`)
);
CREATE TABLE IF NOT EXISTS `creature_template_movement` (
`CreatureId` INTEGER  NOT NULL DEFAULT '0',
`Ground` INTEGER  NOT NULL DEFAULT '1',
`Swim` INTEGER  NOT NULL DEFAULT '1',
`Flight` INTEGER  NOT NULL DEFAULT '0',
`Rooted` INTEGER  NOT NULL DEFAULT '0',
`Chase` INTEGER  NOT NULL DEFAULT '0',
`Random` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`CreatureId`)
);
CREATE TABLE IF NOT EXISTS `creature_template_npcbot_appearance` (
`entry` INTEGER  NOT NULL,
`name*` TEXT DEFAULT 'unk',
`gender` INTEGER  NOT NULL DEFAULT '0',
`skin` INTEGER  NOT NULL DEFAULT '0',
`face` INTEGER  NOT NULL DEFAULT '0',
`hair` INTEGER  NOT NULL DEFAULT '0',
`haircolor` INTEGER  NOT NULL DEFAULT '0',
`features` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `creature_template_npcbot_extras` (
`entry` INTEGER  NOT NULL,
`class` INTEGER  NOT NULL DEFAULT '1',
`race` INTEGER  NOT NULL DEFAULT '1',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `creature_template_outfits` (
`entry` INTEGER  NOT NULL,
`race` INTEGER  NOT NULL DEFAULT '1',
`gender` INTEGER  NOT NULL DEFAULT '0',
`skin` INTEGER  NOT NULL DEFAULT '0',
`face` INTEGER  NOT NULL DEFAULT '0',
`hair` INTEGER  NOT NULL DEFAULT '0',
`haircolor` INTEGER  NOT NULL DEFAULT '0',
`facialhair` INTEGER  NOT NULL DEFAULT '0',
`head` INTEGER  NOT NULL DEFAULT '0',
`shoulders` INTEGER  NOT NULL DEFAULT '0',
`body` INTEGER  NOT NULL DEFAULT '0',
`chest` INTEGER  NOT NULL DEFAULT '0',
`waist` INTEGER  NOT NULL DEFAULT '0',
`legs` INTEGER  NOT NULL DEFAULT '0',
`feet` INTEGER  NOT NULL DEFAULT '0',
`wrists` INTEGER  NOT NULL DEFAULT '0',
`hands` INTEGER  NOT NULL DEFAULT '0',
`back` INTEGER  NOT NULL DEFAULT '0',
`tabard` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `creature_template_resistance` (
`CreatureID` INTEGER  NOT NULL,
`School` INTEGER  NOT NULL,
`Resistance` INTEGER DEFAULT NULL,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`CreatureID`,`School`)
);
CREATE TABLE IF NOT EXISTS `creature_template_spell` (
`CreatureID` INTEGER  NOT NULL,
`Index` INTEGER  NOT NULL DEFAULT '0',
`Spell` INTEGER  DEFAULT NULL,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`CreatureID`,`Index`)
);
CREATE TABLE IF NOT EXISTS `creature_text` (
`CreatureID` INTEGER  NOT NULL DEFAULT '0',
`GroupID` INTEGER  NOT NULL DEFAULT '0',
`ID` INTEGER  NOT NULL DEFAULT '0',
`Text` TEXT,
`Type` INTEGER  NOT NULL DEFAULT '0',
`Language` INTEGER NOT NULL DEFAULT '0',
`Probability` REAL  NOT NULL DEFAULT '0',
`Emote` INTEGER  NOT NULL DEFAULT '0',
`Duration` INTEGER  NOT NULL DEFAULT '0',
`Sound` INTEGER  NOT NULL DEFAULT '0',
`BroadcastTextId` INTEGER NOT NULL DEFAULT '0',
`TextRange` INTEGER  NOT NULL DEFAULT '0',
`comment` TEXT DEFAULT '',
PRIMARY KEY (`CreatureID`,`GroupID`,`ID`)
);
CREATE TABLE IF NOT EXISTS `creature_text_locale` (
`CreatureID` INTEGER  NOT NULL DEFAULT '0',
`GroupID` INTEGER  NOT NULL DEFAULT '0',
`ID` INTEGER  NOT NULL DEFAULT '0',
`Locale` TEXT NOT NULL,
`Text` TEXT,
PRIMARY KEY (`CreatureID`,`GroupID`,`ID`,`Locale`)
);
CREATE TABLE IF NOT EXISTS `custom_npc_tele_association` (
`cat_id` INTEGER  NOT NULL DEFAULT '0',
`dest_id` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`cat_id`,`dest_id`)
);
CREATE TABLE IF NOT EXISTS `custom_npc_tele_category` (
`id` INTEGER  NOT NULL DEFAULT '0',
`icon` TEXT NOT NULL DEFAULT 'inv_misc_shadowegg',
`size` TEXT NOT NULL DEFAULT '30',
`colour` TEXT NOT NULL DEFAULT '000000',
`name` TEXT NOT NULL DEFAULT '',
`flag` INTEGER  NOT NULL DEFAULT '0',
`data0` INTEGER  NOT NULL DEFAULT '0',
`data1` INTEGER  NOT NULL DEFAULT '0',
`name_loc1` TEXT NOT NULL DEFAULT '',
`name_loc2` TEXT NOT NULL DEFAULT '',
`name_loc3` TEXT NOT NULL DEFAULT '',
`name_loc4` TEXT NOT NULL DEFAULT '',
`name_loc5` TEXT NOT NULL DEFAULT '',
`name_loc6` TEXT NOT NULL DEFAULT '',
`name_loc7` TEXT NOT NULL DEFAULT '',
`name_loc8` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `custom_npc_tele_destination` (
`id` INTEGER  NOT NULL,
`icon` TEXT NOT NULL DEFAULT 'inv_misc_shadowegg',
`size` TEXT NOT NULL DEFAULT '30',
`colour` TEXT NOT NULL DEFAULT '000000',
`name` TEXT NOT NULL DEFAULT '',
`pos_X` REAL NOT NULL DEFAULT '0',
`pos_Y` REAL NOT NULL DEFAULT '0',
`pos_Z` REAL NOT NULL DEFAULT '0',
`map` INTEGER  NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`level` INTEGER  NOT NULL DEFAULT '0',
`cost` INTEGER  NOT NULL DEFAULT '0',
`name_loc1` TEXT NOT NULL DEFAULT '',
`name_loc2` TEXT NOT NULL DEFAULT '',
`name_loc3` TEXT NOT NULL DEFAULT '',
`name_loc4` TEXT NOT NULL DEFAULT '',
`name_loc5` TEXT NOT NULL DEFAULT '',
`name_loc6` TEXT NOT NULL DEFAULT '',
`name_loc7` TEXT NOT NULL DEFAULT '',
`name_loc8` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `disables` (
`sourceType` INTEGER  NOT NULL,
`entry` INTEGER  NOT NULL,
`flags` INTEGER NOT NULL,
`params_0` TEXT NOT NULL DEFAULT '',
`params_1` TEXT NOT NULL DEFAULT '',
`comment` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`sourceType`,`entry`)
);
CREATE TABLE IF NOT EXISTS `disenchant_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `event_scripts` (
`id` INTEGER  NOT NULL DEFAULT '0',
`delay` INTEGER  NOT NULL DEFAULT '0',
`command` INTEGER  NOT NULL DEFAULT '0',
`datalong` INTEGER  NOT NULL DEFAULT '0',
`datalong2` INTEGER  NOT NULL DEFAULT '0',
`dataint` INTEGER NOT NULL DEFAULT '0',
`x` REAL NOT NULL DEFAULT '0',
`y` REAL NOT NULL DEFAULT '0',
`z` REAL NOT NULL DEFAULT '0',
`o` REAL NOT NULL DEFAULT '0',
`Comment` TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS `exploration_basexp` (
`level` INTEGER  NOT NULL DEFAULT '0',
`basexp` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`level`)
);
CREATE TABLE IF NOT EXISTS `fishing_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `game_event` (
`eventEntry` INTEGER  NOT NULL,
`start_time` TEXT NULL DEFAULT NULL,
`end_time` TEXT NULL DEFAULT NULL,
`occurence` INTEGER  NOT NULL DEFAULT '5184000',
`length` INTEGER  NOT NULL DEFAULT '2592000',
`holiday` INTEGER  NOT NULL DEFAULT '0',
`holidayStage` INTEGER  NOT NULL DEFAULT '0',
`description` TEXT DEFAULT NULL,
`world_event` INTEGER  NOT NULL DEFAULT '0',
`announce` INTEGER  DEFAULT '2',
PRIMARY KEY (`eventEntry`)
);
CREATE TABLE IF NOT EXISTS `game_event_arena_seasons` (
`eventEntry` INTEGER  NOT NULL,
`season` INTEGER  NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS `game_event_arena_seasons__season` ON `game_event_arena_seasons` (`season`,`eventEntry`);
CREATE TABLE IF NOT EXISTS `game_event_battleground_holiday` (
`EventEntry` INTEGER  NOT NULL,
`BattlegroundID` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`EventEntry`)
);
CREATE TABLE IF NOT EXISTS `game_event_condition` (
`eventEntry` INTEGER  NOT NULL,
`condition_id` INTEGER  NOT NULL DEFAULT '0',
`req_num` REAL DEFAULT '0',
`max_world_state_field` INTEGER  NOT NULL DEFAULT '0',
`done_world_state_field` INTEGER  NOT NULL DEFAULT '0',
`description` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`eventEntry`,`condition_id`)
);
CREATE TABLE IF NOT EXISTS `game_event_creature` (
`eventEntry` INTEGER NOT NULL,
`guid` INTEGER  NOT NULL,
PRIMARY KEY (`guid`,`eventEntry`)
);
CREATE TABLE IF NOT EXISTS `game_event_creature_quest` (
`eventEntry` INTEGER  NOT NULL,
`id` INTEGER  NOT NULL DEFAULT '0',
`quest` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`quest`)
);
CREATE TABLE IF NOT EXISTS `game_event_gameobject` (
`eventEntry` INTEGER NOT NULL,
`guid` INTEGER  NOT NULL,
PRIMARY KEY (`guid`,`eventEntry`)
);
CREATE TABLE IF NOT EXISTS `game_event_gameobject_quest` (
`eventEntry` INTEGER  NOT NULL,
`id` INTEGER  NOT NULL DEFAULT '0',
`quest` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`quest`,`eventEntry`)
);
CREATE TABLE IF NOT EXISTS `game_event_model_equip` (
`eventEntry` INTEGER NOT NULL,
`guid` INTEGER  NOT NULL DEFAULT '0',
`modelid` INTEGER  NOT NULL DEFAULT '0',
`equipment_id` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`guid`)
);
CREATE TABLE IF NOT EXISTS `game_event_npc_vendor` (
`eventEntry` INTEGER NOT NULL,
`guid` INTEGER  NOT NULL DEFAULT '0',
`slot` INTEGER NOT NULL DEFAULT '0',
`item` INTEGER  NOT NULL DEFAULT '0',
`maxcount` INTEGER  NOT NULL DEFAULT '0',
`incrtime` INTEGER  NOT NULL DEFAULT '0',
`ExtendedCost` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`guid`,`item`)
);
CREATE INDEX IF NOT EXISTS `game_event_npc_vendor__slot` ON `game_event_npc_vendor` (`slot`);
CREATE TABLE IF NOT EXISTS `game_event_npcflag` (
`eventEntry` INTEGER  NOT NULL,
`guid` INTEGER  NOT NULL DEFAULT '0',
`npcflag` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`guid`,`eventEntry`)
);
CREATE TABLE IF NOT EXISTS `game_event_pool` (
`eventEntry` INTEGER NOT NULL,
`pool_entry` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`pool_entry`)
);
CREATE TABLE IF NOT EXISTS `game_event_prerequisite` (
`eventEntry` INTEGER  NOT NULL,
`prerequisite_event` INTEGER  NOT NULL,
PRIMARY KEY (`eventEntry`,`prerequisite_event`)
);
CREATE TABLE IF NOT EXISTS `game_event_quest_condition` (
`eventEntry` INTEGER  NOT NULL,
`quest` INTEGER  NOT NULL DEFAULT '0',
`condition_id` INTEGER  NOT NULL DEFAULT '0',
`num` REAL DEFAULT '0',
PRIMARY KEY (`quest`)
);
CREATE TABLE IF NOT EXISTS `game_event_seasonal_questrelation` (
`questId` INTEGER  NOT NULL,
`eventEntry` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`questId`,`eventEntry`)
);
CREATE INDEX IF NOT EXISTS `game_event_seasonal_questrelation__idx_quest` ON `game_event_seasonal_questrelation` (`questId`);
CREATE TABLE IF NOT EXISTS `game_tele` (
`id` INTEGER  NOT NULL,
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`map` INTEGER  NOT NULL DEFAULT '0',
`name` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `game_weather` (
`zone` INTEGER  NOT NULL DEFAULT '0',
`spring_rain_chance` INTEGER  NOT NULL DEFAULT '25',
`spring_snow_chance` INTEGER  NOT NULL DEFAULT '25',
`spring_storm_chance` INTEGER  NOT NULL DEFAULT '25',
`summer_rain_chance` INTEGER  NOT NULL DEFAULT '25',
`summer_snow_chance` INTEGER  NOT NULL DEFAULT '25',
`summer_storm_chance` INTEGER  NOT NULL DEFAULT '25',
`fall_rain_chance` INTEGER  NOT NULL DEFAULT '25',
`fall_snow_chance` INTEGER  NOT NULL DEFAULT '25',
`fall_storm_chance` INTEGER  NOT NULL DEFAULT '25',
`winter_rain_chance` INTEGER  NOT NULL DEFAULT '25',
`winter_snow_chance` INTEGER  NOT NULL DEFAULT '25',
`winter_storm_chance` INTEGER  NOT NULL DEFAULT '25',
`ScriptName` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`zone`)
);
CREATE TABLE IF NOT EXISTS `gameobject` (
`guid` INTEGER  NOT NULL,
`id` INTEGER  NOT NULL DEFAULT '0',
`map` INTEGER  NOT NULL DEFAULT '0',
`zoneId` INTEGER  NOT NULL DEFAULT '0',
`areaId` INTEGER  NOT NULL DEFAULT '0',
`spawnMask` INTEGER  NOT NULL DEFAULT '1',
`phaseMask` INTEGER  NOT NULL DEFAULT '1',
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`rotation0` REAL NOT NULL DEFAULT '0',
`rotation1` REAL NOT NULL DEFAULT '0',
`rotation2` REAL NOT NULL DEFAULT '0',
`rotation3` REAL NOT NULL DEFAULT '0',
`spawntimesecs` INTEGER NOT NULL DEFAULT '0',
`animprogress` INTEGER  NOT NULL DEFAULT '0',
`state` INTEGER  NOT NULL DEFAULT '0',
`ScriptName` TEXT DEFAULT '',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`guid`)
);
CREATE INDEX IF NOT EXISTS `gameobject__idx_map` ON `gameobject` (`map`);
CREATE INDEX IF NOT EXISTS `gameobject__idx_id` ON `gameobject` (`id`);
CREATE INDEX IF NOT EXISTS `gameobject__idx_map_coords` ON `gameobject` (`map`, `position_x`, `position_y`);
CREATE TABLE IF NOT EXISTS `gameobject_addon` (
`guid` INTEGER  NOT NULL DEFAULT '0',
`parent_rotation0` REAL NOT NULL DEFAULT '0',
`parent_rotation1` REAL NOT NULL DEFAULT '0',
`parent_rotation2` REAL NOT NULL DEFAULT '0',
`parent_rotation3` REAL NOT NULL DEFAULT '1',
`invisibilityType` INTEGER  NOT NULL DEFAULT '0',
`invisibilityValue` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`guid`)
);
CREATE TABLE IF NOT EXISTS `gameobject_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `gameobject_overrides` (
`spawnId` INTEGER  NOT NULL DEFAULT '0',
`faction` INTEGER  NOT NULL DEFAULT '0',
`flags` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`spawnId`)
);
CREATE TABLE IF NOT EXISTS `gameobject_questender` (
`id` INTEGER  NOT NULL DEFAULT '0',
`quest` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`quest`)
);
CREATE TABLE IF NOT EXISTS `gameobject_questitem` (
`GameObjectEntry` INTEGER  NOT NULL DEFAULT '0',
`Idx` INTEGER  NOT NULL DEFAULT '0',
`ItemId` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`GameObjectEntry`,`Idx`)
);
CREATE TABLE IF NOT EXISTS `gameobject_queststarter` (
`id` INTEGER  NOT NULL DEFAULT '0',
`quest` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`quest`)
);
CREATE TABLE IF NOT EXISTS `gameobject_template` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`type` INTEGER  NOT NULL DEFAULT '0',
`displayId` INTEGER  NOT NULL DEFAULT '0',
`name` TEXT NOT NULL DEFAULT '',
`IconName` TEXT NOT NULL DEFAULT '',
`castBarCaption` TEXT NOT NULL DEFAULT '',
`unk1` TEXT NOT NULL DEFAULT '',
`size` REAL NOT NULL DEFAULT '1',
`Data0` INTEGER  NOT NULL DEFAULT '0',
`Data1` INTEGER NOT NULL DEFAULT '0',
`Data2` INTEGER  NOT NULL DEFAULT '0',
`Data3` INTEGER  NOT NULL DEFAULT '0',
`Data4` INTEGER  NOT NULL DEFAULT '0',
`Data5` INTEGER  NOT NULL DEFAULT '0',
`Data6` INTEGER NOT NULL DEFAULT '0',
`Data7` INTEGER  NOT NULL DEFAULT '0',
`Data8` INTEGER  NOT NULL DEFAULT '0',
`Data9` INTEGER  NOT NULL DEFAULT '0',
`Data10` INTEGER  NOT NULL DEFAULT '0',
`Data11` INTEGER  NOT NULL DEFAULT '0',
`Data12` INTEGER  NOT NULL DEFAULT '0',
`Data13` INTEGER  NOT NULL DEFAULT '0',
`Data14` INTEGER  NOT NULL DEFAULT '0',
`Data15` INTEGER  NOT NULL DEFAULT '0',
`Data16` INTEGER  NOT NULL DEFAULT '0',
`Data17` INTEGER  NOT NULL DEFAULT '0',
`Data18` INTEGER  NOT NULL DEFAULT '0',
`Data19` INTEGER  NOT NULL DEFAULT '0',
`Data20` INTEGER  NOT NULL DEFAULT '0',
`Data21` INTEGER  NOT NULL DEFAULT '0',
`Data22` INTEGER  NOT NULL DEFAULT '0',
`Data23` INTEGER  NOT NULL DEFAULT '0',
`AIName` TEXT NOT NULL DEFAULT '',
`ScriptName` TEXT NOT NULL DEFAULT '',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE INDEX IF NOT EXISTS `gameobject_template__idx_name` ON `gameobject_template` (`name`);
CREATE TABLE IF NOT EXISTS `gameobject_template_addon` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`faction` INTEGER  NOT NULL DEFAULT '0',
`flags` INTEGER  NOT NULL DEFAULT '0',
`mingold` INTEGER  NOT NULL DEFAULT '0',
`maxgold` INTEGER  NOT NULL DEFAULT '0',
`artkit0` INTEGER NOT NULL DEFAULT '0',
`artkit1` INTEGER NOT NULL DEFAULT '0',
`artkit2` INTEGER NOT NULL DEFAULT '0',
`artkit3` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `gameobject_template_locale` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`name` TEXT,
`castBarCaption` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`entry`,`locale`)
);
CREATE TABLE IF NOT EXISTS `gossip_menu` (
`MenuID` INTEGER  NOT NULL DEFAULT '0',
`TextID` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`MenuID`,`TextID`)
);
CREATE TABLE IF NOT EXISTS `gossip_menu_option` (
`MenuID` INTEGER  NOT NULL DEFAULT '0',
`OptionID` INTEGER  NOT NULL DEFAULT '0',
`OptionIcon` INTEGER  NOT NULL DEFAULT '0',
`OptionText` TEXT,
`OptionBroadcastTextID` INTEGER NOT NULL DEFAULT '0',
`OptionType` INTEGER  NOT NULL DEFAULT '0',
`OptionNpcFlag` INTEGER  NOT NULL DEFAULT '0',
`ActionMenuID` INTEGER  NOT NULL DEFAULT '0',
`ActionPoiID` INTEGER  NOT NULL DEFAULT '0',
`BoxCoded` INTEGER  NOT NULL DEFAULT '0',
`BoxMoney` INTEGER  NOT NULL DEFAULT '0',
`BoxText` TEXT,
`BoxBroadcastTextID` INTEGER NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`MenuID`,`OptionID`)
);
CREATE TABLE IF NOT EXISTS `gossip_menu_option_locale` (
`MenuID` INTEGER  NOT NULL DEFAULT '0',
`OptionID` INTEGER  NOT NULL DEFAULT '0',
`Locale` TEXT NOT NULL,
`OptionText` TEXT,
`BoxText` TEXT,
PRIMARY KEY (`MenuID`,`OptionID`,`Locale`)
);
CREATE TABLE IF NOT EXISTS `graveyard_zone` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`GhostZone` INTEGER  NOT NULL DEFAULT '0',
`Faction` INTEGER  NOT NULL DEFAULT '0',
`Comment` TEXT,
PRIMARY KEY (`ID`,`GhostZone`)
);
CREATE TABLE IF NOT EXISTS `holiday_dates` (
`id` INTEGER  NOT NULL,
`date_id` INTEGER  NOT NULL,
`date_value` INTEGER  NOT NULL,
`holiday_duration` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`date_id`)
);
CREATE TABLE IF NOT EXISTS `instance_encounters` (
`entry` INTEGER  NOT NULL,
`creditType` INTEGER  NOT NULL DEFAULT '0',
`creditEntry` INTEGER  NOT NULL DEFAULT '0',
`lastEncounterDungeon` INTEGER  NOT NULL DEFAULT '0',
`comment` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `instance_spawn_groups` (
`instanceMapId` INTEGER  NOT NULL,
`bossStateId` INTEGER  NOT NULL,
`bossStates` INTEGER  NOT NULL,
`spawnGroupId` INTEGER  NOT NULL,
`flags` INTEGER  NOT NULL,
PRIMARY KEY (`instanceMapId`,`bossStateId`,`spawnGroupId`,`bossStates`)
);
CREATE TABLE IF NOT EXISTS `instance_template` (
`map` INTEGER  NOT NULL,
`parent` INTEGER  NOT NULL,
`script` TEXT NOT NULL DEFAULT '',
`allowMount` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`map`)
);
CREATE TABLE IF NOT EXISTS `item_enchantment_template` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`ench` INTEGER  NOT NULL DEFAULT '0',
`chance` REAL  NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`,`ench`)
);
CREATE TABLE IF NOT EXISTS `item_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `item_set_names` (
`entry` INTEGER  NOT NULL,
`name` TEXT NOT NULL DEFAULT '',
`InventoryType` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `item_set_names_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Name` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `item_template` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`class` INTEGER  NOT NULL DEFAULT '0',
`subclass` INTEGER  NOT NULL DEFAULT '0',
`SoundOverrideSubclass` INTEGER NOT NULL DEFAULT '-1',
`name` TEXT NOT NULL DEFAULT '',
`displayid` INTEGER  NOT NULL DEFAULT '0',
`Quality` INTEGER  NOT NULL DEFAULT '0',
`Flags` INTEGER  NOT NULL DEFAULT '0',
`FlagsExtra` INTEGER  NOT NULL DEFAULT '0',
`BuyCount` INTEGER  NOT NULL DEFAULT '1',
`BuyPrice` INTEGER NOT NULL DEFAULT '0',
`SellPrice` INTEGER  NOT NULL DEFAULT '0',
`InventoryType` INTEGER  NOT NULL DEFAULT '0',
`AllowableClass` INTEGER NOT NULL DEFAULT '-1',
`AllowableRace` INTEGER NOT NULL DEFAULT '-1',
`ItemLevel` INTEGER  NOT NULL DEFAULT '0',
`RequiredLevel` INTEGER  NOT NULL DEFAULT '0',
`RequiredSkill` INTEGER  NOT NULL DEFAULT '0',
`RequiredSkillRank` INTEGER  NOT NULL DEFAULT '0',
`requiredspell` INTEGER  NOT NULL DEFAULT '0',
`requiredhonorrank` INTEGER  NOT NULL DEFAULT '0',
`RequiredCityRank` INTEGER  NOT NULL DEFAULT '0',
`RequiredReputationFaction` INTEGER  NOT NULL DEFAULT '0',
`RequiredReputationRank` INTEGER  NOT NULL DEFAULT '0',
`maxcount` INTEGER NOT NULL DEFAULT '0',
`stackable` INTEGER DEFAULT '1',
`ContainerSlots` INTEGER  NOT NULL DEFAULT '0',
`StatsCount` INTEGER  NOT NULL DEFAULT '0',
`stat_type1` INTEGER  NOT NULL DEFAULT '0',
`stat_value1` INTEGER NOT NULL DEFAULT '0',
`stat_type2` INTEGER  NOT NULL DEFAULT '0',
`stat_value2` INTEGER NOT NULL DEFAULT '0',
`stat_type3` INTEGER  NOT NULL DEFAULT '0',
`stat_value3` INTEGER NOT NULL DEFAULT '0',
`stat_type4` INTEGER  NOT NULL DEFAULT '0',
`stat_value4` INTEGER NOT NULL DEFAULT '0',
`stat_type5` INTEGER  NOT NULL DEFAULT '0',
`stat_value5` INTEGER NOT NULL DEFAULT '0',
`stat_type6` INTEGER  NOT NULL DEFAULT '0',
`stat_value6` INTEGER NOT NULL DEFAULT '0',
`stat_type7` INTEGER  NOT NULL DEFAULT '0',
`stat_value7` INTEGER NOT NULL DEFAULT '0',
`stat_type8` INTEGER  NOT NULL DEFAULT '0',
`stat_value8` INTEGER NOT NULL DEFAULT '0',
`stat_type9` INTEGER  NOT NULL DEFAULT '0',
`stat_value9` INTEGER NOT NULL DEFAULT '0',
`stat_type10` INTEGER  NOT NULL DEFAULT '0',
`stat_value10` INTEGER NOT NULL DEFAULT '0',
`ScalingStatDistribution` INTEGER NOT NULL DEFAULT '0',
`ScalingStatValue` INTEGER  NOT NULL DEFAULT '0',
`dmg_min1` REAL NOT NULL DEFAULT '0',
`dmg_max1` REAL NOT NULL DEFAULT '0',
`dmg_type1` INTEGER  NOT NULL DEFAULT '0',
`dmg_min2` REAL NOT NULL DEFAULT '0',
`dmg_max2` REAL NOT NULL DEFAULT '0',
`dmg_type2` INTEGER  NOT NULL DEFAULT '0',
`armor` INTEGER  NOT NULL DEFAULT '0',
`holy_res` INTEGER  NOT NULL DEFAULT '0',
`fire_res` INTEGER  NOT NULL DEFAULT '0',
`nature_res` INTEGER  NOT NULL DEFAULT '0',
`frost_res` INTEGER  NOT NULL DEFAULT '0',
`shadow_res` INTEGER  NOT NULL DEFAULT '0',
`arcane_res` INTEGER  NOT NULL DEFAULT '0',
`delay` INTEGER  NOT NULL DEFAULT '1000',
`ammo_type` INTEGER  NOT NULL DEFAULT '0',
`RangedModRange` REAL NOT NULL DEFAULT '0',
`spellid_1` INTEGER NOT NULL DEFAULT '0',
`spelltrigger_1` INTEGER  NOT NULL DEFAULT '0',
`spellcharges_1` INTEGER NOT NULL DEFAULT '0',
`spellppmRate_1` REAL NOT NULL DEFAULT '0',
`spellcooldown_1` INTEGER NOT NULL DEFAULT '-1',
`spellcategory_1` INTEGER  NOT NULL DEFAULT '0',
`spellcategorycooldown_1` INTEGER NOT NULL DEFAULT '-1',
`spellid_2` INTEGER NOT NULL DEFAULT '0',
`spelltrigger_2` INTEGER  NOT NULL DEFAULT '0',
`spellcharges_2` INTEGER NOT NULL DEFAULT '0',
`spellppmRate_2` REAL NOT NULL DEFAULT '0',
`spellcooldown_2` INTEGER NOT NULL DEFAULT '-1',
`spellcategory_2` INTEGER  NOT NULL DEFAULT '0',
`spellcategorycooldown_2` INTEGER NOT NULL DEFAULT '-1',
`spellid_3` INTEGER NOT NULL DEFAULT '0',
`spelltrigger_3` INTEGER  NOT NULL DEFAULT '0',
`spellcharges_3` INTEGER NOT NULL DEFAULT '0',
`spellppmRate_3` REAL NOT NULL DEFAULT '0',
`spellcooldown_3` INTEGER NOT NULL DEFAULT '-1',
`spellcategory_3` INTEGER  NOT NULL DEFAULT '0',
`spellcategorycooldown_3` INTEGER NOT NULL DEFAULT '-1',
`spellid_4` INTEGER NOT NULL DEFAULT '0',
`spelltrigger_4` INTEGER  NOT NULL DEFAULT '0',
`spellcharges_4` INTEGER NOT NULL DEFAULT '0',
`spellppmRate_4` REAL NOT NULL DEFAULT '0',
`spellcooldown_4` INTEGER NOT NULL DEFAULT '-1',
`spellcategory_4` INTEGER  NOT NULL DEFAULT '0',
`spellcategorycooldown_4` INTEGER NOT NULL DEFAULT '-1',
`spellid_5` INTEGER NOT NULL DEFAULT '0',
`spelltrigger_5` INTEGER  NOT NULL DEFAULT '0',
`spellcharges_5` INTEGER NOT NULL DEFAULT '0',
`spellppmRate_5` REAL NOT NULL DEFAULT '0',
`spellcooldown_5` INTEGER NOT NULL DEFAULT '-1',
`spellcategory_5` INTEGER  NOT NULL DEFAULT '0',
`spellcategorycooldown_5` INTEGER NOT NULL DEFAULT '-1',
`bonding` INTEGER  NOT NULL DEFAULT '0',
`description` TEXT NOT NULL DEFAULT '',
`PageText` INTEGER  NOT NULL DEFAULT '0',
`LanguageID` INTEGER  NOT NULL DEFAULT '0',
`PageMaterial` INTEGER  NOT NULL DEFAULT '0',
`startquest` INTEGER  NOT NULL DEFAULT '0',
`lockid` INTEGER  NOT NULL DEFAULT '0',
`Material` INTEGER NOT NULL DEFAULT '0',
`sheath` INTEGER  NOT NULL DEFAULT '0',
`RandomProperty` INTEGER NOT NULL DEFAULT '0',
`RandomSuffix` INTEGER  NOT NULL DEFAULT '0',
`block` INTEGER  NOT NULL DEFAULT '0',
`itemset` INTEGER  NOT NULL DEFAULT '0',
`MaxDurability` INTEGER  NOT NULL DEFAULT '0',
`area` INTEGER  NOT NULL DEFAULT '0',
`Map` INTEGER NOT NULL DEFAULT '0',
`BagFamily` INTEGER NOT NULL DEFAULT '0',
`TotemCategory` INTEGER NOT NULL DEFAULT '0',
`socketColor_1` INTEGER NOT NULL DEFAULT '0',
`socketContent_1` INTEGER NOT NULL DEFAULT '0',
`socketColor_2` INTEGER NOT NULL DEFAULT '0',
`socketContent_2` INTEGER NOT NULL DEFAULT '0',
`socketColor_3` INTEGER NOT NULL DEFAULT '0',
`socketContent_3` INTEGER NOT NULL DEFAULT '0',
`socketBonus` INTEGER NOT NULL DEFAULT '0',
`GemProperties` INTEGER NOT NULL DEFAULT '0',
`RequiredDisenchantSkill` INTEGER NOT NULL DEFAULT '-1',
`ArmorDamageModifier` REAL NOT NULL DEFAULT '0',
`duration` INTEGER  NOT NULL DEFAULT '0',
`ItemLimitCategory` INTEGER NOT NULL DEFAULT '0',
`HolidayId` INTEGER  NOT NULL DEFAULT '0',
`ScriptName` TEXT NOT NULL DEFAULT '',
`DisenchantID` INTEGER  NOT NULL DEFAULT '0',
`FoodType` INTEGER  NOT NULL DEFAULT '0',
`minMoneyLoot` INTEGER  NOT NULL DEFAULT '0',
`maxMoneyLoot` INTEGER  NOT NULL DEFAULT '0',
`flagsCustom` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE INDEX IF NOT EXISTS `item_template__idx_name` ON `item_template` (`name`);
CREATE INDEX IF NOT EXISTS `item_template__items_index` ON `item_template` (`class`);
CREATE TABLE IF NOT EXISTS `item_template_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Name` TEXT,
`Description` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `lfg_dungeon_rewards` (
`dungeonId` INTEGER  NOT NULL DEFAULT '0',
`maxLevel` INTEGER  NOT NULL DEFAULT '0',
`firstQuestId` INTEGER  NOT NULL DEFAULT '0',
`otherQuestId` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`dungeonId`,`maxLevel`)
);
CREATE TABLE IF NOT EXISTS `lfg_dungeon_template` (
`dungeonId` INTEGER  NOT NULL DEFAULT '0',
`name` TEXT   DEFAULT NULL,
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`dungeonId`)
);
CREATE TABLE IF NOT EXISTS `linked_respawn` (
`guid` INTEGER  NOT NULL,
`linkedGuid` INTEGER  NOT NULL,
`linkType` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`guid`,`linkType`)
);
CREATE TABLE IF NOT EXISTS `mail_level_reward` (
`level` INTEGER  NOT NULL DEFAULT '0',
`raceMask` INTEGER  NOT NULL DEFAULT '0',
`mailTemplateId` INTEGER  NOT NULL DEFAULT '0',
`senderEntry` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`level`,`raceMask`)
);
CREATE TABLE IF NOT EXISTS `mail_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `milling_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `npc_spellclick_spells` (
`npc_entry` INTEGER  NOT NULL,
`spell_id` INTEGER  NOT NULL,
`cast_flags` INTEGER  NOT NULL,
`user_type` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`npc_entry`,`spell_id`)
);
CREATE TABLE IF NOT EXISTS `npc_text` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`text0_0` TEXT,
`text0_1` TEXT,
`BroadcastTextID0` INTEGER NOT NULL DEFAULT '0',
`lang0` INTEGER  NOT NULL DEFAULT '0',
`Probability0` REAL NOT NULL DEFAULT '0',
`EmoteDelay0_0` INTEGER  NOT NULL DEFAULT '0',
`Emote0_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay0_1` INTEGER  NOT NULL DEFAULT '0',
`Emote0_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay0_2` INTEGER  NOT NULL DEFAULT '0',
`Emote0_2` INTEGER  NOT NULL DEFAULT '0',
`text1_0` TEXT,
`text1_1` TEXT,
`BroadcastTextID1` INTEGER NOT NULL DEFAULT '0',
`lang1` INTEGER  NOT NULL DEFAULT '0',
`Probability1` REAL NOT NULL DEFAULT '0',
`EmoteDelay1_0` INTEGER  NOT NULL DEFAULT '0',
`Emote1_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay1_1` INTEGER  NOT NULL DEFAULT '0',
`Emote1_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay1_2` INTEGER  NOT NULL DEFAULT '0',
`Emote1_2` INTEGER  NOT NULL DEFAULT '0',
`text2_0` TEXT,
`text2_1` TEXT,
`BroadcastTextID2` INTEGER NOT NULL DEFAULT '0',
`lang2` INTEGER  NOT NULL DEFAULT '0',
`Probability2` REAL NOT NULL DEFAULT '0',
`EmoteDelay2_0` INTEGER  NOT NULL DEFAULT '0',
`Emote2_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay2_1` INTEGER  NOT NULL DEFAULT '0',
`Emote2_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay2_2` INTEGER  NOT NULL DEFAULT '0',
`Emote2_2` INTEGER  NOT NULL DEFAULT '0',
`text3_0` TEXT,
`text3_1` TEXT,
`BroadcastTextID3` INTEGER NOT NULL DEFAULT '0',
`lang3` INTEGER  NOT NULL DEFAULT '0',
`Probability3` REAL NOT NULL DEFAULT '0',
`EmoteDelay3_0` INTEGER  NOT NULL DEFAULT '0',
`Emote3_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay3_1` INTEGER  NOT NULL DEFAULT '0',
`Emote3_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay3_2` INTEGER  NOT NULL DEFAULT '0',
`Emote3_2` INTEGER  NOT NULL DEFAULT '0',
`text4_0` TEXT,
`text4_1` TEXT,
`BroadcastTextID4` INTEGER NOT NULL DEFAULT '0',
`lang4` INTEGER  NOT NULL DEFAULT '0',
`Probability4` REAL NOT NULL DEFAULT '0',
`EmoteDelay4_0` INTEGER  NOT NULL DEFAULT '0',
`Emote4_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay4_1` INTEGER  NOT NULL DEFAULT '0',
`Emote4_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay4_2` INTEGER  NOT NULL DEFAULT '0',
`Emote4_2` INTEGER  NOT NULL DEFAULT '0',
`text5_0` TEXT,
`text5_1` TEXT,
`BroadcastTextID5` INTEGER NOT NULL DEFAULT '0',
`lang5` INTEGER  NOT NULL DEFAULT '0',
`Probability5` REAL NOT NULL DEFAULT '0',
`EmoteDelay5_0` INTEGER  NOT NULL DEFAULT '0',
`Emote5_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay5_1` INTEGER  NOT NULL DEFAULT '0',
`Emote5_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay5_2` INTEGER  NOT NULL DEFAULT '0',
`Emote5_2` INTEGER  NOT NULL DEFAULT '0',
`text6_0` TEXT,
`text6_1` TEXT,
`BroadcastTextID6` INTEGER NOT NULL DEFAULT '0',
`lang6` INTEGER  NOT NULL DEFAULT '0',
`Probability6` REAL NOT NULL DEFAULT '0',
`EmoteDelay6_0` INTEGER  NOT NULL DEFAULT '0',
`Emote6_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay6_1` INTEGER  NOT NULL DEFAULT '0',
`Emote6_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay6_2` INTEGER  NOT NULL DEFAULT '0',
`Emote6_2` INTEGER  NOT NULL DEFAULT '0',
`text7_0` TEXT,
`text7_1` TEXT,
`BroadcastTextID7` INTEGER NOT NULL DEFAULT '0',
`lang7` INTEGER  NOT NULL DEFAULT '0',
`Probability7` REAL NOT NULL DEFAULT '0',
`EmoteDelay7_0` INTEGER  NOT NULL DEFAULT '0',
`Emote7_0` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay7_1` INTEGER  NOT NULL DEFAULT '0',
`Emote7_1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay7_2` INTEGER  NOT NULL DEFAULT '0',
`Emote7_2` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `npc_text_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Locale` TEXT NOT NULL,
`Text0_0` TEXT,
`Text0_1` TEXT,
`Text1_0` TEXT,
`Text1_1` TEXT,
`Text2_0` TEXT,
`Text2_1` TEXT,
`Text3_0` TEXT,
`Text3_1` TEXT,
`Text4_0` TEXT,
`Text4_1` TEXT,
`Text5_0` TEXT,
`Text5_1` TEXT,
`Text6_0` TEXT,
`Text6_1` TEXT,
`Text7_0` TEXT,
`Text7_1` TEXT,
PRIMARY KEY (`ID`,`Locale`)
);
CREATE TABLE IF NOT EXISTS `npc_vendor` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`slot` INTEGER NOT NULL DEFAULT '0',
`item` INTEGER NOT NULL DEFAULT '0',
`maxcount` INTEGER  NOT NULL DEFAULT '0',
`incrtime` INTEGER  NOT NULL DEFAULT '0',
`ExtendedCost` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`entry`,`item`,`ExtendedCost`)
);
CREATE INDEX IF NOT EXISTS `npc_vendor__slot` ON `npc_vendor` (`slot`);
CREATE TABLE IF NOT EXISTS `outdoorpvp_template` (
`TypeId` INTEGER  NOT NULL,
`ScriptName` TEXT NOT NULL DEFAULT '',
`comment` TEXT,
PRIMARY KEY (`TypeId`)
);
CREATE TABLE IF NOT EXISTS `page_text` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Text` TEXT NOT NULL,
`NextPageID` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `page_text_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Text` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `pet_levelstats` (
`creature_entry` INTEGER  NOT NULL,
`level` INTEGER  NOT NULL,
`hp` INTEGER  NOT NULL,
`mana` INTEGER  NOT NULL,
`armor` INTEGER  NOT NULL DEFAULT '0',
`str` INTEGER  NOT NULL,
`agi` INTEGER  NOT NULL,
`sta` INTEGER  NOT NULL,
`inte` INTEGER  NOT NULL,
`spi` INTEGER  NOT NULL,
`min_dmg` INTEGER  NOT NULL DEFAULT '0',
`max_dmg` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`creature_entry`,`level`)
);
CREATE TABLE IF NOT EXISTS `pet_name_generation` (
`id` INTEGER  NOT NULL,
`word` TEXT NOT NULL,
`entry` INTEGER  NOT NULL DEFAULT '0',
`half` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `pickpocketing_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `player_classlevelstats` (
`class` INTEGER  NOT NULL,
`level` INTEGER  NOT NULL,
`basehp` INTEGER  NOT NULL,
`basemana` INTEGER  NOT NULL,
PRIMARY KEY (`class`,`level`)
);
CREATE TABLE IF NOT EXISTS `player_factionchange_achievement` (
`alliance_id` INTEGER NOT NULL,
`horde_id` INTEGER NOT NULL,
PRIMARY KEY (`alliance_id`,`horde_id`)
);
CREATE TABLE IF NOT EXISTS `player_factionchange_items` (
`race_A` INTEGER NOT NULL,
`alliance_id` INTEGER NOT NULL,
`commentA` TEXT,
`race_H` INTEGER NOT NULL,
`horde_id` INTEGER NOT NULL,
`commentH` TEXT,
PRIMARY KEY (`alliance_id`,`horde_id`)
);
CREATE TABLE IF NOT EXISTS `player_factionchange_quests` (
`alliance_id` INTEGER  NOT NULL,
`horde_id` INTEGER  NOT NULL,
PRIMARY KEY (`alliance_id`,`horde_id`)
);
CREATE UNIQUE INDEX IF NOT EXISTS `player_factionchange_quests__alliance_uniq` ON `player_factionchange_quests` (`alliance_id`);
CREATE UNIQUE INDEX IF NOT EXISTS `player_factionchange_quests__horde_uniq` ON `player_factionchange_quests` (`horde_id`);
CREATE TABLE IF NOT EXISTS `player_factionchange_reputations` (
`alliance_id` INTEGER NOT NULL,
`horde_id` INTEGER NOT NULL,
PRIMARY KEY (`alliance_id`,`horde_id`)
);
CREATE TABLE IF NOT EXISTS `player_factionchange_spells` (
`alliance_id` INTEGER NOT NULL,
`horde_id` INTEGER NOT NULL,
PRIMARY KEY (`alliance_id`,`horde_id`)
);
CREATE TABLE IF NOT EXISTS `player_factionchange_titles` (
`alliance_id` INTEGER NOT NULL,
`horde_id` INTEGER NOT NULL,
PRIMARY KEY (`alliance_id`,`horde_id`)
);
CREATE TABLE IF NOT EXISTS `player_levelstats` (
`race` INTEGER  NOT NULL,
`class` INTEGER  NOT NULL,
`level` INTEGER  NOT NULL,
`str` INTEGER  NOT NULL,
`agi` INTEGER  NOT NULL,
`sta` INTEGER  NOT NULL,
`inte` INTEGER  NOT NULL,
`spi` INTEGER  NOT NULL,
PRIMARY KEY (`race`,`class`,`level`)
);
CREATE TABLE IF NOT EXISTS `player_totem_model` (
`TotemSlot` INTEGER  NOT NULL,
`RaceId` INTEGER  NOT NULL,
`DisplayId` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`TotemSlot`,`RaceId`)
);
CREATE TABLE IF NOT EXISTS `player_xp_for_level` (
`Level` INTEGER  NOT NULL,
`Experience` INTEGER  NOT NULL,
PRIMARY KEY (`Level`)
);
CREATE TABLE IF NOT EXISTS `playercreateinfo` (
`race` INTEGER  NOT NULL DEFAULT '0',
`class` INTEGER  NOT NULL DEFAULT '0',
`map` INTEGER  NOT NULL DEFAULT '0',
`zone` INTEGER  NOT NULL DEFAULT '0',
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
PRIMARY KEY (`race`,`class`)
);
CREATE TABLE IF NOT EXISTS `playercreateinfo_action` (
`race` INTEGER  NOT NULL DEFAULT '0',
`class` INTEGER  NOT NULL DEFAULT '0',
`button` INTEGER  NOT NULL DEFAULT '0',
`action` INTEGER  NOT NULL DEFAULT '0',
`type` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`race`,`class`,`button`)
);
CREATE INDEX IF NOT EXISTS `playercreateinfo_action__playercreateinfo_race_class_index` ON `playercreateinfo_action` (`race`,`class`);
CREATE TABLE IF NOT EXISTS `playercreateinfo_cast_spell` (
`raceMask` INTEGER  NOT NULL DEFAULT '0',
`classMask` INTEGER  NOT NULL DEFAULT '0',
`spell` INTEGER  NOT NULL DEFAULT '0',
`note` TEXT DEFAULT NULL
);
CREATE TABLE IF NOT EXISTS `playercreateinfo_item` (
`race` INTEGER  NOT NULL DEFAULT '0',
`class` INTEGER  NOT NULL DEFAULT '0',
`itemid` INTEGER  NOT NULL DEFAULT '0',
`amount` INTEGER NOT NULL DEFAULT '1'
);
CREATE INDEX IF NOT EXISTS `playercreateinfo_item__playercreateinfo_race_class_index` ON `playercreateinfo_item` (`race`,`class`);
CREATE TABLE IF NOT EXISTS `playercreateinfo_skills` (
`raceMask` INTEGER  NOT NULL,
`classMask` INTEGER  NOT NULL,
`skill` INTEGER  NOT NULL,
`rank` INTEGER  NOT NULL DEFAULT '0',
`comment` TEXT DEFAULT NULL,
PRIMARY KEY (`raceMask`,`classMask`,`skill`)
);
CREATE TABLE IF NOT EXISTS `playercreateinfo_spell_custom` (
`racemask` INTEGER  NOT NULL DEFAULT '0',
`classmask` INTEGER  NOT NULL DEFAULT '0',
`Spell` INTEGER  NOT NULL DEFAULT '0',
`Note` TEXT DEFAULT NULL,
PRIMARY KEY (`racemask`,`classmask`,`Spell`)
);
CREATE TABLE IF NOT EXISTS `points_of_interest` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`PositionX` REAL NOT NULL DEFAULT '0',
`PositionY` REAL NOT NULL DEFAULT '0',
`Icon` INTEGER  NOT NULL DEFAULT '0',
`Flags` INTEGER  NOT NULL DEFAULT '0',
`Importance` INTEGER  NOT NULL DEFAULT '0',
`Name` TEXT NOT NULL,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `points_of_interest_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Name` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `pool_members` (
`type` INTEGER  NOT NULL,
`spawnId` INTEGER  NOT NULL,
`poolSpawnId` INTEGER  NOT NULL,
`chance` REAL NOT NULL,
`description` TEXT DEFAULT NULL,
PRIMARY KEY (`type`,`spawnId`)
);
CREATE TABLE IF NOT EXISTS `pool_template` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`max_limit` INTEGER  NOT NULL DEFAULT '0',
`description` TEXT DEFAULT NULL,
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `prospecting_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `quest_details` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Emote1` INTEGER  NOT NULL DEFAULT '0',
`Emote2` INTEGER  NOT NULL DEFAULT '0',
`Emote3` INTEGER  NOT NULL DEFAULT '0',
`Emote4` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay2` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay3` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay4` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `quest_greeting` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Type` INTEGER  NOT NULL DEFAULT '0',
`GreetEmoteType` INTEGER  NOT NULL DEFAULT '0',
`GreetEmoteDelay` INTEGER  NOT NULL DEFAULT '0',
`Greeting` TEXT,
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`,`Type`)
);
CREATE TABLE IF NOT EXISTS `quest_greeting_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Type` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Greeting` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`Type`,`locale`)
);
CREATE TABLE IF NOT EXISTS `quest_mail_sender` (
`QuestId` INTEGER  NOT NULL DEFAULT '0',
`RewardMailSenderEntry` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`QuestId`)
);
CREATE TABLE IF NOT EXISTS `quest_offer_reward` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`Emote1` INTEGER  NOT NULL DEFAULT '0',
`Emote2` INTEGER  NOT NULL DEFAULT '0',
`Emote3` INTEGER  NOT NULL DEFAULT '0',
`Emote4` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay1` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay2` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay3` INTEGER  NOT NULL DEFAULT '0',
`EmoteDelay4` INTEGER  NOT NULL DEFAULT '0',
`RewardText` TEXT,
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `quest_offer_reward_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`RewardText` TEXT,
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `quest_poi` (
`QuestID` INTEGER  NOT NULL DEFAULT '0',
`id` INTEGER  NOT NULL DEFAULT '0',
`ObjectiveIndex` INTEGER NOT NULL DEFAULT '0',
`MapID` INTEGER  NOT NULL DEFAULT '0',
`WorldMapAreaId` INTEGER  NOT NULL DEFAULT '0',
`Floor` INTEGER  NOT NULL DEFAULT '0',
`Priority` INTEGER  NOT NULL DEFAULT '0',
`Flags` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`QuestID`,`id`)
);
CREATE INDEX IF NOT EXISTS `quest_poi__idx` ON `quest_poi` (`QuestID`,`id`);
CREATE TABLE IF NOT EXISTS `quest_poi_points` (
`QuestID` INTEGER  NOT NULL DEFAULT '0',
`Idx1` INTEGER  NOT NULL DEFAULT '0',
`Idx2` INTEGER  NOT NULL DEFAULT '0',
`X` INTEGER NOT NULL DEFAULT '0',
`Y` INTEGER NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`QuestID`,`Idx1`,`Idx2`)
);
CREATE INDEX IF NOT EXISTS `quest_poi_points__questId_id` ON `quest_poi_points` (`QuestID`,`Idx1`);
CREATE TABLE IF NOT EXISTS `quest_pool_members` (
`questId` INTEGER  NOT NULL,
`poolId` INTEGER  NOT NULL,
`poolIndex` INTEGER  NOT NULL,
`description` TEXT DEFAULT NULL,
PRIMARY KEY (`questId`)
);
CREATE TABLE IF NOT EXISTS `quest_pool_template` (
`poolId` INTEGER  NOT NULL,
`numActive` INTEGER  NOT NULL,
`description` TEXT DEFAULT NULL,
PRIMARY KEY (`poolId`)
);
CREATE TABLE IF NOT EXISTS `quest_request_items` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`EmoteOnComplete` INTEGER  NOT NULL DEFAULT '0',
`EmoteOnIncomplete` INTEGER  NOT NULL DEFAULT '0',
`CompletionText` TEXT,
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `quest_request_items_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`CompletionText` TEXT,
`VerifiedBuild` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `quest_template` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`QuestType` INTEGER  NOT NULL DEFAULT '2',
`QuestLevel` INTEGER NOT NULL DEFAULT '1',
`MinLevel` INTEGER  NOT NULL DEFAULT '0',
`QuestSortID` INTEGER NOT NULL DEFAULT '0',
`QuestInfoID` INTEGER  NOT NULL DEFAULT '0',
`SuggestedGroupNum` INTEGER  NOT NULL DEFAULT '0',
`RequiredFactionId1` INTEGER  NOT NULL DEFAULT '0',
`RequiredFactionId2` INTEGER  NOT NULL DEFAULT '0',
`RequiredFactionValue1` INTEGER NOT NULL DEFAULT '0',
`RequiredFactionValue2` INTEGER NOT NULL DEFAULT '0',
`RewardNextQuest` INTEGER  NOT NULL DEFAULT '0',
`RewardXPDifficulty` INTEGER  NOT NULL DEFAULT '0',
`RewardMoney` INTEGER NOT NULL DEFAULT '0',
`RewardBonusMoney` INTEGER  NOT NULL DEFAULT '0',
`RewardDisplaySpell` INTEGER  NOT NULL DEFAULT '0',
`RewardSpell` INTEGER NOT NULL DEFAULT '0',
`RewardHonor` INTEGER NOT NULL DEFAULT '0',
`RewardKillHonor` REAL NOT NULL DEFAULT '0',
`StartItem` INTEGER  NOT NULL DEFAULT '0',
`Flags` INTEGER  NOT NULL DEFAULT '0',
`RequiredPlayerKills` INTEGER  NOT NULL DEFAULT '0',
`RewardItem1` INTEGER  NOT NULL DEFAULT '0',
`RewardAmount1` INTEGER  NOT NULL DEFAULT '0',
`RewardItem2` INTEGER  NOT NULL DEFAULT '0',
`RewardAmount2` INTEGER  NOT NULL DEFAULT '0',
`RewardItem3` INTEGER  NOT NULL DEFAULT '0',
`RewardAmount3` INTEGER  NOT NULL DEFAULT '0',
`RewardItem4` INTEGER  NOT NULL DEFAULT '0',
`RewardAmount4` INTEGER  NOT NULL DEFAULT '0',
`ItemDrop1` INTEGER  NOT NULL DEFAULT '0',
`ItemDropQuantity1` INTEGER  NOT NULL DEFAULT '0',
`ItemDrop2` INTEGER  NOT NULL DEFAULT '0',
`ItemDropQuantity2` INTEGER  NOT NULL DEFAULT '0',
`ItemDrop3` INTEGER  NOT NULL DEFAULT '0',
`ItemDropQuantity3` INTEGER  NOT NULL DEFAULT '0',
`ItemDrop4` INTEGER  NOT NULL DEFAULT '0',
`ItemDropQuantity4` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemID1` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemQuantity1` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemID2` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemQuantity2` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemID3` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemQuantity3` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemID4` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemQuantity4` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemID5` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemQuantity5` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemID6` INTEGER  NOT NULL DEFAULT '0',
`RewardChoiceItemQuantity6` INTEGER  NOT NULL DEFAULT '0',
`POIContinent` INTEGER  NOT NULL DEFAULT '0',
`POIx` REAL NOT NULL DEFAULT '0',
`POIy` REAL NOT NULL DEFAULT '0',
`POIPriority` INTEGER  NOT NULL DEFAULT '0',
`RewardTitle` INTEGER  NOT NULL DEFAULT '0',
`RewardTalents` INTEGER  NOT NULL DEFAULT '0',
`RewardArenaPoints` INTEGER  NOT NULL DEFAULT '0',
`RewardFactionID1` INTEGER  NOT NULL DEFAULT '0',
`RewardFactionValue1` INTEGER NOT NULL DEFAULT '0',
`RewardFactionOverride1` INTEGER NOT NULL DEFAULT '0',
`RewardFactionID2` INTEGER  NOT NULL DEFAULT '0',
`RewardFactionValue2` INTEGER NOT NULL DEFAULT '0',
`RewardFactionOverride2` INTEGER NOT NULL DEFAULT '0',
`RewardFactionID3` INTEGER  NOT NULL DEFAULT '0',
`RewardFactionValue3` INTEGER NOT NULL DEFAULT '0',
`RewardFactionOverride3` INTEGER NOT NULL DEFAULT '0',
`RewardFactionID4` INTEGER  NOT NULL DEFAULT '0',
`RewardFactionValue4` INTEGER NOT NULL DEFAULT '0',
`RewardFactionOverride4` INTEGER NOT NULL DEFAULT '0',
`RewardFactionID5` INTEGER  NOT NULL DEFAULT '0',
`RewardFactionValue5` INTEGER NOT NULL DEFAULT '0',
`RewardFactionOverride5` INTEGER NOT NULL DEFAULT '0',
`TimeAllowed` INTEGER  NOT NULL DEFAULT '0',
`AllowableRaces` INTEGER  NOT NULL DEFAULT '0',
`LogTitle` TEXT,
`LogDescription` TEXT,
`QuestDescription` TEXT,
`AreaDescription` TEXT,
`QuestCompletionLog` TEXT,
`RequiredNpcOrGo1` INTEGER NOT NULL DEFAULT '0',
`RequiredNpcOrGo2` INTEGER NOT NULL DEFAULT '0',
`RequiredNpcOrGo3` INTEGER NOT NULL DEFAULT '0',
`RequiredNpcOrGo4` INTEGER NOT NULL DEFAULT '0',
`RequiredNpcOrGoCount1` INTEGER  NOT NULL DEFAULT '0',
`RequiredNpcOrGoCount2` INTEGER  NOT NULL DEFAULT '0',
`RequiredNpcOrGoCount3` INTEGER  NOT NULL DEFAULT '0',
`RequiredNpcOrGoCount4` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemId1` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemId2` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemId3` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemId4` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemId5` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemId6` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemCount1` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemCount2` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemCount3` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemCount4` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemCount5` INTEGER  NOT NULL DEFAULT '0',
`RequiredItemCount6` INTEGER  NOT NULL DEFAULT '0',
`Unknown0` INTEGER  NOT NULL DEFAULT '0',
`ObjectiveText1` TEXT,
`ObjectiveText2` TEXT,
`ObjectiveText3` TEXT,
`ObjectiveText4` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `quest_template_addon` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`MaxLevel` INTEGER  NOT NULL DEFAULT '0',
`AllowableClasses` INTEGER  NOT NULL DEFAULT '0',
`SourceSpellID` INTEGER  NOT NULL DEFAULT '0',
`PrevQuestID` INTEGER NOT NULL DEFAULT '0',
`NextQuestID` INTEGER  NOT NULL DEFAULT '0',
`ExclusiveGroup` INTEGER NOT NULL DEFAULT '0',
`BreadcrumbForQuestId` INTEGER NOT NULL DEFAULT '0',
`RewardMailTemplateID` INTEGER  NOT NULL DEFAULT '0',
`RewardMailDelay` INTEGER  NOT NULL DEFAULT '0',
`RequiredSkillID` INTEGER  NOT NULL DEFAULT '0',
`RequiredSkillPoints` INTEGER  NOT NULL DEFAULT '0',
`RequiredMinRepFaction` INTEGER  NOT NULL DEFAULT '0',
`RequiredMaxRepFaction` INTEGER  NOT NULL DEFAULT '0',
`RequiredMinRepValue` INTEGER NOT NULL DEFAULT '0',
`RequiredMaxRepValue` INTEGER NOT NULL DEFAULT '0',
`ProvidedItemCount` INTEGER  NOT NULL DEFAULT '0',
`SpecialFlags` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`ID`)
);
CREATE TABLE IF NOT EXISTS `quest_template_locale` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Title` TEXT,
`Details` TEXT,
`Objectives` TEXT,
`EndText` TEXT,
`CompletedText` TEXT,
`ObjectiveText1` TEXT,
`ObjectiveText2` TEXT,
`ObjectiveText3` TEXT,
`ObjectiveText4` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`locale`)
);
CREATE TABLE IF NOT EXISTS `reference_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `reputation_reward_rate` (
`faction` INTEGER  NOT NULL DEFAULT '0',
`quest_rate` REAL NOT NULL DEFAULT '1',
`quest_daily_rate` REAL NOT NULL DEFAULT '1',
`quest_weekly_rate` REAL NOT NULL DEFAULT '1',
`quest_monthly_rate` REAL NOT NULL DEFAULT '1',
`quest_repeatable_rate` REAL NOT NULL DEFAULT '1',
`creature_rate` REAL NOT NULL DEFAULT '1',
`spell_rate` REAL NOT NULL DEFAULT '1',
PRIMARY KEY (`faction`)
);
CREATE TABLE IF NOT EXISTS `reputation_spillover_template` (
`faction` INTEGER  NOT NULL DEFAULT '0',
`faction1` INTEGER  NOT NULL DEFAULT '0',
`rate_1` REAL NOT NULL DEFAULT '0',
`rank_1` INTEGER  NOT NULL DEFAULT '0',
`faction2` INTEGER  NOT NULL DEFAULT '0',
`rate_2` REAL NOT NULL DEFAULT '0',
`rank_2` INTEGER  NOT NULL DEFAULT '0',
`faction3` INTEGER  NOT NULL DEFAULT '0',
`rate_3` REAL NOT NULL DEFAULT '0',
`rank_3` INTEGER  NOT NULL DEFAULT '0',
`faction4` INTEGER  NOT NULL DEFAULT '0',
`rate_4` REAL NOT NULL DEFAULT '0',
`rank_4` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`faction`)
);
CREATE TABLE IF NOT EXISTS `script_spline_chain_meta` (
`entry` INTEGER  NOT NULL,
`chainId` INTEGER  NOT NULL,
`splineId` INTEGER  NOT NULL,
`expectedDuration` INTEGER  NOT NULL,
`msUntilNext` INTEGER  NOT NULL,
`velocity` REAL  DEFAULT '0',
PRIMARY KEY (`entry`,`chainId`,`splineId`)
);
CREATE TABLE IF NOT EXISTS `script_spline_chain_waypoints` (
`entry` INTEGER  NOT NULL,
`chainId` INTEGER  NOT NULL,
`splineId` INTEGER  NOT NULL,
`wpId` INTEGER  NOT NULL,
`x` REAL NOT NULL,
`y` REAL NOT NULL,
`z` REAL NOT NULL,
PRIMARY KEY (`entry`,`chainId`,`splineId`,`wpId`)
);
CREATE TABLE IF NOT EXISTS `script_waypoint` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`pointid` INTEGER  NOT NULL DEFAULT '0',
`location_x` REAL NOT NULL DEFAULT '0',
`location_y` REAL NOT NULL DEFAULT '0',
`location_z` REAL NOT NULL DEFAULT '0',
`waittime` INTEGER  NOT NULL DEFAULT '0',
`point_comment` TEXT,
PRIMARY KEY (`entry`,`pointid`)
);
CREATE TABLE IF NOT EXISTS `skill_discovery_template` (
`spellId` INTEGER  NOT NULL DEFAULT '0',
`reqSpell` INTEGER  NOT NULL DEFAULT '0',
`reqSkillValue` INTEGER  NOT NULL DEFAULT '0',
`chance` REAL NOT NULL DEFAULT '0',
PRIMARY KEY (`spellId`,`reqSpell`)
);
CREATE TABLE IF NOT EXISTS `skill_extra_item_template` (
`spellId` INTEGER  NOT NULL DEFAULT '0',
`requiredSpecialization` INTEGER  NOT NULL DEFAULT '0',
`additionalCreateChance` REAL NOT NULL DEFAULT '0',
`additionalMaxNum` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`spellId`)
);
CREATE TABLE IF NOT EXISTS `skill_fishing_base_level` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`skill` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `skill_perfect_item_template` (
`spellId` INTEGER  NOT NULL DEFAULT '0',
`requiredSpecialization` INTEGER  NOT NULL DEFAULT '0',
`perfectCreateChance` REAL NOT NULL DEFAULT '0',
`perfectItemType` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`spellId`)
);
CREATE TABLE IF NOT EXISTS `skinning_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `smart_scripts` (
`entryorguid` INTEGER NOT NULL,
`source_type` INTEGER  NOT NULL DEFAULT '0',
`id` INTEGER  NOT NULL DEFAULT '0',
`link` INTEGER  NOT NULL DEFAULT '0',
`event_type` INTEGER  NOT NULL DEFAULT '0',
`event_phase_mask` INTEGER  NOT NULL DEFAULT '0',
`event_chance` INTEGER  NOT NULL DEFAULT '100',
`event_flags` INTEGER  NOT NULL DEFAULT '0',
`event_param1` INTEGER  NOT NULL DEFAULT '0',
`event_param2` INTEGER  NOT NULL DEFAULT '0',
`event_param3` INTEGER  NOT NULL DEFAULT '0',
`event_param4` INTEGER  NOT NULL DEFAULT '0',
`event_param5` INTEGER  NOT NULL DEFAULT '0',
`action_type` INTEGER  NOT NULL DEFAULT '0',
`action_param1` INTEGER  NOT NULL DEFAULT '0',
`action_param2` INTEGER  NOT NULL DEFAULT '0',
`action_param3` INTEGER  NOT NULL DEFAULT '0',
`action_param4` INTEGER  NOT NULL DEFAULT '0',
`action_param5` INTEGER  NOT NULL DEFAULT '0',
`action_param6` INTEGER  NOT NULL DEFAULT '0',
`target_type` INTEGER  NOT NULL DEFAULT '0',
`target_param1` INTEGER  NOT NULL DEFAULT '0',
`target_param2` INTEGER  NOT NULL DEFAULT '0',
`target_param3` INTEGER  NOT NULL DEFAULT '0',
`target_param4` INTEGER  NOT NULL DEFAULT '0',
`target_x` REAL NOT NULL DEFAULT '0',
`target_y` REAL NOT NULL DEFAULT '0',
`target_z` REAL NOT NULL DEFAULT '0',
`target_o` REAL NOT NULL DEFAULT '0',
`comment` TEXT NOT NULL,
PRIMARY KEY (`entryorguid`,`source_type`,`id`,`link`)
);
CREATE TABLE IF NOT EXISTS `spawn_group` (
`groupId` INTEGER  NOT NULL,
`spawnType` INTEGER  NOT NULL,
`spawnId` INTEGER  NOT NULL,
PRIMARY KEY (`groupId`,`spawnType`,`spawnId`)
);
CREATE TABLE IF NOT EXISTS `spawn_group_template` (
`groupId` INTEGER  NOT NULL,
`groupName` TEXT NOT NULL,
`groupFlags` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`groupId`)
);
CREATE TABLE IF NOT EXISTS `spell_area` (
`spell` INTEGER  NOT NULL DEFAULT '0',
`area` INTEGER  NOT NULL DEFAULT '0',
`quest_start` INTEGER  NOT NULL DEFAULT '0',
`quest_end` INTEGER  NOT NULL DEFAULT '0',
`aura_spell` INTEGER NOT NULL DEFAULT '0',
`racemask` INTEGER  NOT NULL DEFAULT '0',
`gender` INTEGER  NOT NULL DEFAULT '2',
`autocast` INTEGER  NOT NULL DEFAULT '0',
`quest_start_status` INTEGER NOT NULL DEFAULT '64',
`quest_end_status` INTEGER NOT NULL DEFAULT '11',
PRIMARY KEY (`spell`,`area`,`quest_start`,`aura_spell`,`racemask`,`gender`)
);
CREATE TABLE IF NOT EXISTS `spell_bonus_data` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`direct_bonus` REAL NOT NULL DEFAULT '0',
`dot_bonus` REAL NOT NULL DEFAULT '0',
`ap_bonus` REAL NOT NULL DEFAULT '0',
`ap_dot_bonus` REAL NOT NULL DEFAULT '0',
`comments` TEXT DEFAULT NULL,
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `spell_custom_attr` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`attributes` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `spell_dbc` (
`Id` INTEGER  NOT NULL,
`Dispel` INTEGER  NOT NULL DEFAULT '0',
`Mechanic` INTEGER  NOT NULL DEFAULT '0',
`Attributes` INTEGER  NOT NULL DEFAULT '0',
`AttributesEx` INTEGER  NOT NULL DEFAULT '0',
`AttributesEx2` INTEGER  NOT NULL DEFAULT '0',
`AttributesEx3` INTEGER  NOT NULL DEFAULT '0',
`AttributesEx4` INTEGER  NOT NULL DEFAULT '0',
`AttributesEx5` INTEGER  NOT NULL DEFAULT '0',
`AttributesEx6` INTEGER  NOT NULL DEFAULT '0',
`AttributesEx7` INTEGER  NOT NULL DEFAULT '0',
`Stances` INTEGER  NOT NULL DEFAULT '0',
`StancesNot` INTEGER  NOT NULL DEFAULT '0',
`Targets` INTEGER  NOT NULL DEFAULT '0',
`CastingTimeIndex` INTEGER  NOT NULL DEFAULT '1',
`AuraInterruptFlags` INTEGER  NOT NULL DEFAULT '0',
`ProcFlags` INTEGER  NOT NULL DEFAULT '0',
`ProcChance` INTEGER  NOT NULL DEFAULT '0',
`ProcCharges` INTEGER  NOT NULL DEFAULT '0',
`MaxLevel` INTEGER  NOT NULL DEFAULT '0',
`BaseLevel` INTEGER  NOT NULL DEFAULT '0',
`SpellLevel` INTEGER  NOT NULL DEFAULT '0',
`DurationIndex` INTEGER  NOT NULL DEFAULT '0',
`RangeIndex` INTEGER  NOT NULL DEFAULT '1',
`StackAmount` INTEGER  NOT NULL DEFAULT '0',
`EquippedItemClass` INTEGER NOT NULL DEFAULT '-1',
`EquippedItemSubClassMask` INTEGER NOT NULL DEFAULT '0',
`EquippedItemInventoryTypeMask` INTEGER NOT NULL DEFAULT '0',
`Effect1` INTEGER  NOT NULL DEFAULT '0',
`Effect2` INTEGER  NOT NULL DEFAULT '0',
`Effect3` INTEGER  NOT NULL DEFAULT '0',
`EffectDieSides1` INTEGER NOT NULL DEFAULT '0',
`EffectDieSides2` INTEGER NOT NULL DEFAULT '0',
`EffectDieSides3` INTEGER NOT NULL DEFAULT '0',
`EffectRealPointsPerLevel1` REAL NOT NULL DEFAULT '0',
`EffectRealPointsPerLevel2` REAL NOT NULL DEFAULT '0',
`EffectRealPointsPerLevel3` REAL NOT NULL DEFAULT '0',
`EffectBasePoints1` INTEGER NOT NULL DEFAULT '0',
`EffectBasePoints2` INTEGER NOT NULL DEFAULT '0',
`EffectBasePoints3` INTEGER NOT NULL DEFAULT '0',
`EffectMechanic1` INTEGER  NOT NULL DEFAULT '0',
`EffectMechanic2` INTEGER  NOT NULL DEFAULT '0',
`EffectMechanic3` INTEGER  NOT NULL DEFAULT '0',
`EffectImplicitTargetA1` INTEGER  NOT NULL DEFAULT '0',
`EffectImplicitTargetA2` INTEGER  NOT NULL DEFAULT '0',
`EffectImplicitTargetA3` INTEGER  NOT NULL DEFAULT '0',
`EffectImplicitTargetB1` INTEGER  NOT NULL DEFAULT '0',
`EffectImplicitTargetB2` INTEGER  NOT NULL DEFAULT '0',
`EffectImplicitTargetB3` INTEGER  NOT NULL DEFAULT '0',
`EffectRadiusIndex1` INTEGER  NOT NULL DEFAULT '0',
`EffectRadiusIndex2` INTEGER  NOT NULL DEFAULT '0',
`EffectRadiusIndex3` INTEGER  NOT NULL DEFAULT '0',
`EffectApplyAuraName1` INTEGER  NOT NULL DEFAULT '0',
`EffectApplyAuraName2` INTEGER  NOT NULL DEFAULT '0',
`EffectApplyAuraName3` INTEGER  NOT NULL DEFAULT '0',
`EffectAmplitude1` INTEGER NOT NULL DEFAULT '0',
`EffectAmplitude2` INTEGER NOT NULL DEFAULT '0',
`EffectAmplitude3` INTEGER NOT NULL DEFAULT '0',
`EffectMultipleValue1` REAL NOT NULL DEFAULT '0',
`EffectMultipleValue2` REAL NOT NULL DEFAULT '0',
`EffectMultipleValue3` REAL NOT NULL DEFAULT '0',
`EffectItemType1` INTEGER  NOT NULL DEFAULT '0',
`EffectItemType2` INTEGER  NOT NULL DEFAULT '0',
`EffectItemType3` INTEGER  NOT NULL DEFAULT '0',
`EffectMiscValue1` INTEGER NOT NULL DEFAULT '0',
`EffectMiscValue2` INTEGER NOT NULL DEFAULT '0',
`EffectMiscValue3` INTEGER NOT NULL DEFAULT '0',
`EffectMiscValueB1` INTEGER NOT NULL DEFAULT '0',
`EffectMiscValueB2` INTEGER NOT NULL DEFAULT '0',
`EffectMiscValueB3` INTEGER NOT NULL DEFAULT '0',
`EffectTriggerSpell1` INTEGER  NOT NULL DEFAULT '0',
`EffectTriggerSpell2` INTEGER  NOT NULL DEFAULT '0',
`EffectTriggerSpell3` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskA1` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskA2` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskA3` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskB1` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskB2` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskB3` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskC1` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskC2` INTEGER  NOT NULL DEFAULT '0',
`EffectSpellClassMaskC3` INTEGER  NOT NULL DEFAULT '0',
`SpellName` TEXT DEFAULT NULL,
`MaxTargetLevel` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyName` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyFlags1` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyFlags2` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyFlags3` INTEGER  NOT NULL DEFAULT '0',
`MaxAffectedTargets` INTEGER  NOT NULL DEFAULT '0',
`DmgClass` INTEGER  NOT NULL DEFAULT '0',
`PreventionType` INTEGER  NOT NULL DEFAULT '0',
`DmgMultiplier1` REAL NOT NULL DEFAULT '0',
`DmgMultiplier2` REAL NOT NULL DEFAULT '0',
`DmgMultiplier3` REAL NOT NULL DEFAULT '0',
`AreaGroupId` INTEGER NOT NULL DEFAULT '0',
`SchoolMask` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`Id`)
);
CREATE TABLE IF NOT EXISTS `spell_enchant_proc_data` (
`EnchantID` INTEGER  NOT NULL,
`Chance` REAL NOT NULL DEFAULT '0',
`ProcsPerMinute` REAL NOT NULL DEFAULT '0',
`HitMask` INTEGER  NOT NULL DEFAULT '0',
`AttributesMask` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`EnchantID`)
);
CREATE TABLE IF NOT EXISTS `spell_group` (
`id` INTEGER  NOT NULL DEFAULT '0',
`spell_id` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`spell_id`)
);
CREATE TABLE IF NOT EXISTS `spell_group_stack_rules` (
`group_id` INTEGER  NOT NULL DEFAULT '0',
`stack_rule` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`group_id`)
);
CREATE TABLE IF NOT EXISTS `spell_learn_spell` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`SpellID` INTEGER  NOT NULL DEFAULT '0',
`Active` INTEGER  NOT NULL DEFAULT '1',
PRIMARY KEY (`entry`,`SpellID`)
);
CREATE TABLE IF NOT EXISTS `spell_linked_spell` (
`spell_trigger` INTEGER NOT NULL,
`spell_effect` INTEGER NOT NULL DEFAULT '0',
`type` INTEGER  NOT NULL DEFAULT '0',
`comment` TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS `spell_linked_spell__trigger_effect_type` ON `spell_linked_spell` (`spell_trigger`,`spell_effect`,`type`);
CREATE TABLE IF NOT EXISTS `spell_loot_template` (
`Entry` INTEGER  NOT NULL DEFAULT '0',
`Item` INTEGER  NOT NULL DEFAULT '0',
`Reference` INTEGER  NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '100',
`QuestRequired` INTEGER NOT NULL DEFAULT '0',
`LootMode` INTEGER  NOT NULL DEFAULT '1',
`GroupId` INTEGER  NOT NULL DEFAULT '0',
`MinCount` INTEGER  NOT NULL DEFAULT '1',
`MaxCount` INTEGER  NOT NULL DEFAULT '1',
`Comment` TEXT DEFAULT NULL,
PRIMARY KEY (`Entry`,`Item`)
);
CREATE TABLE IF NOT EXISTS `spell_pet_auras` (
`spell` INTEGER  NOT NULL,
`effectId` INTEGER  NOT NULL DEFAULT '0',
`pet` INTEGER  NOT NULL DEFAULT '0',
`aura` INTEGER  NOT NULL,
PRIMARY KEY (`spell`,`effectId`,`pet`)
);
CREATE TABLE IF NOT EXISTS `spell_proc` (
`SpellId` INTEGER NOT NULL DEFAULT '0',
`SchoolMask` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyName` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyMask0` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyMask1` INTEGER  NOT NULL DEFAULT '0',
`SpellFamilyMask2` INTEGER  NOT NULL DEFAULT '0',
`ProcFlags` INTEGER  NOT NULL DEFAULT '0',
`SpellTypeMask` INTEGER  NOT NULL DEFAULT '0',
`SpellPhaseMask` INTEGER  NOT NULL DEFAULT '0',
`HitMask` INTEGER  NOT NULL DEFAULT '0',
`AttributesMask` INTEGER  NOT NULL DEFAULT '0',
`DisableEffectsMask` INTEGER  NOT NULL DEFAULT '0',
`ProcsPerMinute` REAL NOT NULL DEFAULT '0',
`Chance` REAL NOT NULL DEFAULT '0',
`Cooldown` INTEGER  NOT NULL DEFAULT '0',
`Charges` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`SpellId`)
);
CREATE TABLE IF NOT EXISTS `spell_ranks` (
`first_spell_id` INTEGER  NOT NULL DEFAULT '0',
`spell_id` INTEGER  NOT NULL DEFAULT '0',
`rank` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`first_spell_id`,`rank`)
);
CREATE UNIQUE INDEX IF NOT EXISTS `spell_ranks__spell_id` ON `spell_ranks` (`spell_id`);
CREATE TABLE IF NOT EXISTS `spell_required` (
`spell_id` INTEGER NOT NULL DEFAULT '0',
`req_spell` INTEGER NOT NULL DEFAULT '0',
PRIMARY KEY (`spell_id`,`req_spell`)
);
CREATE TABLE IF NOT EXISTS `spell_script_names` (
`spell_id` INTEGER NOT NULL,
`ScriptName` TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS `spell_script_names__spell_id` ON `spell_script_names` (`spell_id`,`ScriptName`);
CREATE TABLE IF NOT EXISTS `spell_scripts` (
`id` INTEGER  NOT NULL DEFAULT '0',
`effIndex` INTEGER  NOT NULL DEFAULT '0',
`delay` INTEGER  NOT NULL DEFAULT '0',
`command` INTEGER  NOT NULL DEFAULT '0',
`datalong` INTEGER  NOT NULL DEFAULT '0',
`datalong2` INTEGER  NOT NULL DEFAULT '0',
`dataint` INTEGER NOT NULL DEFAULT '0',
`x` REAL NOT NULL DEFAULT '0',
`y` REAL NOT NULL DEFAULT '0',
`z` REAL NOT NULL DEFAULT '0',
`o` REAL NOT NULL DEFAULT '0',
`Comment` TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS `spell_target_position` (
`ID` INTEGER  NOT NULL DEFAULT '0',
`EffectIndex` INTEGER  NOT NULL DEFAULT '0',
`MapID` INTEGER  NOT NULL DEFAULT '0',
`PositionX` REAL NOT NULL DEFAULT '0',
`PositionY` REAL NOT NULL DEFAULT '0',
`PositionZ` REAL NOT NULL DEFAULT '0',
`Orientation` REAL NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`ID`,`EffectIndex`)
);
CREATE TABLE IF NOT EXISTS `spell_threat` (
`entry` INTEGER  NOT NULL,
`flatMod` INTEGER DEFAULT NULL,
`pctMod` REAL NOT NULL DEFAULT '1',
`apPctMod` REAL NOT NULL DEFAULT '0',
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `spelldifficulty_dbc` (
`id` INTEGER  NOT NULL DEFAULT '0',
`spellid0` INTEGER  NOT NULL DEFAULT '0',
`spellid1` INTEGER  NOT NULL DEFAULT '0',
`spellid2` INTEGER  NOT NULL DEFAULT '0',
`spellid3` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `trainer` (
`Id` INTEGER  NOT NULL DEFAULT '0',
`Type` INTEGER  NOT NULL DEFAULT '2',
`Requirement` INTEGER  NOT NULL DEFAULT '0',
`Greeting` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`Id`)
);
CREATE TABLE IF NOT EXISTS `trainer_locale` (
`Id` INTEGER  NOT NULL DEFAULT '0',
`locale` TEXT NOT NULL,
`Greeting_lang` TEXT,
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`Id`,`locale`)
);
CREATE TABLE IF NOT EXISTS `trainer_spell` (
`TrainerId` INTEGER  NOT NULL DEFAULT '0',
`SpellId` INTEGER  NOT NULL DEFAULT '0',
`MoneyCost` INTEGER  NOT NULL DEFAULT '0',
`ReqSkillLine` INTEGER  NOT NULL DEFAULT '0',
`ReqSkillRank` INTEGER  NOT NULL DEFAULT '0',
`ReqAbility1` INTEGER  NOT NULL DEFAULT '0',
`ReqAbility2` INTEGER  NOT NULL DEFAULT '0',
`ReqAbility3` INTEGER  NOT NULL DEFAULT '0',
`ReqLevel` INTEGER  NOT NULL DEFAULT '0',
`VerifiedBuild` INTEGER DEFAULT '0',
PRIMARY KEY (`TrainerId`,`SpellId`)
);
CREATE TABLE IF NOT EXISTS `transports` (
`guid` INTEGER  NOT NULL,
`entry` INTEGER  NOT NULL DEFAULT '0',
`name` TEXT,
`ScriptName` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`guid`)
);
CREATE UNIQUE INDEX IF NOT EXISTS `transports__idx_entry` ON `transports` (`entry`);
CREATE TABLE IF NOT EXISTS `trinity_string` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`content_default` TEXT NOT NULL,
`content_loc1` TEXT,
`content_loc2` TEXT,
`content_loc3` TEXT,
`content_loc4` TEXT,
`content_loc5` TEXT,
`content_loc6` TEXT,
`content_loc7` TEXT,
`content_loc8` TEXT,
PRIMARY KEY (`entry`)
);
CREATE TABLE IF NOT EXISTS `updates` (
`name` TEXT NOT NULL,
`hash` TEXT DEFAULT '',
`state` TEXT NOT NULL DEFAULT 'RELEASED',
`timestamp` TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
`speed` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`name`)
);
CREATE TABLE IF NOT EXISTS `updates_include` (
`path` TEXT NOT NULL,
`state` TEXT NOT NULL DEFAULT 'RELEASED',
PRIMARY KEY (`path`)
);
CREATE TABLE IF NOT EXISTS `vehicle_accessory` (
`guid` INTEGER  NOT NULL DEFAULT '0',
`accessory_entry` INTEGER  NOT NULL DEFAULT '0',
`seat_id` INTEGER NOT NULL DEFAULT '0',
`minion` INTEGER  NOT NULL DEFAULT '0',
`description` TEXT NOT NULL,
`summontype` INTEGER  NOT NULL DEFAULT '6',
`summontimer` INTEGER  NOT NULL DEFAULT '30000',
PRIMARY KEY (`guid`,`seat_id`)
);
CREATE TABLE IF NOT EXISTS `vehicle_seat_addon` (
`SeatEntry` INTEGER  NOT NULL,
`SeatOrientation` REAL DEFAULT '0',
`ExitParamX` REAL DEFAULT '0',
`ExitParamY` REAL DEFAULT '0',
`ExitParamZ` REAL DEFAULT '0',
`ExitParamO` REAL DEFAULT '0',
`ExitParamValue` INTEGER DEFAULT '0',
PRIMARY KEY (`SeatEntry`)
);
CREATE TABLE IF NOT EXISTS `vehicle_template_accessory` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`accessory_entry` INTEGER  NOT NULL DEFAULT '0',
`seat_id` INTEGER NOT NULL DEFAULT '0',
`minion` INTEGER  NOT NULL DEFAULT '0',
`description` TEXT NOT NULL,
`summontype` INTEGER  NOT NULL DEFAULT '6',
`summontimer` INTEGER  NOT NULL DEFAULT '30000',
PRIMARY KEY (`entry`,`seat_id`)
);
CREATE TABLE IF NOT EXISTS `version` (
`core_version` TEXT NOT NULL DEFAULT '',
`core_revision` TEXT DEFAULT NULL,
`db_version` TEXT DEFAULT NULL,
`cache_id` INTEGER DEFAULT '0',
PRIMARY KEY (`core_version`)
);
CREATE TABLE IF NOT EXISTS `warden_checks` (
`id` INTEGER  NOT NULL,
`type` INTEGER  DEFAULT NULL,
`str` TEXT DEFAULT NULL,
`address` INTEGER  DEFAULT NULL,
`length` INTEGER  DEFAULT NULL,
`comment` TEXT DEFAULT NULL,
`data` BLOB DEFAULT NULL,
`result` BLOB DEFAULT NULL,
PRIMARY KEY (`id`)
);
CREATE TABLE IF NOT EXISTS `waypoint_data` (
`id` INTEGER  NOT NULL DEFAULT '0',
`point` INTEGER  NOT NULL DEFAULT '0',
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`delay` INTEGER  NOT NULL DEFAULT '0',
`move_type` INTEGER NOT NULL DEFAULT '0',
`action` INTEGER NOT NULL DEFAULT '0',
`action_chance` INTEGER NOT NULL DEFAULT '100',
`wpguid` INTEGER  NOT NULL DEFAULT '0',
PRIMARY KEY (`id`,`point`)
);
CREATE TABLE IF NOT EXISTS `waypoint_scripts` (
`id` INTEGER  NOT NULL DEFAULT '0',
`delay` INTEGER  NOT NULL DEFAULT '0',
`command` INTEGER  NOT NULL DEFAULT '0',
`datalong` INTEGER  NOT NULL DEFAULT '0',
`datalong2` INTEGER  NOT NULL DEFAULT '0',
`dataint` INTEGER  NOT NULL DEFAULT '0',
`x` REAL NOT NULL DEFAULT '0',
`y` REAL NOT NULL DEFAULT '0',
`z` REAL NOT NULL DEFAULT '0',
`o` REAL NOT NULL DEFAULT '0',
`guid` INTEGER NOT NULL DEFAULT '0',
`Comment` TEXT NOT NULL DEFAULT '',
PRIMARY KEY (`guid`)
);
CREATE TABLE IF NOT EXISTS `waypoints` (
`entry` INTEGER  NOT NULL DEFAULT '0',
`pointid` INTEGER  NOT NULL DEFAULT '0',
`position_x` REAL NOT NULL DEFAULT '0',
`position_y` REAL NOT NULL DEFAULT '0',
`position_z` REAL NOT NULL DEFAULT '0',
`orientation` REAL NOT NULL DEFAULT '0',
`delay` INTEGER  NOT NULL DEFAULT '0',
`point_comment` TEXT,
PRIMARY KEY (`entry`,`pointid`)
);
