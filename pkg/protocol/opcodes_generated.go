package protocol

type Opcode uint16

const (
	OpcodeCMSG_ACCEPT_LEVEL_GRANT                               Opcode = 0x420
	OpcodeCMSG_ACCEPT_TRADE                                     Opcode = 0x11A
	OpcodeCMSG_ACTIVATETAXI                                     Opcode = 0x1AD
	OpcodeCMSG_ACTIVATETAXIEXPRESS                              Opcode = 0x312
	OpcodeCMSG_ACTIVE_PVP_CHEAT                                 Opcode = 0x399
	OpcodeCMSG_ADD_FRIEND                                       Opcode = 0x069
	OpcodeCMSG_ADD_IGNORE                                       Opcode = 0x06C
	OpcodeCMSG_ADD_PVP_MEDAL_CHEAT                              Opcode = 0x289
	OpcodeCMSG_ADD_VOICE_IGNORE                                 Opcode = 0x3DB
	OpcodeCMSG_ADVANCE_SPAWN_TIME                               Opcode = 0x031
	OpcodeCMSG_AFK_MONITOR_INFO_CLEAR                           Opcode = 0x505
	OpcodeCMSG_AFK_MONITOR_INFO_REQUEST                         Opcode = 0x503
	OpcodeCMSG_ALTER_APPEARANCE                                 Opcode = 0x426
	OpcodeCMSG_AREATRIGGER                                      Opcode = 0x0B4
	OpcodeCMSG_AREA_SPIRIT_HEALER_QUERY                         Opcode = 0x2E2
	OpcodeCMSG_AREA_SPIRIT_HEALER_QUEUE                         Opcode = 0x2E3
	OpcodeCMSG_ARENA_TEAM_ACCEPT                                Opcode = 0x351
	OpcodeCMSG_ARENA_TEAM_CREATE                                Opcode = 0x348
	OpcodeCMSG_ARENA_TEAM_DECLINE                               Opcode = 0x352
	OpcodeCMSG_ARENA_TEAM_DISBAND                               Opcode = 0x355
	OpcodeCMSG_ARENA_TEAM_INVITE                                Opcode = 0x34F
	OpcodeCMSG_ARENA_TEAM_LEADER                                Opcode = 0x356
	OpcodeCMSG_ARENA_TEAM_LEAVE                                 Opcode = 0x353
	OpcodeCMSG_ARENA_TEAM_QUERY                                 Opcode = 0x34B
	OpcodeCMSG_ARENA_TEAM_REMOVE                                Opcode = 0x354
	OpcodeCMSG_ARENA_TEAM_ROSTER                                Opcode = 0x34D
	OpcodeCMSG_ATTACK_STOP                                      Opcode = 0x142
	OpcodeCMSG_ATTACK_SWING                                     Opcode = 0x141
	OpcodeCMSG_AUCTION_LIST_BIDDER_ITEMS                        Opcode = 0x264
	OpcodeCMSG_AUCTION_LIST_ITEMS                               Opcode = 0x258
	OpcodeCMSG_AUCTION_LIST_OWNER_ITEMS                         Opcode = 0x259
	OpcodeCMSG_AUCTION_LIST_PENDING_SALES                       Opcode = 0x48F
	OpcodeCMSG_AUCTION_PLACE_BID                                Opcode = 0x25A
	OpcodeCMSG_AUCTION_REMOVE_ITEM                              Opcode = 0x257
	OpcodeCMSG_AUCTION_SELL_ITEM                                Opcode = 0x256
	OpcodeCMSG_AUTH_SESSION                                     Opcode = 0x1ED
	OpcodeCMSG_AUTH_SRP6_BEGIN                                  Opcode = 0x033
	OpcodeCMSG_AUTH_SRP6_PROOF                                  Opcode = 0x034
	OpcodeCMSG_AUTH_SRP6_RECODE                                 Opcode = 0x035
	OpcodeCMSG_AUTOBANK_ITEM                                    Opcode = 0x283
	OpcodeCMSG_AUTOEQUIP_GROUND_ITEM                            Opcode = 0x106
	OpcodeCMSG_AUTOEQUIP_ITEM                                   Opcode = 0x10A
	OpcodeCMSG_AUTOEQUIP_ITEM_SLOT                              Opcode = 0x10F
	OpcodeCMSG_AUTOSTORE_BAG_ITEM                               Opcode = 0x10B
	OpcodeCMSG_AUTOSTORE_BANK_ITEM                              Opcode = 0x282
	OpcodeCMSG_AUTOSTORE_GROUND_ITEM                            Opcode = 0x107
	OpcodeCMSG_AUTOSTORE_LOOT_ITEM                              Opcode = 0x108
	OpcodeCMSG_BANKER_ACTIVATE                                  Opcode = 0x1B7
	OpcodeCMSG_BATTLEFIELD_JOIN                                 Opcode = 0x23E
	OpcodeCMSG_BATTLEFIELD_LIST                                 Opcode = 0x23C
	OpcodeCMSG_BATTLEFIELD_MANAGER_ADVANCE_STATE                Opcode = 0x4E9
	OpcodeCMSG_BATTLEFIELD_MANAGER_SET_NEXT_TRANSITION_TIME     Opcode = 0x4EA
	OpcodeCMSG_BATTLEFIELD_MGR_ENTRY_INVITE_RESPONSE            Opcode = 0x4DF
	OpcodeCMSG_BATTLEFIELD_MGR_EXIT_REQUEST                     Opcode = 0x4E7
	OpcodeCMSG_BATTLEFIELD_MGR_QUEUE_INVITE_RESPONSE            Opcode = 0x4E2
	OpcodeCMSG_BATTLEFIELD_MGR_QUEUE_REQUEST                    Opcode = 0x4E3
	OpcodeCMSG_BATTLEFIELD_PORT                                 Opcode = 0x2D5
	OpcodeCMSG_BATTLEFIELD_STATUS                               Opcode = 0x2D3
	OpcodeCMSG_BATTLEMASTER_HELLO                               Opcode = 0x2D7
	OpcodeCMSG_BATTLEMASTER_JOIN                                Opcode = 0x2EE
	OpcodeCMSG_BATTLEMASTER_JOIN_ARENA                          Opcode = 0x358
	OpcodeCMSG_BEASTMASTER                                      Opcode = 0x021
	OpcodeCMSG_BEGIN_TRADE                                      Opcode = 0x117
	OpcodeCMSG_BINDER_ACTIVATE                                  Opcode = 0x1B5
	OpcodeCMSG_BOOTME                                           Opcode = 0x001
	OpcodeCMSG_BOT_DETECTED                                     Opcode = 0x3C0
	OpcodeCMSG_BOT_DETECTED2                                    Opcode = 0x017
	OpcodeCMSG_BUG                                              Opcode = 0x1CA
	OpcodeCMSG_BUSY_TRADE                                       Opcode = 0x118
	OpcodeCMSG_BUYBACK_ITEM                                     Opcode = 0x290
	OpcodeCMSG_BUY_BANK_SLOT                                    Opcode = 0x1B9
	OpcodeCMSG_BUY_ITEM                                         Opcode = 0x1A2
	OpcodeCMSG_BUY_ITEM_IN_SLOT                                 Opcode = 0x1A3
	OpcodeCMSG_BUY_LOTTERY_TICKET_OBSOLETE                      Opcode = 0x336
	OpcodeCMSG_BUY_STABLE_SLOT                                  Opcode = 0x272
	OpcodeCMSG_CALENDAR_ADD_EVENT                               Opcode = 0x42D
	OpcodeCMSG_CALENDAR_ARENA_TEAM                              Opcode = 0x42C
	OpcodeCMSG_CALENDAR_COMPLAIN                                Opcode = 0x446
	OpcodeCMSG_CALENDAR_COPY_EVENT                              Opcode = 0x430
	OpcodeCMSG_CALENDAR_EVENT_INVITE                            Opcode = 0x431
	OpcodeCMSG_CALENDAR_EVENT_INVITE_NOTES                      Opcode = 0x45F
	OpcodeCMSG_CALENDAR_EVENT_MODERATOR_STATUS                  Opcode = 0x435
	OpcodeCMSG_CALENDAR_EVENT_REMOVE_INVITE                     Opcode = 0x433
	OpcodeCMSG_CALENDAR_EVENT_RSVP                              Opcode = 0x432
	OpcodeCMSG_CALENDAR_EVENT_SIGNUP                            Opcode = 0x4BA
	OpcodeCMSG_CALENDAR_EVENT_STATUS                            Opcode = 0x434
	OpcodeCMSG_CALENDAR_GET_CALENDAR                            Opcode = 0x429
	OpcodeCMSG_CALENDAR_GET_EVENT                               Opcode = 0x42A
	OpcodeCMSG_CALENDAR_GET_NUM_PENDING                         Opcode = 0x447
	OpcodeCMSG_CALENDAR_GUILD_FILTER                            Opcode = 0x42B
	OpcodeCMSG_CALENDAR_REMOVE_EVENT                            Opcode = 0x42F
	OpcodeCMSG_CALENDAR_UPDATE_EVENT                            Opcode = 0x42E
	OpcodeCMSG_CANCEL_AURA                                      Opcode = 0x136
	OpcodeCMSG_CANCEL_AUTO_REPEAT_SPELL                         Opcode = 0x26D
	OpcodeCMSG_CANCEL_CAST                                      Opcode = 0x12F
	OpcodeCMSG_CANCEL_CHANNELLING                               Opcode = 0x13B
	OpcodeCMSG_CANCEL_GROWTH_AURA                               Opcode = 0x29B
	OpcodeCMSG_CANCEL_MOUNT_AURA                                Opcode = 0x375
	OpcodeCMSG_CANCEL_TEMP_ENCHANTMENT                          Opcode = 0x379
	OpcodeCMSG_CANCEL_TRADE                                     Opcode = 0x11C
	OpcodeCMSG_CAST_SPELL                                       Opcode = 0x12E
	OpcodeCMSG_CHANGEPLAYER_DIFFICULTY                          Opcode = 0x1FD
	OpcodeCMSG_CHANGE_GDF_ARENA_RATING                          Opcode = 0x4AC
	OpcodeCMSG_CHANGE_PERSONAL_ARENA_RATING                     Opcode = 0x425
	OpcodeCMSG_CHANGE_SEATS_ON_CONTROLLED_VEHICLE               Opcode = 0x49B
	OpcodeCMSG_CHANNEL_ANNOUNCEMENTS                            Opcode = 0x0A7
	OpcodeCMSG_CHANNEL_BAN                                      Opcode = 0x0A5
	OpcodeCMSG_CHANNEL_DISPLAY_LIST                             Opcode = 0x3D2
	OpcodeCMSG_CHANNEL_INVITE                                   Opcode = 0x0A3
	OpcodeCMSG_CHANNEL_KICK                                     Opcode = 0x0A4
	OpcodeCMSG_CHANNEL_LIST                                     Opcode = 0x09A
	OpcodeCMSG_CHANNEL_MODERATE                                 Opcode = 0x0A8
	OpcodeCMSG_CHANNEL_MODERATOR                                Opcode = 0x09F
	OpcodeCMSG_CHANNEL_MUTE                                     Opcode = 0x0A1
	OpcodeCMSG_CHANNEL_OWNER                                    Opcode = 0x09E
	OpcodeCMSG_CHANNEL_PASSWORD                                 Opcode = 0x09C
	OpcodeCMSG_CHANNEL_SET_OWNER                                Opcode = 0x09D
	OpcodeCMSG_CHANNEL_SILENCE_ALL                              Opcode = 0x3CD
	OpcodeCMSG_CHANNEL_SILENCE_VOICE                            Opcode = 0x3CC
	OpcodeCMSG_CHANNEL_UNBAN                                    Opcode = 0x0A6
	OpcodeCMSG_CHANNEL_UNMODERATOR                              Opcode = 0x0A0
	OpcodeCMSG_CHANNEL_UNMUTE                                   Opcode = 0x0A2
	OpcodeCMSG_CHANNEL_UNSILENCE_ALL                            Opcode = 0x3CF
	OpcodeCMSG_CHANNEL_UNSILENCE_VOICE                          Opcode = 0x3CE
	OpcodeCMSG_CHANNEL_VOICE_OFF                                Opcode = 0x3D7
	OpcodeCMSG_CHANNEL_VOICE_ON                                 Opcode = 0x3D6
	OpcodeCMSG_CHARACTER_POINT_CHEAT                            Opcode = 0x223
	OpcodeCMSG_CHAR_CREATE                                      Opcode = 0x036
	OpcodeCMSG_CHAR_CUSTOMIZE                                   Opcode = 0x473
	OpcodeCMSG_CHAR_DELETE                                      Opcode = 0x038
	OpcodeCMSG_CHAR_ENUM                                        Opcode = 0x037
	OpcodeCMSG_CHAR_FACTION_CHANGE                              Opcode = 0x4D9
	OpcodeCMSG_CHAR_RACE_CHANGE                                 Opcode = 0x4F8
	OpcodeCMSG_CHAR_RENAME                                      Opcode = 0x2C7
	OpcodeCMSG_CHAT_FILTERED                                    Opcode = 0x331
	OpcodeCMSG_CHAT_IGNORED                                     Opcode = 0x225
	OpcodeCMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY                      Opcode = 0x39A
	OpcodeCMSG_CHEAT_PLAYER_LOGIN                               Opcode = 0x3C2
	OpcodeCMSG_CHEAT_PLAYER_LOOKUP                              Opcode = 0x3C3
	OpcodeCMSG_CHEAT_SETMONEY                                   Opcode = 0x024
	OpcodeCMSG_CHEAT_SET_ARENA_CURRENCY                         Opcode = 0x37C
	OpcodeCMSG_CHEAT_SET_HONOR_CURRENCY                         Opcode = 0x37B
	OpcodeCMSG_CHECK_LOGIN_CRITERIA                             Opcode = 0x4A2
	OpcodeCMSG_CLEAR_CHANNEL_WATCH                              Opcode = 0x3F3
	OpcodeCMSG_CLEAR_EXPLORATION                                Opcode = 0x237
	OpcodeCMSG_CLEAR_HOLIDAY_BG_WIN_TIME                        Opcode = 0x51A
	OpcodeCMSG_CLEAR_QUEST                                      Opcode = 0x02C
	OpcodeCMSG_CLEAR_RANDOM_BG_WIN_TIME                         Opcode = 0x519
	OpcodeCMSG_CLEAR_SERVER_BUCK_DATA                           Opcode = 0x41C
	OpcodeCMSG_CLEAR_TRADE_ITEM                                 Opcode = 0x11E
	OpcodeCMSG_COMMENTATOR_ENABLE                               Opcode = 0x3B5
	OpcodeCMSG_COMMENTATOR_ENTER_INSTANCE                       Opcode = 0x3BC
	OpcodeCMSG_COMMENTATOR_EXIT_INSTANCE                        Opcode = 0x3BD
	OpcodeCMSG_COMMENTATOR_GET_MAP_INFO                         Opcode = 0x3B7
	OpcodeCMSG_COMMENTATOR_GET_PLAYER_INFO                      Opcode = 0x3B9
	OpcodeCMSG_COMMENTATOR_INSTANCE_COMMAND                     Opcode = 0x3BE
	OpcodeCMSG_COMMENTATOR_SKIRMISH_QUEUE_COMMAND               Opcode = 0x51B
	OpcodeCMSG_COMPLAIN                                         Opcode = 0x3C7
	OpcodeCMSG_COMPLETE_ACHIEVEMENT_CHEAT                       Opcode = 0x46E
	OpcodeCMSG_COMPLETE_CINEMATIC                               Opcode = 0x0FC
	OpcodeCMSG_COMPLETE_MOVIE                                   Opcode = 0x465
	OpcodeCMSG_CONTACT_LIST                                     Opcode = 0x066
	OpcodeCMSG_CONTROLLER_EJECT_PASSENGER                       Opcode = 0x4A9
	OpcodeCMSG_COOLDOWN_CHEAT                                   Opcode = 0x028
	OpcodeCMSG_CORPSE_MAP_POSITION_QUERY                        Opcode = 0x4B6
	OpcodeCMSG_CREATEGAMEOBJECT                                 Opcode = 0x014
	OpcodeCMSG_CREATEITEM                                       Opcode = 0x013
	OpcodeCMSG_CREATEMONSTER                                    Opcode = 0x011
	OpcodeCMSG_CREATURE_QUERY                                   Opcode = 0x060
	OpcodeCMSG_DANCE_QUERY                                      Opcode = 0x451
	OpcodeCMSG_DBLOOKUP                                         Opcode = 0x002
	OpcodeCMSG_DEBUG_ACTIONS_START                              Opcode = 0x315
	OpcodeCMSG_DEBUG_ACTIONS_STOP                               Opcode = 0x316
	OpcodeCMSG_DEBUG_AISTATE                                    Opcode = 0x02E
	OpcodeCMSG_DEBUG_CHANGECELLZONE                             Opcode = 0x00C
	OpcodeCMSG_DEBUG_LIST_TARGETS                               Opcode = 0x3D8
	OpcodeCMSG_DEBUG_PASSIVE_AURA                               Opcode = 0x140
	OpcodeCMSG_DEBUG_SERVER_GEO                                 Opcode = 0x4FB
	OpcodeCMSG_DECHARGE                                         Opcode = 0x204
	OpcodeCMSG_DECLINE_CHANNEL_INVITE                           Opcode = 0x410
	OpcodeCMSG_DELETEEQUIPMENT_SET                              Opcode = 0x13E
	OpcodeCMSG_DELETE_DANCE                                     Opcode = 0x454
	OpcodeCMSG_DEL_FRIEND                                       Opcode = 0x06A
	OpcodeCMSG_DEL_IGNORE                                       Opcode = 0x06D
	OpcodeCMSG_DEL_PVP_MEDAL_CHEAT                              Opcode = 0x28A
	OpcodeCMSG_DEL_VOICE_IGNORE                                 Opcode = 0x3DC
	OpcodeCMSG_DESTROYITEM                                      Opcode = 0x111
	OpcodeCMSG_DESTROYMONSTER                                   Opcode = 0x012
	OpcodeCMSG_DESTROY_ITEMS                                    Opcode = 0x0B2
	OpcodeCMSG_DISABLE_PVP_CHEAT                                Opcode = 0x030
	OpcodeCMSG_DISMISS_CONTROLLED_VEHICLE                       Opcode = 0x46D
	OpcodeCMSG_DISMISS_CRITTER                                  Opcode = 0x48D
	OpcodeCMSG_DROP_NEW_CONNECTION                              Opcode = 0x513
	OpcodeCMSG_DUEL_ACCEPTED                                    Opcode = 0x16C
	OpcodeCMSG_DUEL_CANCELLED                                   Opcode = 0x16D
	OpcodeCMSG_DUMP_OBJECTS                                     Opcode = 0x48B
	OpcodeCMSG_EMOTE                                            Opcode = 0x102
	OpcodeCMSG_ENABLETAXI                                       Opcode = 0x493
	OpcodeCMSG_ENABLE_DAMAGE_LOG                                Opcode = 0x27D
	OpcodeCMSG_END_BATTLEFIELD_CHEAT                            Opcode = 0x4CC
	OpcodeCMSG_EQUIPMENT_SET_SAVE                               Opcode = 0x4BD
	OpcodeCMSG_EQUIPMENT_SET_USE                                Opcode = 0x4D5
	OpcodeCMSG_EXPIRE_RAID_INSTANCE                             Opcode = 0x415
	OpcodeCMSG_FAR_SIGHT                                        Opcode = 0x27A
	OpcodeCMSG_FLAG_QUEST                                       Opcode = 0x02A
	OpcodeCMSG_FLAG_QUEST_FINISH                                Opcode = 0x02B
	OpcodeCMSG_FLOOD_GRACE_CHEAT                                Opcode = 0x497
	OpcodeCMSG_FORCEACTION                                      Opcode = 0x018
	OpcodeCMSG_FORCEACTIONONOTHER                               Opcode = 0x019
	OpcodeCMSG_FORCEACTIONSHOW                                  Opcode = 0x01A
	OpcodeCMSG_FORCE_ANIM                                       Opcode = 0x4D7
	OpcodeCMSG_FORCE_FLIGHT_BACK_SPEED_CHANGE_ACK               Opcode = 0x384
	OpcodeCMSG_FORCE_FLIGHT_SPEED_CHANGE_ACK                    Opcode = 0x382
	OpcodeCMSG_FORCE_MOVE_ROOT_ACK                              Opcode = 0x0E9
	OpcodeCMSG_FORCE_MOVE_UNROOT_ACK                            Opcode = 0x0EB
	OpcodeCMSG_FORCE_PITCH_RATE_CHANGE_ACK                      Opcode = 0x45D
	OpcodeCMSG_FORCE_RUN_BACK_SPEED_CHANGE_ACK                  Opcode = 0x0E5
	OpcodeCMSG_FORCE_RUN_SPEED_CHANGE_ACK                       Opcode = 0x0E3
	OpcodeCMSG_FORCE_SAY_CHEAT                                  Opcode = 0x47E
	OpcodeCMSG_FORCE_SWIM_BACK_SPEED_CHANGE_ACK                 Opcode = 0x2DD
	OpcodeCMSG_FORCE_SWIM_SPEED_CHANGE_ACK                      Opcode = 0x0E7
	OpcodeCMSG_FORCE_TURN_RATE_CHANGE_ACK                       Opcode = 0x2DF
	OpcodeCMSG_FORCE_WALK_SPEED_CHANGE_ACK                      Opcode = 0x2DB
	OpcodeCMSG_GAMEOBJECT_QUERY                                 Opcode = 0x05E
	OpcodeCMSG_GAMEOBJ_REPORT_USE                               Opcode = 0x481
	OpcodeCMSG_GAMEOBJ_USE                                      Opcode = 0x0B1
	OpcodeCMSG_GAMESPEED_SET                                    Opcode = 0x046
	OpcodeCMSG_GAMETIME_SET                                     Opcode = 0x044
	OpcodeCMSG_GETDEATHBINDZONE                                 Opcode = 0x156
	OpcodeCMSG_GET_CHANNEL_MEMBER_COUNT                         Opcode = 0x3D4
	OpcodeCMSG_GET_MAIL_LIST                                    Opcode = 0x23A
	OpcodeCMSG_GET_MIRRORIMAGE_DATA                             Opcode = 0x401
	OpcodeCMSG_GHOST                                            Opcode = 0x1E5
	OpcodeCMSG_GMRESPONSE_CREATE_TICKET                         Opcode = 0x4F3
	OpcodeCMSG_GMRESPONSE_RESOLVE                               Opcode = 0x4F0
	OpcodeCMSG_GMSURVEY_SUBMIT                                  Opcode = 0x32A
	OpcodeCMSG_GMTICKETSYSTEM_TOGGLE                            Opcode = 0x29A
	OpcodeCMSG_GMTICKET_CREATE                                  Opcode = 0x205
	OpcodeCMSG_GMTICKET_DELETETICKET                            Opcode = 0x217
	OpcodeCMSG_GMTICKET_GETTICKET                               Opcode = 0x211
	OpcodeCMSG_GMTICKET_SYSTEMSTATUS                            Opcode = 0x21A
	OpcodeCMSG_GMTICKET_UPDATETEXT                              Opcode = 0x207
	OpcodeCMSG_GM_CHARACTER_RESTORE                             Opcode = 0x3FA
	OpcodeCMSG_GM_CHARACTER_SAVE                                Opcode = 0x3FB
	OpcodeCMSG_GM_CREATE_ITEM_TARGET                            Opcode = 0x210
	OpcodeCMSG_GM_DESTROY_ONLINE_CORPSE                         Opcode = 0x311
	OpcodeCMSG_GM_FREEZE                                        Opcode = 0x22D
	OpcodeCMSG_GM_GRANT_ACHIEVEMENT                             Opcode = 0x4C4
	OpcodeCMSG_GM_INVIS                                         Opcode = 0x1E6
	OpcodeCMSG_GM_MOVECORPSE                                    Opcode = 0x22C
	OpcodeCMSG_GM_NUKE                                          Opcode = 0x1FA
	OpcodeCMSG_GM_NUKE_ACCOUNT                                  Opcode = 0x30F
	OpcodeCMSG_GM_NUKE_CHARACTER                                Opcode = 0x507
	OpcodeCMSG_GM_REMOVE_ACHIEVEMENT                            Opcode = 0x4C5
	OpcodeCMSG_GM_REPORT_LAG                                    Opcode = 0x502
	OpcodeCMSG_GM_REQUEST_PLAYER_INFO                           Opcode = 0x22F
	OpcodeCMSG_GM_RESURRECT                                     Opcode = 0x22A
	OpcodeCMSG_GM_REVEALTO                                      Opcode = 0x229
	OpcodeCMSG_GM_SET_CRITERIA_FOR_PLAYER                       Opcode = 0x4C6
	OpcodeCMSG_GM_SET_SECURITY_GROUP                            Opcode = 0x1F9
	OpcodeCMSG_GM_SHOW_COMPLAINTS                               Opcode = 0x3CA
	OpcodeCMSG_GM_SILENCE                                       Opcode = 0x228
	OpcodeCMSG_GM_SUMMONMOB                                     Opcode = 0x22B
	OpcodeCMSG_GM_TEACH                                         Opcode = 0x20F
	OpcodeCMSG_GM_UBERINVIS                                     Opcode = 0x22E
	OpcodeCMSG_GM_UNSQUELCH                                     Opcode = 0x3CB
	OpcodeCMSG_GM_UNTEACH                                       Opcode = 0x2E5
	OpcodeCMSG_GM_UPDATE_TICKET_STATUS                          Opcode = 0x327
	OpcodeCMSG_GM_VISION                                        Opcode = 0x226
	OpcodeCMSG_GM_WHISPER                                       Opcode = 0x3B2
	OpcodeCMSG_GODMODE                                          Opcode = 0x022
	OpcodeCMSG_GOSSIP_HELLO                                     Opcode = 0x17B
	OpcodeCMSG_GOSSIP_SELECT_OPTION                             Opcode = 0x17C
	OpcodeCMSG_GRANT_LEVEL                                      Opcode = 0x40D
	OpcodeCMSG_GROUP_ACCEPT                                     Opcode = 0x072
	OpcodeCMSG_GROUP_ASSISTANT_LEADER                           Opcode = 0x28F
	OpcodeCMSG_GROUP_CANCEL                                     Opcode = 0x070
	OpcodeCMSG_GROUP_CHANGE_SUB_GROUP                           Opcode = 0x27E
	OpcodeCMSG_GROUP_DECLINE                                    Opcode = 0x073
	OpcodeCMSG_GROUP_DISBAND                                    Opcode = 0x07B
	OpcodeCMSG_GROUP_INVITE                                     Opcode = 0x06E
	OpcodeCMSG_GROUP_RAID_CONVERT                               Opcode = 0x28E
	OpcodeCMSG_GROUP_SET_LEADER                                 Opcode = 0x078
	OpcodeCMSG_GROUP_SWAP_SUB_GROUP                             Opcode = 0x280
	OpcodeCMSG_GROUP_UNINVITE                                   Opcode = 0x075
	OpcodeCMSG_GROUP_UNINVITE_GUID                              Opcode = 0x076
	OpcodeCMSG_GUILD_ACCEPT                                     Opcode = 0x084
	OpcodeCMSG_GUILD_ADD_RANK                                   Opcode = 0x232
	OpcodeCMSG_GUILD_BANKER_ACTIVATE                            Opcode = 0x3E6
	OpcodeCMSG_GUILD_BANK_BUY_TAB                               Opcode = 0x3EA
	OpcodeCMSG_GUILD_BANK_DEPOSIT_MONEY                         Opcode = 0x3EC
	OpcodeCMSG_GUILD_BANK_QUERY_TAB                             Opcode = 0x3E7
	OpcodeCMSG_GUILD_BANK_SWAP_ITEMS                            Opcode = 0x3E9
	OpcodeCMSG_GUILD_BANK_UPDATE_TAB                            Opcode = 0x3EB
	OpcodeCMSG_GUILD_BANK_WITHDRAW_MONEY                        Opcode = 0x3ED
	OpcodeCMSG_GUILD_CREATE                                     Opcode = 0x081
	OpcodeCMSG_GUILD_DECLINE                                    Opcode = 0x085
	OpcodeCMSG_GUILD_DEL_RANK                                   Opcode = 0x233
	OpcodeCMSG_GUILD_DEMOTE                                     Opcode = 0x08C
	OpcodeCMSG_GUILD_DISBAND                                    Opcode = 0x08F
	OpcodeCMSG_GUILD_INFO                                       Opcode = 0x087
	OpcodeCMSG_GUILD_INFO_TEXT                                  Opcode = 0x2FC
	OpcodeCMSG_GUILD_INVITE                                     Opcode = 0x082
	OpcodeCMSG_GUILD_LEADER                                     Opcode = 0x090
	OpcodeCMSG_GUILD_LEAVE                                      Opcode = 0x08D
	OpcodeCMSG_GUILD_MOTD                                       Opcode = 0x091
	OpcodeCMSG_GUILD_PROMOTE                                    Opcode = 0x08B
	OpcodeCMSG_GUILD_QUERY                                      Opcode = 0x054
	OpcodeCMSG_GUILD_RANK                                       Opcode = 0x231
	OpcodeCMSG_GUILD_REMOVE                                     Opcode = 0x08E
	OpcodeCMSG_GUILD_ROSTER                                     Opcode = 0x089
	OpcodeCMSG_GUILD_SET_OFFICER_NOTE                           Opcode = 0x235
	OpcodeCMSG_GUILD_SET_PUBLIC_NOTE                            Opcode = 0x234
	OpcodeCMSG_HEARTH_AND_RESURRECT                             Opcode = 0x49C
	OpcodeCMSG_IGNORE_DIMINISHING_RETURNS_CHEAT                 Opcode = 0x405
	OpcodeCMSG_IGNORE_KNOCKBACK_CHEAT                           Opcode = 0x32C
	OpcodeCMSG_IGNORE_REQUIREMENTS_CHEAT                        Opcode = 0x3A8
	OpcodeCMSG_IGNORE_TRADE                                     Opcode = 0x119
	OpcodeCMSG_INITIATE_TRADE                                   Opcode = 0x116
	OpcodeCMSG_INSPECT                                          Opcode = 0x114
	OpcodeCMSG_INSTANCE_LOCK_RESPONSE                           Opcode = 0x13F
	OpcodeCMSG_ITEM_NAME_QUERY                                  Opcode = 0x2C4
	OpcodeCMSG_ITEM_QUERY_MULTIPLE                              Opcode = 0x057
	OpcodeCMSG_ITEM_QUERY_SINGLE                                Opcode = 0x056
	OpcodeCMSG_ITEM_REFUND                                      Opcode = 0x4B4
	OpcodeCMSG_ITEM_REFUND_INFO                                 Opcode = 0x4B3
	OpcodeCMSG_ITEM_TEXT_QUERY                                  Opcode = 0x243
	OpcodeCMSG_JOIN_CHANNEL                                     Opcode = 0x097
	OpcodeCMSG_KEEP_ALIVE                                       Opcode = 0x407
	OpcodeCMSG_LEARN_DANCE_MOVE                                 Opcode = 0x456
	OpcodeCMSG_LEARN_PREVIEW_TALENTS                            Opcode = 0x4C1
	OpcodeCMSG_LEARN_PREVIEW_TALENTS_PET                        Opcode = 0x4C2
	OpcodeCMSG_LEARN_SPELL                                      Opcode = 0x010
	OpcodeCMSG_LEARN_TALENT                                     Opcode = 0x251
	OpcodeCMSG_LEAVE_BATTLEFIELD                                Opcode = 0x2E1
	OpcodeCMSG_LEAVE_CHANNEL                                    Opcode = 0x098
	OpcodeCMSG_LEVEL_CHEAT                                      Opcode = 0x025
	OpcodeCMSG_LFD_PARTY_LOCK_INFO_REQUEST                      Opcode = 0x371
	OpcodeCMSG_LFD_PLAYER_LOCK_INFO_REQUEST                     Opcode = 0x36E
	OpcodeCMSG_LFG_GET_STATUS                                   Opcode = 0x296
	OpcodeCMSG_LFG_JOIN                                         Opcode = 0x35C
	OpcodeCMSG_LFG_LEAVE                                        Opcode = 0x35D
	OpcodeCMSG_LFG_PROPOSAL_RESULT                              Opcode = 0x362
	OpcodeCMSG_LFG_SET_BOOT_VOTE                                Opcode = 0x36C
	OpcodeCMSG_LFG_SET_NEEDS                                    Opcode = 0x36B
	OpcodeCMSG_LFG_SET_ROLES                                    Opcode = 0x36A
	OpcodeCMSG_LFG_TELEPORT                                     Opcode = 0x370
	OpcodeCMSG_LIST_INVENTORY                                   Opcode = 0x19E
	OpcodeCMSG_LOAD_DANCES                                      Opcode = 0x44D
	OpcodeCMSG_LOGOUT_CANCEL                                    Opcode = 0x04E
	OpcodeCMSG_LOGOUT_REQUEST                                   Opcode = 0x04B
	OpcodeCMSG_LOOT                                             Opcode = 0x15D
	OpcodeCMSG_LOOT_MASTER_GIVE                                 Opcode = 0x2A3
	OpcodeCMSG_LOOT_METHOD                                      Opcode = 0x07A
	OpcodeCMSG_LOOT_MONEY                                       Opcode = 0x15E
	OpcodeCMSG_LOOT_RELEASE                                     Opcode = 0x15F
	OpcodeCMSG_LOOT_ROLL                                        Opcode = 0x2A0
	OpcodeCMSG_LOTTERY_QUERY_OBSOLETE                           Opcode = 0x334
	OpcodeCMSG_LUA_USAGE                                        Opcode = 0x323
	OpcodeCMSG_MAELSTROM_GM_SENT_MAIL                           Opcode = 0x395
	OpcodeCMSG_MAELSTROM_INVALIDATE_CACHE                       Opcode = 0x387
	OpcodeCMSG_MAELSTROM_RENAME_GUILD                           Opcode = 0x400
	OpcodeCMSG_MAIL_CREATE_TEXT_ITEM                            Opcode = 0x24A
	OpcodeCMSG_MAIL_DELETE                                      Opcode = 0x249
	OpcodeCMSG_MAIL_MARK_AS_READ                                Opcode = 0x247
	OpcodeCMSG_MAIL_RETURN_TO_SENDER                            Opcode = 0x248
	OpcodeCMSG_MAIL_TAKE_ITEM                                   Opcode = 0x246
	OpcodeCMSG_MAIL_TAKE_MONEY                                  Opcode = 0x245
	OpcodeCMSG_MAKEMONSTERATTACKGUID                            Opcode = 0x016
	OpcodeCMSG_MESSAGECHAT                                      Opcode = 0x095
	OpcodeCMSG_MINIGAME_MOVE                                    Opcode = 0x2F8
	OpcodeCMSG_MOUNTSPECIAL_ANIM                                Opcode = 0x171
	OpcodeCMSG_MOVE_CHARACTER_CHEAT                             Opcode = 0x00D
	OpcodeCMSG_MOVE_CHARM_PORT_CHEAT                            Opcode = 0x0E0
	OpcodeCMSG_MOVE_CHNG_TRANSPORT                              Opcode = 0x38D
	OpcodeCMSG_MOVE_FALL_RESET                                  Opcode = 0x2CA
	OpcodeCMSG_MOVE_FEATHER_FALL_ACK                            Opcode = 0x2CF
	OpcodeCMSG_MOVE_GRAVITY_DISABLE_ACK                         Opcode = 0x4CF
	OpcodeCMSG_MOVE_GRAVITY_ENABLE_ACK                          Opcode = 0x4D1
	OpcodeCMSG_MOVE_HOVER_ACK                                   Opcode = 0x0F6
	OpcodeCMSG_MOVE_KNOCK_BACK_ACK                              Opcode = 0x0F0
	OpcodeCMSG_MOVE_NOT_ACTIVE_MOVER                            Opcode = 0x2D1
	OpcodeCMSG_MOVE_SET_CAN_FLY_ACK                             Opcode = 0x345
	OpcodeCMSG_MOVE_SET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY_ACK Opcode = 0x340
	OpcodeCMSG_MOVE_SET_COLLISION_HGT_ACK                       Opcode = 0x517
	OpcodeCMSG_MOVE_SET_FLY                                     Opcode = 0x346
	OpcodeCMSG_MOVE_SET_RAW_POSITION                            Opcode = 0x0E1
	OpcodeCMSG_MOVE_SET_RUN_SPEED                               Opcode = 0x3AB
	OpcodeCMSG_MOVE_SPLINE_DONE                                 Opcode = 0x2C9
	OpcodeCMSG_MOVE_START_SWIM_CHEAT                            Opcode = 0x2D8
	OpcodeCMSG_MOVE_STOP_SWIM_CHEAT                             Opcode = 0x2D9
	OpcodeCMSG_MOVE_TIME_SKIPPED                                Opcode = 0x2CE
	OpcodeCMSG_MOVE_WATER_WALK_ACK                              Opcode = 0x2D0
	OpcodeCMSG_NAME_QUERY                                       Opcode = 0x050
	OpcodeCMSG_NEW_SPELL_SLOT                                   Opcode = 0x12D
	OpcodeCMSG_NEXT_CINEMATIC_CAMERA                            Opcode = 0x0FB
	OpcodeCMSG_NO_SPELL_VARIANCE                                Opcode = 0x416
	OpcodeCMSG_NPC_TEXT_QUERY                                   Opcode = 0x17F
	OpcodeCMSG_OFFER_PETITION                                   Opcode = 0x1C3
	OpcodeCMSG_OPENING_CINEMATIC                                Opcode = 0x0F9
	OpcodeCMSG_OPEN_ITEM                                        Opcode = 0x0AC
	OpcodeCMSG_OPT_OUT_OF_LOOT                                  Opcode = 0x409
	OpcodeCMSG_PAGE_TEXT_QUERY                                  Opcode = 0x05A
	OpcodeCMSG_PARTY_SILENCE                                    Opcode = 0x3DD
	OpcodeCMSG_PARTY_UNSILENCE                                  Opcode = 0x3DE
	OpcodeCMSG_PERFORM_ACTION_SET                               Opcode = 0x14C
	OpcodeCMSG_PETGODMODE                                       Opcode = 0x01C
	OpcodeCMSG_PETITION_BUY                                     Opcode = 0x1BD
	OpcodeCMSG_PETITION_QUERY                                   Opcode = 0x1C6
	OpcodeCMSG_PETITION_SHOWLIST                                Opcode = 0x1BB
	OpcodeCMSG_PETITION_SHOW_SIGNATURES                         Opcode = 0x1BE
	OpcodeCMSG_PETITION_SIGN                                    Opcode = 0x1C0
	OpcodeCMSG_PET_ABANDON                                      Opcode = 0x176
	OpcodeCMSG_PET_ACTION                                       Opcode = 0x175
	OpcodeCMSG_PET_CANCEL_AURA                                  Opcode = 0x26B
	OpcodeCMSG_PET_CAST_SPELL                                   Opcode = 0x1F0
	OpcodeCMSG_PET_LEARN_TALENT                                 Opcode = 0x47A
	OpcodeCMSG_PET_LEVEL_CHEAT                                  Opcode = 0x026
	OpcodeCMSG_PET_NAME_QUERY                                   Opcode = 0x052
	OpcodeCMSG_PET_RENAME                                       Opcode = 0x177
	OpcodeCMSG_PET_SET_ACTION                                   Opcode = 0x174
	OpcodeCMSG_PET_SPELL_AUTOCAST                               Opcode = 0x2F3
	OpcodeCMSG_PET_STOP_ATTACK                                  Opcode = 0x2EA
	OpcodeCMSG_PET_UNLEARN                                      Opcode = 0x2F0
	OpcodeCMSG_PET_UNLEARN_TALENTS                              Opcode = 0x47B
	OpcodeCMSG_PING                                             Opcode = 0x1DC
	OpcodeCMSG_PLAYED_TIME                                      Opcode = 0x1CC
	OpcodeCMSG_PLAYER_AI_CHEAT                                  Opcode = 0x26C
	OpcodeCMSG_PLAYER_LOGIN                                     Opcode = 0x03D
	OpcodeCMSG_PLAYER_LOGOUT                                    Opcode = 0x04A
	OpcodeCMSG_PLAYER_VEHICLE_ENTER                             Opcode = 0x4A8
	OpcodeCMSG_PLAY_DANCE                                       Opcode = 0x44B
	OpcodeCMSG_PROFILEDATA_REQUEST                              Opcode = 0x4C9
	OpcodeCMSG_PUSHQUESTTOPARTY                                 Opcode = 0x19D
	OpcodeCMSG_PVP_QUEUE_STATS_REQUEST                          Opcode = 0x4DB
	OpcodeCMSG_QUERY_INSPECT_ACHIEVEMENTS                       Opcode = 0x46B
	OpcodeCMSG_QUERY_OBJECT_POSITION                            Opcode = 0x004
	OpcodeCMSG_QUERY_OBJECT_ROTATION                            Opcode = 0x006
	OpcodeCMSG_QUERY_QUESTS_COMPLETED                           Opcode = 0x500
	OpcodeCMSG_QUERY_SERVER_BUCK_DATA                           Opcode = 0x41B
	OpcodeCMSG_QUERY_TIME                                       Opcode = 0x1CE
	OpcodeCMSG_QUERY_VEHICLE_STATUS                             Opcode = 0x4A5
	OpcodeCMSG_QUESTGIVER_ACCEPT_QUEST                          Opcode = 0x189
	OpcodeCMSG_QUESTGIVER_CANCEL                                Opcode = 0x190
	OpcodeCMSG_QUESTGIVER_CHOOSE_REWARD                         Opcode = 0x18E
	OpcodeCMSG_QUESTGIVER_COMPLETE_QUEST                        Opcode = 0x18A
	OpcodeCMSG_QUESTGIVER_HELLO                                 Opcode = 0x184
	OpcodeCMSG_QUESTGIVER_QUERY_QUEST                           Opcode = 0x186
	OpcodeCMSG_QUESTGIVER_QUEST_AUTOLAUNCH                      Opcode = 0x187
	OpcodeCMSG_QUESTGIVER_REQUEST_REWARD                        Opcode = 0x18C
	OpcodeCMSG_QUESTGIVER_STATUS_MULTIPLE_QUERY                 Opcode = 0x417
	OpcodeCMSG_QUESTGIVER_STATUS_QUERY                          Opcode = 0x182
	OpcodeCMSG_QUESTLOG_REMOVE_QUEST                            Opcode = 0x194
	OpcodeCMSG_QUESTLOG_SWAP_QUEST                              Opcode = 0x193
	OpcodeCMSG_QUEST_CONFIRM_ACCEPT                             Opcode = 0x19B
	OpcodeCMSG_QUEST_POI_QUERY                                  Opcode = 0x1E3
	OpcodeCMSG_QUEST_QUERY                                      Opcode = 0x05C
	OpcodeCMSG_READY_FOR_ACCOUNT_DATA_TIMES                     Opcode = 0x4FF
	OpcodeCMSG_READ_ITEM                                        Opcode = 0x0AD
	OpcodeCMSG_REALM_SPLIT                                      Opcode = 0x38C
	OpcodeCMSG_RECHARGE                                         Opcode = 0x00F
	OpcodeCMSG_RECLAIM_CORPSE                                   Opcode = 0x1D2
	OpcodeCMSG_REDIRECTION_AUTH_PROOF                           Opcode = 0x512
	OpcodeCMSG_REDIRECTION_FAILED                               Opcode = 0x50E
	OpcodeCMSG_REFER_A_FRIEND                                   Opcode = 0x40E
	OpcodeCMSG_REMOVE_GLYPH                                     Opcode = 0x48A
	OpcodeCMSG_REPAIR_ITEM                                      Opcode = 0x2A8
	OpcodeCMSG_REPOP_REQUEST                                    Opcode = 0x15A
	OpcodeCMSG_REPORT_PVP_AFK                                   Opcode = 0x3E4
	OpcodeCMSG_REQUEST_ACCOUNT_DATA                             Opcode = 0x20A
	OpcodeCMSG_REQUEST_PARTY_MEMBER_STATS                       Opcode = 0x27F
	OpcodeCMSG_REQUEST_PET_INFO                                 Opcode = 0x279
	OpcodeCMSG_REQUEST_RAID_INFO                                Opcode = 0x2CD
	OpcodeCMSG_REQUEST_VEHICLE_EXIT                             Opcode = 0x476
	OpcodeCMSG_REQUEST_VEHICLE_NEXT_SEAT                        Opcode = 0x478
	OpcodeCMSG_REQUEST_VEHICLE_PREV_SEAT                        Opcode = 0x477
	OpcodeCMSG_REQUEST_VEHICLE_SWITCH_SEAT                      Opcode = 0x479
	OpcodeCMSG_RESET_FACTION_CHEAT                              Opcode = 0x281
	OpcodeCMSG_RESET_INSTANCES                                  Opcode = 0x31D
	OpcodeCMSG_RESURRECT_RESPONSE                               Opcode = 0x15C
	OpcodeCMSG_RUN_SCRIPT                                       Opcode = 0x2B5
	OpcodeCMSG_SAVE_DANCE                                       Opcode = 0x449
	OpcodeCMSG_SAVE_PLAYER                                      Opcode = 0x153
	OpcodeCMSG_SEARCH_LFG_JOIN                                  Opcode = 0x35E
	OpcodeCMSG_SEARCH_LFG_LEAVE                                 Opcode = 0x35F
	OpcodeCMSG_SELF_RES                                         Opcode = 0x2B3
	OpcodeCMSG_SELL_ITEM                                        Opcode = 0x1A0
	OpcodeCMSG_SEND_COMBAT_TRIGGER                              Opcode = 0x394
	OpcodeCMSG_SEND_EVENT                                       Opcode = 0x02D
	OpcodeCMSG_SEND_GENERAL_TRIGGER                             Opcode = 0x393
	OpcodeCMSG_SEND_LOCAL_EVENT                                 Opcode = 0x392
	OpcodeCMSG_SEND_MAIL                                        Opcode = 0x238
	OpcodeCMSG_SERVERINFO                                       Opcode = 0x4F4
	OpcodeCMSG_SERVERTIME                                       Opcode = 0x048
	OpcodeCMSG_SERVER_BROADCAST                                 Opcode = 0x2B2
	OpcodeCMSG_SERVER_COMMAND                                   Opcode = 0x227
	OpcodeCMSG_SERVER_INFO_QUERY                                Opcode = 0x4A0
	OpcodeCMSG_SETDEATHBINDPOINT                                Opcode = 0x154
	OpcodeCMSG_SET_ACTIONBAR_TOGGLES                            Opcode = 0x2BF
	OpcodeCMSG_SET_ACTION_BUTTON                                Opcode = 0x128
	OpcodeCMSG_SET_ACTIVE_MOVER                                 Opcode = 0x26A
	OpcodeCMSG_SET_ACTIVE_TALENT_GROUP_OBSOLETE                 Opcode = 0x4C3
	OpcodeCMSG_SET_ACTIVE_VOICE_CHANNEL                         Opcode = 0x3D3
	OpcodeCMSG_SET_ALLOW_LOW_LEVEL_RAID1                        Opcode = 0x508
	OpcodeCMSG_SET_ALLOW_LOW_LEVEL_RAID2                        Opcode = 0x509
	OpcodeCMSG_SET_AMMO                                         Opcode = 0x268
	OpcodeCMSG_SET_ARENA_MEMBER_SEASON_GAMES                    Opcode = 0x4B1
	OpcodeCMSG_SET_ARENA_MEMBER_WEEKLY_GAMES                    Opcode = 0x4B0
	OpcodeCMSG_SET_ARENA_TEAM_RATING_BY_INDEX                   Opcode = 0x4AD
	OpcodeCMSG_SET_ARENA_TEAM_SEASON_GAMES                      Opcode = 0x4AF
	OpcodeCMSG_SET_ARENA_TEAM_WEEKLY_GAMES                      Opcode = 0x4AE
	OpcodeCMSG_SET_BREATH                                       Opcode = 0x4A4
	OpcodeCMSG_SET_CHANNEL_WATCH                                Opcode = 0x3EF
	OpcodeCMSG_SET_CHARACTER_MODEL                              Opcode = 0x50C
	OpcodeCMSG_SET_CONTACT_NOTES                                Opcode = 0x06B
	OpcodeCMSG_SET_CRITERIA_CHEAT                               Opcode = 0x470
	OpcodeCMSG_SET_DURABILITY_CHEAT                             Opcode = 0x287
	OpcodeCMSG_SET_EXPLORATION                                  Opcode = 0x2BE
	OpcodeCMSG_SET_EXPLORATION_ALL                              Opcode = 0x31B
	OpcodeCMSG_SET_FACTION_ATWAR                                Opcode = 0x125
	OpcodeCMSG_SET_FACTION_CHEAT                                Opcode = 0x126
	OpcodeCMSG_SET_FACTION_INACTIVE                             Opcode = 0x317
	OpcodeCMSG_SET_GLYPH                                        Opcode = 0x467
	OpcodeCMSG_SET_GLYPH_SLOT                                   Opcode = 0x466
	OpcodeCMSG_SET_GRANTABLE_LEVELS                             Opcode = 0x40C
	OpcodeCMSG_SET_GUILD_BANK_TEXT                              Opcode = 0x40B
	OpcodeCMSG_SET_LFG_COMMENT                                  Opcode = 0x366
	OpcodeCMSG_SET_PAID_SERVICE_CHEAT                           Opcode = 0x4DD
	OpcodeCMSG_SET_PLAYER_DECLINED_NAMES                        Opcode = 0x419
	OpcodeCMSG_SET_PVP_RANK_CHEAT                               Opcode = 0x288
	OpcodeCMSG_SET_PVP_TITLE                                    Opcode = 0x28B
	OpcodeCMSG_SET_RUNE_COOLDOWN                                Opcode = 0x459
	OpcodeCMSG_SET_RUNE_COUNT                                   Opcode = 0x458
	OpcodeCMSG_SET_SAVED_INSTANCE_EXTEND                        Opcode = 0x292
	OpcodeCMSG_SET_SELECTION                                    Opcode = 0x13D
	OpcodeCMSG_SET_SHEATHED                                     Opcode = 0x1E0
	OpcodeCMSG_SET_SKILL_CHEAT                                  Opcode = 0x1D8
	OpcodeCMSG_SET_STAT_CHEAT                                   Opcode = 0x21D
	OpcodeCMSG_SET_TAXI_BENCHMARK_MODE                          Opcode = 0x389
	OpcodeCMSG_SET_TITLE                                        Opcode = 0x374
	OpcodeCMSG_SET_TITLE_SUFFIX                                 Opcode = 0x3F7
	OpcodeCMSG_SET_TRADE_GOLD                                   Opcode = 0x11F
	OpcodeCMSG_SET_TRADE_ITEM                                   Opcode = 0x11D
	OpcodeCMSG_SET_VEHICLE_REC_ID_ACK                           Opcode = 0x240
	OpcodeCMSG_SET_WATCHED_FACTION                              Opcode = 0x318
	OpcodeCMSG_SET_WORLDSTATE                                   Opcode = 0x027
	OpcodeCMSG_SHOWING_CLOAK                                    Opcode = 0x2BA
	OpcodeCMSG_SHOWING_HELM                                     Opcode = 0x2B9
	OpcodeCMSG_SKILL_BUY_RANK                                   Opcode = 0x220
	OpcodeCMSG_SKILL_BUY_STEP                                   Opcode = 0x21F
	OpcodeCMSG_SOCKET_GEMS                                      Opcode = 0x347
	OpcodeCMSG_SPELLCLICK                                       Opcode = 0x3F8
	OpcodeCMSG_SPIRIT_HEALER_ACTIVATE                           Opcode = 0x21C
	OpcodeCMSG_SPLIT_ITEM                                       Opcode = 0x10E
	OpcodeCMSG_STABLE_PET                                       Opcode = 0x270
	OpcodeCMSG_STABLE_REVIVE_PET                                Opcode = 0x274
	OpcodeCMSG_STABLE_SWAP_PET                                  Opcode = 0x275
	OpcodeCMSG_STANDSTATECHANGE                                 Opcode = 0x101
	OpcodeCMSG_START_BATTLEFIELD_CHEAT                          Opcode = 0x4CB
	OpcodeCMSG_START_QUEST                                      Opcode = 0x489
	OpcodeCMSG_STOP_DANCE                                       Opcode = 0x44E
	OpcodeCMSG_STORE_LOOT_IN_SLOT                               Opcode = 0x109
	OpcodeCMSG_SUMMON_RESPONSE                                  Opcode = 0x2AC
	OpcodeCMSG_SUSPEND_COMMS_ACK                                Opcode = 0x510
	OpcodeCMSG_SWAP_INV_ITEM                                    Opcode = 0x10D
	OpcodeCMSG_SWAP_ITEM                                        Opcode = 0x10C
	OpcodeCMSG_SYNC_DANCE                                       Opcode = 0x450
	OpcodeCMSG_TARGET_CAST                                      Opcode = 0x3D0
	OpcodeCMSG_TARGET_SCRIPT_CAST                               Opcode = 0x3D1
	OpcodeCMSG_TAXICLEARALLNODES                                Opcode = 0x1A6
	OpcodeCMSG_TAXICLEARNODE                                    Opcode = 0x241
	OpcodeCMSG_TAXIENABLEALLNODES                               Opcode = 0x1A7
	OpcodeCMSG_TAXIENABLENODE                                   Opcode = 0x242
	OpcodeCMSG_TAXINODE_STATUS_QUERY                            Opcode = 0x1AA
	OpcodeCMSG_TAXIQUERYAVAILABLENODES                          Opcode = 0x1AC
	OpcodeCMSG_TAXISHOWNODES                                    Opcode = 0x1A8
	OpcodeCMSG_TELEPORT_TO_UNIT                                 Opcode = 0x009
	OpcodeCMSG_TEST_DROP_RATE                                   Opcode = 0x294
	OpcodeCMSG_TEXT_EMOTE                                       Opcode = 0x104
	OpcodeCMSG_TIME_SYNC_RESP                                   Opcode = 0x391
	OpcodeCMSG_TOGGLE_PVP                                       Opcode = 0x253
	OpcodeCMSG_TOGGLE_XP_GAIN                                   Opcode = 0x4EC
	OpcodeCMSG_TOTEM_DESTROYED                                  Opcode = 0x414
	OpcodeCMSG_TRAINER_BUY_SPELL                                Opcode = 0x1B2
	OpcodeCMSG_TRAINER_LIST                                     Opcode = 0x1B0
	OpcodeCMSG_TRIGGER_CINEMATIC_CHEAT                          Opcode = 0x0F8
	OpcodeCMSG_TURN_IN_PETITION                                 Opcode = 0x1C4
	OpcodeCMSG_TUTORIAL_CLEAR                                   Opcode = 0x0FF
	OpcodeCMSG_TUTORIAL_FLAG                                    Opcode = 0x0FE
	OpcodeCMSG_TUTORIAL_RESET                                   Opcode = 0x100
	OpcodeCMSG_UNACCEPT_TRADE                                   Opcode = 0x11B
	OpcodeCMSG_UNCLAIM_LICENSE                                  Opcode = 0x110
	OpcodeCMSG_UNDRESSPLAYER                                    Opcode = 0x020
	OpcodeCMSG_UNITANIMTIER_CHEAT                               Opcode = 0x472
	OpcodeCMSG_UNLEARN_DANCE_MOVE                               Opcode = 0x457
	OpcodeCMSG_UNLEARN_SKILL                                    Opcode = 0x202
	OpcodeCMSG_UNLEARN_SPELL                                    Opcode = 0x201
	OpcodeCMSG_UNLEARN_TALENTS                                  Opcode = 0x213
	OpcodeCMSG_UNSTABLE_PET                                     Opcode = 0x271
	OpcodeCMSG_UNUSED5                                          Opcode = 0x4B8
	OpcodeCMSG_UNUSED6                                          Opcode = 0x4B9
	OpcodeCMSG_UPDATE_ACCOUNT_DATA                              Opcode = 0x20B
	OpcodeCMSG_UPDATE_MISSILE_TRAJECTORY                        Opcode = 0x462
	OpcodeCMSG_UPDATE_PROJECTILE_POSITION                       Opcode = 0x4BE
	OpcodeCMSG_USE_ITEM                                         Opcode = 0x0AB
	OpcodeCMSG_USE_SKILL_CHEAT                                  Opcode = 0x029
	OpcodeCMSG_VOICE_SESSION_ENABLE                             Opcode = 0x3AF
	OpcodeCMSG_VOICE_SET_TALKER_MUTED_REQUEST                   Opcode = 0x3A1
	OpcodeCMSG_WARDEN_DATA                                      Opcode = 0x2E7
	OpcodeCMSG_WEATHER_SPEED_CHEAT                              Opcode = 0x01F
	OpcodeCMSG_WHO                                              Opcode = 0x062
	OpcodeCMSG_WHOIS                                            Opcode = 0x064
	OpcodeCMSG_WORLD_STATE_UI_TIMER_UPDATE                      Opcode = 0x4F6
	OpcodeCMSG_WORLD_TELEPORT                                   Opcode = 0x008
	OpcodeCMSG_WRAP_ITEM                                        Opcode = 0x1D3
	OpcodeCMSG_XP_CHEAT                                         Opcode = 0x221
	OpcodeCMSG_ZONEUPDATE                                       Opcode = 0x1F4
	OpcodeCMSG_ZONE_MAP                                         Opcode = 0x00A
	OpcodeMSG_AUCTION_HELLO                                     Opcode = 0x255
	OpcodeMSG_BATTLEGROUND_PLAYER_POSITIONS                     Opcode = 0x2E9
	OpcodeMSG_CHANNEL_START                                     Opcode = 0x139
	OpcodeMSG_CHANNEL_UPDATE                                    Opcode = 0x13A
	OpcodeMSG_CORPSE_QUERY                                      Opcode = 0x216
	OpcodeMSG_DELAY_GHOST_TELEPORT                              Opcode = 0x32E
	OpcodeMSG_DEV_SHOWLABEL                                     Opcode = 0x2AD
	OpcodeMSG_GM_ACCOUNT_ONLINE                                 Opcode = 0x26E
	OpcodeMSG_GM_BIND_OTHER                                     Opcode = 0x1E8
	OpcodeMSG_GM_CHANGE_ARENA_RATING                            Opcode = 0x40F
	OpcodeMSG_GM_DESTROY_CORPSE                                 Opcode = 0x310
	OpcodeMSG_GM_GEARRATING                                     Opcode = 0x3B4
	OpcodeMSG_GM_RESETINSTANCELIMIT                             Opcode = 0x33C
	OpcodeMSG_GM_SHOWLABEL                                      Opcode = 0x1EF
	OpcodeMSG_GM_SUMMON                                         Opcode = 0x1E9
	OpcodeMSG_GUILD_BANK_LOG_QUERY                              Opcode = 0x3EE
	OpcodeMSG_GUILD_BANK_MONEY_WITHDRAWN                        Opcode = 0x3FE
	OpcodeMSG_GUILD_EVENT_LOG_QUERY                             Opcode = 0x3FF
	OpcodeMSG_GUILD_PERMISSIONS                                 Opcode = 0x3FD
	OpcodeMSG_INSPECT_ARENA_TEAMS                               Opcode = 0x377
	OpcodeMSG_INSPECT_HONOR_STATS                               Opcode = 0x2D6
	OpcodeMSG_LIST_STABLED_PETS                                 Opcode = 0x26F
	OpcodeMSG_MINIMAP_PING                                      Opcode = 0x1D5
	OpcodeMSG_MOVE_FALL_LAND                                    Opcode = 0x0C9
	OpcodeMSG_MOVE_FEATHER_FALL                                 Opcode = 0x2B0
	OpcodeMSG_MOVE_GRAVITY_CHNG                                 Opcode = 0x4D2
	OpcodeMSG_MOVE_HEARTBEAT                                    Opcode = 0x0EE
	OpcodeMSG_MOVE_HOVER                                        Opcode = 0x0F7
	OpcodeMSG_MOVE_JUMP                                         Opcode = 0x0BB
	OpcodeMSG_MOVE_KNOCK_BACK                                   Opcode = 0x0F1
	OpcodeMSG_MOVE_ROOT                                         Opcode = 0x0EC
	OpcodeMSG_MOVE_SET_ALL_SPEED_CHEAT                          Opcode = 0x0D6
	OpcodeMSG_MOVE_SET_COLLISION_HGT                            Opcode = 0x518
	OpcodeMSG_MOVE_SET_FACING                                   Opcode = 0x0DA
	OpcodeMSG_MOVE_SET_FLIGHT_BACK_SPEED                        Opcode = 0x380
	OpcodeMSG_MOVE_SET_FLIGHT_BACK_SPEED_CHEAT                  Opcode = 0x37F
	OpcodeMSG_MOVE_SET_FLIGHT_SPEED                             Opcode = 0x37E
	OpcodeMSG_MOVE_SET_FLIGHT_SPEED_CHEAT                       Opcode = 0x37D
	OpcodeMSG_MOVE_SET_PITCH                                    Opcode = 0x0DB
	OpcodeMSG_MOVE_SET_PITCH_RATE                               Opcode = 0x45B
	OpcodeMSG_MOVE_SET_PITCH_RATE_CHEAT                         Opcode = 0x45A
	OpcodeMSG_MOVE_SET_RUN_BACK_SPEED                           Opcode = 0x0CF
	OpcodeMSG_MOVE_SET_RUN_BACK_SPEED_CHEAT                     Opcode = 0x0CE
	OpcodeMSG_MOVE_SET_RUN_MODE                                 Opcode = 0x0C2
	OpcodeMSG_MOVE_SET_RUN_SPEED                                Opcode = 0x0CD
	OpcodeMSG_MOVE_SET_RUN_SPEED_CHEAT                          Opcode = 0x0CC
	OpcodeMSG_MOVE_SET_SWIM_BACK_SPEED                          Opcode = 0x0D5
	OpcodeMSG_MOVE_SET_SWIM_BACK_SPEED_CHEAT                    Opcode = 0x0D4
	OpcodeMSG_MOVE_SET_SWIM_SPEED                               Opcode = 0x0D3
	OpcodeMSG_MOVE_SET_SWIM_SPEED_CHEAT                         Opcode = 0x0D2
	OpcodeMSG_MOVE_SET_TURN_RATE                                Opcode = 0x0D8
	OpcodeMSG_MOVE_SET_TURN_RATE_CHEAT                          Opcode = 0x0D7
	OpcodeMSG_MOVE_SET_WALK_MODE                                Opcode = 0x0C3
	OpcodeMSG_MOVE_SET_WALK_SPEED                               Opcode = 0x0D1
	OpcodeMSG_MOVE_SET_WALK_SPEED_CHEAT                         Opcode = 0x0D0
	OpcodeMSG_MOVE_START_ASCEND                                 Opcode = 0x359
	OpcodeMSG_MOVE_START_BACKWARD                               Opcode = 0x0B6
	OpcodeMSG_MOVE_START_DESCEND                                Opcode = 0x3A7
	OpcodeMSG_MOVE_START_FORWARD                                Opcode = 0x0B5
	OpcodeMSG_MOVE_START_PITCH_DOWN                             Opcode = 0x0C0
	OpcodeMSG_MOVE_START_PITCH_UP                               Opcode = 0x0BF
	OpcodeMSG_MOVE_START_STRAFE_LEFT                            Opcode = 0x0B8
	OpcodeMSG_MOVE_START_STRAFE_RIGHT                           Opcode = 0x0B9
	OpcodeMSG_MOVE_START_SWIM                                   Opcode = 0x0CA
	OpcodeMSG_MOVE_START_SWIM_CHEAT                             Opcode = 0x341
	OpcodeMSG_MOVE_START_TURN_LEFT                              Opcode = 0x0BC
	OpcodeMSG_MOVE_START_TURN_RIGHT                             Opcode = 0x0BD
	OpcodeMSG_MOVE_STOP                                         Opcode = 0x0B7
	OpcodeMSG_MOVE_STOP_ASCEND                                  Opcode = 0x35A
	OpcodeMSG_MOVE_STOP_PITCH                                   Opcode = 0x0C1
	OpcodeMSG_MOVE_STOP_STRAFE                                  Opcode = 0x0BA
	OpcodeMSG_MOVE_STOP_SWIM                                    Opcode = 0x0CB
	OpcodeMSG_MOVE_STOP_SWIM_CHEAT                              Opcode = 0x342
	OpcodeMSG_MOVE_STOP_TURN                                    Opcode = 0x0BE
	OpcodeMSG_MOVE_TELEPORT                                     Opcode = 0x0C5
	OpcodeMSG_MOVE_TELEPORT_ACK                                 Opcode = 0x0C7
	OpcodeMSG_MOVE_TELEPORT_CHEAT                               Opcode = 0x0C6
	OpcodeMSG_MOVE_TIME_SKIPPED                                 Opcode = 0x319
	OpcodeMSG_MOVE_TOGGLE_COLLISION_CHEAT                       Opcode = 0x0D9
	OpcodeMSG_MOVE_TOGGLE_FALL_LOGGING                          Opcode = 0x0C8
	OpcodeMSG_MOVE_TOGGLE_LOGGING                               Opcode = 0x0C4
	OpcodeMSG_MOVE_UNROOT                                       Opcode = 0x0ED
	OpcodeMSG_MOVE_UPDATE_CAN_FLY                               Opcode = 0x3AD
	OpcodeMSG_MOVE_UPDATE_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY   Opcode = 0x34A
	OpcodeMSG_MOVE_WATER_WALK                                   Opcode = 0x2B1
	OpcodeMSG_MOVE_WORLDPORT_ACK                                Opcode = 0x0DC
	OpcodeMSG_NOTIFY_PARTY_SQUELCH                              Opcode = 0x3DF
	OpcodeMSG_PARTY_ASSIGNMENT                                  Opcode = 0x38E
	OpcodeMSG_PETITION_DECLINE                                  Opcode = 0x1C2
	OpcodeMSG_PETITION_RENAME                                   Opcode = 0x2C1
	OpcodeMSG_PVP_LOG_DATA                                      Opcode = 0x2E0
	OpcodeMSG_QUERY_GUILD_BANK_TEXT                             Opcode = 0x40A
	OpcodeMSG_QUERY_NEXT_MAIL_TIME                              Opcode = 0x284
	OpcodeMSG_QUEST_PUSH_RESULT                                 Opcode = 0x276
	OpcodeMSG_RAID_READY_CHECK                                  Opcode = 0x322
	OpcodeMSG_RAID_READY_CHECK_CONFIRM                          Opcode = 0x3AE
	OpcodeMSG_RAID_READY_CHECK_FINISHED                         Opcode = 0x3C6
	OpcodeMSG_RAID_TARGET_UPDATE                                Opcode = 0x321
	OpcodeMSG_RANDOM_ROLL                                       Opcode = 0x1FB
	OpcodeMSG_SAVE_GUILD_EMBLEM                                 Opcode = 0x1F1
	OpcodeMSG_SET_DUNGEON_DIFFICULTY                            Opcode = 0x329
	OpcodeMSG_SET_RAID_DIFFICULTY                               Opcode = 0x4EB
	OpcodeMSG_TABARDVENDOR_ACTIVATE                             Opcode = 0x1F2
	OpcodeMSG_TALENT_WIPE_CONFIRM                               Opcode = 0x2AA
	OpcodeMSG_VIEW_PHASE_SHIFT                                  Opcode = 0x4F9
	OpcodeNULL_OPCODE                                           Opcode = 0x0000
	OpcodeNUM_MSG_TYPES                                         Opcode = 0x51F
	OpcodePROCESS_INPLACE                                       Opcode = 0
	OpcodeSMSG_ACCOUNT_DATA_TIMES                               Opcode = 0x209
	OpcodeSMSG_ACHIEVEMENT_DELETED                              Opcode = 0x49F
	OpcodeSMSG_ACHIEVEMENT_EARNED                               Opcode = 0x468
	OpcodeSMSG_ACTION_BUTTONS                                   Opcode = 0x129
	OpcodeSMSG_ACTIVATETAXIREPLY                                Opcode = 0x1AE
	OpcodeSMSG_ADDON_INFO                                       Opcode = 0x2EF
	OpcodeSMSG_ADD_RUNE_POWER                                   Opcode = 0x488
	OpcodeSMSG_AFK_MONITOR_INFO_RESPONSE                        Opcode = 0x504
	OpcodeSMSG_AI_REACTION                                      Opcode = 0x13C
	OpcodeSMSG_ALL_ACHIEVEMENT_DATA                             Opcode = 0x47D
	OpcodeSMSG_AREA_SPIRIT_HEALER_TIME                          Opcode = 0x2E4
	OpcodeSMSG_AREA_TRIGGER_MESSAGE                             Opcode = 0x2B8
	OpcodeSMSG_ARENA_ERROR                                      Opcode = 0x376
	OpcodeSMSG_ARENA_TEAM_CHANGE_FAILED_QUEUED                  Opcode = 0x4C8
	OpcodeSMSG_ARENA_TEAM_COMMAND_RESULT                        Opcode = 0x349
	OpcodeSMSG_ARENA_TEAM_EVENT                                 Opcode = 0x357
	OpcodeSMSG_ARENA_TEAM_INVITE                                Opcode = 0x350
	OpcodeSMSG_ARENA_TEAM_QUERY_RESPONSE                        Opcode = 0x34C
	OpcodeSMSG_ARENA_TEAM_ROSTER                                Opcode = 0x34E
	OpcodeSMSG_ARENA_TEAM_STATS                                 Opcode = 0x35B
	OpcodeSMSG_ARENA_UNIT_DESTROYED                             Opcode = 0x4C7
	OpcodeSMSG_ATTACKERSTATEUPDATE                              Opcode = 0x14A
	OpcodeSMSG_ATTACK_START                                     Opcode = 0x143
	OpcodeSMSG_ATTACK_STOP                                      Opcode = 0x144
	OpcodeSMSG_ATTACK_SWING_BAD_FACING                          Opcode = 0x146
	OpcodeSMSG_ATTACK_SWING_CANT_ATTACK                         Opcode = 0x149
	OpcodeSMSG_ATTACK_SWING_DEAD_TARGET                         Opcode = 0x148
	OpcodeSMSG_ATTACK_SWING_NOT_IN_RANGE                        Opcode = 0x145
	OpcodeSMSG_AUCTION_BIDDER_LIST_RESULT                       Opcode = 0x265
	OpcodeSMSG_AUCTION_BIDDER_NOTIFICATION                      Opcode = 0x25E
	OpcodeSMSG_AUCTION_COMMAND_RESULT                           Opcode = 0x25B
	OpcodeSMSG_AUCTION_LIST_PENDING_SALES                       Opcode = 0x490
	OpcodeSMSG_AUCTION_LIST_RESULT                              Opcode = 0x25C
	OpcodeSMSG_AUCTION_OWNER_LIST_RESULT                        Opcode = 0x25D
	OpcodeSMSG_AUCTION_OWNER_NOTIFICATION                       Opcode = 0x25F
	OpcodeSMSG_AUCTION_REMOVED_NOTIFICATION                     Opcode = 0x28D
	OpcodeSMSG_AURACASTLOG                                      Opcode = 0x1D1
	OpcodeSMSG_AURA_UPDATE                                      Opcode = 0x496
	OpcodeSMSG_AURA_UPDATE_ALL                                  Opcode = 0x495
	OpcodeSMSG_AUTH_CHALLENGE                                   Opcode = 0x1EC
	OpcodeSMSG_AUTH_RESPONSE                                    Opcode = 0x1EE
	OpcodeSMSG_AUTH_SRP6_RESPONSE                               Opcode = 0x039
	OpcodeSMSG_AVAILABLE_VOICE_CHANNEL                          Opcode = 0x3DA
	OpcodeSMSG_BARBER_SHOP_RESULT                               Opcode = 0x428
	OpcodeSMSG_BATTLEFIELD_LIST                                 Opcode = 0x23D
	OpcodeSMSG_BATTLEFIELD_MGR_EJECTED                          Opcode = 0x4E6
	OpcodeSMSG_BATTLEFIELD_MGR_EJECT_PENDING                    Opcode = 0x4E5
	OpcodeSMSG_BATTLEFIELD_MGR_ENTERED                          Opcode = 0x4E0
	OpcodeSMSG_BATTLEFIELD_MGR_ENTRY_INVITE                     Opcode = 0x4DE
	OpcodeSMSG_BATTLEFIELD_MGR_QUEUE_INVITE                     Opcode = 0x4E1
	OpcodeSMSG_BATTLEFIELD_MGR_QUEUE_REQUEST_RESPONSE           Opcode = 0x4E4
	OpcodeSMSG_BATTLEFIELD_MGR_STATE_CHANGE                     Opcode = 0x4E8
	OpcodeSMSG_BATTLEFIELD_PORT_DENIED                          Opcode = 0x14B
	OpcodeSMSG_BATTLEFIELD_STATUS                               Opcode = 0x2D4
	OpcodeSMSG_BATTLEGROUND_INFO_THROTTLED                      Opcode = 0x4A6
	OpcodeSMSG_BATTLEGROUND_PLAYER_JOINED                       Opcode = 0x2EC
	OpcodeSMSG_BATTLEGROUND_PLAYER_LEFT                         Opcode = 0x2ED
	OpcodeSMSG_BINDER_CONFIRM                                   Opcode = 0x2EB
	OpcodeSMSG_BINDZONEREPLY                                    Opcode = 0x157
	OpcodeSMSG_BIND_POINT_UPDATE                                Opcode = 0x155
	OpcodeSMSG_BREAK_TARGET                                     Opcode = 0x152
	OpcodeSMSG_BUY_BANK_SLOT_RESULT                             Opcode = 0x1BA
	OpcodeSMSG_BUY_FAILED                                       Opcode = 0x1A5
	OpcodeSMSG_BUY_ITEM                                         Opcode = 0x1A4
	OpcodeSMSG_CALENDAR_ARENA_TEAM                              Opcode = 0x439
	OpcodeSMSG_CALENDAR_CLEAR_PENDING_ACTION                    Opcode = 0x4BB
	OpcodeSMSG_CALENDAR_COMMAND_RESULT                          Opcode = 0x43D
	OpcodeSMSG_CALENDAR_EVENT_INVITE                            Opcode = 0x43A
	OpcodeSMSG_CALENDAR_EVENT_INVITE_ALERT                      Opcode = 0x440
	OpcodeSMSG_CALENDAR_EVENT_INVITE_NOTES                      Opcode = 0x460
	OpcodeSMSG_CALENDAR_EVENT_INVITE_NOTES_ALERT                Opcode = 0x461
	OpcodeSMSG_CALENDAR_EVENT_INVITE_REMOVED                    Opcode = 0x43B
	OpcodeSMSG_CALENDAR_EVENT_INVITE_REMOVED_ALERT              Opcode = 0x441
	OpcodeSMSG_CALENDAR_EVENT_INVITE_STATUS_ALERT               Opcode = 0x442
	OpcodeSMSG_CALENDAR_EVENT_MODERATOR_STATUS_ALERT            Opcode = 0x445
	OpcodeSMSG_CALENDAR_EVENT_REMOVED_ALERT                     Opcode = 0x443
	OpcodeSMSG_CALENDAR_EVENT_STATUS                            Opcode = 0x43C
	OpcodeSMSG_CALENDAR_EVENT_UPDATED_ALERT                     Opcode = 0x444
	OpcodeSMSG_CALENDAR_FILTER_GUILD                            Opcode = 0x438
	OpcodeSMSG_CALENDAR_RAID_LOCKOUT_ADDED                      Opcode = 0x43E
	OpcodeSMSG_CALENDAR_RAID_LOCKOUT_REMOVED                    Opcode = 0x43F
	OpcodeSMSG_CALENDAR_RAID_LOCKOUT_UPDATED                    Opcode = 0x471
	OpcodeSMSG_CALENDAR_SEND_CALENDAR                           Opcode = 0x436
	OpcodeSMSG_CALENDAR_SEND_EVENT                              Opcode = 0x437
	OpcodeSMSG_CALENDAR_SEND_NUM_PENDING                        Opcode = 0x448
	OpcodeSMSG_CAMERA_SHAKE                                     Opcode = 0x50A
	OpcodeSMSG_CANCEL_AUTO_REPEAT                               Opcode = 0x29C
	OpcodeSMSG_CANCEL_COMBAT                                    Opcode = 0x14E
	OpcodeSMSG_CAST_FAILED                                      Opcode = 0x130
	OpcodeSMSG_CHANGEPLAYER_DIFFICULTY_RESULT                   Opcode = 0x20E
	OpcodeSMSG_CHANNEL_LIST                                     Opcode = 0x09B
	OpcodeSMSG_CHANNEL_MEMBER_COUNT                             Opcode = 0x3D5
	OpcodeSMSG_CHANNEL_NOTIFY                                   Opcode = 0x099
	OpcodeSMSG_CHARACTER_LOGIN_FAILED                           Opcode = 0x041
	OpcodeSMSG_CHARACTER_PROFILE                                Opcode = 0x338
	OpcodeSMSG_CHARACTER_PROFILE_REALM_CONNECTED                Opcode = 0x339
	OpcodeSMSG_CHAR_CREATE                                      Opcode = 0x03A
	OpcodeSMSG_CHAR_CUSTOMIZE                                   Opcode = 0x474
	OpcodeSMSG_CHAR_DELETE                                      Opcode = 0x03C
	OpcodeSMSG_CHAR_ENUM                                        Opcode = 0x03B
	OpcodeSMSG_CHAR_FACTION_CHANGE                              Opcode = 0x4DA
	OpcodeSMSG_CHAR_RENAME                                      Opcode = 0x2C8
	OpcodeSMSG_CHAT_NOT_IN_PARTY                                Opcode = 0x299
	OpcodeSMSG_CHAT_PLAYER_AMBIGUOUS                            Opcode = 0x32D
	OpcodeSMSG_CHAT_PLAYER_NOT_FOUND                            Opcode = 0x2A9
	OpcodeSMSG_CHAT_RESTRICTED                                  Opcode = 0x2FD
	OpcodeSMSG_CHAT_SERVER_MESSAGE                              Opcode = 0x291
	OpcodeSMSG_CHAT_WRONG_FACTION                               Opcode = 0x219
	OpcodeSMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY_RESPONSE             Opcode = 0x39B
	OpcodeSMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY_RESPONSE_WRITE_FILE  Opcode = 0x39C
	OpcodeSMSG_CHEAT_PLAYER_LOOKUP                              Opcode = 0x3C4
	OpcodeSMSG_CHECK_FOR_BOTS                                   Opcode = 0x015
	OpcodeSMSG_CLEAR_COOLDOWN                                   Opcode = 0x1DE
	OpcodeSMSG_CLEAR_EXTRA_AURA_INFO_OBSOLETE                   Opcode = 0x3A6
	OpcodeSMSG_CLEAR_FAR_SIGHT_IMMEDIATE                        Opcode = 0x20D
	OpcodeSMSG_CLEAR_TARGET                                     Opcode = 0x3BF
	OpcodeSMSG_CLIENTCACHE_VERSION                              Opcode = 0x4AB
	OpcodeSMSG_CLIENT_CONTROL_UPDATE                            Opcode = 0x159
	OpcodeSMSG_COMBAT_EVENT_FAILED                              Opcode = 0x261
	OpcodeSMSG_COMMENTATOR_GET_PLAYER_INFO                      Opcode = 0x3BA
	OpcodeSMSG_COMMENTATOR_MAP_INFO                             Opcode = 0x3B8
	OpcodeSMSG_COMMENTATOR_PLAYER_INFO                          Opcode = 0x3BB
	OpcodeSMSG_COMMENTATOR_SKIRMISH_QUEUE_RESULT1               Opcode = 0x51C
	OpcodeSMSG_COMMENTATOR_SKIRMISH_QUEUE_RESULT2               Opcode = 0x51D
	OpcodeSMSG_COMMENTATOR_STATE_CHANGED                        Opcode = 0x3B6
	OpcodeSMSG_COMPLAIN_RESULT                                  Opcode = 0x3C8
	OpcodeSMSG_COMPRESSED_MOVES                                 Opcode = 0x2FB
	OpcodeSMSG_COMPRESSED_UPDATE_OBJECT                         Opcode = 0x1F6
	OpcodeSMSG_COMSAT_CONNECT_FAIL                              Opcode = 0x3E2
	OpcodeSMSG_COMSAT_DISCONNECT                                Opcode = 0x3E1
	OpcodeSMSG_COMSAT_RECONNECT_TRY                             Opcode = 0x3E0
	OpcodeSMSG_CONTACT_LIST                                     Opcode = 0x067
	OpcodeSMSG_CONVERT_RUNE                                     Opcode = 0x486
	OpcodeSMSG_COOLDOWN_CHEAT                                   Opcode = 0x1E1
	OpcodeSMSG_COOLDOWN_EVENT                                   Opcode = 0x135
	OpcodeSMSG_CORPSE_MAP_POSITION_QUERY_RESPONSE               Opcode = 0x4B7
	OpcodeSMSG_CORPSE_NOT_IN_INSTANCE                           Opcode = 0x506
	OpcodeSMSG_CORPSE_RECLAIM_DELAY                             Opcode = 0x269
	OpcodeSMSG_CREATURE_QUERY_RESPONSE                          Opcode = 0x061
	OpcodeSMSG_CRITERIA_DELETED                                 Opcode = 0x49E
	OpcodeSMSG_CRITERIA_UPDATE                                  Opcode = 0x46A
	OpcodeSMSG_CROSSED_INEBRIATION_THRESHOLD                    Opcode = 0x3C1
	OpcodeSMSG_DAMAGE_CALC_LOG                                  Opcode = 0x27C
	OpcodeSMSG_DANCE_QUERY_RESPONSE                             Opcode = 0x452
	OpcodeSMSG_DBLOOKUP                                         Opcode = 0x003
	OpcodeSMSG_DEATH_RELEASE_LOC                                Opcode = 0x378
	OpcodeSMSG_DEBUGAURAPROC                                    Opcode = 0x24D
	OpcodeSMSG_DEBUG_AISTATE                                    Opcode = 0x02F
	OpcodeSMSG_DEBUG_LIST_TARGETS                               Opcode = 0x3D9
	OpcodeSMSG_DEBUG_SERVER_GEO                                 Opcode = 0x4FC
	OpcodeSMSG_DEFENSE_MESSAGE                                  Opcode = 0x33A
	OpcodeSMSG_DESTROY_OBJECT                                   Opcode = 0x0AA
	OpcodeSMSG_DESTRUCTIBLE_BUILDING_DAMAGE                     Opcode = 0x032
	OpcodeSMSG_DISMOUNT                                         Opcode = 0x3AC
	OpcodeSMSG_DISMOUNTRESULT                                   Opcode = 0x16F
	OpcodeSMSG_DISPEL_FAILED                                    Opcode = 0x262
	OpcodeSMSG_DUEL_COMPLETE                                    Opcode = 0x16A
	OpcodeSMSG_DUEL_COUNTDOWN                                   Opcode = 0x2B7
	OpcodeSMSG_DUEL_INBOUNDS                                    Opcode = 0x169
	OpcodeSMSG_DUEL_OUTOFBOUNDS                                 Opcode = 0x168
	OpcodeSMSG_DUEL_REQUESTED                                   Opcode = 0x167
	OpcodeSMSG_DUEL_WINNER                                      Opcode = 0x16B
	OpcodeSMSG_DUMP_OBJECTS_DATA                                Opcode = 0x48C
	OpcodeSMSG_DURABILITY_DAMAGE_DEATH                          Opcode = 0x2BD
	OpcodeSMSG_DYNAMIC_DROP_ROLL_RESULT                         Opcode = 0x469
	OpcodeSMSG_ECHO_PARTY_SQUELCH                               Opcode = 0x3F6
	OpcodeSMSG_EMOTE                                            Opcode = 0x103
	OpcodeSMSG_ENABLE_BARBER_SHOP                               Opcode = 0x427
	OpcodeSMSG_ENCHANTMENTLOG                                   Opcode = 0x1D7
	OpcodeSMSG_ENVIRONMENTAL_DAMAGE_LOG                         Opcode = 0x1FC
	OpcodeSMSG_EQUIPMENT_SET_LIST                               Opcode = 0x4BC
	OpcodeSMSG_EQUIPMENT_SET_SAVED                              Opcode = 0x137
	OpcodeSMSG_EQUIPMENT_SET_USE_RESULT                         Opcode = 0x4D6
	OpcodeSMSG_EXPECTED_SPAM_RECORDS                            Opcode = 0x332
	OpcodeSMSG_EXPLORATION_EXPERIENCE                           Opcode = 0x1F8
	OpcodeSMSG_FEATURE_SYSTEM_STATUS                            Opcode = 0x3C9
	OpcodeSMSG_FEIGN_DEATH_RESISTED                             Opcode = 0x2B4
	OpcodeSMSG_FISH_ESCAPED                                     Opcode = 0x1C9
	OpcodeSMSG_FISH_NOT_HOOKED                                  Opcode = 0x1C8
	OpcodeSMSG_FLIGHT_SPLINE_SYNC                               Opcode = 0x388
	OpcodeSMSG_FORCEACTIONSHOW                                  Opcode = 0x01B
	OpcodeSMSG_FORCED_DEATH_UPDATE                              Opcode = 0x37A
	OpcodeSMSG_FORCE_ANIM                                       Opcode = 0x4D8
	OpcodeSMSG_FORCE_DISPLAY_UPDATE                             Opcode = 0x403
	OpcodeSMSG_FORCE_FLIGHT_BACK_SPEED_CHANGE                   Opcode = 0x383
	OpcodeSMSG_FORCE_FLIGHT_SPEED_CHANGE                        Opcode = 0x381
	OpcodeSMSG_FORCE_MOVE_ROOT                                  Opcode = 0x0E8
	OpcodeSMSG_FORCE_MOVE_UNROOT                                Opcode = 0x0EA
	OpcodeSMSG_FORCE_PITCH_RATE_CHANGE                          Opcode = 0x45C
	OpcodeSMSG_FORCE_RUN_BACK_SPEED_CHANGE                      Opcode = 0x0E4
	OpcodeSMSG_FORCE_RUN_SPEED_CHANGE                           Opcode = 0x0E2
	OpcodeSMSG_FORCE_SEND_QUEUED_PACKETS                        Opcode = 0x511
	OpcodeSMSG_FORCE_SET_VEHICLE_REC_ID                         Opcode = 0x23F
	OpcodeSMSG_FORCE_SWIM_BACK_SPEED_CHANGE                     Opcode = 0x2DC
	OpcodeSMSG_FORCE_SWIM_SPEED_CHANGE                          Opcode = 0x0E6
	OpcodeSMSG_FORCE_TURN_RATE_CHANGE                           Opcode = 0x2DE
	OpcodeSMSG_FORCE_WALK_SPEED_CHANGE                          Opcode = 0x2DA
	OpcodeSMSG_FRIEND_STATUS                                    Opcode = 0x068
	OpcodeSMSG_GAMEOBJECT_CUSTOM_ANIM                           Opcode = 0x0B3
	OpcodeSMSG_GAMEOBJECT_DESPAWN_ANIM                          Opcode = 0x215
	OpcodeSMSG_GAMEOBJECT_PAGETEXT                              Opcode = 0x1DF
	OpcodeSMSG_GAMEOBJECT_QUERY_RESPONSE                        Opcode = 0x05F
	OpcodeSMSG_GAMEOBJECT_RESET_STATE                           Opcode = 0x2A7
	OpcodeSMSG_GAMESPEED_SET                                    Opcode = 0x047
	OpcodeSMSG_GAMETIMEBIAS_SET                                 Opcode = 0x314
	OpcodeSMSG_GAMETIME_SET                                     Opcode = 0x045
	OpcodeSMSG_GAMETIME_UPDATE                                  Opcode = 0x043
	OpcodeSMSG_GHOSTEE_GONE                                     Opcode = 0x326
	OpcodeSMSG_GMRESPONSE_CREATE_TICKET                         Opcode = 0x4F2
	OpcodeSMSG_GMRESPONSE_DB_ERROR                              Opcode = 0x4EE
	OpcodeSMSG_GMRESPONSE_RECEIVED                              Opcode = 0x4EF
	OpcodeSMSG_GMRESPONSE_STATUS_UPDATE                         Opcode = 0x4F1
	OpcodeSMSG_GMTICKET_CREATE                                  Opcode = 0x206
	OpcodeSMSG_GMTICKET_DELETETICKET                            Opcode = 0x218
	OpcodeSMSG_GMTICKET_GETTICKET                               Opcode = 0x212
	OpcodeSMSG_GMTICKET_SYSTEMSTATUS                            Opcode = 0x21B
	OpcodeSMSG_GMTICKET_UPDATETEXT                              Opcode = 0x208
	OpcodeSMSG_GM_MESSAGECHAT                                   Opcode = 0x3B3
	OpcodeSMSG_GM_PLAYER_INFO                                   Opcode = 0x230
	OpcodeSMSG_GM_TICKET_STATUS_UPDATE                          Opcode = 0x328
	OpcodeSMSG_GODMODE                                          Opcode = 0x023
	OpcodeSMSG_GOGOGO_OBSOLETE                                  Opcode = 0x3F5
	OpcodeSMSG_GOSSIP_COMPLETE                                  Opcode = 0x17E
	OpcodeSMSG_GOSSIP_MESSAGE                                   Opcode = 0x17D
	OpcodeSMSG_GOSSIP_POI                                       Opcode = 0x224
	OpcodeSMSG_GROUPACTION_THROTTLED                            Opcode = 0x411
	OpcodeSMSG_GROUP_CANCEL                                     Opcode = 0x071
	OpcodeSMSG_GROUP_DECLINE                                    Opcode = 0x074
	OpcodeSMSG_GROUP_DESTROYED                                  Opcode = 0x07C
	OpcodeSMSG_GROUP_INVITE                                     Opcode = 0x06F
	OpcodeSMSG_GROUP_JOINED_BATTLEGROUND                        Opcode = 0x2E8
	OpcodeSMSG_GROUP_LIST                                       Opcode = 0x07D
	OpcodeSMSG_GROUP_SET_LEADER                                 Opcode = 0x079
	OpcodeSMSG_GROUP_UNINVITE                                   Opcode = 0x077
	OpcodeSMSG_GUILD_BANK_LIST                                  Opcode = 0x3E8
	OpcodeSMSG_GUILD_COMMAND_RESULT                             Opcode = 0x093
	OpcodeSMSG_GUILD_DECLINE                                    Opcode = 0x086
	OpcodeSMSG_GUILD_EVENT                                      Opcode = 0x092
	OpcodeSMSG_GUILD_INFO                                       Opcode = 0x088
	OpcodeSMSG_GUILD_INVITE                                     Opcode = 0x083
	OpcodeSMSG_GUILD_QUERY_RESPONSE                             Opcode = 0x055
	OpcodeSMSG_GUILD_ROSTER                                     Opcode = 0x08A
	OpcodeSMSG_HEALTH_UPDATE                                    Opcode = 0x47F
	OpcodeSMSG_HIGHEST_THREAT_UPDATE                            Opcode = 0x482
	OpcodeSMSG_IGNORE_DIMINISHING_RETURNS_CHEAT                 Opcode = 0x406
	OpcodeSMSG_IGNORE_REQUIREMENTS_CHEAT                        Opcode = 0x3A9
	OpcodeSMSG_INITIALIZE_FACTIONS                              Opcode = 0x122
	OpcodeSMSG_INITIAL_SPELLS                                   Opcode = 0x12A
	OpcodeSMSG_INIT_EXTRA_AURA_INFO_OBSOLETE                    Opcode = 0x3A3
	OpcodeSMSG_INIT_WORLD_STATES                                Opcode = 0x2C2
	OpcodeSMSG_INSPECT_RESULTS_UPDATE                           Opcode = 0x115
	OpcodeSMSG_INSPECT_TALENT                                   Opcode = 0x3F4
	OpcodeSMSG_INSTANCE_DIFFICULTY                              Opcode = 0x33B
	OpcodeSMSG_INSTANCE_LOCK_WARNING_QUERY                      Opcode = 0x147
	OpcodeSMSG_INSTANCE_RESET                                   Opcode = 0x31E
	OpcodeSMSG_INSTANCE_RESET_FAILED                            Opcode = 0x31F
	OpcodeSMSG_INSTANCE_SAVE_CREATED                            Opcode = 0x2CB
	OpcodeSMSG_INVALIDATE_DANCE                                 Opcode = 0x453
	OpcodeSMSG_INVALIDATE_PLAYER                                Opcode = 0x31C
	OpcodeSMSG_INVALID_PROMOTION_CODE                           Opcode = 0x1E7
	OpcodeSMSG_INVENTORY_CHANGE_FAILURE                         Opcode = 0x112
	OpcodeSMSG_ITEM_COOLDOWN                                    Opcode = 0x0B0
	OpcodeSMSG_ITEM_ENCHANT_TIME_UPDATE                         Opcode = 0x1EB
	OpcodeSMSG_ITEM_NAME_QUERY_RESPONSE                         Opcode = 0x2C5
	OpcodeSMSG_ITEM_PUSH_RESULT                                 Opcode = 0x166
	OpcodeSMSG_ITEM_QUERY_MULTIPLE_RESPONSE                     Opcode = 0x059
	OpcodeSMSG_ITEM_QUERY_SINGLE_RESPONSE                       Opcode = 0x058
	OpcodeSMSG_ITEM_REFUND_INFO_RESPONSE                        Opcode = 0x4B2
	OpcodeSMSG_ITEM_REFUND_RESULT                               Opcode = 0x4B5
	OpcodeSMSG_ITEM_TEXT_QUERY_RESPONSE                         Opcode = 0x244
	OpcodeSMSG_ITEM_TIME_UPDATE                                 Opcode = 0x1EA
	OpcodeSMSG_JOINED_BATTLEGROUND_QUEUE                        Opcode = 0x38A
	OpcodeSMSG_KICK_REASON                                      Opcode = 0x3C5
	OpcodeSMSG_LEARNED_DANCE_MOVES                              Opcode = 0x455
	OpcodeSMSG_LEARNED_SPELL                                    Opcode = 0x12B
	OpcodeSMSG_LEVELUP_INFO                                     Opcode = 0x1D4
	OpcodeSMSG_LFG_BOOT_PROPOSAL_UPDATE                         Opcode = 0x36D
	OpcodeSMSG_LFG_DISABLED                                     Opcode = 0x398
	OpcodeSMSG_LFG_JOIN_RESULT                                  Opcode = 0x364
	OpcodeSMSG_LFG_OFFER_CONTINUE                               Opcode = 0x293
	OpcodeSMSG_LFG_PARTY_INFO                                   Opcode = 0x372
	OpcodeSMSG_LFG_PLAYER_INFO                                  Opcode = 0x36F
	OpcodeSMSG_LFG_PLAYER_REWARD                                Opcode = 0x1FF
	OpcodeSMSG_LFG_PROPOSAL_UPDATE                              Opcode = 0x361
	OpcodeSMSG_LFG_QUEUE_STATUS                                 Opcode = 0x365
	OpcodeSMSG_LFG_ROLE_CHECK_UPDATE                            Opcode = 0x363
	OpcodeSMSG_LFG_ROLE_CHOSEN                                  Opcode = 0x2BB
	OpcodeSMSG_LFG_TELEPORT_DENIED                              Opcode = 0x200
	OpcodeSMSG_LFG_UPDATE_PARTY                                 Opcode = 0x368
	OpcodeSMSG_LFG_UPDATE_PLAYER                                Opcode = 0x367
	OpcodeSMSG_LFG_UPDATE_SEARCH                                Opcode = 0x369
	OpcodeSMSG_LIST_INVENTORY                                   Opcode = 0x19F
	OpcodeSMSG_LOGIN_SET_TIME_SPEED                             Opcode = 0x042
	OpcodeSMSG_LOGIN_VERIFY_WORLD                               Opcode = 0x236
	OpcodeSMSG_LOGOUT_CANCEL_ACK                                Opcode = 0x04F
	OpcodeSMSG_LOGOUT_COMPLETE                                  Opcode = 0x04D
	OpcodeSMSG_LOGOUT_RESPONSE                                  Opcode = 0x04C
	OpcodeSMSG_LOG_XPGAIN                                       Opcode = 0x1D0
	OpcodeSMSG_LOOT_ALL_PASSED                                  Opcode = 0x29E
	OpcodeSMSG_LOOT_CLEAR_MONEY                                 Opcode = 0x165
	OpcodeSMSG_LOOT_ITEM_NOTIFY                                 Opcode = 0x164
	OpcodeSMSG_LOOT_LIST                                        Opcode = 0x3F9
	OpcodeSMSG_LOOT_MASTER_LIST                                 Opcode = 0x2A4
	OpcodeSMSG_LOOT_MONEY_NOTIFY                                Opcode = 0x163
	OpcodeSMSG_LOOT_RELEASE_RESPONSE                            Opcode = 0x161
	OpcodeSMSG_LOOT_REMOVED                                     Opcode = 0x162
	OpcodeSMSG_LOOT_RESPONSE                                    Opcode = 0x160
	OpcodeSMSG_LOOT_ROLL                                        Opcode = 0x2A2
	OpcodeSMSG_LOOT_ROLL_WON                                    Opcode = 0x29F
	OpcodeSMSG_LOOT_SLOT_CHANGED                                Opcode = 0x4FD
	OpcodeSMSG_LOOT_START_ROLL                                  Opcode = 0x2A1
	OpcodeSMSG_LOTTERY_QUERY_RESULT_OBSOLETE                    Opcode = 0x335
	OpcodeSMSG_LOTTERY_RESULT_OBSOLETE                          Opcode = 0x337
	OpcodeSMSG_MAIL_LIST_RESULT                                 Opcode = 0x23B
	OpcodeSMSG_MESSAGECHAT                                      Opcode = 0x096
	OpcodeSMSG_MINIGAME_MOVE_FAILED                             Opcode = 0x2F9
	OpcodeSMSG_MINIGAME_SETUP                                   Opcode = 0x2F6
	OpcodeSMSG_MINIGAME_STATE                                   Opcode = 0x2F7
	OpcodeSMSG_MIRRORIMAGE_DATA                                 Opcode = 0x402
	OpcodeSMSG_MODIFY_COOLDOWN                                  Opcode = 0x491
	OpcodeSMSG_MONSTER_MOVE                                     Opcode = 0x0DD
	OpcodeSMSG_MONSTER_MOVE_TRANSPORT                           Opcode = 0x2AE
	OpcodeSMSG_MOTD                                             Opcode = 0x33D
	OpcodeSMSG_MOUNTSPECIAL_ANIM                                Opcode = 0x172
	OpcodeSMSG_MOUNT_RESULT                                     Opcode = 0x16E
	OpcodeSMSG_MOVE_CHARACTER_CHEAT                             Opcode = 0x00E
	OpcodeSMSG_MOVE_FEATHER_FALL                                Opcode = 0x0F2
	OpcodeSMSG_MOVE_GRAVITY_DISABLE                             Opcode = 0x4CE
	OpcodeSMSG_MOVE_GRAVITY_ENABLE                              Opcode = 0x4D0
	OpcodeSMSG_MOVE_KNOCK_BACK                                  Opcode = 0x0EF
	OpcodeSMSG_MOVE_LAND_WALK                                   Opcode = 0x0DF
	OpcodeSMSG_MOVE_NORMAL_FALL                                 Opcode = 0x0F3
	OpcodeSMSG_MOVE_SET_CAN_FLY                                 Opcode = 0x343
	OpcodeSMSG_MOVE_SET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY     Opcode = 0x33E
	OpcodeSMSG_MOVE_SET_COLLISION_HGT                           Opcode = 0x516
	OpcodeSMSG_MOVE_SET_HOVER                                   Opcode = 0x0F4
	OpcodeSMSG_MOVE_UNSET_CAN_FLY                               Opcode = 0x344
	OpcodeSMSG_MOVE_UNSET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY   Opcode = 0x33F
	OpcodeSMSG_MOVE_UNSET_HOVER                                 Opcode = 0x0F5
	OpcodeSMSG_MOVE_WATER_WALK                                  Opcode = 0x0DE
	OpcodeSMSG_MULTIPLE_MOVES                                   Opcode = 0x51E
	OpcodeSMSG_MULTIPLE_PACKETS                                 Opcode = 0x4CD
	OpcodeSMSG_NAME_QUERY_RESPONSE                              Opcode = 0x051
	OpcodeSMSG_NEW_TAXI_PATH                                    Opcode = 0x1AF
	OpcodeSMSG_NEW_WORLD                                        Opcode = 0x03E
	OpcodeSMSG_NOTIFICATION                                     Opcode = 0x1CB
	OpcodeSMSG_NOTIFY_DANCE                                     Opcode = 0x44A
	OpcodeSMSG_NOTIFY_DEST_LOC_SPELL_CAST                       Opcode = 0x48E
	OpcodeSMSG_NPC_TEXT_UPDATE                                  Opcode = 0x180
	OpcodeSMSG_NPC_WONT_TALK                                    Opcode = 0x181
	OpcodeSMSG_OFFER_PETITION_ERROR                             Opcode = 0x38F
	OpcodeSMSG_ON_CANCEL_EXPECTED_RIDE_VEHICLE_AURA             Opcode = 0x49D
	OpcodeSMSG_OPEN_CONTAINER                                   Opcode = 0x113
	OpcodeSMSG_OPEN_LFG_DUNGEON_FINDER                          Opcode = 0x515
	OpcodeSMSG_OVERRIDE_LIGHT                                   Opcode = 0x412
	OpcodeSMSG_PAGE_TEXT_QUERY_RESPONSE                         Opcode = 0x05B
	OpcodeSMSG_PARTYKILLLOG                                     Opcode = 0x1F5
	OpcodeSMSG_PARTY_COMMAND_RESULT                             Opcode = 0x07F
	OpcodeSMSG_PARTY_MEMBER_STATS                               Opcode = 0x07E
	OpcodeSMSG_PARTY_MEMBER_STATS_FULL                          Opcode = 0x2F2
	OpcodeSMSG_PAUSE_MIRROR_TIMER                               Opcode = 0x1DA
	OpcodeSMSG_PERIODICAURALOG                                  Opcode = 0x24E
	OpcodeSMSG_PETGODMODE                                       Opcode = 0x01D
	OpcodeSMSG_PETITION_QUERY_RESPONSE                          Opcode = 0x1C7
	OpcodeSMSG_PETITION_SHOWLIST                                Opcode = 0x1BC
	OpcodeSMSG_PETITION_SHOW_SIGNATURES                         Opcode = 0x1BF
	OpcodeSMSG_PETITION_SIGN_RESULTS                            Opcode = 0x1C1
	OpcodeSMSG_PET_ACTION_FEEDBACK                              Opcode = 0x2C6
	OpcodeSMSG_PET_ACTION_SOUND                                 Opcode = 0x324
	OpcodeSMSG_PET_BROKEN                                       Opcode = 0x2AF
	OpcodeSMSG_PET_CAST_FAILED                                  Opcode = 0x138
	OpcodeSMSG_PET_DISMISS_SOUND                                Opcode = 0x325
	OpcodeSMSG_PET_GUIDS                                        Opcode = 0x4AA
	OpcodeSMSG_PET_LEARNED_SPELL                                Opcode = 0x499
	OpcodeSMSG_PET_MODE                                         Opcode = 0x17A
	OpcodeSMSG_PET_NAME_INVALID                                 Opcode = 0x178
	OpcodeSMSG_PET_NAME_QUERY_RESPONSE                          Opcode = 0x053
	OpcodeSMSG_PET_RENAMEABLE                                   Opcode = 0x475
	OpcodeSMSG_PET_SPELLS                                       Opcode = 0x179
	OpcodeSMSG_PET_TAME_FAILURE                                 Opcode = 0x173
	OpcodeSMSG_PET_UNLEARNED_SPELL                              Opcode = 0x49A
	OpcodeSMSG_PET_UNLEARN_CONFIRM                              Opcode = 0x2F1
	OpcodeSMSG_PET_UPDATE_COMBO_POINTS                          Opcode = 0x492
	OpcodeSMSG_PLAYED_TIME                                      Opcode = 0x1CD
	OpcodeSMSG_PLAYERBINDERROR                                  Opcode = 0x1B6
	OpcodeSMSG_PLAYER_BOUND                                     Opcode = 0x158
	OpcodeSMSG_PLAYER_SKINNED                                   Opcode = 0x2BC
	OpcodeSMSG_PLAYER_VEHICLE_DATA                              Opcode = 0x4A7
	OpcodeSMSG_PLAY_DANCE                                       Opcode = 0x44C
	OpcodeSMSG_PLAY_MUSIC                                       Opcode = 0x277
	OpcodeSMSG_PLAY_OBJECT_SOUND                                Opcode = 0x278
	OpcodeSMSG_PLAY_SOUND                                       Opcode = 0x2D2
	OpcodeSMSG_PLAY_SPELL_IMPACT                                Opcode = 0x1F7
	OpcodeSMSG_PLAY_SPELL_VISUAL                                Opcode = 0x1F3
	OpcodeSMSG_PLAY_TIME_WARNING                                Opcode = 0x2F5
	OpcodeSMSG_PONG                                             Opcode = 0x1DD
	OpcodeSMSG_POWER_UPDATE                                     Opcode = 0x480
	OpcodeSMSG_PRE_RESURRECT                                    Opcode = 0x494
	OpcodeSMSG_PROCRESIST                                       Opcode = 0x260
	OpcodeSMSG_PROFILEDATA_RESPONSE                             Opcode = 0x4CA
	OpcodeSMSG_PROPOSE_LEVEL_GRANT                              Opcode = 0x41F
	OpcodeSMSG_PVP_CREDIT                                       Opcode = 0x28C
	OpcodeSMSG_PVP_QUEUE_STATS                                  Opcode = 0x4DC
	OpcodeSMSG_QUERY_OBJECT_POSITION                            Opcode = 0x005
	OpcodeSMSG_QUERY_OBJECT_ROTATION                            Opcode = 0x007
	OpcodeSMSG_QUERY_QUESTS_COMPLETED_RESPONSE                  Opcode = 0x501
	OpcodeSMSG_QUERY_TIME_RESPONSE                              Opcode = 0x1CF
	OpcodeSMSG_QUESTGIVER_QUEST_COMPLETE                        Opcode = 0x191
	OpcodeSMSG_QUESTGIVER_QUEST_FAILED                          Opcode = 0x192
	OpcodeSMSG_QUESTGIVER_QUEST_INVALID                         Opcode = 0x18F
	OpcodeSMSG_QUESTGIVER_QUEST_LIST                            Opcode = 0x185
	OpcodeSMSG_QUESTGIVER_REQUEST_ITEMS                         Opcode = 0x18B
	OpcodeSMSG_QUESTGIVER_STATUS                                Opcode = 0x183
	OpcodeSMSG_QUESTGIVER_STATUS_MULTIPLE                       Opcode = 0x418
	OpcodeSMSG_QUESTLOG_FULL                                    Opcode = 0x195
	OpcodeSMSG_QUESTUPDATE_ADD_ITEM                             Opcode = 0x19A
	OpcodeSMSG_QUESTUPDATE_ADD_KILL                             Opcode = 0x199
	OpcodeSMSG_QUESTUPDATE_ADD_PVP_KILL                         Opcode = 0x46F
	OpcodeSMSG_QUESTUPDATE_COMPLETE                             Opcode = 0x198
	OpcodeSMSG_QUESTUPDATE_FAILED                               Opcode = 0x196
	OpcodeSMSG_QUESTUPDATE_FAILEDTIMER                          Opcode = 0x197
	OpcodeSMSG_QUEST_CONFIRM_ACCEPT                             Opcode = 0x19C
	OpcodeSMSG_QUEST_FORCE_REMOVE                               Opcode = 0x21E
	OpcodeSMSG_QUEST_GIVER_OFFER_REWARD_MESSAGE                 Opcode = 0x18D
	OpcodeSMSG_QUEST_GIVER_QUEST_DETAILS                        Opcode = 0x188
	OpcodeSMSG_QUEST_POI_QUERY_RESPONSE                         Opcode = 0x1E4
	OpcodeSMSG_QUEST_QUERY_RESPONSE                             Opcode = 0x05D
	OpcodeSMSG_RAID_GROUP_ONLY                                  Opcode = 0x286
	OpcodeSMSG_RAID_INSTANCE_INFO                               Opcode = 0x2CC
	OpcodeSMSG_RAID_INSTANCE_MESSAGE                            Opcode = 0x2FA
	OpcodeSMSG_RAID_READY_CHECK_ERROR                           Opcode = 0x408
	OpcodeSMSG_READ_ITEM_FAILED                                 Opcode = 0x0AF
	OpcodeSMSG_READ_ITEM_OK                                     Opcode = 0x0AE
	OpcodeSMSG_REALM_SPLIT                                      Opcode = 0x38B
	OpcodeSMSG_REAL_GROUP_UPDATE                                Opcode = 0x397
	OpcodeSMSG_RECEIVED_MAIL                                    Opcode = 0x285
	OpcodeSMSG_REDIRECT_CLIENT                                  Opcode = 0x50D
	OpcodeSMSG_REFER_A_FRIEND_EXPIRED                           Opcode = 0x01E
	OpcodeSMSG_REFER_A_FRIEND_FAILURE                           Opcode = 0x421
	OpcodeSMSG_REMOVED_FROM_PVP_QUEUE                           Opcode = 0x170
	OpcodeSMSG_REMOVED_SPELL                                    Opcode = 0x203
	OpcodeSMSG_REPORT_PVP_AFK_RESULT                            Opcode = 0x3E5
	OpcodeSMSG_RESET_FAILED_NOTIFY                              Opcode = 0x396
	OpcodeSMSG_RESET_RANGED_COMBAT_TIMER                        Opcode = 0x298
	OpcodeSMSG_RESISTLOG                                        Opcode = 0x1D6
	OpcodeSMSG_RESPOND_INSPECT_ACHIEVEMENTS                     Opcode = 0x46C
	OpcodeSMSG_RESUME_CAST_BAR                                  Opcode = 0x14D
	OpcodeSMSG_RESURRECT_FAILED                                 Opcode = 0x252
	OpcodeSMSG_RESURRECT_REQUEST                                Opcode = 0x15B
	OpcodeSMSG_RESYNC_RUNES                                     Opcode = 0x487
	OpcodeSMSG_RWHOIS                                           Opcode = 0x1FE
	OpcodeSMSG_SCRIPT_MESSAGE                                   Opcode = 0x2B6
	OpcodeSMSG_SELL_ITEM                                        Opcode = 0x1A1
	OpcodeSMSG_SEND_ALL_COMBAT_LOG                              Opcode = 0x514
	OpcodeSMSG_SEND_MAIL_RESULT                                 Opcode = 0x239
	OpcodeSMSG_SEND_UNLEARN_SPELLS                              Opcode = 0x41E
	OpcodeSMSG_SERVERINFO                                       Opcode = 0x4F5
	OpcodeSMSG_SERVERTIME                                       Opcode = 0x049
	OpcodeSMSG_SERVER_BUCK_DATA                                 Opcode = 0x41D
	OpcodeSMSG_SERVER_BUCK_DATA_START                           Opcode = 0x4A3
	OpcodeSMSG_SERVER_FIRST_ACHIEVEMENT                         Opcode = 0x498
	OpcodeSMSG_SERVER_INFO_RESPONSE                             Opcode = 0x4A1
	OpcodeSMSG_SET_EXTRA_AURA_INFO_NEED_UPDATE_OBSOLETE         Opcode = 0x3A5
	OpcodeSMSG_SET_EXTRA_AURA_INFO_OBSOLETE                     Opcode = 0x3A4
	OpcodeSMSG_SET_FACTION_ATWAR                                Opcode = 0x313
	OpcodeSMSG_SET_FACTION_STANDING                             Opcode = 0x124
	OpcodeSMSG_SET_FACTION_VISIBLE                              Opcode = 0x123
	OpcodeSMSG_SET_FLAT_SPELL_MODIFIER                          Opcode = 0x266
	OpcodeSMSG_SET_FORCED_REACTIONS                             Opcode = 0x2A5
	OpcodeSMSG_SET_PCT_SPELL_MODIFIER                           Opcode = 0x267
	OpcodeSMSG_SET_PHASE_SHIFT                                  Opcode = 0x47C
	OpcodeSMSG_SET_PLAYER_DECLINED_NAMES_RESULT                 Opcode = 0x41A
	OpcodeSMSG_SET_PROFICIENCY                                  Opcode = 0x127
	OpcodeSMSG_SET_PROJECTILE_POSITION                          Opcode = 0x4BF
	OpcodeSMSG_SHOWTAXINODES                                    Opcode = 0x1A9
	OpcodeSMSG_SHOW_BANK                                        Opcode = 0x1B8
	OpcodeSMSG_SHOW_MAILBOX                                     Opcode = 0x297
	OpcodeSMSG_SOCKET_GEMS_RESULT                               Opcode = 0x50B
	OpcodeSMSG_SPELLBREAKLOG                                    Opcode = 0x14F
	OpcodeSMSG_SPELLDAMAGESHIELD                                Opcode = 0x24F
	OpcodeSMSG_SPELLDISPELLOG                                   Opcode = 0x27B
	OpcodeSMSG_SPELLENERGIZELOG                                 Opcode = 0x151
	OpcodeSMSG_SPELLHEALLOG                                     Opcode = 0x150
	OpcodeSMSG_SPELLINSTAKILLLOG                                Opcode = 0x32F
	OpcodeSMSG_SPELLLOGEXECUTE                                  Opcode = 0x24C
	OpcodeSMSG_SPELLLOGMISS                                     Opcode = 0x24B
	OpcodeSMSG_SPELLNONMELEEDAMAGELOG                           Opcode = 0x250
	OpcodeSMSG_SPELLORDAMAGE_IMMUNE                             Opcode = 0x263
	OpcodeSMSG_SPELLSTEALLOG                                    Opcode = 0x333
	OpcodeSMSG_SPELL_CHANCE_PROC_LOG                            Opcode = 0x3AA
	OpcodeSMSG_SPELL_CHANCE_RESIST_PUSHBACK                     Opcode = 0x404
	OpcodeSMSG_SPELL_COOLDOWN                                   Opcode = 0x134
	OpcodeSMSG_SPELL_DELAYED                                    Opcode = 0x1E2
	OpcodeSMSG_SPELL_FAILED_OTHER                               Opcode = 0x2A6
	OpcodeSMSG_SPELL_FAILURE                                    Opcode = 0x133
	OpcodeSMSG_SPELL_GO                                         Opcode = 0x132
	OpcodeSMSG_SPELL_START                                      Opcode = 0x131
	OpcodeSMSG_SPELL_UPDATE_CHAIN_TARGETS                       Opcode = 0x330
	OpcodeSMSG_SPIRIT_HEALER_CONFIRM                            Opcode = 0x222
	OpcodeSMSG_SPLINE_MOVE_FEATHER_FALL                         Opcode = 0x305
	OpcodeSMSG_SPLINE_MOVE_GRAVITY_DISABLE                      Opcode = 0x4D3
	OpcodeSMSG_SPLINE_MOVE_GRAVITY_ENABLE                       Opcode = 0x4D4
	OpcodeSMSG_SPLINE_MOVE_LAND_WALK                            Opcode = 0x30A
	OpcodeSMSG_SPLINE_MOVE_NORMAL_FALL                          Opcode = 0x306
	OpcodeSMSG_SPLINE_MOVE_ROOT                                 Opcode = 0x31A
	OpcodeSMSG_SPLINE_MOVE_SET_FLYING                           Opcode = 0x422
	OpcodeSMSG_SPLINE_MOVE_SET_HOVER                            Opcode = 0x307
	OpcodeSMSG_SPLINE_MOVE_SET_RUN_MODE                         Opcode = 0x30D
	OpcodeSMSG_SPLINE_MOVE_SET_WALK_MODE                        Opcode = 0x30E
	OpcodeSMSG_SPLINE_MOVE_START_SWIM                           Opcode = 0x30B
	OpcodeSMSG_SPLINE_MOVE_STOP_SWIM                            Opcode = 0x30C
	OpcodeSMSG_SPLINE_MOVE_UNROOT                               Opcode = 0x304
	OpcodeSMSG_SPLINE_MOVE_UNSET_FLYING                         Opcode = 0x423
	OpcodeSMSG_SPLINE_MOVE_UNSET_HOVER                          Opcode = 0x308
	OpcodeSMSG_SPLINE_MOVE_WATER_WALK                           Opcode = 0x309
	OpcodeSMSG_SPLINE_SET_FLIGHT_BACK_SPEED                     Opcode = 0x386
	OpcodeSMSG_SPLINE_SET_FLIGHT_SPEED                          Opcode = 0x385
	OpcodeSMSG_SPLINE_SET_PITCH_RATE                            Opcode = 0x45E
	OpcodeSMSG_SPLINE_SET_RUN_BACK_SPEED                        Opcode = 0x2FF
	OpcodeSMSG_SPLINE_SET_RUN_SPEED                             Opcode = 0x2FE
	OpcodeSMSG_SPLINE_SET_SWIM_BACK_SPEED                       Opcode = 0x302
	OpcodeSMSG_SPLINE_SET_SWIM_SPEED                            Opcode = 0x300
	OpcodeSMSG_SPLINE_SET_TURN_RATE                             Opcode = 0x303
	OpcodeSMSG_SPLINE_SET_WALK_SPEED                            Opcode = 0x301
	OpcodeSMSG_STABLE_RESULT                                    Opcode = 0x273
	OpcodeSMSG_STANDSTATE_UPDATE                                Opcode = 0x29D
	OpcodeSMSG_START_MIRROR_TIMER                               Opcode = 0x1D9
	OpcodeSMSG_STOP_DANCE                                       Opcode = 0x44F
	OpcodeSMSG_STOP_MIRROR_TIMER                                Opcode = 0x1DB
	OpcodeSMSG_SUMMON_CANCEL                                    Opcode = 0x424
	OpcodeSMSG_SUMMON_REQUEST                                   Opcode = 0x2AB
	OpcodeSMSG_SUPERCEDED_SPELL                                 Opcode = 0x12C
	OpcodeSMSG_SUSPEND_COMMS                                    Opcode = 0x50F
	OpcodeSMSG_TALENTS_INFO                                     Opcode = 0x4C0
	OpcodeSMSG_TALENTS_INVOLUNTARILY_RESET                      Opcode = 0x4FA
	OpcodeSMSG_TAXINODE_STATUS                                  Opcode = 0x1AB
	OpcodeSMSG_TEST_DROP_RATE_RESULT                            Opcode = 0x295
	OpcodeSMSG_TEXT_EMOTE                                       Opcode = 0x105
	OpcodeSMSG_THREAT_CLEAR                                     Opcode = 0x485
	OpcodeSMSG_THREAT_REMOVE                                    Opcode = 0x484
	OpcodeSMSG_THREAT_UPDATE                                    Opcode = 0x483
	OpcodeSMSG_TIME_SYNC_REQ                                    Opcode = 0x390
	OpcodeSMSG_TITLE_EARNED                                     Opcode = 0x373
	OpcodeSMSG_TOGGLE_XP_GAIN                                   Opcode = 0x4ED
	OpcodeSMSG_TOTEM_CREATED                                    Opcode = 0x413
	OpcodeSMSG_TRADE_STATUS                                     Opcode = 0x120
	OpcodeSMSG_TRADE_STATUS_EXTENDED                            Opcode = 0x121
	OpcodeSMSG_TRAINER_BUY_FAILED                               Opcode = 0x1B4
	OpcodeSMSG_TRAINER_BUY_SUCCEEDED                            Opcode = 0x1B3
	OpcodeSMSG_TRAINER_LIST                                     Opcode = 0x1B1
	OpcodeSMSG_TRANSFER_ABORTED                                 Opcode = 0x040
	OpcodeSMSG_TRANSFER_PENDING                                 Opcode = 0x03F
	OpcodeSMSG_TRIGGER_CINEMATIC                                Opcode = 0x0FA
	OpcodeSMSG_TRIGGER_MOVIE                                    Opcode = 0x464
	OpcodeSMSG_TURN_IN_PETITION_RESULTS                         Opcode = 0x1C5
	OpcodeSMSG_TUTORIAL_FLAGS                                   Opcode = 0x0FD
	OpcodeSMSG_UPDATE_ACCOUNT_DATA                              Opcode = 0x20C
	OpcodeSMSG_UPDATE_ACCOUNT_DATA_COMPLETE                     Opcode = 0x463
	OpcodeSMSG_UPDATE_COMBO_POINTS                              Opcode = 0x39D
	OpcodeSMSG_UPDATE_INSTANCE_ENCOUNTER_UNIT                   Opcode = 0x214
	OpcodeSMSG_UPDATE_INSTANCE_OWNERSHIP                        Opcode = 0x32B
	OpcodeSMSG_UPDATE_LAST_INSTANCE                             Opcode = 0x320
	OpcodeSMSG_UPDATE_LFG_LIST                                  Opcode = 0x360
	OpcodeSMSG_UPDATE_OBJECT                                    Opcode = 0x0A9
	OpcodeSMSG_UPDATE_WORLD_STATE                               Opcode = 0x2C3
	OpcodeSMSG_USERLIST_ADD                                     Opcode = 0x3F0
	OpcodeSMSG_USERLIST_REMOVE                                  Opcode = 0x3F1
	OpcodeSMSG_USERLIST_UPDATE                                  Opcode = 0x3F2
	OpcodeSMSG_VOICESESSION_FULL                                Opcode = 0x3FC
	OpcodeSMSG_VOICE_CHAT_STATUS                                Opcode = 0x3E3
	OpcodeSMSG_VOICE_PARENTAL_CONTROLS                          Opcode = 0x3B1
	OpcodeSMSG_VOICE_SESSION_ADJUST_PRIORITY                    Opcode = 0x3A0
	OpcodeSMSG_VOICE_SESSION_ENABLE                             Opcode = 0x3B0
	OpcodeSMSG_VOICE_SESSION_LEAVE                              Opcode = 0x39F
	OpcodeSMSG_VOICE_SESSION_ROSTER_UPDATE                      Opcode = 0x39E
	OpcodeSMSG_VOICE_SET_TALKER_MUTED                           Opcode = 0x3A2
	OpcodeSMSG_WARDEN_DATA                                      Opcode = 0x2E6
	OpcodeSMSG_WEATHER                                          Opcode = 0x2F4
	OpcodeSMSG_WHO                                              Opcode = 0x063
	OpcodeSMSG_WHOIS                                            Opcode = 0x065
	OpcodeSMSG_WORLD_STATE_UI_TIMER_UPDATE                      Opcode = 0x4F7
	OpcodeSMSG_ZONE_MAP                                         Opcode = 0x00B
	OpcodeSMSG_ZONE_UNDER_ATTACK                                Opcode = 0x254
	OpcodeSTATUS_AUTHED                                         Opcode = 0
	OpcodeUMSG_DELETE_GUILD_CHARTER                             Opcode = 0x2C0
	OpcodeUMSG_UPDATE_GROUP_INFO                                Opcode = 0x4FE
	OpcodeUMSG_UPDATE_GROUP_MEMBERS                             Opcode = 0x080
	OpcodeUMSG_UPDATE_GUILD                                     Opcode = 0x094
)

var OpcodeNames = map[Opcode]string{
	OpcodeCMSG_ACCEPT_LEVEL_GRANT:                               "CMSG_ACCEPT_LEVEL_GRANT",
	OpcodeCMSG_ACCEPT_TRADE:                                     "CMSG_ACCEPT_TRADE",
	OpcodeCMSG_ACTIVATETAXI:                                     "CMSG_ACTIVATETAXI",
	OpcodeCMSG_ACTIVATETAXIEXPRESS:                              "CMSG_ACTIVATETAXIEXPRESS",
	OpcodeCMSG_ACTIVE_PVP_CHEAT:                                 "CMSG_ACTIVE_PVP_CHEAT",
	OpcodeCMSG_ADD_FRIEND:                                       "CMSG_ADD_FRIEND",
	OpcodeCMSG_ADD_IGNORE:                                       "CMSG_ADD_IGNORE",
	OpcodeCMSG_ADD_PVP_MEDAL_CHEAT:                              "CMSG_ADD_PVP_MEDAL_CHEAT",
	OpcodeCMSG_ADD_VOICE_IGNORE:                                 "CMSG_ADD_VOICE_IGNORE",
	OpcodeCMSG_ADVANCE_SPAWN_TIME:                               "CMSG_ADVANCE_SPAWN_TIME",
	OpcodeCMSG_AFK_MONITOR_INFO_CLEAR:                           "CMSG_AFK_MONITOR_INFO_CLEAR",
	OpcodeCMSG_AFK_MONITOR_INFO_REQUEST:                         "CMSG_AFK_MONITOR_INFO_REQUEST",
	OpcodeCMSG_ALTER_APPEARANCE:                                 "CMSG_ALTER_APPEARANCE",
	OpcodeCMSG_AREATRIGGER:                                      "CMSG_AREATRIGGER",
	OpcodeCMSG_AREA_SPIRIT_HEALER_QUERY:                         "CMSG_AREA_SPIRIT_HEALER_QUERY",
	OpcodeCMSG_AREA_SPIRIT_HEALER_QUEUE:                         "CMSG_AREA_SPIRIT_HEALER_QUEUE",
	OpcodeCMSG_ARENA_TEAM_ACCEPT:                                "CMSG_ARENA_TEAM_ACCEPT",
	OpcodeCMSG_ARENA_TEAM_CREATE:                                "CMSG_ARENA_TEAM_CREATE",
	OpcodeCMSG_ARENA_TEAM_DECLINE:                               "CMSG_ARENA_TEAM_DECLINE",
	OpcodeCMSG_ARENA_TEAM_DISBAND:                               "CMSG_ARENA_TEAM_DISBAND",
	OpcodeCMSG_ARENA_TEAM_INVITE:                                "CMSG_ARENA_TEAM_INVITE",
	OpcodeCMSG_ARENA_TEAM_LEADER:                                "CMSG_ARENA_TEAM_LEADER",
	OpcodeCMSG_ARENA_TEAM_LEAVE:                                 "CMSG_ARENA_TEAM_LEAVE",
	OpcodeCMSG_ARENA_TEAM_QUERY:                                 "CMSG_ARENA_TEAM_QUERY",
	OpcodeCMSG_ARENA_TEAM_REMOVE:                                "CMSG_ARENA_TEAM_REMOVE",
	OpcodeCMSG_ARENA_TEAM_ROSTER:                                "CMSG_ARENA_TEAM_ROSTER",
	OpcodeCMSG_ATTACK_STOP:                                      "CMSG_ATTACK_STOP",
	OpcodeCMSG_ATTACK_SWING:                                     "CMSG_ATTACK_SWING",
	OpcodeCMSG_AUCTION_LIST_BIDDER_ITEMS:                        "CMSG_AUCTION_LIST_BIDDER_ITEMS",
	OpcodeCMSG_AUCTION_LIST_ITEMS:                               "CMSG_AUCTION_LIST_ITEMS",
	OpcodeCMSG_AUCTION_LIST_OWNER_ITEMS:                         "CMSG_AUCTION_LIST_OWNER_ITEMS",
	OpcodeCMSG_AUCTION_LIST_PENDING_SALES:                       "CMSG_AUCTION_LIST_PENDING_SALES",
	OpcodeCMSG_AUCTION_PLACE_BID:                                "CMSG_AUCTION_PLACE_BID",
	OpcodeCMSG_AUCTION_REMOVE_ITEM:                              "CMSG_AUCTION_REMOVE_ITEM",
	OpcodeCMSG_AUCTION_SELL_ITEM:                                "CMSG_AUCTION_SELL_ITEM",
	OpcodeCMSG_AUTH_SESSION:                                     "CMSG_AUTH_SESSION",
	OpcodeCMSG_AUTH_SRP6_BEGIN:                                  "CMSG_AUTH_SRP6_BEGIN",
	OpcodeCMSG_AUTH_SRP6_PROOF:                                  "CMSG_AUTH_SRP6_PROOF",
	OpcodeCMSG_AUTH_SRP6_RECODE:                                 "CMSG_AUTH_SRP6_RECODE",
	OpcodeCMSG_AUTOBANK_ITEM:                                    "CMSG_AUTOBANK_ITEM",
	OpcodeCMSG_AUTOEQUIP_GROUND_ITEM:                            "CMSG_AUTOEQUIP_GROUND_ITEM",
	OpcodeCMSG_AUTOEQUIP_ITEM:                                   "CMSG_AUTOEQUIP_ITEM",
	OpcodeCMSG_AUTOEQUIP_ITEM_SLOT:                              "CMSG_AUTOEQUIP_ITEM_SLOT",
	OpcodeCMSG_AUTOSTORE_BAG_ITEM:                               "CMSG_AUTOSTORE_BAG_ITEM",
	OpcodeCMSG_AUTOSTORE_BANK_ITEM:                              "CMSG_AUTOSTORE_BANK_ITEM",
	OpcodeCMSG_AUTOSTORE_GROUND_ITEM:                            "CMSG_AUTOSTORE_GROUND_ITEM",
	OpcodeCMSG_AUTOSTORE_LOOT_ITEM:                              "CMSG_AUTOSTORE_LOOT_ITEM",
	OpcodeCMSG_BANKER_ACTIVATE:                                  "CMSG_BANKER_ACTIVATE",
	OpcodeCMSG_BATTLEFIELD_JOIN:                                 "CMSG_BATTLEFIELD_JOIN",
	OpcodeCMSG_BATTLEFIELD_LIST:                                 "CMSG_BATTLEFIELD_LIST",
	OpcodeCMSG_BATTLEFIELD_MANAGER_ADVANCE_STATE:                "CMSG_BATTLEFIELD_MANAGER_ADVANCE_STATE",
	OpcodeCMSG_BATTLEFIELD_MANAGER_SET_NEXT_TRANSITION_TIME:     "CMSG_BATTLEFIELD_MANAGER_SET_NEXT_TRANSITION_TIME",
	OpcodeCMSG_BATTLEFIELD_MGR_ENTRY_INVITE_RESPONSE:            "CMSG_BATTLEFIELD_MGR_ENTRY_INVITE_RESPONSE",
	OpcodeCMSG_BATTLEFIELD_MGR_EXIT_REQUEST:                     "CMSG_BATTLEFIELD_MGR_EXIT_REQUEST",
	OpcodeCMSG_BATTLEFIELD_MGR_QUEUE_INVITE_RESPONSE:            "CMSG_BATTLEFIELD_MGR_QUEUE_INVITE_RESPONSE",
	OpcodeCMSG_BATTLEFIELD_MGR_QUEUE_REQUEST:                    "CMSG_BATTLEFIELD_MGR_QUEUE_REQUEST",
	OpcodeCMSG_BATTLEFIELD_PORT:                                 "CMSG_BATTLEFIELD_PORT",
	OpcodeCMSG_BATTLEFIELD_STATUS:                               "CMSG_BATTLEFIELD_STATUS",
	OpcodeCMSG_BATTLEMASTER_HELLO:                               "CMSG_BATTLEMASTER_HELLO",
	OpcodeCMSG_BATTLEMASTER_JOIN:                                "CMSG_BATTLEMASTER_JOIN",
	OpcodeCMSG_BATTLEMASTER_JOIN_ARENA:                          "CMSG_BATTLEMASTER_JOIN_ARENA",
	OpcodeCMSG_BEASTMASTER:                                      "CMSG_BEASTMASTER",
	OpcodeCMSG_BEGIN_TRADE:                                      "CMSG_BEGIN_TRADE",
	OpcodeCMSG_BINDER_ACTIVATE:                                  "CMSG_BINDER_ACTIVATE",
	OpcodeCMSG_BOOTME:                                           "CMSG_BOOTME",
	OpcodeCMSG_BOT_DETECTED:                                     "CMSG_BOT_DETECTED",
	OpcodeCMSG_BOT_DETECTED2:                                    "CMSG_BOT_DETECTED2",
	OpcodeCMSG_BUG:                                              "CMSG_BUG",
	OpcodeCMSG_BUSY_TRADE:                                       "CMSG_BUSY_TRADE",
	OpcodeCMSG_BUYBACK_ITEM:                                     "CMSG_BUYBACK_ITEM",
	OpcodeCMSG_BUY_BANK_SLOT:                                    "CMSG_BUY_BANK_SLOT",
	OpcodeCMSG_BUY_ITEM:                                         "CMSG_BUY_ITEM",
	OpcodeCMSG_BUY_ITEM_IN_SLOT:                                 "CMSG_BUY_ITEM_IN_SLOT",
	OpcodeCMSG_BUY_LOTTERY_TICKET_OBSOLETE:                      "CMSG_BUY_LOTTERY_TICKET_OBSOLETE",
	OpcodeCMSG_BUY_STABLE_SLOT:                                  "CMSG_BUY_STABLE_SLOT",
	OpcodeCMSG_CALENDAR_ADD_EVENT:                               "CMSG_CALENDAR_ADD_EVENT",
	OpcodeCMSG_CALENDAR_ARENA_TEAM:                              "CMSG_CALENDAR_ARENA_TEAM",
	OpcodeCMSG_CALENDAR_COMPLAIN:                                "CMSG_CALENDAR_COMPLAIN",
	OpcodeCMSG_CALENDAR_COPY_EVENT:                              "CMSG_CALENDAR_COPY_EVENT",
	OpcodeCMSG_CALENDAR_EVENT_INVITE:                            "CMSG_CALENDAR_EVENT_INVITE",
	OpcodeCMSG_CALENDAR_EVENT_INVITE_NOTES:                      "CMSG_CALENDAR_EVENT_INVITE_NOTES",
	OpcodeCMSG_CALENDAR_EVENT_MODERATOR_STATUS:                  "CMSG_CALENDAR_EVENT_MODERATOR_STATUS",
	OpcodeCMSG_CALENDAR_EVENT_REMOVE_INVITE:                     "CMSG_CALENDAR_EVENT_REMOVE_INVITE",
	OpcodeCMSG_CALENDAR_EVENT_RSVP:                              "CMSG_CALENDAR_EVENT_RSVP",
	OpcodeCMSG_CALENDAR_EVENT_SIGNUP:                            "CMSG_CALENDAR_EVENT_SIGNUP",
	OpcodeCMSG_CALENDAR_EVENT_STATUS:                            "CMSG_CALENDAR_EVENT_STATUS",
	OpcodeCMSG_CALENDAR_GET_CALENDAR:                            "CMSG_CALENDAR_GET_CALENDAR",
	OpcodeCMSG_CALENDAR_GET_EVENT:                               "CMSG_CALENDAR_GET_EVENT",
	OpcodeCMSG_CALENDAR_GET_NUM_PENDING:                         "CMSG_CALENDAR_GET_NUM_PENDING",
	OpcodeCMSG_CALENDAR_GUILD_FILTER:                            "CMSG_CALENDAR_GUILD_FILTER",
	OpcodeCMSG_CALENDAR_REMOVE_EVENT:                            "CMSG_CALENDAR_REMOVE_EVENT",
	OpcodeCMSG_CALENDAR_UPDATE_EVENT:                            "CMSG_CALENDAR_UPDATE_EVENT",
	OpcodeCMSG_CANCEL_AURA:                                      "CMSG_CANCEL_AURA",
	OpcodeCMSG_CANCEL_AUTO_REPEAT_SPELL:                         "CMSG_CANCEL_AUTO_REPEAT_SPELL",
	OpcodeCMSG_CANCEL_CAST:                                      "CMSG_CANCEL_CAST",
	OpcodeCMSG_CANCEL_CHANNELLING:                               "CMSG_CANCEL_CHANNELLING",
	OpcodeCMSG_CANCEL_GROWTH_AURA:                               "CMSG_CANCEL_GROWTH_AURA",
	OpcodeCMSG_CANCEL_MOUNT_AURA:                                "CMSG_CANCEL_MOUNT_AURA",
	OpcodeCMSG_CANCEL_TEMP_ENCHANTMENT:                          "CMSG_CANCEL_TEMP_ENCHANTMENT",
	OpcodeCMSG_CANCEL_TRADE:                                     "CMSG_CANCEL_TRADE",
	OpcodeCMSG_CAST_SPELL:                                       "CMSG_CAST_SPELL",
	OpcodeCMSG_CHANGEPLAYER_DIFFICULTY:                          "CMSG_CHANGEPLAYER_DIFFICULTY",
	OpcodeCMSG_CHANGE_GDF_ARENA_RATING:                          "CMSG_CHANGE_GDF_ARENA_RATING",
	OpcodeCMSG_CHANGE_PERSONAL_ARENA_RATING:                     "CMSG_CHANGE_PERSONAL_ARENA_RATING",
	OpcodeCMSG_CHANGE_SEATS_ON_CONTROLLED_VEHICLE:               "CMSG_CHANGE_SEATS_ON_CONTROLLED_VEHICLE",
	OpcodeCMSG_CHANNEL_ANNOUNCEMENTS:                            "CMSG_CHANNEL_ANNOUNCEMENTS",
	OpcodeCMSG_CHANNEL_BAN:                                      "CMSG_CHANNEL_BAN",
	OpcodeCMSG_CHANNEL_DISPLAY_LIST:                             "CMSG_CHANNEL_DISPLAY_LIST",
	OpcodeCMSG_CHANNEL_INVITE:                                   "CMSG_CHANNEL_INVITE",
	OpcodeCMSG_CHANNEL_KICK:                                     "CMSG_CHANNEL_KICK",
	OpcodeCMSG_CHANNEL_LIST:                                     "CMSG_CHANNEL_LIST",
	OpcodeCMSG_CHANNEL_MODERATE:                                 "CMSG_CHANNEL_MODERATE",
	OpcodeCMSG_CHANNEL_MODERATOR:                                "CMSG_CHANNEL_MODERATOR",
	OpcodeCMSG_CHANNEL_MUTE:                                     "CMSG_CHANNEL_MUTE",
	OpcodeCMSG_CHANNEL_OWNER:                                    "CMSG_CHANNEL_OWNER",
	OpcodeCMSG_CHANNEL_PASSWORD:                                 "CMSG_CHANNEL_PASSWORD",
	OpcodeCMSG_CHANNEL_SET_OWNER:                                "CMSG_CHANNEL_SET_OWNER",
	OpcodeCMSG_CHANNEL_SILENCE_ALL:                              "CMSG_CHANNEL_SILENCE_ALL",
	OpcodeCMSG_CHANNEL_SILENCE_VOICE:                            "CMSG_CHANNEL_SILENCE_VOICE",
	OpcodeCMSG_CHANNEL_UNBAN:                                    "CMSG_CHANNEL_UNBAN",
	OpcodeCMSG_CHANNEL_UNMODERATOR:                              "CMSG_CHANNEL_UNMODERATOR",
	OpcodeCMSG_CHANNEL_UNMUTE:                                   "CMSG_CHANNEL_UNMUTE",
	OpcodeCMSG_CHANNEL_UNSILENCE_ALL:                            "CMSG_CHANNEL_UNSILENCE_ALL",
	OpcodeCMSG_CHANNEL_UNSILENCE_VOICE:                          "CMSG_CHANNEL_UNSILENCE_VOICE",
	OpcodeCMSG_CHANNEL_VOICE_OFF:                                "CMSG_CHANNEL_VOICE_OFF",
	OpcodeCMSG_CHANNEL_VOICE_ON:                                 "CMSG_CHANNEL_VOICE_ON",
	OpcodeCMSG_CHARACTER_POINT_CHEAT:                            "CMSG_CHARACTER_POINT_CHEAT",
	OpcodeCMSG_CHAR_CREATE:                                      "CMSG_CHAR_CREATE",
	OpcodeCMSG_CHAR_CUSTOMIZE:                                   "CMSG_CHAR_CUSTOMIZE",
	OpcodeCMSG_CHAR_DELETE:                                      "CMSG_CHAR_DELETE",
	OpcodeCMSG_CHAR_ENUM:                                        "CMSG_CHAR_ENUM",
	OpcodeCMSG_CHAR_FACTION_CHANGE:                              "CMSG_CHAR_FACTION_CHANGE",
	OpcodeCMSG_CHAR_RACE_CHANGE:                                 "CMSG_CHAR_RACE_CHANGE",
	OpcodeCMSG_CHAR_RENAME:                                      "CMSG_CHAR_RENAME",
	OpcodeCMSG_CHAT_FILTERED:                                    "CMSG_CHAT_FILTERED",
	OpcodeCMSG_CHAT_IGNORED:                                     "CMSG_CHAT_IGNORED",
	OpcodeCMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY:                      "CMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY",
	OpcodeCMSG_CHEAT_PLAYER_LOGIN:                               "CMSG_CHEAT_PLAYER_LOGIN",
	OpcodeCMSG_CHEAT_PLAYER_LOOKUP:                              "CMSG_CHEAT_PLAYER_LOOKUP",
	OpcodeCMSG_CHEAT_SETMONEY:                                   "CMSG_CHEAT_SETMONEY",
	OpcodeCMSG_CHEAT_SET_ARENA_CURRENCY:                         "CMSG_CHEAT_SET_ARENA_CURRENCY",
	OpcodeCMSG_CHEAT_SET_HONOR_CURRENCY:                         "CMSG_CHEAT_SET_HONOR_CURRENCY",
	OpcodeCMSG_CHECK_LOGIN_CRITERIA:                             "CMSG_CHECK_LOGIN_CRITERIA",
	OpcodeCMSG_CLEAR_CHANNEL_WATCH:                              "CMSG_CLEAR_CHANNEL_WATCH",
	OpcodeCMSG_CLEAR_EXPLORATION:                                "CMSG_CLEAR_EXPLORATION",
	OpcodeCMSG_CLEAR_HOLIDAY_BG_WIN_TIME:                        "CMSG_CLEAR_HOLIDAY_BG_WIN_TIME",
	OpcodeCMSG_CLEAR_QUEST:                                      "CMSG_CLEAR_QUEST",
	OpcodeCMSG_CLEAR_RANDOM_BG_WIN_TIME:                         "CMSG_CLEAR_RANDOM_BG_WIN_TIME",
	OpcodeCMSG_CLEAR_SERVER_BUCK_DATA:                           "CMSG_CLEAR_SERVER_BUCK_DATA",
	OpcodeCMSG_CLEAR_TRADE_ITEM:                                 "CMSG_CLEAR_TRADE_ITEM",
	OpcodeCMSG_COMMENTATOR_ENABLE:                               "CMSG_COMMENTATOR_ENABLE",
	OpcodeCMSG_COMMENTATOR_ENTER_INSTANCE:                       "CMSG_COMMENTATOR_ENTER_INSTANCE",
	OpcodeCMSG_COMMENTATOR_EXIT_INSTANCE:                        "CMSG_COMMENTATOR_EXIT_INSTANCE",
	OpcodeCMSG_COMMENTATOR_GET_MAP_INFO:                         "CMSG_COMMENTATOR_GET_MAP_INFO",
	OpcodeCMSG_COMMENTATOR_GET_PLAYER_INFO:                      "CMSG_COMMENTATOR_GET_PLAYER_INFO",
	OpcodeCMSG_COMMENTATOR_INSTANCE_COMMAND:                     "CMSG_COMMENTATOR_INSTANCE_COMMAND",
	OpcodeCMSG_COMMENTATOR_SKIRMISH_QUEUE_COMMAND:               "CMSG_COMMENTATOR_SKIRMISH_QUEUE_COMMAND",
	OpcodeCMSG_COMPLAIN:                                         "CMSG_COMPLAIN",
	OpcodeCMSG_COMPLETE_ACHIEVEMENT_CHEAT:                       "CMSG_COMPLETE_ACHIEVEMENT_CHEAT",
	OpcodeCMSG_COMPLETE_CINEMATIC:                               "CMSG_COMPLETE_CINEMATIC",
	OpcodeCMSG_COMPLETE_MOVIE:                                   "CMSG_COMPLETE_MOVIE",
	OpcodeCMSG_CONTACT_LIST:                                     "CMSG_CONTACT_LIST",
	OpcodeCMSG_CONTROLLER_EJECT_PASSENGER:                       "CMSG_CONTROLLER_EJECT_PASSENGER",
	OpcodeCMSG_COOLDOWN_CHEAT:                                   "CMSG_COOLDOWN_CHEAT",
	OpcodeCMSG_CORPSE_MAP_POSITION_QUERY:                        "CMSG_CORPSE_MAP_POSITION_QUERY",
	OpcodeCMSG_CREATEGAMEOBJECT:                                 "CMSG_CREATEGAMEOBJECT",
	OpcodeCMSG_CREATEITEM:                                       "CMSG_CREATEITEM",
	OpcodeCMSG_CREATEMONSTER:                                    "CMSG_CREATEMONSTER",
	OpcodeCMSG_CREATURE_QUERY:                                   "CMSG_CREATURE_QUERY",
	OpcodeCMSG_DANCE_QUERY:                                      "CMSG_DANCE_QUERY",
	OpcodeCMSG_DBLOOKUP:                                         "CMSG_DBLOOKUP",
	OpcodeCMSG_DEBUG_ACTIONS_START:                              "CMSG_DEBUG_ACTIONS_START",
	OpcodeCMSG_DEBUG_ACTIONS_STOP:                               "CMSG_DEBUG_ACTIONS_STOP",
	OpcodeCMSG_DEBUG_AISTATE:                                    "CMSG_DEBUG_AISTATE",
	OpcodeCMSG_DEBUG_CHANGECELLZONE:                             "CMSG_DEBUG_CHANGECELLZONE",
	OpcodeCMSG_DEBUG_LIST_TARGETS:                               "CMSG_DEBUG_LIST_TARGETS",
	OpcodeCMSG_DEBUG_PASSIVE_AURA:                               "CMSG_DEBUG_PASSIVE_AURA",
	OpcodeCMSG_DEBUG_SERVER_GEO:                                 "CMSG_DEBUG_SERVER_GEO",
	OpcodeCMSG_DECHARGE:                                         "CMSG_DECHARGE",
	OpcodeCMSG_DECLINE_CHANNEL_INVITE:                           "CMSG_DECLINE_CHANNEL_INVITE",
	OpcodeCMSG_DELETEEQUIPMENT_SET:                              "CMSG_DELETEEQUIPMENT_SET",
	OpcodeCMSG_DELETE_DANCE:                                     "CMSG_DELETE_DANCE",
	OpcodeCMSG_DEL_FRIEND:                                       "CMSG_DEL_FRIEND",
	OpcodeCMSG_DEL_IGNORE:                                       "CMSG_DEL_IGNORE",
	OpcodeCMSG_DEL_PVP_MEDAL_CHEAT:                              "CMSG_DEL_PVP_MEDAL_CHEAT",
	OpcodeCMSG_DEL_VOICE_IGNORE:                                 "CMSG_DEL_VOICE_IGNORE",
	OpcodeCMSG_DESTROYITEM:                                      "CMSG_DESTROYITEM",
	OpcodeCMSG_DESTROYMONSTER:                                   "CMSG_DESTROYMONSTER",
	OpcodeCMSG_DESTROY_ITEMS:                                    "CMSG_DESTROY_ITEMS",
	OpcodeCMSG_DISABLE_PVP_CHEAT:                                "CMSG_DISABLE_PVP_CHEAT",
	OpcodeCMSG_DISMISS_CONTROLLED_VEHICLE:                       "CMSG_DISMISS_CONTROLLED_VEHICLE",
	OpcodeCMSG_DISMISS_CRITTER:                                  "CMSG_DISMISS_CRITTER",
	OpcodeCMSG_DROP_NEW_CONNECTION:                              "CMSG_DROP_NEW_CONNECTION",
	OpcodeCMSG_DUEL_ACCEPTED:                                    "CMSG_DUEL_ACCEPTED",
	OpcodeCMSG_DUEL_CANCELLED:                                   "CMSG_DUEL_CANCELLED",
	OpcodeCMSG_DUMP_OBJECTS:                                     "CMSG_DUMP_OBJECTS",
	OpcodeCMSG_EMOTE:                                            "CMSG_EMOTE",
	OpcodeCMSG_ENABLETAXI:                                       "CMSG_ENABLETAXI",
	OpcodeCMSG_ENABLE_DAMAGE_LOG:                                "CMSG_ENABLE_DAMAGE_LOG",
	OpcodeCMSG_END_BATTLEFIELD_CHEAT:                            "CMSG_END_BATTLEFIELD_CHEAT",
	OpcodeCMSG_EQUIPMENT_SET_SAVE:                               "CMSG_EQUIPMENT_SET_SAVE",
	OpcodeCMSG_EQUIPMENT_SET_USE:                                "CMSG_EQUIPMENT_SET_USE",
	OpcodeCMSG_EXPIRE_RAID_INSTANCE:                             "CMSG_EXPIRE_RAID_INSTANCE",
	OpcodeCMSG_FAR_SIGHT:                                        "CMSG_FAR_SIGHT",
	OpcodeCMSG_FLAG_QUEST:                                       "CMSG_FLAG_QUEST",
	OpcodeCMSG_FLAG_QUEST_FINISH:                                "CMSG_FLAG_QUEST_FINISH",
	OpcodeCMSG_FLOOD_GRACE_CHEAT:                                "CMSG_FLOOD_GRACE_CHEAT",
	OpcodeCMSG_FORCEACTION:                                      "CMSG_FORCEACTION",
	OpcodeCMSG_FORCEACTIONONOTHER:                               "CMSG_FORCEACTIONONOTHER",
	OpcodeCMSG_FORCEACTIONSHOW:                                  "CMSG_FORCEACTIONSHOW",
	OpcodeCMSG_FORCE_ANIM:                                       "CMSG_FORCE_ANIM",
	OpcodeCMSG_FORCE_FLIGHT_BACK_SPEED_CHANGE_ACK:               "CMSG_FORCE_FLIGHT_BACK_SPEED_CHANGE_ACK",
	OpcodeCMSG_FORCE_FLIGHT_SPEED_CHANGE_ACK:                    "CMSG_FORCE_FLIGHT_SPEED_CHANGE_ACK",
	OpcodeCMSG_FORCE_MOVE_ROOT_ACK:                              "CMSG_FORCE_MOVE_ROOT_ACK",
	OpcodeCMSG_FORCE_MOVE_UNROOT_ACK:                            "CMSG_FORCE_MOVE_UNROOT_ACK",
	OpcodeCMSG_FORCE_PITCH_RATE_CHANGE_ACK:                      "CMSG_FORCE_PITCH_RATE_CHANGE_ACK",
	OpcodeCMSG_FORCE_RUN_BACK_SPEED_CHANGE_ACK:                  "CMSG_FORCE_RUN_BACK_SPEED_CHANGE_ACK",
	OpcodeCMSG_FORCE_RUN_SPEED_CHANGE_ACK:                       "CMSG_FORCE_RUN_SPEED_CHANGE_ACK",
	OpcodeCMSG_FORCE_SAY_CHEAT:                                  "CMSG_FORCE_SAY_CHEAT",
	OpcodeCMSG_FORCE_SWIM_BACK_SPEED_CHANGE_ACK:                 "CMSG_FORCE_SWIM_BACK_SPEED_CHANGE_ACK",
	OpcodeCMSG_FORCE_SWIM_SPEED_CHANGE_ACK:                      "CMSG_FORCE_SWIM_SPEED_CHANGE_ACK",
	OpcodeCMSG_FORCE_TURN_RATE_CHANGE_ACK:                       "CMSG_FORCE_TURN_RATE_CHANGE_ACK",
	OpcodeCMSG_FORCE_WALK_SPEED_CHANGE_ACK:                      "CMSG_FORCE_WALK_SPEED_CHANGE_ACK",
	OpcodeCMSG_GAMEOBJECT_QUERY:                                 "CMSG_GAMEOBJECT_QUERY",
	OpcodeCMSG_GAMEOBJ_REPORT_USE:                               "CMSG_GAMEOBJ_REPORT_USE",
	OpcodeCMSG_GAMEOBJ_USE:                                      "CMSG_GAMEOBJ_USE",
	OpcodeCMSG_GAMESPEED_SET:                                    "CMSG_GAMESPEED_SET",
	OpcodeCMSG_GAMETIME_SET:                                     "CMSG_GAMETIME_SET",
	OpcodeCMSG_GETDEATHBINDZONE:                                 "CMSG_GETDEATHBINDZONE",
	OpcodeCMSG_GET_CHANNEL_MEMBER_COUNT:                         "CMSG_GET_CHANNEL_MEMBER_COUNT",
	OpcodeCMSG_GET_MAIL_LIST:                                    "CMSG_GET_MAIL_LIST",
	OpcodeCMSG_GET_MIRRORIMAGE_DATA:                             "CMSG_GET_MIRRORIMAGE_DATA",
	OpcodeCMSG_GHOST:                                            "CMSG_GHOST",
	OpcodeCMSG_GMRESPONSE_CREATE_TICKET:                         "CMSG_GMRESPONSE_CREATE_TICKET",
	OpcodeCMSG_GMRESPONSE_RESOLVE:                               "CMSG_GMRESPONSE_RESOLVE",
	OpcodeCMSG_GMSURVEY_SUBMIT:                                  "CMSG_GMSURVEY_SUBMIT",
	OpcodeCMSG_GMTICKETSYSTEM_TOGGLE:                            "CMSG_GMTICKETSYSTEM_TOGGLE",
	OpcodeCMSG_GMTICKET_CREATE:                                  "CMSG_GMTICKET_CREATE",
	OpcodeCMSG_GMTICKET_DELETETICKET:                            "CMSG_GMTICKET_DELETETICKET",
	OpcodeCMSG_GMTICKET_GETTICKET:                               "CMSG_GMTICKET_GETTICKET",
	OpcodeCMSG_GMTICKET_SYSTEMSTATUS:                            "CMSG_GMTICKET_SYSTEMSTATUS",
	OpcodeCMSG_GMTICKET_UPDATETEXT:                              "CMSG_GMTICKET_UPDATETEXT",
	OpcodeCMSG_GM_CHARACTER_RESTORE:                             "CMSG_GM_CHARACTER_RESTORE",
	OpcodeCMSG_GM_CHARACTER_SAVE:                                "CMSG_GM_CHARACTER_SAVE",
	OpcodeCMSG_GM_CREATE_ITEM_TARGET:                            "CMSG_GM_CREATE_ITEM_TARGET",
	OpcodeCMSG_GM_DESTROY_ONLINE_CORPSE:                         "CMSG_GM_DESTROY_ONLINE_CORPSE",
	OpcodeCMSG_GM_FREEZE:                                        "CMSG_GM_FREEZE",
	OpcodeCMSG_GM_GRANT_ACHIEVEMENT:                             "CMSG_GM_GRANT_ACHIEVEMENT",
	OpcodeCMSG_GM_INVIS:                                         "CMSG_GM_INVIS",
	OpcodeCMSG_GM_MOVECORPSE:                                    "CMSG_GM_MOVECORPSE",
	OpcodeCMSG_GM_NUKE:                                          "CMSG_GM_NUKE",
	OpcodeCMSG_GM_NUKE_ACCOUNT:                                  "CMSG_GM_NUKE_ACCOUNT",
	OpcodeCMSG_GM_NUKE_CHARACTER:                                "CMSG_GM_NUKE_CHARACTER",
	OpcodeCMSG_GM_REMOVE_ACHIEVEMENT:                            "CMSG_GM_REMOVE_ACHIEVEMENT",
	OpcodeCMSG_GM_REPORT_LAG:                                    "CMSG_GM_REPORT_LAG",
	OpcodeCMSG_GM_REQUEST_PLAYER_INFO:                           "CMSG_GM_REQUEST_PLAYER_INFO",
	OpcodeCMSG_GM_RESURRECT:                                     "CMSG_GM_RESURRECT",
	OpcodeCMSG_GM_REVEALTO:                                      "CMSG_GM_REVEALTO",
	OpcodeCMSG_GM_SET_CRITERIA_FOR_PLAYER:                       "CMSG_GM_SET_CRITERIA_FOR_PLAYER",
	OpcodeCMSG_GM_SET_SECURITY_GROUP:                            "CMSG_GM_SET_SECURITY_GROUP",
	OpcodeCMSG_GM_SHOW_COMPLAINTS:                               "CMSG_GM_SHOW_COMPLAINTS",
	OpcodeCMSG_GM_SILENCE:                                       "CMSG_GM_SILENCE",
	OpcodeCMSG_GM_SUMMONMOB:                                     "CMSG_GM_SUMMONMOB",
	OpcodeCMSG_GM_TEACH:                                         "CMSG_GM_TEACH",
	OpcodeCMSG_GM_UBERINVIS:                                     "CMSG_GM_UBERINVIS",
	OpcodeCMSG_GM_UNSQUELCH:                                     "CMSG_GM_UNSQUELCH",
	OpcodeCMSG_GM_UNTEACH:                                       "CMSG_GM_UNTEACH",
	OpcodeCMSG_GM_UPDATE_TICKET_STATUS:                          "CMSG_GM_UPDATE_TICKET_STATUS",
	OpcodeCMSG_GM_VISION:                                        "CMSG_GM_VISION",
	OpcodeCMSG_GM_WHISPER:                                       "CMSG_GM_WHISPER",
	OpcodeCMSG_GODMODE:                                          "CMSG_GODMODE",
	OpcodeCMSG_GOSSIP_HELLO:                                     "CMSG_GOSSIP_HELLO",
	OpcodeCMSG_GOSSIP_SELECT_OPTION:                             "CMSG_GOSSIP_SELECT_OPTION",
	OpcodeCMSG_GRANT_LEVEL:                                      "CMSG_GRANT_LEVEL",
	OpcodeCMSG_GROUP_ACCEPT:                                     "CMSG_GROUP_ACCEPT",
	OpcodeCMSG_GROUP_ASSISTANT_LEADER:                           "CMSG_GROUP_ASSISTANT_LEADER",
	OpcodeCMSG_GROUP_CANCEL:                                     "CMSG_GROUP_CANCEL",
	OpcodeCMSG_GROUP_CHANGE_SUB_GROUP:                           "CMSG_GROUP_CHANGE_SUB_GROUP",
	OpcodeCMSG_GROUP_DECLINE:                                    "CMSG_GROUP_DECLINE",
	OpcodeCMSG_GROUP_DISBAND:                                    "CMSG_GROUP_DISBAND",
	OpcodeCMSG_GROUP_INVITE:                                     "CMSG_GROUP_INVITE",
	OpcodeCMSG_GROUP_RAID_CONVERT:                               "CMSG_GROUP_RAID_CONVERT",
	OpcodeCMSG_GROUP_SET_LEADER:                                 "CMSG_GROUP_SET_LEADER",
	OpcodeCMSG_GROUP_SWAP_SUB_GROUP:                             "CMSG_GROUP_SWAP_SUB_GROUP",
	OpcodeCMSG_GROUP_UNINVITE:                                   "CMSG_GROUP_UNINVITE",
	OpcodeCMSG_GROUP_UNINVITE_GUID:                              "CMSG_GROUP_UNINVITE_GUID",
	OpcodeCMSG_GUILD_ACCEPT:                                     "CMSG_GUILD_ACCEPT",
	OpcodeCMSG_GUILD_ADD_RANK:                                   "CMSG_GUILD_ADD_RANK",
	OpcodeCMSG_GUILD_BANKER_ACTIVATE:                            "CMSG_GUILD_BANKER_ACTIVATE",
	OpcodeCMSG_GUILD_BANK_BUY_TAB:                               "CMSG_GUILD_BANK_BUY_TAB",
	OpcodeCMSG_GUILD_BANK_DEPOSIT_MONEY:                         "CMSG_GUILD_BANK_DEPOSIT_MONEY",
	OpcodeCMSG_GUILD_BANK_QUERY_TAB:                             "CMSG_GUILD_BANK_QUERY_TAB",
	OpcodeCMSG_GUILD_BANK_SWAP_ITEMS:                            "CMSG_GUILD_BANK_SWAP_ITEMS",
	OpcodeCMSG_GUILD_BANK_UPDATE_TAB:                            "CMSG_GUILD_BANK_UPDATE_TAB",
	OpcodeCMSG_GUILD_BANK_WITHDRAW_MONEY:                        "CMSG_GUILD_BANK_WITHDRAW_MONEY",
	OpcodeCMSG_GUILD_CREATE:                                     "CMSG_GUILD_CREATE",
	OpcodeCMSG_GUILD_DECLINE:                                    "CMSG_GUILD_DECLINE",
	OpcodeCMSG_GUILD_DEL_RANK:                                   "CMSG_GUILD_DEL_RANK",
	OpcodeCMSG_GUILD_DEMOTE:                                     "CMSG_GUILD_DEMOTE",
	OpcodeCMSG_GUILD_DISBAND:                                    "CMSG_GUILD_DISBAND",
	OpcodeCMSG_GUILD_INFO:                                       "CMSG_GUILD_INFO",
	OpcodeCMSG_GUILD_INFO_TEXT:                                  "CMSG_GUILD_INFO_TEXT",
	OpcodeCMSG_GUILD_INVITE:                                     "CMSG_GUILD_INVITE",
	OpcodeCMSG_GUILD_LEADER:                                     "CMSG_GUILD_LEADER",
	OpcodeCMSG_GUILD_LEAVE:                                      "CMSG_GUILD_LEAVE",
	OpcodeCMSG_GUILD_MOTD:                                       "CMSG_GUILD_MOTD",
	OpcodeCMSG_GUILD_PROMOTE:                                    "CMSG_GUILD_PROMOTE",
	OpcodeCMSG_GUILD_QUERY:                                      "CMSG_GUILD_QUERY",
	OpcodeCMSG_GUILD_RANK:                                       "CMSG_GUILD_RANK",
	OpcodeCMSG_GUILD_REMOVE:                                     "CMSG_GUILD_REMOVE",
	OpcodeCMSG_GUILD_ROSTER:                                     "CMSG_GUILD_ROSTER",
	OpcodeCMSG_GUILD_SET_OFFICER_NOTE:                           "CMSG_GUILD_SET_OFFICER_NOTE",
	OpcodeCMSG_GUILD_SET_PUBLIC_NOTE:                            "CMSG_GUILD_SET_PUBLIC_NOTE",
	OpcodeCMSG_HEARTH_AND_RESURRECT:                             "CMSG_HEARTH_AND_RESURRECT",
	OpcodeCMSG_IGNORE_DIMINISHING_RETURNS_CHEAT:                 "CMSG_IGNORE_DIMINISHING_RETURNS_CHEAT",
	OpcodeCMSG_IGNORE_KNOCKBACK_CHEAT:                           "CMSG_IGNORE_KNOCKBACK_CHEAT",
	OpcodeCMSG_IGNORE_REQUIREMENTS_CHEAT:                        "CMSG_IGNORE_REQUIREMENTS_CHEAT",
	OpcodeCMSG_IGNORE_TRADE:                                     "CMSG_IGNORE_TRADE",
	OpcodeCMSG_INITIATE_TRADE:                                   "CMSG_INITIATE_TRADE",
	OpcodeCMSG_INSPECT:                                          "CMSG_INSPECT",
	OpcodeCMSG_INSTANCE_LOCK_RESPONSE:                           "CMSG_INSTANCE_LOCK_RESPONSE",
	OpcodeCMSG_ITEM_NAME_QUERY:                                  "CMSG_ITEM_NAME_QUERY",
	OpcodeCMSG_ITEM_QUERY_MULTIPLE:                              "CMSG_ITEM_QUERY_MULTIPLE",
	OpcodeCMSG_ITEM_QUERY_SINGLE:                                "CMSG_ITEM_QUERY_SINGLE",
	OpcodeCMSG_ITEM_REFUND:                                      "CMSG_ITEM_REFUND",
	OpcodeCMSG_ITEM_REFUND_INFO:                                 "CMSG_ITEM_REFUND_INFO",
	OpcodeCMSG_ITEM_TEXT_QUERY:                                  "CMSG_ITEM_TEXT_QUERY",
	OpcodeCMSG_JOIN_CHANNEL:                                     "CMSG_JOIN_CHANNEL",
	OpcodeCMSG_KEEP_ALIVE:                                       "CMSG_KEEP_ALIVE",
	OpcodeCMSG_LEARN_DANCE_MOVE:                                 "CMSG_LEARN_DANCE_MOVE",
	OpcodeCMSG_LEARN_PREVIEW_TALENTS:                            "CMSG_LEARN_PREVIEW_TALENTS",
	OpcodeCMSG_LEARN_PREVIEW_TALENTS_PET:                        "CMSG_LEARN_PREVIEW_TALENTS_PET",
	OpcodeCMSG_LEARN_SPELL:                                      "CMSG_LEARN_SPELL",
	OpcodeCMSG_LEARN_TALENT:                                     "CMSG_LEARN_TALENT",
	OpcodeCMSG_LEAVE_BATTLEFIELD:                                "CMSG_LEAVE_BATTLEFIELD",
	OpcodeCMSG_LEAVE_CHANNEL:                                    "CMSG_LEAVE_CHANNEL",
	OpcodeCMSG_LEVEL_CHEAT:                                      "CMSG_LEVEL_CHEAT",
	OpcodeCMSG_LFD_PARTY_LOCK_INFO_REQUEST:                      "CMSG_LFD_PARTY_LOCK_INFO_REQUEST",
	OpcodeCMSG_LFD_PLAYER_LOCK_INFO_REQUEST:                     "CMSG_LFD_PLAYER_LOCK_INFO_REQUEST",
	OpcodeCMSG_LFG_GET_STATUS:                                   "CMSG_LFG_GET_STATUS",
	OpcodeCMSG_LFG_JOIN:                                         "CMSG_LFG_JOIN",
	OpcodeCMSG_LFG_LEAVE:                                        "CMSG_LFG_LEAVE",
	OpcodeCMSG_LFG_PROPOSAL_RESULT:                              "CMSG_LFG_PROPOSAL_RESULT",
	OpcodeCMSG_LFG_SET_BOOT_VOTE:                                "CMSG_LFG_SET_BOOT_VOTE",
	OpcodeCMSG_LFG_SET_NEEDS:                                    "CMSG_LFG_SET_NEEDS",
	OpcodeCMSG_LFG_SET_ROLES:                                    "CMSG_LFG_SET_ROLES",
	OpcodeCMSG_LFG_TELEPORT:                                     "CMSG_LFG_TELEPORT",
	OpcodeCMSG_LIST_INVENTORY:                                   "CMSG_LIST_INVENTORY",
	OpcodeCMSG_LOAD_DANCES:                                      "CMSG_LOAD_DANCES",
	OpcodeCMSG_LOGOUT_CANCEL:                                    "CMSG_LOGOUT_CANCEL",
	OpcodeCMSG_LOGOUT_REQUEST:                                   "CMSG_LOGOUT_REQUEST",
	OpcodeCMSG_LOOT:                                             "CMSG_LOOT",
	OpcodeCMSG_LOOT_MASTER_GIVE:                                 "CMSG_LOOT_MASTER_GIVE",
	OpcodeCMSG_LOOT_METHOD:                                      "CMSG_LOOT_METHOD",
	OpcodeCMSG_LOOT_MONEY:                                       "CMSG_LOOT_MONEY",
	OpcodeCMSG_LOOT_RELEASE:                                     "CMSG_LOOT_RELEASE",
	OpcodeCMSG_LOOT_ROLL:                                        "CMSG_LOOT_ROLL",
	OpcodeCMSG_LOTTERY_QUERY_OBSOLETE:                           "CMSG_LOTTERY_QUERY_OBSOLETE",
	OpcodeCMSG_LUA_USAGE:                                        "CMSG_LUA_USAGE",
	OpcodeCMSG_MAELSTROM_GM_SENT_MAIL:                           "CMSG_MAELSTROM_GM_SENT_MAIL",
	OpcodeCMSG_MAELSTROM_INVALIDATE_CACHE:                       "CMSG_MAELSTROM_INVALIDATE_CACHE",
	OpcodeCMSG_MAELSTROM_RENAME_GUILD:                           "CMSG_MAELSTROM_RENAME_GUILD",
	OpcodeCMSG_MAIL_CREATE_TEXT_ITEM:                            "CMSG_MAIL_CREATE_TEXT_ITEM",
	OpcodeCMSG_MAIL_DELETE:                                      "CMSG_MAIL_DELETE",
	OpcodeCMSG_MAIL_MARK_AS_READ:                                "CMSG_MAIL_MARK_AS_READ",
	OpcodeCMSG_MAIL_RETURN_TO_SENDER:                            "CMSG_MAIL_RETURN_TO_SENDER",
	OpcodeCMSG_MAIL_TAKE_ITEM:                                   "CMSG_MAIL_TAKE_ITEM",
	OpcodeCMSG_MAIL_TAKE_MONEY:                                  "CMSG_MAIL_TAKE_MONEY",
	OpcodeCMSG_MAKEMONSTERATTACKGUID:                            "CMSG_MAKEMONSTERATTACKGUID",
	OpcodeCMSG_MESSAGECHAT:                                      "CMSG_MESSAGECHAT",
	OpcodeCMSG_MINIGAME_MOVE:                                    "CMSG_MINIGAME_MOVE",
	OpcodeCMSG_MOUNTSPECIAL_ANIM:                                "CMSG_MOUNTSPECIAL_ANIM",
	OpcodeCMSG_MOVE_CHARACTER_CHEAT:                             "CMSG_MOVE_CHARACTER_CHEAT",
	OpcodeCMSG_MOVE_CHARM_PORT_CHEAT:                            "CMSG_MOVE_CHARM_PORT_CHEAT",
	OpcodeCMSG_MOVE_CHNG_TRANSPORT:                              "CMSG_MOVE_CHNG_TRANSPORT",
	OpcodeCMSG_MOVE_FALL_RESET:                                  "CMSG_MOVE_FALL_RESET",
	OpcodeCMSG_MOVE_FEATHER_FALL_ACK:                            "CMSG_MOVE_FEATHER_FALL_ACK",
	OpcodeCMSG_MOVE_GRAVITY_DISABLE_ACK:                         "CMSG_MOVE_GRAVITY_DISABLE_ACK",
	OpcodeCMSG_MOVE_GRAVITY_ENABLE_ACK:                          "CMSG_MOVE_GRAVITY_ENABLE_ACK",
	OpcodeCMSG_MOVE_HOVER_ACK:                                   "CMSG_MOVE_HOVER_ACK",
	OpcodeCMSG_MOVE_KNOCK_BACK_ACK:                              "CMSG_MOVE_KNOCK_BACK_ACK",
	OpcodeCMSG_MOVE_NOT_ACTIVE_MOVER:                            "CMSG_MOVE_NOT_ACTIVE_MOVER",
	OpcodeCMSG_MOVE_SET_CAN_FLY_ACK:                             "CMSG_MOVE_SET_CAN_FLY_ACK",
	OpcodeCMSG_MOVE_SET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY_ACK: "CMSG_MOVE_SET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY_ACK",
	OpcodeCMSG_MOVE_SET_COLLISION_HGT_ACK:                       "CMSG_MOVE_SET_COLLISION_HGT_ACK",
	OpcodeCMSG_MOVE_SET_FLY:                                     "CMSG_MOVE_SET_FLY",
	OpcodeCMSG_MOVE_SET_RAW_POSITION:                            "CMSG_MOVE_SET_RAW_POSITION",
	OpcodeCMSG_MOVE_SET_RUN_SPEED:                               "CMSG_MOVE_SET_RUN_SPEED",
	OpcodeCMSG_MOVE_SPLINE_DONE:                                 "CMSG_MOVE_SPLINE_DONE",
	OpcodeCMSG_MOVE_START_SWIM_CHEAT:                            "CMSG_MOVE_START_SWIM_CHEAT",
	OpcodeCMSG_MOVE_STOP_SWIM_CHEAT:                             "CMSG_MOVE_STOP_SWIM_CHEAT",
	OpcodeCMSG_MOVE_TIME_SKIPPED:                                "CMSG_MOVE_TIME_SKIPPED",
	OpcodeCMSG_MOVE_WATER_WALK_ACK:                              "CMSG_MOVE_WATER_WALK_ACK",
	OpcodeCMSG_NAME_QUERY:                                       "CMSG_NAME_QUERY",
	OpcodeCMSG_NEW_SPELL_SLOT:                                   "CMSG_NEW_SPELL_SLOT",
	OpcodeCMSG_NEXT_CINEMATIC_CAMERA:                            "CMSG_NEXT_CINEMATIC_CAMERA",
	OpcodeCMSG_NO_SPELL_VARIANCE:                                "CMSG_NO_SPELL_VARIANCE",
	OpcodeCMSG_NPC_TEXT_QUERY:                                   "CMSG_NPC_TEXT_QUERY",
	OpcodeCMSG_OFFER_PETITION:                                   "CMSG_OFFER_PETITION",
	OpcodeCMSG_OPENING_CINEMATIC:                                "CMSG_OPENING_CINEMATIC",
	OpcodeCMSG_OPEN_ITEM:                                        "CMSG_OPEN_ITEM",
	OpcodeCMSG_OPT_OUT_OF_LOOT:                                  "CMSG_OPT_OUT_OF_LOOT",
	OpcodeCMSG_PAGE_TEXT_QUERY:                                  "CMSG_PAGE_TEXT_QUERY",
	OpcodeCMSG_PARTY_SILENCE:                                    "CMSG_PARTY_SILENCE",
	OpcodeCMSG_PARTY_UNSILENCE:                                  "CMSG_PARTY_UNSILENCE",
	OpcodeCMSG_PERFORM_ACTION_SET:                               "CMSG_PERFORM_ACTION_SET",
	OpcodeCMSG_PETGODMODE:                                       "CMSG_PETGODMODE",
	OpcodeCMSG_PETITION_BUY:                                     "CMSG_PETITION_BUY",
	OpcodeCMSG_PETITION_QUERY:                                   "CMSG_PETITION_QUERY",
	OpcodeCMSG_PETITION_SHOWLIST:                                "CMSG_PETITION_SHOWLIST",
	OpcodeCMSG_PETITION_SHOW_SIGNATURES:                         "CMSG_PETITION_SHOW_SIGNATURES",
	OpcodeCMSG_PETITION_SIGN:                                    "CMSG_PETITION_SIGN",
	OpcodeCMSG_PET_ABANDON:                                      "CMSG_PET_ABANDON",
	OpcodeCMSG_PET_ACTION:                                       "CMSG_PET_ACTION",
	OpcodeCMSG_PET_CANCEL_AURA:                                  "CMSG_PET_CANCEL_AURA",
	OpcodeCMSG_PET_CAST_SPELL:                                   "CMSG_PET_CAST_SPELL",
	OpcodeCMSG_PET_LEARN_TALENT:                                 "CMSG_PET_LEARN_TALENT",
	OpcodeCMSG_PET_LEVEL_CHEAT:                                  "CMSG_PET_LEVEL_CHEAT",
	OpcodeCMSG_PET_NAME_QUERY:                                   "CMSG_PET_NAME_QUERY",
	OpcodeCMSG_PET_RENAME:                                       "CMSG_PET_RENAME",
	OpcodeCMSG_PET_SET_ACTION:                                   "CMSG_PET_SET_ACTION",
	OpcodeCMSG_PET_SPELL_AUTOCAST:                               "CMSG_PET_SPELL_AUTOCAST",
	OpcodeCMSG_PET_STOP_ATTACK:                                  "CMSG_PET_STOP_ATTACK",
	OpcodeCMSG_PET_UNLEARN:                                      "CMSG_PET_UNLEARN",
	OpcodeCMSG_PET_UNLEARN_TALENTS:                              "CMSG_PET_UNLEARN_TALENTS",
	OpcodeCMSG_PING:                                             "CMSG_PING",
	OpcodeCMSG_PLAYED_TIME:                                      "CMSG_PLAYED_TIME",
	OpcodeCMSG_PLAYER_AI_CHEAT:                                  "CMSG_PLAYER_AI_CHEAT",
	OpcodeCMSG_PLAYER_LOGIN:                                     "CMSG_PLAYER_LOGIN",
	OpcodeCMSG_PLAYER_LOGOUT:                                    "CMSG_PLAYER_LOGOUT",
	OpcodeCMSG_PLAYER_VEHICLE_ENTER:                             "CMSG_PLAYER_VEHICLE_ENTER",
	OpcodeCMSG_PLAY_DANCE:                                       "CMSG_PLAY_DANCE",
	OpcodeCMSG_PROFILEDATA_REQUEST:                              "CMSG_PROFILEDATA_REQUEST",
	OpcodeCMSG_PUSHQUESTTOPARTY:                                 "CMSG_PUSHQUESTTOPARTY",
	OpcodeCMSG_PVP_QUEUE_STATS_REQUEST:                          "CMSG_PVP_QUEUE_STATS_REQUEST",
	OpcodeCMSG_QUERY_INSPECT_ACHIEVEMENTS:                       "CMSG_QUERY_INSPECT_ACHIEVEMENTS",
	OpcodeCMSG_QUERY_OBJECT_POSITION:                            "CMSG_QUERY_OBJECT_POSITION",
	OpcodeCMSG_QUERY_OBJECT_ROTATION:                            "CMSG_QUERY_OBJECT_ROTATION",
	OpcodeCMSG_QUERY_QUESTS_COMPLETED:                           "CMSG_QUERY_QUESTS_COMPLETED",
	OpcodeCMSG_QUERY_SERVER_BUCK_DATA:                           "CMSG_QUERY_SERVER_BUCK_DATA",
	OpcodeCMSG_QUERY_TIME:                                       "CMSG_QUERY_TIME",
	OpcodeCMSG_QUERY_VEHICLE_STATUS:                             "CMSG_QUERY_VEHICLE_STATUS",
	OpcodeCMSG_QUESTGIVER_ACCEPT_QUEST:                          "CMSG_QUESTGIVER_ACCEPT_QUEST",
	OpcodeCMSG_QUESTGIVER_CANCEL:                                "CMSG_QUESTGIVER_CANCEL",
	OpcodeCMSG_QUESTGIVER_CHOOSE_REWARD:                         "CMSG_QUESTGIVER_CHOOSE_REWARD",
	OpcodeCMSG_QUESTGIVER_COMPLETE_QUEST:                        "CMSG_QUESTGIVER_COMPLETE_QUEST",
	OpcodeCMSG_QUESTGIVER_HELLO:                                 "CMSG_QUESTGIVER_HELLO",
	OpcodeCMSG_QUESTGIVER_QUERY_QUEST:                           "CMSG_QUESTGIVER_QUERY_QUEST",
	OpcodeCMSG_QUESTGIVER_QUEST_AUTOLAUNCH:                      "CMSG_QUESTGIVER_QUEST_AUTOLAUNCH",
	OpcodeCMSG_QUESTGIVER_REQUEST_REWARD:                        "CMSG_QUESTGIVER_REQUEST_REWARD",
	OpcodeCMSG_QUESTGIVER_STATUS_MULTIPLE_QUERY:                 "CMSG_QUESTGIVER_STATUS_MULTIPLE_QUERY",
	OpcodeCMSG_QUESTGIVER_STATUS_QUERY:                          "CMSG_QUESTGIVER_STATUS_QUERY",
	OpcodeCMSG_QUESTLOG_REMOVE_QUEST:                            "CMSG_QUESTLOG_REMOVE_QUEST",
	OpcodeCMSG_QUESTLOG_SWAP_QUEST:                              "CMSG_QUESTLOG_SWAP_QUEST",
	OpcodeCMSG_QUEST_CONFIRM_ACCEPT:                             "CMSG_QUEST_CONFIRM_ACCEPT",
	OpcodeCMSG_QUEST_POI_QUERY:                                  "CMSG_QUEST_POI_QUERY",
	OpcodeCMSG_QUEST_QUERY:                                      "CMSG_QUEST_QUERY",
	OpcodeCMSG_READY_FOR_ACCOUNT_DATA_TIMES:                     "CMSG_READY_FOR_ACCOUNT_DATA_TIMES",
	OpcodeCMSG_READ_ITEM:                                        "CMSG_READ_ITEM",
	OpcodeCMSG_REALM_SPLIT:                                      "CMSG_REALM_SPLIT",
	OpcodeCMSG_RECHARGE:                                         "CMSG_RECHARGE",
	OpcodeCMSG_RECLAIM_CORPSE:                                   "CMSG_RECLAIM_CORPSE",
	OpcodeCMSG_REDIRECTION_AUTH_PROOF:                           "CMSG_REDIRECTION_AUTH_PROOF",
	OpcodeCMSG_REDIRECTION_FAILED:                               "CMSG_REDIRECTION_FAILED",
	OpcodeCMSG_REFER_A_FRIEND:                                   "CMSG_REFER_A_FRIEND",
	OpcodeCMSG_REMOVE_GLYPH:                                     "CMSG_REMOVE_GLYPH",
	OpcodeCMSG_REPAIR_ITEM:                                      "CMSG_REPAIR_ITEM",
	OpcodeCMSG_REPOP_REQUEST:                                    "CMSG_REPOP_REQUEST",
	OpcodeCMSG_REPORT_PVP_AFK:                                   "CMSG_REPORT_PVP_AFK",
	OpcodeCMSG_REQUEST_ACCOUNT_DATA:                             "CMSG_REQUEST_ACCOUNT_DATA",
	OpcodeCMSG_REQUEST_PARTY_MEMBER_STATS:                       "CMSG_REQUEST_PARTY_MEMBER_STATS",
	OpcodeCMSG_REQUEST_PET_INFO:                                 "CMSG_REQUEST_PET_INFO",
	OpcodeCMSG_REQUEST_RAID_INFO:                                "CMSG_REQUEST_RAID_INFO",
	OpcodeCMSG_REQUEST_VEHICLE_EXIT:                             "CMSG_REQUEST_VEHICLE_EXIT",
	OpcodeCMSG_REQUEST_VEHICLE_NEXT_SEAT:                        "CMSG_REQUEST_VEHICLE_NEXT_SEAT",
	OpcodeCMSG_REQUEST_VEHICLE_PREV_SEAT:                        "CMSG_REQUEST_VEHICLE_PREV_SEAT",
	OpcodeCMSG_REQUEST_VEHICLE_SWITCH_SEAT:                      "CMSG_REQUEST_VEHICLE_SWITCH_SEAT",
	OpcodeCMSG_RESET_FACTION_CHEAT:                              "CMSG_RESET_FACTION_CHEAT",
	OpcodeCMSG_RESET_INSTANCES:                                  "CMSG_RESET_INSTANCES",
	OpcodeCMSG_RESURRECT_RESPONSE:                               "CMSG_RESURRECT_RESPONSE",
	OpcodeCMSG_RUN_SCRIPT:                                       "CMSG_RUN_SCRIPT",
	OpcodeCMSG_SAVE_DANCE:                                       "CMSG_SAVE_DANCE",
	OpcodeCMSG_SAVE_PLAYER:                                      "CMSG_SAVE_PLAYER",
	OpcodeCMSG_SEARCH_LFG_JOIN:                                  "CMSG_SEARCH_LFG_JOIN",
	OpcodeCMSG_SEARCH_LFG_LEAVE:                                 "CMSG_SEARCH_LFG_LEAVE",
	OpcodeCMSG_SELF_RES:                                         "CMSG_SELF_RES",
	OpcodeCMSG_SELL_ITEM:                                        "CMSG_SELL_ITEM",
	OpcodeCMSG_SEND_COMBAT_TRIGGER:                              "CMSG_SEND_COMBAT_TRIGGER",
	OpcodeCMSG_SEND_EVENT:                                       "CMSG_SEND_EVENT",
	OpcodeCMSG_SEND_GENERAL_TRIGGER:                             "CMSG_SEND_GENERAL_TRIGGER",
	OpcodeCMSG_SEND_LOCAL_EVENT:                                 "CMSG_SEND_LOCAL_EVENT",
	OpcodeCMSG_SEND_MAIL:                                        "CMSG_SEND_MAIL",
	OpcodeCMSG_SERVERINFO:                                       "CMSG_SERVERINFO",
	OpcodeCMSG_SERVERTIME:                                       "CMSG_SERVERTIME",
	OpcodeCMSG_SERVER_BROADCAST:                                 "CMSG_SERVER_BROADCAST",
	OpcodeCMSG_SERVER_COMMAND:                                   "CMSG_SERVER_COMMAND",
	OpcodeCMSG_SERVER_INFO_QUERY:                                "CMSG_SERVER_INFO_QUERY",
	OpcodeCMSG_SETDEATHBINDPOINT:                                "CMSG_SETDEATHBINDPOINT",
	OpcodeCMSG_SET_ACTIONBAR_TOGGLES:                            "CMSG_SET_ACTIONBAR_TOGGLES",
	OpcodeCMSG_SET_ACTION_BUTTON:                                "CMSG_SET_ACTION_BUTTON",
	OpcodeCMSG_SET_ACTIVE_MOVER:                                 "CMSG_SET_ACTIVE_MOVER",
	OpcodeCMSG_SET_ACTIVE_TALENT_GROUP_OBSOLETE:                 "CMSG_SET_ACTIVE_TALENT_GROUP_OBSOLETE",
	OpcodeCMSG_SET_ACTIVE_VOICE_CHANNEL:                         "CMSG_SET_ACTIVE_VOICE_CHANNEL",
	OpcodeCMSG_SET_ALLOW_LOW_LEVEL_RAID1:                        "CMSG_SET_ALLOW_LOW_LEVEL_RAID1",
	OpcodeCMSG_SET_ALLOW_LOW_LEVEL_RAID2:                        "CMSG_SET_ALLOW_LOW_LEVEL_RAID2",
	OpcodeCMSG_SET_AMMO:                                         "CMSG_SET_AMMO",
	OpcodeCMSG_SET_ARENA_MEMBER_SEASON_GAMES:                    "CMSG_SET_ARENA_MEMBER_SEASON_GAMES",
	OpcodeCMSG_SET_ARENA_MEMBER_WEEKLY_GAMES:                    "CMSG_SET_ARENA_MEMBER_WEEKLY_GAMES",
	OpcodeCMSG_SET_ARENA_TEAM_RATING_BY_INDEX:                   "CMSG_SET_ARENA_TEAM_RATING_BY_INDEX",
	OpcodeCMSG_SET_ARENA_TEAM_SEASON_GAMES:                      "CMSG_SET_ARENA_TEAM_SEASON_GAMES",
	OpcodeCMSG_SET_ARENA_TEAM_WEEKLY_GAMES:                      "CMSG_SET_ARENA_TEAM_WEEKLY_GAMES",
	OpcodeCMSG_SET_BREATH:                                       "CMSG_SET_BREATH",
	OpcodeCMSG_SET_CHANNEL_WATCH:                                "CMSG_SET_CHANNEL_WATCH",
	OpcodeCMSG_SET_CHARACTER_MODEL:                              "CMSG_SET_CHARACTER_MODEL",
	OpcodeCMSG_SET_CONTACT_NOTES:                                "CMSG_SET_CONTACT_NOTES",
	OpcodeCMSG_SET_CRITERIA_CHEAT:                               "CMSG_SET_CRITERIA_CHEAT",
	OpcodeCMSG_SET_DURABILITY_CHEAT:                             "CMSG_SET_DURABILITY_CHEAT",
	OpcodeCMSG_SET_EXPLORATION:                                  "CMSG_SET_EXPLORATION",
	OpcodeCMSG_SET_EXPLORATION_ALL:                              "CMSG_SET_EXPLORATION_ALL",
	OpcodeCMSG_SET_FACTION_ATWAR:                                "CMSG_SET_FACTION_ATWAR",
	OpcodeCMSG_SET_FACTION_CHEAT:                                "CMSG_SET_FACTION_CHEAT",
	OpcodeCMSG_SET_FACTION_INACTIVE:                             "CMSG_SET_FACTION_INACTIVE",
	OpcodeCMSG_SET_GLYPH:                                        "CMSG_SET_GLYPH",
	OpcodeCMSG_SET_GLYPH_SLOT:                                   "CMSG_SET_GLYPH_SLOT",
	OpcodeCMSG_SET_GRANTABLE_LEVELS:                             "CMSG_SET_GRANTABLE_LEVELS",
	OpcodeCMSG_SET_GUILD_BANK_TEXT:                              "CMSG_SET_GUILD_BANK_TEXT",
	OpcodeCMSG_SET_LFG_COMMENT:                                  "CMSG_SET_LFG_COMMENT",
	OpcodeCMSG_SET_PAID_SERVICE_CHEAT:                           "CMSG_SET_PAID_SERVICE_CHEAT",
	OpcodeCMSG_SET_PLAYER_DECLINED_NAMES:                        "CMSG_SET_PLAYER_DECLINED_NAMES",
	OpcodeCMSG_SET_PVP_RANK_CHEAT:                               "CMSG_SET_PVP_RANK_CHEAT",
	OpcodeCMSG_SET_PVP_TITLE:                                    "CMSG_SET_PVP_TITLE",
	OpcodeCMSG_SET_RUNE_COOLDOWN:                                "CMSG_SET_RUNE_COOLDOWN",
	OpcodeCMSG_SET_RUNE_COUNT:                                   "CMSG_SET_RUNE_COUNT",
	OpcodeCMSG_SET_SAVED_INSTANCE_EXTEND:                        "CMSG_SET_SAVED_INSTANCE_EXTEND",
	OpcodeCMSG_SET_SELECTION:                                    "CMSG_SET_SELECTION",
	OpcodeCMSG_SET_SHEATHED:                                     "CMSG_SET_SHEATHED",
	OpcodeCMSG_SET_SKILL_CHEAT:                                  "CMSG_SET_SKILL_CHEAT",
	OpcodeCMSG_SET_STAT_CHEAT:                                   "CMSG_SET_STAT_CHEAT",
	OpcodeCMSG_SET_TAXI_BENCHMARK_MODE:                          "CMSG_SET_TAXI_BENCHMARK_MODE",
	OpcodeCMSG_SET_TITLE:                                        "CMSG_SET_TITLE",
	OpcodeCMSG_SET_TITLE_SUFFIX:                                 "CMSG_SET_TITLE_SUFFIX",
	OpcodeCMSG_SET_TRADE_GOLD:                                   "CMSG_SET_TRADE_GOLD",
	OpcodeCMSG_SET_TRADE_ITEM:                                   "CMSG_SET_TRADE_ITEM",
	OpcodeCMSG_SET_VEHICLE_REC_ID_ACK:                           "CMSG_SET_VEHICLE_REC_ID_ACK",
	OpcodeCMSG_SET_WATCHED_FACTION:                              "CMSG_SET_WATCHED_FACTION",
	OpcodeCMSG_SET_WORLDSTATE:                                   "CMSG_SET_WORLDSTATE",
	OpcodeCMSG_SHOWING_CLOAK:                                    "CMSG_SHOWING_CLOAK",
	OpcodeCMSG_SHOWING_HELM:                                     "CMSG_SHOWING_HELM",
	OpcodeCMSG_SKILL_BUY_RANK:                                   "CMSG_SKILL_BUY_RANK",
	OpcodeCMSG_SKILL_BUY_STEP:                                   "CMSG_SKILL_BUY_STEP",
	OpcodeCMSG_SOCKET_GEMS:                                      "CMSG_SOCKET_GEMS",
	OpcodeCMSG_SPELLCLICK:                                       "CMSG_SPELLCLICK",
	OpcodeCMSG_SPIRIT_HEALER_ACTIVATE:                           "CMSG_SPIRIT_HEALER_ACTIVATE",
	OpcodeCMSG_SPLIT_ITEM:                                       "CMSG_SPLIT_ITEM",
	OpcodeCMSG_STABLE_PET:                                       "CMSG_STABLE_PET",
	OpcodeCMSG_STABLE_REVIVE_PET:                                "CMSG_STABLE_REVIVE_PET",
	OpcodeCMSG_STABLE_SWAP_PET:                                  "CMSG_STABLE_SWAP_PET",
	OpcodeCMSG_STANDSTATECHANGE:                                 "CMSG_STANDSTATECHANGE",
	OpcodeCMSG_START_BATTLEFIELD_CHEAT:                          "CMSG_START_BATTLEFIELD_CHEAT",
	OpcodeCMSG_START_QUEST:                                      "CMSG_START_QUEST",
	OpcodeCMSG_STOP_DANCE:                                       "CMSG_STOP_DANCE",
	OpcodeCMSG_STORE_LOOT_IN_SLOT:                               "CMSG_STORE_LOOT_IN_SLOT",
	OpcodeCMSG_SUMMON_RESPONSE:                                  "CMSG_SUMMON_RESPONSE",
	OpcodeCMSG_SUSPEND_COMMS_ACK:                                "CMSG_SUSPEND_COMMS_ACK",
	OpcodeCMSG_SWAP_INV_ITEM:                                    "CMSG_SWAP_INV_ITEM",
	OpcodeCMSG_SWAP_ITEM:                                        "CMSG_SWAP_ITEM",
	OpcodeCMSG_SYNC_DANCE:                                       "CMSG_SYNC_DANCE",
	OpcodeCMSG_TARGET_CAST:                                      "CMSG_TARGET_CAST",
	OpcodeCMSG_TARGET_SCRIPT_CAST:                               "CMSG_TARGET_SCRIPT_CAST",
	OpcodeCMSG_TAXICLEARALLNODES:                                "CMSG_TAXICLEARALLNODES",
	OpcodeCMSG_TAXICLEARNODE:                                    "CMSG_TAXICLEARNODE",
	OpcodeCMSG_TAXIENABLEALLNODES:                               "CMSG_TAXIENABLEALLNODES",
	OpcodeCMSG_TAXIENABLENODE:                                   "CMSG_TAXIENABLENODE",
	OpcodeCMSG_TAXINODE_STATUS_QUERY:                            "CMSG_TAXINODE_STATUS_QUERY",
	OpcodeCMSG_TAXIQUERYAVAILABLENODES:                          "CMSG_TAXIQUERYAVAILABLENODES",
	OpcodeCMSG_TAXISHOWNODES:                                    "CMSG_TAXISHOWNODES",
	OpcodeCMSG_TELEPORT_TO_UNIT:                                 "CMSG_TELEPORT_TO_UNIT",
	OpcodeCMSG_TEST_DROP_RATE:                                   "CMSG_TEST_DROP_RATE",
	OpcodeCMSG_TEXT_EMOTE:                                       "CMSG_TEXT_EMOTE",
	OpcodeCMSG_TIME_SYNC_RESP:                                   "CMSG_TIME_SYNC_RESP",
	OpcodeCMSG_TOGGLE_PVP:                                       "CMSG_TOGGLE_PVP",
	OpcodeCMSG_TOGGLE_XP_GAIN:                                   "CMSG_TOGGLE_XP_GAIN",
	OpcodeCMSG_TOTEM_DESTROYED:                                  "CMSG_TOTEM_DESTROYED",
	OpcodeCMSG_TRAINER_BUY_SPELL:                                "CMSG_TRAINER_BUY_SPELL",
	OpcodeCMSG_TRAINER_LIST:                                     "CMSG_TRAINER_LIST",
	OpcodeCMSG_TRIGGER_CINEMATIC_CHEAT:                          "CMSG_TRIGGER_CINEMATIC_CHEAT",
	OpcodeCMSG_TURN_IN_PETITION:                                 "CMSG_TURN_IN_PETITION",
	OpcodeCMSG_TUTORIAL_CLEAR:                                   "CMSG_TUTORIAL_CLEAR",
	OpcodeCMSG_TUTORIAL_FLAG:                                    "CMSG_TUTORIAL_FLAG",
	OpcodeCMSG_TUTORIAL_RESET:                                   "CMSG_TUTORIAL_RESET",
	OpcodeCMSG_UNACCEPT_TRADE:                                   "CMSG_UNACCEPT_TRADE",
	OpcodeCMSG_UNCLAIM_LICENSE:                                  "CMSG_UNCLAIM_LICENSE",
	OpcodeCMSG_UNDRESSPLAYER:                                    "CMSG_UNDRESSPLAYER",
	OpcodeCMSG_UNITANIMTIER_CHEAT:                               "CMSG_UNITANIMTIER_CHEAT",
	OpcodeCMSG_UNLEARN_DANCE_MOVE:                               "CMSG_UNLEARN_DANCE_MOVE",
	OpcodeCMSG_UNLEARN_SKILL:                                    "CMSG_UNLEARN_SKILL",
	OpcodeCMSG_UNLEARN_SPELL:                                    "CMSG_UNLEARN_SPELL",
	OpcodeCMSG_UNLEARN_TALENTS:                                  "CMSG_UNLEARN_TALENTS",
	OpcodeCMSG_UNSTABLE_PET:                                     "CMSG_UNSTABLE_PET",
	OpcodeCMSG_UNUSED5:                                          "CMSG_UNUSED5",
	OpcodeCMSG_UNUSED6:                                          "CMSG_UNUSED6",
	OpcodeCMSG_UPDATE_ACCOUNT_DATA:                              "CMSG_UPDATE_ACCOUNT_DATA",
	OpcodeCMSG_UPDATE_MISSILE_TRAJECTORY:                        "CMSG_UPDATE_MISSILE_TRAJECTORY",
	OpcodeCMSG_UPDATE_PROJECTILE_POSITION:                       "CMSG_UPDATE_PROJECTILE_POSITION",
	OpcodeCMSG_USE_ITEM:                                         "CMSG_USE_ITEM",
	OpcodeCMSG_USE_SKILL_CHEAT:                                  "CMSG_USE_SKILL_CHEAT",
	OpcodeCMSG_VOICE_SESSION_ENABLE:                             "CMSG_VOICE_SESSION_ENABLE",
	OpcodeCMSG_VOICE_SET_TALKER_MUTED_REQUEST:                   "CMSG_VOICE_SET_TALKER_MUTED_REQUEST",
	OpcodeCMSG_WARDEN_DATA:                                      "CMSG_WARDEN_DATA",
	OpcodeCMSG_WEATHER_SPEED_CHEAT:                              "CMSG_WEATHER_SPEED_CHEAT",
	OpcodeCMSG_WHO:                                              "CMSG_WHO",
	OpcodeCMSG_WHOIS:                                            "CMSG_WHOIS",
	OpcodeCMSG_WORLD_STATE_UI_TIMER_UPDATE:                      "CMSG_WORLD_STATE_UI_TIMER_UPDATE",
	OpcodeCMSG_WORLD_TELEPORT:                                   "CMSG_WORLD_TELEPORT",
	OpcodeCMSG_WRAP_ITEM:                                        "CMSG_WRAP_ITEM",
	OpcodeCMSG_XP_CHEAT:                                         "CMSG_XP_CHEAT",
	OpcodeCMSG_ZONEUPDATE:                                       "CMSG_ZONEUPDATE",
	OpcodeCMSG_ZONE_MAP:                                         "CMSG_ZONE_MAP",
	OpcodeMSG_AUCTION_HELLO:                                     "MSG_AUCTION_HELLO",
	OpcodeMSG_BATTLEGROUND_PLAYER_POSITIONS:                     "MSG_BATTLEGROUND_PLAYER_POSITIONS",
	OpcodeMSG_CHANNEL_START:                                     "MSG_CHANNEL_START",
	OpcodeMSG_CHANNEL_UPDATE:                                    "MSG_CHANNEL_UPDATE",
	OpcodeMSG_CORPSE_QUERY:                                      "MSG_CORPSE_QUERY",
	OpcodeMSG_DELAY_GHOST_TELEPORT:                              "MSG_DELAY_GHOST_TELEPORT",
	OpcodeMSG_DEV_SHOWLABEL:                                     "MSG_DEV_SHOWLABEL",
	OpcodeMSG_GM_ACCOUNT_ONLINE:                                 "MSG_GM_ACCOUNT_ONLINE",
	OpcodeMSG_GM_BIND_OTHER:                                     "MSG_GM_BIND_OTHER",
	OpcodeMSG_GM_CHANGE_ARENA_RATING:                            "MSG_GM_CHANGE_ARENA_RATING",
	OpcodeMSG_GM_DESTROY_CORPSE:                                 "MSG_GM_DESTROY_CORPSE",
	OpcodeMSG_GM_GEARRATING:                                     "MSG_GM_GEARRATING",
	OpcodeMSG_GM_RESETINSTANCELIMIT:                             "MSG_GM_RESETINSTANCELIMIT",
	OpcodeMSG_GM_SHOWLABEL:                                      "MSG_GM_SHOWLABEL",
	OpcodeMSG_GM_SUMMON:                                         "MSG_GM_SUMMON",
	OpcodeMSG_GUILD_BANK_LOG_QUERY:                              "MSG_GUILD_BANK_LOG_QUERY",
	OpcodeMSG_GUILD_BANK_MONEY_WITHDRAWN:                        "MSG_GUILD_BANK_MONEY_WITHDRAWN",
	OpcodeMSG_GUILD_EVENT_LOG_QUERY:                             "MSG_GUILD_EVENT_LOG_QUERY",
	OpcodeMSG_GUILD_PERMISSIONS:                                 "MSG_GUILD_PERMISSIONS",
	OpcodeMSG_INSPECT_ARENA_TEAMS:                               "MSG_INSPECT_ARENA_TEAMS",
	OpcodeMSG_INSPECT_HONOR_STATS:                               "MSG_INSPECT_HONOR_STATS",
	OpcodeMSG_LIST_STABLED_PETS:                                 "MSG_LIST_STABLED_PETS",
	OpcodeMSG_MINIMAP_PING:                                      "MSG_MINIMAP_PING",
	OpcodeMSG_MOVE_FALL_LAND:                                    "MSG_MOVE_FALL_LAND",
	OpcodeMSG_MOVE_FEATHER_FALL:                                 "MSG_MOVE_FEATHER_FALL",
	OpcodeMSG_MOVE_GRAVITY_CHNG:                                 "MSG_MOVE_GRAVITY_CHNG",
	OpcodeMSG_MOVE_HEARTBEAT:                                    "MSG_MOVE_HEARTBEAT",
	OpcodeMSG_MOVE_HOVER:                                        "MSG_MOVE_HOVER",
	OpcodeMSG_MOVE_JUMP:                                         "MSG_MOVE_JUMP",
	OpcodeMSG_MOVE_KNOCK_BACK:                                   "MSG_MOVE_KNOCK_BACK",
	OpcodeMSG_MOVE_ROOT:                                         "MSG_MOVE_ROOT",
	OpcodeMSG_MOVE_SET_ALL_SPEED_CHEAT:                          "MSG_MOVE_SET_ALL_SPEED_CHEAT",
	OpcodeMSG_MOVE_SET_COLLISION_HGT:                            "MSG_MOVE_SET_COLLISION_HGT",
	OpcodeMSG_MOVE_SET_FACING:                                   "MSG_MOVE_SET_FACING",
	OpcodeMSG_MOVE_SET_FLIGHT_BACK_SPEED:                        "MSG_MOVE_SET_FLIGHT_BACK_SPEED",
	OpcodeMSG_MOVE_SET_FLIGHT_BACK_SPEED_CHEAT:                  "MSG_MOVE_SET_FLIGHT_BACK_SPEED_CHEAT",
	OpcodeMSG_MOVE_SET_FLIGHT_SPEED:                             "MSG_MOVE_SET_FLIGHT_SPEED",
	OpcodeMSG_MOVE_SET_FLIGHT_SPEED_CHEAT:                       "MSG_MOVE_SET_FLIGHT_SPEED_CHEAT",
	OpcodeMSG_MOVE_SET_PITCH:                                    "MSG_MOVE_SET_PITCH",
	OpcodeMSG_MOVE_SET_PITCH_RATE:                               "MSG_MOVE_SET_PITCH_RATE",
	OpcodeMSG_MOVE_SET_PITCH_RATE_CHEAT:                         "MSG_MOVE_SET_PITCH_RATE_CHEAT",
	OpcodeMSG_MOVE_SET_RUN_BACK_SPEED:                           "MSG_MOVE_SET_RUN_BACK_SPEED",
	OpcodeMSG_MOVE_SET_RUN_BACK_SPEED_CHEAT:                     "MSG_MOVE_SET_RUN_BACK_SPEED_CHEAT",
	OpcodeMSG_MOVE_SET_RUN_MODE:                                 "MSG_MOVE_SET_RUN_MODE",
	OpcodeMSG_MOVE_SET_RUN_SPEED:                                "MSG_MOVE_SET_RUN_SPEED",
	OpcodeMSG_MOVE_SET_RUN_SPEED_CHEAT:                          "MSG_MOVE_SET_RUN_SPEED_CHEAT",
	OpcodeMSG_MOVE_SET_SWIM_BACK_SPEED:                          "MSG_MOVE_SET_SWIM_BACK_SPEED",
	OpcodeMSG_MOVE_SET_SWIM_BACK_SPEED_CHEAT:                    "MSG_MOVE_SET_SWIM_BACK_SPEED_CHEAT",
	OpcodeMSG_MOVE_SET_SWIM_SPEED:                               "MSG_MOVE_SET_SWIM_SPEED",
	OpcodeMSG_MOVE_SET_SWIM_SPEED_CHEAT:                         "MSG_MOVE_SET_SWIM_SPEED_CHEAT",
	OpcodeMSG_MOVE_SET_TURN_RATE:                                "MSG_MOVE_SET_TURN_RATE",
	OpcodeMSG_MOVE_SET_TURN_RATE_CHEAT:                          "MSG_MOVE_SET_TURN_RATE_CHEAT",
	OpcodeMSG_MOVE_SET_WALK_MODE:                                "MSG_MOVE_SET_WALK_MODE",
	OpcodeMSG_MOVE_SET_WALK_SPEED:                               "MSG_MOVE_SET_WALK_SPEED",
	OpcodeMSG_MOVE_SET_WALK_SPEED_CHEAT:                         "MSG_MOVE_SET_WALK_SPEED_CHEAT",
	OpcodeMSG_MOVE_START_ASCEND:                                 "MSG_MOVE_START_ASCEND",
	OpcodeMSG_MOVE_START_BACKWARD:                               "MSG_MOVE_START_BACKWARD",
	OpcodeMSG_MOVE_START_DESCEND:                                "MSG_MOVE_START_DESCEND",
	OpcodeMSG_MOVE_START_FORWARD:                                "MSG_MOVE_START_FORWARD",
	OpcodeMSG_MOVE_START_PITCH_DOWN:                             "MSG_MOVE_START_PITCH_DOWN",
	OpcodeMSG_MOVE_START_PITCH_UP:                               "MSG_MOVE_START_PITCH_UP",
	OpcodeMSG_MOVE_START_STRAFE_LEFT:                            "MSG_MOVE_START_STRAFE_LEFT",
	OpcodeMSG_MOVE_START_STRAFE_RIGHT:                           "MSG_MOVE_START_STRAFE_RIGHT",
	OpcodeMSG_MOVE_START_SWIM:                                   "MSG_MOVE_START_SWIM",
	OpcodeMSG_MOVE_START_SWIM_CHEAT:                             "MSG_MOVE_START_SWIM_CHEAT",
	OpcodeMSG_MOVE_START_TURN_LEFT:                              "MSG_MOVE_START_TURN_LEFT",
	OpcodeMSG_MOVE_START_TURN_RIGHT:                             "MSG_MOVE_START_TURN_RIGHT",
	OpcodeMSG_MOVE_STOP:                                         "MSG_MOVE_STOP",
	OpcodeMSG_MOVE_STOP_ASCEND:                                  "MSG_MOVE_STOP_ASCEND",
	OpcodeMSG_MOVE_STOP_PITCH:                                   "MSG_MOVE_STOP_PITCH",
	OpcodeMSG_MOVE_STOP_STRAFE:                                  "MSG_MOVE_STOP_STRAFE",
	OpcodeMSG_MOVE_STOP_SWIM:                                    "MSG_MOVE_STOP_SWIM",
	OpcodeMSG_MOVE_STOP_SWIM_CHEAT:                              "MSG_MOVE_STOP_SWIM_CHEAT",
	OpcodeMSG_MOVE_STOP_TURN:                                    "MSG_MOVE_STOP_TURN",
	OpcodeMSG_MOVE_TELEPORT:                                     "MSG_MOVE_TELEPORT",
	OpcodeMSG_MOVE_TELEPORT_ACK:                                 "MSG_MOVE_TELEPORT_ACK",
	OpcodeMSG_MOVE_TELEPORT_CHEAT:                               "MSG_MOVE_TELEPORT_CHEAT",
	OpcodeMSG_MOVE_TIME_SKIPPED:                                 "MSG_MOVE_TIME_SKIPPED",
	OpcodeMSG_MOVE_TOGGLE_COLLISION_CHEAT:                       "MSG_MOVE_TOGGLE_COLLISION_CHEAT",
	OpcodeMSG_MOVE_TOGGLE_FALL_LOGGING:                          "MSG_MOVE_TOGGLE_FALL_LOGGING",
	OpcodeMSG_MOVE_TOGGLE_LOGGING:                               "MSG_MOVE_TOGGLE_LOGGING",
	OpcodeMSG_MOVE_UNROOT:                                       "MSG_MOVE_UNROOT",
	OpcodeMSG_MOVE_UPDATE_CAN_FLY:                               "MSG_MOVE_UPDATE_CAN_FLY",
	OpcodeMSG_MOVE_UPDATE_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY:   "MSG_MOVE_UPDATE_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY",
	OpcodeMSG_MOVE_WATER_WALK:                                   "MSG_MOVE_WATER_WALK",
	OpcodeMSG_MOVE_WORLDPORT_ACK:                                "MSG_MOVE_WORLDPORT_ACK",
	OpcodeMSG_NOTIFY_PARTY_SQUELCH:                              "MSG_NOTIFY_PARTY_SQUELCH",
	OpcodeMSG_PARTY_ASSIGNMENT:                                  "MSG_PARTY_ASSIGNMENT",
	OpcodeMSG_PETITION_DECLINE:                                  "MSG_PETITION_DECLINE",
	OpcodeMSG_PETITION_RENAME:                                   "MSG_PETITION_RENAME",
	OpcodeMSG_PVP_LOG_DATA:                                      "MSG_PVP_LOG_DATA",
	OpcodeMSG_QUERY_GUILD_BANK_TEXT:                             "MSG_QUERY_GUILD_BANK_TEXT",
	OpcodeMSG_QUERY_NEXT_MAIL_TIME:                              "MSG_QUERY_NEXT_MAIL_TIME",
	OpcodeMSG_QUEST_PUSH_RESULT:                                 "MSG_QUEST_PUSH_RESULT",
	OpcodeMSG_RAID_READY_CHECK:                                  "MSG_RAID_READY_CHECK",
	OpcodeMSG_RAID_READY_CHECK_CONFIRM:                          "MSG_RAID_READY_CHECK_CONFIRM",
	OpcodeMSG_RAID_READY_CHECK_FINISHED:                         "MSG_RAID_READY_CHECK_FINISHED",
	OpcodeMSG_RAID_TARGET_UPDATE:                                "MSG_RAID_TARGET_UPDATE",
	OpcodeMSG_RANDOM_ROLL:                                       "MSG_RANDOM_ROLL",
	OpcodeMSG_SAVE_GUILD_EMBLEM:                                 "MSG_SAVE_GUILD_EMBLEM",
	OpcodeMSG_SET_DUNGEON_DIFFICULTY:                            "MSG_SET_DUNGEON_DIFFICULTY",
	OpcodeMSG_SET_RAID_DIFFICULTY:                               "MSG_SET_RAID_DIFFICULTY",
	OpcodeMSG_TABARDVENDOR_ACTIVATE:                             "MSG_TABARDVENDOR_ACTIVATE",
	OpcodeMSG_TALENT_WIPE_CONFIRM:                               "MSG_TALENT_WIPE_CONFIRM",
	OpcodeMSG_VIEW_PHASE_SHIFT:                                  "MSG_VIEW_PHASE_SHIFT",
	OpcodeNULL_OPCODE:                                           "NULL_OPCODE",
	OpcodeNUM_MSG_TYPES:                                         "NUM_MSG_TYPES",
	OpcodeSMSG_ACCOUNT_DATA_TIMES:                               "SMSG_ACCOUNT_DATA_TIMES",
	OpcodeSMSG_ACHIEVEMENT_DELETED:                              "SMSG_ACHIEVEMENT_DELETED",
	OpcodeSMSG_ACHIEVEMENT_EARNED:                               "SMSG_ACHIEVEMENT_EARNED",
	OpcodeSMSG_ACTION_BUTTONS:                                   "SMSG_ACTION_BUTTONS",
	OpcodeSMSG_ACTIVATETAXIREPLY:                                "SMSG_ACTIVATETAXIREPLY",
	OpcodeSMSG_ADDON_INFO:                                       "SMSG_ADDON_INFO",
	OpcodeSMSG_ADD_RUNE_POWER:                                   "SMSG_ADD_RUNE_POWER",
	OpcodeSMSG_AFK_MONITOR_INFO_RESPONSE:                        "SMSG_AFK_MONITOR_INFO_RESPONSE",
	OpcodeSMSG_AI_REACTION:                                      "SMSG_AI_REACTION",
	OpcodeSMSG_ALL_ACHIEVEMENT_DATA:                             "SMSG_ALL_ACHIEVEMENT_DATA",
	OpcodeSMSG_AREA_SPIRIT_HEALER_TIME:                          "SMSG_AREA_SPIRIT_HEALER_TIME",
	OpcodeSMSG_AREA_TRIGGER_MESSAGE:                             "SMSG_AREA_TRIGGER_MESSAGE",
	OpcodeSMSG_ARENA_ERROR:                                      "SMSG_ARENA_ERROR",
	OpcodeSMSG_ARENA_TEAM_CHANGE_FAILED_QUEUED:                  "SMSG_ARENA_TEAM_CHANGE_FAILED_QUEUED",
	OpcodeSMSG_ARENA_TEAM_COMMAND_RESULT:                        "SMSG_ARENA_TEAM_COMMAND_RESULT",
	OpcodeSMSG_ARENA_TEAM_EVENT:                                 "SMSG_ARENA_TEAM_EVENT",
	OpcodeSMSG_ARENA_TEAM_INVITE:                                "SMSG_ARENA_TEAM_INVITE",
	OpcodeSMSG_ARENA_TEAM_QUERY_RESPONSE:                        "SMSG_ARENA_TEAM_QUERY_RESPONSE",
	OpcodeSMSG_ARENA_TEAM_ROSTER:                                "SMSG_ARENA_TEAM_ROSTER",
	OpcodeSMSG_ARENA_TEAM_STATS:                                 "SMSG_ARENA_TEAM_STATS",
	OpcodeSMSG_ARENA_UNIT_DESTROYED:                             "SMSG_ARENA_UNIT_DESTROYED",
	OpcodeSMSG_ATTACKERSTATEUPDATE:                              "SMSG_ATTACKERSTATEUPDATE",
	OpcodeSMSG_ATTACK_START:                                     "SMSG_ATTACK_START",
	OpcodeSMSG_ATTACK_STOP:                                      "SMSG_ATTACK_STOP",
	OpcodeSMSG_ATTACK_SWING_BAD_FACING:                          "SMSG_ATTACK_SWING_BAD_FACING",
	OpcodeSMSG_ATTACK_SWING_CANT_ATTACK:                         "SMSG_ATTACK_SWING_CANT_ATTACK",
	OpcodeSMSG_ATTACK_SWING_DEAD_TARGET:                         "SMSG_ATTACK_SWING_DEAD_TARGET",
	OpcodeSMSG_ATTACK_SWING_NOT_IN_RANGE:                        "SMSG_ATTACK_SWING_NOT_IN_RANGE",
	OpcodeSMSG_AUCTION_BIDDER_LIST_RESULT:                       "SMSG_AUCTION_BIDDER_LIST_RESULT",
	OpcodeSMSG_AUCTION_BIDDER_NOTIFICATION:                      "SMSG_AUCTION_BIDDER_NOTIFICATION",
	OpcodeSMSG_AUCTION_COMMAND_RESULT:                           "SMSG_AUCTION_COMMAND_RESULT",
	OpcodeSMSG_AUCTION_LIST_PENDING_SALES:                       "SMSG_AUCTION_LIST_PENDING_SALES",
	OpcodeSMSG_AUCTION_LIST_RESULT:                              "SMSG_AUCTION_LIST_RESULT",
	OpcodeSMSG_AUCTION_OWNER_LIST_RESULT:                        "SMSG_AUCTION_OWNER_LIST_RESULT",
	OpcodeSMSG_AUCTION_OWNER_NOTIFICATION:                       "SMSG_AUCTION_OWNER_NOTIFICATION",
	OpcodeSMSG_AUCTION_REMOVED_NOTIFICATION:                     "SMSG_AUCTION_REMOVED_NOTIFICATION",
	OpcodeSMSG_AURACASTLOG:                                      "SMSG_AURACASTLOG",
	OpcodeSMSG_AURA_UPDATE:                                      "SMSG_AURA_UPDATE",
	OpcodeSMSG_AURA_UPDATE_ALL:                                  "SMSG_AURA_UPDATE_ALL",
	OpcodeSMSG_AUTH_CHALLENGE:                                   "SMSG_AUTH_CHALLENGE",
	OpcodeSMSG_AUTH_RESPONSE:                                    "SMSG_AUTH_RESPONSE",
	OpcodeSMSG_AUTH_SRP6_RESPONSE:                               "SMSG_AUTH_SRP6_RESPONSE",
	OpcodeSMSG_AVAILABLE_VOICE_CHANNEL:                          "SMSG_AVAILABLE_VOICE_CHANNEL",
	OpcodeSMSG_BARBER_SHOP_RESULT:                               "SMSG_BARBER_SHOP_RESULT",
	OpcodeSMSG_BATTLEFIELD_LIST:                                 "SMSG_BATTLEFIELD_LIST",
	OpcodeSMSG_BATTLEFIELD_MGR_EJECTED:                          "SMSG_BATTLEFIELD_MGR_EJECTED",
	OpcodeSMSG_BATTLEFIELD_MGR_EJECT_PENDING:                    "SMSG_BATTLEFIELD_MGR_EJECT_PENDING",
	OpcodeSMSG_BATTLEFIELD_MGR_ENTERED:                          "SMSG_BATTLEFIELD_MGR_ENTERED",
	OpcodeSMSG_BATTLEFIELD_MGR_ENTRY_INVITE:                     "SMSG_BATTLEFIELD_MGR_ENTRY_INVITE",
	OpcodeSMSG_BATTLEFIELD_MGR_QUEUE_INVITE:                     "SMSG_BATTLEFIELD_MGR_QUEUE_INVITE",
	OpcodeSMSG_BATTLEFIELD_MGR_QUEUE_REQUEST_RESPONSE:           "SMSG_BATTLEFIELD_MGR_QUEUE_REQUEST_RESPONSE",
	OpcodeSMSG_BATTLEFIELD_MGR_STATE_CHANGE:                     "SMSG_BATTLEFIELD_MGR_STATE_CHANGE",
	OpcodeSMSG_BATTLEFIELD_PORT_DENIED:                          "SMSG_BATTLEFIELD_PORT_DENIED",
	OpcodeSMSG_BATTLEFIELD_STATUS:                               "SMSG_BATTLEFIELD_STATUS",
	OpcodeSMSG_BATTLEGROUND_INFO_THROTTLED:                      "SMSG_BATTLEGROUND_INFO_THROTTLED",
	OpcodeSMSG_BATTLEGROUND_PLAYER_JOINED:                       "SMSG_BATTLEGROUND_PLAYER_JOINED",
	OpcodeSMSG_BATTLEGROUND_PLAYER_LEFT:                         "SMSG_BATTLEGROUND_PLAYER_LEFT",
	OpcodeSMSG_BINDER_CONFIRM:                                   "SMSG_BINDER_CONFIRM",
	OpcodeSMSG_BINDZONEREPLY:                                    "SMSG_BINDZONEREPLY",
	OpcodeSMSG_BIND_POINT_UPDATE:                                "SMSG_BIND_POINT_UPDATE",
	OpcodeSMSG_BREAK_TARGET:                                     "SMSG_BREAK_TARGET",
	OpcodeSMSG_BUY_BANK_SLOT_RESULT:                             "SMSG_BUY_BANK_SLOT_RESULT",
	OpcodeSMSG_BUY_FAILED:                                       "SMSG_BUY_FAILED",
	OpcodeSMSG_BUY_ITEM:                                         "SMSG_BUY_ITEM",
	OpcodeSMSG_CALENDAR_ARENA_TEAM:                              "SMSG_CALENDAR_ARENA_TEAM",
	OpcodeSMSG_CALENDAR_CLEAR_PENDING_ACTION:                    "SMSG_CALENDAR_CLEAR_PENDING_ACTION",
	OpcodeSMSG_CALENDAR_COMMAND_RESULT:                          "SMSG_CALENDAR_COMMAND_RESULT",
	OpcodeSMSG_CALENDAR_EVENT_INVITE:                            "SMSG_CALENDAR_EVENT_INVITE",
	OpcodeSMSG_CALENDAR_EVENT_INVITE_ALERT:                      "SMSG_CALENDAR_EVENT_INVITE_ALERT",
	OpcodeSMSG_CALENDAR_EVENT_INVITE_NOTES:                      "SMSG_CALENDAR_EVENT_INVITE_NOTES",
	OpcodeSMSG_CALENDAR_EVENT_INVITE_NOTES_ALERT:                "SMSG_CALENDAR_EVENT_INVITE_NOTES_ALERT",
	OpcodeSMSG_CALENDAR_EVENT_INVITE_REMOVED:                    "SMSG_CALENDAR_EVENT_INVITE_REMOVED",
	OpcodeSMSG_CALENDAR_EVENT_INVITE_REMOVED_ALERT:              "SMSG_CALENDAR_EVENT_INVITE_REMOVED_ALERT",
	OpcodeSMSG_CALENDAR_EVENT_INVITE_STATUS_ALERT:               "SMSG_CALENDAR_EVENT_INVITE_STATUS_ALERT",
	OpcodeSMSG_CALENDAR_EVENT_MODERATOR_STATUS_ALERT:            "SMSG_CALENDAR_EVENT_MODERATOR_STATUS_ALERT",
	OpcodeSMSG_CALENDAR_EVENT_REMOVED_ALERT:                     "SMSG_CALENDAR_EVENT_REMOVED_ALERT",
	OpcodeSMSG_CALENDAR_EVENT_STATUS:                            "SMSG_CALENDAR_EVENT_STATUS",
	OpcodeSMSG_CALENDAR_EVENT_UPDATED_ALERT:                     "SMSG_CALENDAR_EVENT_UPDATED_ALERT",
	OpcodeSMSG_CALENDAR_FILTER_GUILD:                            "SMSG_CALENDAR_FILTER_GUILD",
	OpcodeSMSG_CALENDAR_RAID_LOCKOUT_ADDED:                      "SMSG_CALENDAR_RAID_LOCKOUT_ADDED",
	OpcodeSMSG_CALENDAR_RAID_LOCKOUT_REMOVED:                    "SMSG_CALENDAR_RAID_LOCKOUT_REMOVED",
	OpcodeSMSG_CALENDAR_RAID_LOCKOUT_UPDATED:                    "SMSG_CALENDAR_RAID_LOCKOUT_UPDATED",
	OpcodeSMSG_CALENDAR_SEND_CALENDAR:                           "SMSG_CALENDAR_SEND_CALENDAR",
	OpcodeSMSG_CALENDAR_SEND_EVENT:                              "SMSG_CALENDAR_SEND_EVENT",
	OpcodeSMSG_CALENDAR_SEND_NUM_PENDING:                        "SMSG_CALENDAR_SEND_NUM_PENDING",
	OpcodeSMSG_CAMERA_SHAKE:                                     "SMSG_CAMERA_SHAKE",
	OpcodeSMSG_CANCEL_AUTO_REPEAT:                               "SMSG_CANCEL_AUTO_REPEAT",
	OpcodeSMSG_CANCEL_COMBAT:                                    "SMSG_CANCEL_COMBAT",
	OpcodeSMSG_CAST_FAILED:                                      "SMSG_CAST_FAILED",
	OpcodeSMSG_CHANGEPLAYER_DIFFICULTY_RESULT:                   "SMSG_CHANGEPLAYER_DIFFICULTY_RESULT",
	OpcodeSMSG_CHANNEL_LIST:                                     "SMSG_CHANNEL_LIST",
	OpcodeSMSG_CHANNEL_MEMBER_COUNT:                             "SMSG_CHANNEL_MEMBER_COUNT",
	OpcodeSMSG_CHANNEL_NOTIFY:                                   "SMSG_CHANNEL_NOTIFY",
	OpcodeSMSG_CHARACTER_LOGIN_FAILED:                           "SMSG_CHARACTER_LOGIN_FAILED",
	OpcodeSMSG_CHARACTER_PROFILE:                                "SMSG_CHARACTER_PROFILE",
	OpcodeSMSG_CHARACTER_PROFILE_REALM_CONNECTED:                "SMSG_CHARACTER_PROFILE_REALM_CONNECTED",
	OpcodeSMSG_CHAR_CREATE:                                      "SMSG_CHAR_CREATE",
	OpcodeSMSG_CHAR_CUSTOMIZE:                                   "SMSG_CHAR_CUSTOMIZE",
	OpcodeSMSG_CHAR_DELETE:                                      "SMSG_CHAR_DELETE",
	OpcodeSMSG_CHAR_ENUM:                                        "SMSG_CHAR_ENUM",
	OpcodeSMSG_CHAR_FACTION_CHANGE:                              "SMSG_CHAR_FACTION_CHANGE",
	OpcodeSMSG_CHAR_RENAME:                                      "SMSG_CHAR_RENAME",
	OpcodeSMSG_CHAT_NOT_IN_PARTY:                                "SMSG_CHAT_NOT_IN_PARTY",
	OpcodeSMSG_CHAT_PLAYER_AMBIGUOUS:                            "SMSG_CHAT_PLAYER_AMBIGUOUS",
	OpcodeSMSG_CHAT_PLAYER_NOT_FOUND:                            "SMSG_CHAT_PLAYER_NOT_FOUND",
	OpcodeSMSG_CHAT_RESTRICTED:                                  "SMSG_CHAT_RESTRICTED",
	OpcodeSMSG_CHAT_SERVER_MESSAGE:                              "SMSG_CHAT_SERVER_MESSAGE",
	OpcodeSMSG_CHAT_WRONG_FACTION:                               "SMSG_CHAT_WRONG_FACTION",
	OpcodeSMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY_RESPONSE:             "SMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY_RESPONSE",
	OpcodeSMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY_RESPONSE_WRITE_FILE:  "SMSG_CHEAT_DUMP_ITEMS_DEBUG_ONLY_RESPONSE_WRITE_FILE",
	OpcodeSMSG_CHEAT_PLAYER_LOOKUP:                              "SMSG_CHEAT_PLAYER_LOOKUP",
	OpcodeSMSG_CHECK_FOR_BOTS:                                   "SMSG_CHECK_FOR_BOTS",
	OpcodeSMSG_CLEAR_COOLDOWN:                                   "SMSG_CLEAR_COOLDOWN",
	OpcodeSMSG_CLEAR_EXTRA_AURA_INFO_OBSOLETE:                   "SMSG_CLEAR_EXTRA_AURA_INFO_OBSOLETE",
	OpcodeSMSG_CLEAR_FAR_SIGHT_IMMEDIATE:                        "SMSG_CLEAR_FAR_SIGHT_IMMEDIATE",
	OpcodeSMSG_CLEAR_TARGET:                                     "SMSG_CLEAR_TARGET",
	OpcodeSMSG_CLIENTCACHE_VERSION:                              "SMSG_CLIENTCACHE_VERSION",
	OpcodeSMSG_CLIENT_CONTROL_UPDATE:                            "SMSG_CLIENT_CONTROL_UPDATE",
	OpcodeSMSG_COMBAT_EVENT_FAILED:                              "SMSG_COMBAT_EVENT_FAILED",
	OpcodeSMSG_COMMENTATOR_GET_PLAYER_INFO:                      "SMSG_COMMENTATOR_GET_PLAYER_INFO",
	OpcodeSMSG_COMMENTATOR_MAP_INFO:                             "SMSG_COMMENTATOR_MAP_INFO",
	OpcodeSMSG_COMMENTATOR_PLAYER_INFO:                          "SMSG_COMMENTATOR_PLAYER_INFO",
	OpcodeSMSG_COMMENTATOR_SKIRMISH_QUEUE_RESULT1:               "SMSG_COMMENTATOR_SKIRMISH_QUEUE_RESULT1",
	OpcodeSMSG_COMMENTATOR_SKIRMISH_QUEUE_RESULT2:               "SMSG_COMMENTATOR_SKIRMISH_QUEUE_RESULT2",
	OpcodeSMSG_COMMENTATOR_STATE_CHANGED:                        "SMSG_COMMENTATOR_STATE_CHANGED",
	OpcodeSMSG_COMPLAIN_RESULT:                                  "SMSG_COMPLAIN_RESULT",
	OpcodeSMSG_COMPRESSED_MOVES:                                 "SMSG_COMPRESSED_MOVES",
	OpcodeSMSG_COMPRESSED_UPDATE_OBJECT:                         "SMSG_COMPRESSED_UPDATE_OBJECT",
	OpcodeSMSG_COMSAT_CONNECT_FAIL:                              "SMSG_COMSAT_CONNECT_FAIL",
	OpcodeSMSG_COMSAT_DISCONNECT:                                "SMSG_COMSAT_DISCONNECT",
	OpcodeSMSG_COMSAT_RECONNECT_TRY:                             "SMSG_COMSAT_RECONNECT_TRY",
	OpcodeSMSG_CONTACT_LIST:                                     "SMSG_CONTACT_LIST",
	OpcodeSMSG_CONVERT_RUNE:                                     "SMSG_CONVERT_RUNE",
	OpcodeSMSG_COOLDOWN_CHEAT:                                   "SMSG_COOLDOWN_CHEAT",
	OpcodeSMSG_COOLDOWN_EVENT:                                   "SMSG_COOLDOWN_EVENT",
	OpcodeSMSG_CORPSE_MAP_POSITION_QUERY_RESPONSE:               "SMSG_CORPSE_MAP_POSITION_QUERY_RESPONSE",
	OpcodeSMSG_CORPSE_NOT_IN_INSTANCE:                           "SMSG_CORPSE_NOT_IN_INSTANCE",
	OpcodeSMSG_CORPSE_RECLAIM_DELAY:                             "SMSG_CORPSE_RECLAIM_DELAY",
	OpcodeSMSG_CREATURE_QUERY_RESPONSE:                          "SMSG_CREATURE_QUERY_RESPONSE",
	OpcodeSMSG_CRITERIA_DELETED:                                 "SMSG_CRITERIA_DELETED",
	OpcodeSMSG_CRITERIA_UPDATE:                                  "SMSG_CRITERIA_UPDATE",
	OpcodeSMSG_CROSSED_INEBRIATION_THRESHOLD:                    "SMSG_CROSSED_INEBRIATION_THRESHOLD",
	OpcodeSMSG_DAMAGE_CALC_LOG:                                  "SMSG_DAMAGE_CALC_LOG",
	OpcodeSMSG_DANCE_QUERY_RESPONSE:                             "SMSG_DANCE_QUERY_RESPONSE",
	OpcodeSMSG_DBLOOKUP:                                         "SMSG_DBLOOKUP",
	OpcodeSMSG_DEATH_RELEASE_LOC:                                "SMSG_DEATH_RELEASE_LOC",
	OpcodeSMSG_DEBUGAURAPROC:                                    "SMSG_DEBUGAURAPROC",
	OpcodeSMSG_DEBUG_AISTATE:                                    "SMSG_DEBUG_AISTATE",
	OpcodeSMSG_DEBUG_LIST_TARGETS:                               "SMSG_DEBUG_LIST_TARGETS",
	OpcodeSMSG_DEBUG_SERVER_GEO:                                 "SMSG_DEBUG_SERVER_GEO",
	OpcodeSMSG_DEFENSE_MESSAGE:                                  "SMSG_DEFENSE_MESSAGE",
	OpcodeSMSG_DESTROY_OBJECT:                                   "SMSG_DESTROY_OBJECT",
	OpcodeSMSG_DESTRUCTIBLE_BUILDING_DAMAGE:                     "SMSG_DESTRUCTIBLE_BUILDING_DAMAGE",
	OpcodeSMSG_DISMOUNT:                                         "SMSG_DISMOUNT",
	OpcodeSMSG_DISMOUNTRESULT:                                   "SMSG_DISMOUNTRESULT",
	OpcodeSMSG_DISPEL_FAILED:                                    "SMSG_DISPEL_FAILED",
	OpcodeSMSG_DUEL_COMPLETE:                                    "SMSG_DUEL_COMPLETE",
	OpcodeSMSG_DUEL_COUNTDOWN:                                   "SMSG_DUEL_COUNTDOWN",
	OpcodeSMSG_DUEL_INBOUNDS:                                    "SMSG_DUEL_INBOUNDS",
	OpcodeSMSG_DUEL_OUTOFBOUNDS:                                 "SMSG_DUEL_OUTOFBOUNDS",
	OpcodeSMSG_DUEL_REQUESTED:                                   "SMSG_DUEL_REQUESTED",
	OpcodeSMSG_DUEL_WINNER:                                      "SMSG_DUEL_WINNER",
	OpcodeSMSG_DUMP_OBJECTS_DATA:                                "SMSG_DUMP_OBJECTS_DATA",
	OpcodeSMSG_DURABILITY_DAMAGE_DEATH:                          "SMSG_DURABILITY_DAMAGE_DEATH",
	OpcodeSMSG_DYNAMIC_DROP_ROLL_RESULT:                         "SMSG_DYNAMIC_DROP_ROLL_RESULT",
	OpcodeSMSG_ECHO_PARTY_SQUELCH:                               "SMSG_ECHO_PARTY_SQUELCH",
	OpcodeSMSG_EMOTE:                                            "SMSG_EMOTE",
	OpcodeSMSG_ENABLE_BARBER_SHOP:                               "SMSG_ENABLE_BARBER_SHOP",
	OpcodeSMSG_ENCHANTMENTLOG:                                   "SMSG_ENCHANTMENTLOG",
	OpcodeSMSG_ENVIRONMENTAL_DAMAGE_LOG:                         "SMSG_ENVIRONMENTAL_DAMAGE_LOG",
	OpcodeSMSG_EQUIPMENT_SET_LIST:                               "SMSG_EQUIPMENT_SET_LIST",
	OpcodeSMSG_EQUIPMENT_SET_SAVED:                              "SMSG_EQUIPMENT_SET_SAVED",
	OpcodeSMSG_EQUIPMENT_SET_USE_RESULT:                         "SMSG_EQUIPMENT_SET_USE_RESULT",
	OpcodeSMSG_EXPECTED_SPAM_RECORDS:                            "SMSG_EXPECTED_SPAM_RECORDS",
	OpcodeSMSG_EXPLORATION_EXPERIENCE:                           "SMSG_EXPLORATION_EXPERIENCE",
	OpcodeSMSG_FEATURE_SYSTEM_STATUS:                            "SMSG_FEATURE_SYSTEM_STATUS",
	OpcodeSMSG_FEIGN_DEATH_RESISTED:                             "SMSG_FEIGN_DEATH_RESISTED",
	OpcodeSMSG_FISH_ESCAPED:                                     "SMSG_FISH_ESCAPED",
	OpcodeSMSG_FISH_NOT_HOOKED:                                  "SMSG_FISH_NOT_HOOKED",
	OpcodeSMSG_FLIGHT_SPLINE_SYNC:                               "SMSG_FLIGHT_SPLINE_SYNC",
	OpcodeSMSG_FORCEACTIONSHOW:                                  "SMSG_FORCEACTIONSHOW",
	OpcodeSMSG_FORCED_DEATH_UPDATE:                              "SMSG_FORCED_DEATH_UPDATE",
	OpcodeSMSG_FORCE_ANIM:                                       "SMSG_FORCE_ANIM",
	OpcodeSMSG_FORCE_DISPLAY_UPDATE:                             "SMSG_FORCE_DISPLAY_UPDATE",
	OpcodeSMSG_FORCE_FLIGHT_BACK_SPEED_CHANGE:                   "SMSG_FORCE_FLIGHT_BACK_SPEED_CHANGE",
	OpcodeSMSG_FORCE_FLIGHT_SPEED_CHANGE:                        "SMSG_FORCE_FLIGHT_SPEED_CHANGE",
	OpcodeSMSG_FORCE_MOVE_ROOT:                                  "SMSG_FORCE_MOVE_ROOT",
	OpcodeSMSG_FORCE_MOVE_UNROOT:                                "SMSG_FORCE_MOVE_UNROOT",
	OpcodeSMSG_FORCE_PITCH_RATE_CHANGE:                          "SMSG_FORCE_PITCH_RATE_CHANGE",
	OpcodeSMSG_FORCE_RUN_BACK_SPEED_CHANGE:                      "SMSG_FORCE_RUN_BACK_SPEED_CHANGE",
	OpcodeSMSG_FORCE_RUN_SPEED_CHANGE:                           "SMSG_FORCE_RUN_SPEED_CHANGE",
	OpcodeSMSG_FORCE_SEND_QUEUED_PACKETS:                        "SMSG_FORCE_SEND_QUEUED_PACKETS",
	OpcodeSMSG_FORCE_SET_VEHICLE_REC_ID:                         "SMSG_FORCE_SET_VEHICLE_REC_ID",
	OpcodeSMSG_FORCE_SWIM_BACK_SPEED_CHANGE:                     "SMSG_FORCE_SWIM_BACK_SPEED_CHANGE",
	OpcodeSMSG_FORCE_SWIM_SPEED_CHANGE:                          "SMSG_FORCE_SWIM_SPEED_CHANGE",
	OpcodeSMSG_FORCE_TURN_RATE_CHANGE:                           "SMSG_FORCE_TURN_RATE_CHANGE",
	OpcodeSMSG_FORCE_WALK_SPEED_CHANGE:                          "SMSG_FORCE_WALK_SPEED_CHANGE",
	OpcodeSMSG_FRIEND_STATUS:                                    "SMSG_FRIEND_STATUS",
	OpcodeSMSG_GAMEOBJECT_CUSTOM_ANIM:                           "SMSG_GAMEOBJECT_CUSTOM_ANIM",
	OpcodeSMSG_GAMEOBJECT_DESPAWN_ANIM:                          "SMSG_GAMEOBJECT_DESPAWN_ANIM",
	OpcodeSMSG_GAMEOBJECT_PAGETEXT:                              "SMSG_GAMEOBJECT_PAGETEXT",
	OpcodeSMSG_GAMEOBJECT_QUERY_RESPONSE:                        "SMSG_GAMEOBJECT_QUERY_RESPONSE",
	OpcodeSMSG_GAMEOBJECT_RESET_STATE:                           "SMSG_GAMEOBJECT_RESET_STATE",
	OpcodeSMSG_GAMESPEED_SET:                                    "SMSG_GAMESPEED_SET",
	OpcodeSMSG_GAMETIMEBIAS_SET:                                 "SMSG_GAMETIMEBIAS_SET",
	OpcodeSMSG_GAMETIME_SET:                                     "SMSG_GAMETIME_SET",
	OpcodeSMSG_GAMETIME_UPDATE:                                  "SMSG_GAMETIME_UPDATE",
	OpcodeSMSG_GHOSTEE_GONE:                                     "SMSG_GHOSTEE_GONE",
	OpcodeSMSG_GMRESPONSE_CREATE_TICKET:                         "SMSG_GMRESPONSE_CREATE_TICKET",
	OpcodeSMSG_GMRESPONSE_DB_ERROR:                              "SMSG_GMRESPONSE_DB_ERROR",
	OpcodeSMSG_GMRESPONSE_RECEIVED:                              "SMSG_GMRESPONSE_RECEIVED",
	OpcodeSMSG_GMRESPONSE_STATUS_UPDATE:                         "SMSG_GMRESPONSE_STATUS_UPDATE",
	OpcodeSMSG_GMTICKET_CREATE:                                  "SMSG_GMTICKET_CREATE",
	OpcodeSMSG_GMTICKET_DELETETICKET:                            "SMSG_GMTICKET_DELETETICKET",
	OpcodeSMSG_GMTICKET_GETTICKET:                               "SMSG_GMTICKET_GETTICKET",
	OpcodeSMSG_GMTICKET_SYSTEMSTATUS:                            "SMSG_GMTICKET_SYSTEMSTATUS",
	OpcodeSMSG_GMTICKET_UPDATETEXT:                              "SMSG_GMTICKET_UPDATETEXT",
	OpcodeSMSG_GM_MESSAGECHAT:                                   "SMSG_GM_MESSAGECHAT",
	OpcodeSMSG_GM_PLAYER_INFO:                                   "SMSG_GM_PLAYER_INFO",
	OpcodeSMSG_GM_TICKET_STATUS_UPDATE:                          "SMSG_GM_TICKET_STATUS_UPDATE",
	OpcodeSMSG_GODMODE:                                          "SMSG_GODMODE",
	OpcodeSMSG_GOGOGO_OBSOLETE:                                  "SMSG_GOGOGO_OBSOLETE",
	OpcodeSMSG_GOSSIP_COMPLETE:                                  "SMSG_GOSSIP_COMPLETE",
	OpcodeSMSG_GOSSIP_MESSAGE:                                   "SMSG_GOSSIP_MESSAGE",
	OpcodeSMSG_GOSSIP_POI:                                       "SMSG_GOSSIP_POI",
	OpcodeSMSG_GROUPACTION_THROTTLED:                            "SMSG_GROUPACTION_THROTTLED",
	OpcodeSMSG_GROUP_CANCEL:                                     "SMSG_GROUP_CANCEL",
	OpcodeSMSG_GROUP_DECLINE:                                    "SMSG_GROUP_DECLINE",
	OpcodeSMSG_GROUP_DESTROYED:                                  "SMSG_GROUP_DESTROYED",
	OpcodeSMSG_GROUP_INVITE:                                     "SMSG_GROUP_INVITE",
	OpcodeSMSG_GROUP_JOINED_BATTLEGROUND:                        "SMSG_GROUP_JOINED_BATTLEGROUND",
	OpcodeSMSG_GROUP_LIST:                                       "SMSG_GROUP_LIST",
	OpcodeSMSG_GROUP_SET_LEADER:                                 "SMSG_GROUP_SET_LEADER",
	OpcodeSMSG_GROUP_UNINVITE:                                   "SMSG_GROUP_UNINVITE",
	OpcodeSMSG_GUILD_BANK_LIST:                                  "SMSG_GUILD_BANK_LIST",
	OpcodeSMSG_GUILD_COMMAND_RESULT:                             "SMSG_GUILD_COMMAND_RESULT",
	OpcodeSMSG_GUILD_DECLINE:                                    "SMSG_GUILD_DECLINE",
	OpcodeSMSG_GUILD_EVENT:                                      "SMSG_GUILD_EVENT",
	OpcodeSMSG_GUILD_INFO:                                       "SMSG_GUILD_INFO",
	OpcodeSMSG_GUILD_INVITE:                                     "SMSG_GUILD_INVITE",
	OpcodeSMSG_GUILD_QUERY_RESPONSE:                             "SMSG_GUILD_QUERY_RESPONSE",
	OpcodeSMSG_GUILD_ROSTER:                                     "SMSG_GUILD_ROSTER",
	OpcodeSMSG_HEALTH_UPDATE:                                    "SMSG_HEALTH_UPDATE",
	OpcodeSMSG_HIGHEST_THREAT_UPDATE:                            "SMSG_HIGHEST_THREAT_UPDATE",
	OpcodeSMSG_IGNORE_DIMINISHING_RETURNS_CHEAT:                 "SMSG_IGNORE_DIMINISHING_RETURNS_CHEAT",
	OpcodeSMSG_IGNORE_REQUIREMENTS_CHEAT:                        "SMSG_IGNORE_REQUIREMENTS_CHEAT",
	OpcodeSMSG_INITIALIZE_FACTIONS:                              "SMSG_INITIALIZE_FACTIONS",
	OpcodeSMSG_INITIAL_SPELLS:                                   "SMSG_INITIAL_SPELLS",
	OpcodeSMSG_INIT_EXTRA_AURA_INFO_OBSOLETE:                    "SMSG_INIT_EXTRA_AURA_INFO_OBSOLETE",
	OpcodeSMSG_INIT_WORLD_STATES:                                "SMSG_INIT_WORLD_STATES",
	OpcodeSMSG_INSPECT_RESULTS_UPDATE:                           "SMSG_INSPECT_RESULTS_UPDATE",
	OpcodeSMSG_INSPECT_TALENT:                                   "SMSG_INSPECT_TALENT",
	OpcodeSMSG_INSTANCE_DIFFICULTY:                              "SMSG_INSTANCE_DIFFICULTY",
	OpcodeSMSG_INSTANCE_LOCK_WARNING_QUERY:                      "SMSG_INSTANCE_LOCK_WARNING_QUERY",
	OpcodeSMSG_INSTANCE_RESET:                                   "SMSG_INSTANCE_RESET",
	OpcodeSMSG_INSTANCE_RESET_FAILED:                            "SMSG_INSTANCE_RESET_FAILED",
	OpcodeSMSG_INSTANCE_SAVE_CREATED:                            "SMSG_INSTANCE_SAVE_CREATED",
	OpcodeSMSG_INVALIDATE_DANCE:                                 "SMSG_INVALIDATE_DANCE",
	OpcodeSMSG_INVALIDATE_PLAYER:                                "SMSG_INVALIDATE_PLAYER",
	OpcodeSMSG_INVALID_PROMOTION_CODE:                           "SMSG_INVALID_PROMOTION_CODE",
	OpcodeSMSG_INVENTORY_CHANGE_FAILURE:                         "SMSG_INVENTORY_CHANGE_FAILURE",
	OpcodeSMSG_ITEM_COOLDOWN:                                    "SMSG_ITEM_COOLDOWN",
	OpcodeSMSG_ITEM_ENCHANT_TIME_UPDATE:                         "SMSG_ITEM_ENCHANT_TIME_UPDATE",
	OpcodeSMSG_ITEM_NAME_QUERY_RESPONSE:                         "SMSG_ITEM_NAME_QUERY_RESPONSE",
	OpcodeSMSG_ITEM_PUSH_RESULT:                                 "SMSG_ITEM_PUSH_RESULT",
	OpcodeSMSG_ITEM_QUERY_MULTIPLE_RESPONSE:                     "SMSG_ITEM_QUERY_MULTIPLE_RESPONSE",
	OpcodeSMSG_ITEM_QUERY_SINGLE_RESPONSE:                       "SMSG_ITEM_QUERY_SINGLE_RESPONSE",
	OpcodeSMSG_ITEM_REFUND_INFO_RESPONSE:                        "SMSG_ITEM_REFUND_INFO_RESPONSE",
	OpcodeSMSG_ITEM_REFUND_RESULT:                               "SMSG_ITEM_REFUND_RESULT",
	OpcodeSMSG_ITEM_TEXT_QUERY_RESPONSE:                         "SMSG_ITEM_TEXT_QUERY_RESPONSE",
	OpcodeSMSG_ITEM_TIME_UPDATE:                                 "SMSG_ITEM_TIME_UPDATE",
	OpcodeSMSG_JOINED_BATTLEGROUND_QUEUE:                        "SMSG_JOINED_BATTLEGROUND_QUEUE",
	OpcodeSMSG_KICK_REASON:                                      "SMSG_KICK_REASON",
	OpcodeSMSG_LEARNED_DANCE_MOVES:                              "SMSG_LEARNED_DANCE_MOVES",
	OpcodeSMSG_LEARNED_SPELL:                                    "SMSG_LEARNED_SPELL",
	OpcodeSMSG_LEVELUP_INFO:                                     "SMSG_LEVELUP_INFO",
	OpcodeSMSG_LFG_BOOT_PROPOSAL_UPDATE:                         "SMSG_LFG_BOOT_PROPOSAL_UPDATE",
	OpcodeSMSG_LFG_DISABLED:                                     "SMSG_LFG_DISABLED",
	OpcodeSMSG_LFG_JOIN_RESULT:                                  "SMSG_LFG_JOIN_RESULT",
	OpcodeSMSG_LFG_OFFER_CONTINUE:                               "SMSG_LFG_OFFER_CONTINUE",
	OpcodeSMSG_LFG_PARTY_INFO:                                   "SMSG_LFG_PARTY_INFO",
	OpcodeSMSG_LFG_PLAYER_INFO:                                  "SMSG_LFG_PLAYER_INFO",
	OpcodeSMSG_LFG_PLAYER_REWARD:                                "SMSG_LFG_PLAYER_REWARD",
	OpcodeSMSG_LFG_PROPOSAL_UPDATE:                              "SMSG_LFG_PROPOSAL_UPDATE",
	OpcodeSMSG_LFG_QUEUE_STATUS:                                 "SMSG_LFG_QUEUE_STATUS",
	OpcodeSMSG_LFG_ROLE_CHECK_UPDATE:                            "SMSG_LFG_ROLE_CHECK_UPDATE",
	OpcodeSMSG_LFG_ROLE_CHOSEN:                                  "SMSG_LFG_ROLE_CHOSEN",
	OpcodeSMSG_LFG_TELEPORT_DENIED:                              "SMSG_LFG_TELEPORT_DENIED",
	OpcodeSMSG_LFG_UPDATE_PARTY:                                 "SMSG_LFG_UPDATE_PARTY",
	OpcodeSMSG_LFG_UPDATE_PLAYER:                                "SMSG_LFG_UPDATE_PLAYER",
	OpcodeSMSG_LFG_UPDATE_SEARCH:                                "SMSG_LFG_UPDATE_SEARCH",
	OpcodeSMSG_LIST_INVENTORY:                                   "SMSG_LIST_INVENTORY",
	OpcodeSMSG_LOGIN_SET_TIME_SPEED:                             "SMSG_LOGIN_SET_TIME_SPEED",
	OpcodeSMSG_LOGIN_VERIFY_WORLD:                               "SMSG_LOGIN_VERIFY_WORLD",
	OpcodeSMSG_LOGOUT_CANCEL_ACK:                                "SMSG_LOGOUT_CANCEL_ACK",
	OpcodeSMSG_LOGOUT_COMPLETE:                                  "SMSG_LOGOUT_COMPLETE",
	OpcodeSMSG_LOGOUT_RESPONSE:                                  "SMSG_LOGOUT_RESPONSE",
	OpcodeSMSG_LOG_XPGAIN:                                       "SMSG_LOG_XPGAIN",
	OpcodeSMSG_LOOT_ALL_PASSED:                                  "SMSG_LOOT_ALL_PASSED",
	OpcodeSMSG_LOOT_CLEAR_MONEY:                                 "SMSG_LOOT_CLEAR_MONEY",
	OpcodeSMSG_LOOT_ITEM_NOTIFY:                                 "SMSG_LOOT_ITEM_NOTIFY",
	OpcodeSMSG_LOOT_LIST:                                        "SMSG_LOOT_LIST",
	OpcodeSMSG_LOOT_MASTER_LIST:                                 "SMSG_LOOT_MASTER_LIST",
	OpcodeSMSG_LOOT_MONEY_NOTIFY:                                "SMSG_LOOT_MONEY_NOTIFY",
	OpcodeSMSG_LOOT_RELEASE_RESPONSE:                            "SMSG_LOOT_RELEASE_RESPONSE",
	OpcodeSMSG_LOOT_REMOVED:                                     "SMSG_LOOT_REMOVED",
	OpcodeSMSG_LOOT_RESPONSE:                                    "SMSG_LOOT_RESPONSE",
	OpcodeSMSG_LOOT_ROLL:                                        "SMSG_LOOT_ROLL",
	OpcodeSMSG_LOOT_ROLL_WON:                                    "SMSG_LOOT_ROLL_WON",
	OpcodeSMSG_LOOT_SLOT_CHANGED:                                "SMSG_LOOT_SLOT_CHANGED",
	OpcodeSMSG_LOOT_START_ROLL:                                  "SMSG_LOOT_START_ROLL",
	OpcodeSMSG_LOTTERY_QUERY_RESULT_OBSOLETE:                    "SMSG_LOTTERY_QUERY_RESULT_OBSOLETE",
	OpcodeSMSG_LOTTERY_RESULT_OBSOLETE:                          "SMSG_LOTTERY_RESULT_OBSOLETE",
	OpcodeSMSG_MAIL_LIST_RESULT:                                 "SMSG_MAIL_LIST_RESULT",
	OpcodeSMSG_MESSAGECHAT:                                      "SMSG_MESSAGECHAT",
	OpcodeSMSG_MINIGAME_MOVE_FAILED:                             "SMSG_MINIGAME_MOVE_FAILED",
	OpcodeSMSG_MINIGAME_SETUP:                                   "SMSG_MINIGAME_SETUP",
	OpcodeSMSG_MINIGAME_STATE:                                   "SMSG_MINIGAME_STATE",
	OpcodeSMSG_MIRRORIMAGE_DATA:                                 "SMSG_MIRRORIMAGE_DATA",
	OpcodeSMSG_MODIFY_COOLDOWN:                                  "SMSG_MODIFY_COOLDOWN",
	OpcodeSMSG_MONSTER_MOVE:                                     "SMSG_MONSTER_MOVE",
	OpcodeSMSG_MONSTER_MOVE_TRANSPORT:                           "SMSG_MONSTER_MOVE_TRANSPORT",
	OpcodeSMSG_MOTD:                                             "SMSG_MOTD",
	OpcodeSMSG_MOUNTSPECIAL_ANIM:                                "SMSG_MOUNTSPECIAL_ANIM",
	OpcodeSMSG_MOUNT_RESULT:                                     "SMSG_MOUNT_RESULT",
	OpcodeSMSG_MOVE_CHARACTER_CHEAT:                             "SMSG_MOVE_CHARACTER_CHEAT",
	OpcodeSMSG_MOVE_FEATHER_FALL:                                "SMSG_MOVE_FEATHER_FALL",
	OpcodeSMSG_MOVE_GRAVITY_DISABLE:                             "SMSG_MOVE_GRAVITY_DISABLE",
	OpcodeSMSG_MOVE_GRAVITY_ENABLE:                              "SMSG_MOVE_GRAVITY_ENABLE",
	OpcodeSMSG_MOVE_KNOCK_BACK:                                  "SMSG_MOVE_KNOCK_BACK",
	OpcodeSMSG_MOVE_LAND_WALK:                                   "SMSG_MOVE_LAND_WALK",
	OpcodeSMSG_MOVE_NORMAL_FALL:                                 "SMSG_MOVE_NORMAL_FALL",
	OpcodeSMSG_MOVE_SET_CAN_FLY:                                 "SMSG_MOVE_SET_CAN_FLY",
	OpcodeSMSG_MOVE_SET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY:     "SMSG_MOVE_SET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY",
	OpcodeSMSG_MOVE_SET_COLLISION_HGT:                           "SMSG_MOVE_SET_COLLISION_HGT",
	OpcodeSMSG_MOVE_SET_HOVER:                                   "SMSG_MOVE_SET_HOVER",
	OpcodeSMSG_MOVE_UNSET_CAN_FLY:                               "SMSG_MOVE_UNSET_CAN_FLY",
	OpcodeSMSG_MOVE_UNSET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY:   "SMSG_MOVE_UNSET_CAN_TRANSITION_BETWEEN_SWIM_AND_FLY",
	OpcodeSMSG_MOVE_UNSET_HOVER:                                 "SMSG_MOVE_UNSET_HOVER",
	OpcodeSMSG_MOVE_WATER_WALK:                                  "SMSG_MOVE_WATER_WALK",
	OpcodeSMSG_MULTIPLE_MOVES:                                   "SMSG_MULTIPLE_MOVES",
	OpcodeSMSG_MULTIPLE_PACKETS:                                 "SMSG_MULTIPLE_PACKETS",
	OpcodeSMSG_NAME_QUERY_RESPONSE:                              "SMSG_NAME_QUERY_RESPONSE",
	OpcodeSMSG_NEW_TAXI_PATH:                                    "SMSG_NEW_TAXI_PATH",
	OpcodeSMSG_NEW_WORLD:                                        "SMSG_NEW_WORLD",
	OpcodeSMSG_NOTIFICATION:                                     "SMSG_NOTIFICATION",
	OpcodeSMSG_NOTIFY_DANCE:                                     "SMSG_NOTIFY_DANCE",
	OpcodeSMSG_NOTIFY_DEST_LOC_SPELL_CAST:                       "SMSG_NOTIFY_DEST_LOC_SPELL_CAST",
	OpcodeSMSG_NPC_TEXT_UPDATE:                                  "SMSG_NPC_TEXT_UPDATE",
	OpcodeSMSG_NPC_WONT_TALK:                                    "SMSG_NPC_WONT_TALK",
	OpcodeSMSG_OFFER_PETITION_ERROR:                             "SMSG_OFFER_PETITION_ERROR",
	OpcodeSMSG_ON_CANCEL_EXPECTED_RIDE_VEHICLE_AURA:             "SMSG_ON_CANCEL_EXPECTED_RIDE_VEHICLE_AURA",
	OpcodeSMSG_OPEN_CONTAINER:                                   "SMSG_OPEN_CONTAINER",
	OpcodeSMSG_OPEN_LFG_DUNGEON_FINDER:                          "SMSG_OPEN_LFG_DUNGEON_FINDER",
	OpcodeSMSG_OVERRIDE_LIGHT:                                   "SMSG_OVERRIDE_LIGHT",
	OpcodeSMSG_PAGE_TEXT_QUERY_RESPONSE:                         "SMSG_PAGE_TEXT_QUERY_RESPONSE",
	OpcodeSMSG_PARTYKILLLOG:                                     "SMSG_PARTYKILLLOG",
	OpcodeSMSG_PARTY_COMMAND_RESULT:                             "SMSG_PARTY_COMMAND_RESULT",
	OpcodeSMSG_PARTY_MEMBER_STATS:                               "SMSG_PARTY_MEMBER_STATS",
	OpcodeSMSG_PARTY_MEMBER_STATS_FULL:                          "SMSG_PARTY_MEMBER_STATS_FULL",
	OpcodeSMSG_PAUSE_MIRROR_TIMER:                               "SMSG_PAUSE_MIRROR_TIMER",
	OpcodeSMSG_PERIODICAURALOG:                                  "SMSG_PERIODICAURALOG",
	OpcodeSMSG_PETGODMODE:                                       "SMSG_PETGODMODE",
	OpcodeSMSG_PETITION_QUERY_RESPONSE:                          "SMSG_PETITION_QUERY_RESPONSE",
	OpcodeSMSG_PETITION_SHOWLIST:                                "SMSG_PETITION_SHOWLIST",
	OpcodeSMSG_PETITION_SHOW_SIGNATURES:                         "SMSG_PETITION_SHOW_SIGNATURES",
	OpcodeSMSG_PETITION_SIGN_RESULTS:                            "SMSG_PETITION_SIGN_RESULTS",
	OpcodeSMSG_PET_ACTION_FEEDBACK:                              "SMSG_PET_ACTION_FEEDBACK",
	OpcodeSMSG_PET_ACTION_SOUND:                                 "SMSG_PET_ACTION_SOUND",
	OpcodeSMSG_PET_BROKEN:                                       "SMSG_PET_BROKEN",
	OpcodeSMSG_PET_CAST_FAILED:                                  "SMSG_PET_CAST_FAILED",
	OpcodeSMSG_PET_DISMISS_SOUND:                                "SMSG_PET_DISMISS_SOUND",
	OpcodeSMSG_PET_GUIDS:                                        "SMSG_PET_GUIDS",
	OpcodeSMSG_PET_LEARNED_SPELL:                                "SMSG_PET_LEARNED_SPELL",
	OpcodeSMSG_PET_MODE:                                         "SMSG_PET_MODE",
	OpcodeSMSG_PET_NAME_INVALID:                                 "SMSG_PET_NAME_INVALID",
	OpcodeSMSG_PET_NAME_QUERY_RESPONSE:                          "SMSG_PET_NAME_QUERY_RESPONSE",
	OpcodeSMSG_PET_RENAMEABLE:                                   "SMSG_PET_RENAMEABLE",
	OpcodeSMSG_PET_SPELLS:                                       "SMSG_PET_SPELLS",
	OpcodeSMSG_PET_TAME_FAILURE:                                 "SMSG_PET_TAME_FAILURE",
	OpcodeSMSG_PET_UNLEARNED_SPELL:                              "SMSG_PET_UNLEARNED_SPELL",
	OpcodeSMSG_PET_UNLEARN_CONFIRM:                              "SMSG_PET_UNLEARN_CONFIRM",
	OpcodeSMSG_PET_UPDATE_COMBO_POINTS:                          "SMSG_PET_UPDATE_COMBO_POINTS",
	OpcodeSMSG_PLAYED_TIME:                                      "SMSG_PLAYED_TIME",
	OpcodeSMSG_PLAYERBINDERROR:                                  "SMSG_PLAYERBINDERROR",
	OpcodeSMSG_PLAYER_BOUND:                                     "SMSG_PLAYER_BOUND",
	OpcodeSMSG_PLAYER_SKINNED:                                   "SMSG_PLAYER_SKINNED",
	OpcodeSMSG_PLAYER_VEHICLE_DATA:                              "SMSG_PLAYER_VEHICLE_DATA",
	OpcodeSMSG_PLAY_DANCE:                                       "SMSG_PLAY_DANCE",
	OpcodeSMSG_PLAY_MUSIC:                                       "SMSG_PLAY_MUSIC",
	OpcodeSMSG_PLAY_OBJECT_SOUND:                                "SMSG_PLAY_OBJECT_SOUND",
	OpcodeSMSG_PLAY_SOUND:                                       "SMSG_PLAY_SOUND",
	OpcodeSMSG_PLAY_SPELL_IMPACT:                                "SMSG_PLAY_SPELL_IMPACT",
	OpcodeSMSG_PLAY_SPELL_VISUAL:                                "SMSG_PLAY_SPELL_VISUAL",
	OpcodeSMSG_PLAY_TIME_WARNING:                                "SMSG_PLAY_TIME_WARNING",
	OpcodeSMSG_PONG:                                             "SMSG_PONG",
	OpcodeSMSG_POWER_UPDATE:                                     "SMSG_POWER_UPDATE",
	OpcodeSMSG_PRE_RESURRECT:                                    "SMSG_PRE_RESURRECT",
	OpcodeSMSG_PROCRESIST:                                       "SMSG_PROCRESIST",
	OpcodeSMSG_PROFILEDATA_RESPONSE:                             "SMSG_PROFILEDATA_RESPONSE",
	OpcodeSMSG_PROPOSE_LEVEL_GRANT:                              "SMSG_PROPOSE_LEVEL_GRANT",
	OpcodeSMSG_PVP_CREDIT:                                       "SMSG_PVP_CREDIT",
	OpcodeSMSG_PVP_QUEUE_STATS:                                  "SMSG_PVP_QUEUE_STATS",
	OpcodeSMSG_QUERY_OBJECT_POSITION:                            "SMSG_QUERY_OBJECT_POSITION",
	OpcodeSMSG_QUERY_OBJECT_ROTATION:                            "SMSG_QUERY_OBJECT_ROTATION",
	OpcodeSMSG_QUERY_QUESTS_COMPLETED_RESPONSE:                  "SMSG_QUERY_QUESTS_COMPLETED_RESPONSE",
	OpcodeSMSG_QUERY_TIME_RESPONSE:                              "SMSG_QUERY_TIME_RESPONSE",
	OpcodeSMSG_QUESTGIVER_QUEST_COMPLETE:                        "SMSG_QUESTGIVER_QUEST_COMPLETE",
	OpcodeSMSG_QUESTGIVER_QUEST_FAILED:                          "SMSG_QUESTGIVER_QUEST_FAILED",
	OpcodeSMSG_QUESTGIVER_QUEST_INVALID:                         "SMSG_QUESTGIVER_QUEST_INVALID",
	OpcodeSMSG_QUESTGIVER_QUEST_LIST:                            "SMSG_QUESTGIVER_QUEST_LIST",
	OpcodeSMSG_QUESTGIVER_REQUEST_ITEMS:                         "SMSG_QUESTGIVER_REQUEST_ITEMS",
	OpcodeSMSG_QUESTGIVER_STATUS:                                "SMSG_QUESTGIVER_STATUS",
	OpcodeSMSG_QUESTGIVER_STATUS_MULTIPLE:                       "SMSG_QUESTGIVER_STATUS_MULTIPLE",
	OpcodeSMSG_QUESTLOG_FULL:                                    "SMSG_QUESTLOG_FULL",
	OpcodeSMSG_QUESTUPDATE_ADD_ITEM:                             "SMSG_QUESTUPDATE_ADD_ITEM",
	OpcodeSMSG_QUESTUPDATE_ADD_KILL:                             "SMSG_QUESTUPDATE_ADD_KILL",
	OpcodeSMSG_QUESTUPDATE_ADD_PVP_KILL:                         "SMSG_QUESTUPDATE_ADD_PVP_KILL",
	OpcodeSMSG_QUESTUPDATE_COMPLETE:                             "SMSG_QUESTUPDATE_COMPLETE",
	OpcodeSMSG_QUESTUPDATE_FAILED:                               "SMSG_QUESTUPDATE_FAILED",
	OpcodeSMSG_QUESTUPDATE_FAILEDTIMER:                          "SMSG_QUESTUPDATE_FAILEDTIMER",
	OpcodeSMSG_QUEST_CONFIRM_ACCEPT:                             "SMSG_QUEST_CONFIRM_ACCEPT",
	OpcodeSMSG_QUEST_FORCE_REMOVE:                               "SMSG_QUEST_FORCE_REMOVE",
	OpcodeSMSG_QUEST_GIVER_OFFER_REWARD_MESSAGE:                 "SMSG_QUEST_GIVER_OFFER_REWARD_MESSAGE",
	OpcodeSMSG_QUEST_GIVER_QUEST_DETAILS:                        "SMSG_QUEST_GIVER_QUEST_DETAILS",
	OpcodeSMSG_QUEST_POI_QUERY_RESPONSE:                         "SMSG_QUEST_POI_QUERY_RESPONSE",
	OpcodeSMSG_QUEST_QUERY_RESPONSE:                             "SMSG_QUEST_QUERY_RESPONSE",
	OpcodeSMSG_RAID_GROUP_ONLY:                                  "SMSG_RAID_GROUP_ONLY",
	OpcodeSMSG_RAID_INSTANCE_INFO:                               "SMSG_RAID_INSTANCE_INFO",
	OpcodeSMSG_RAID_INSTANCE_MESSAGE:                            "SMSG_RAID_INSTANCE_MESSAGE",
	OpcodeSMSG_RAID_READY_CHECK_ERROR:                           "SMSG_RAID_READY_CHECK_ERROR",
	OpcodeSMSG_READ_ITEM_FAILED:                                 "SMSG_READ_ITEM_FAILED",
	OpcodeSMSG_READ_ITEM_OK:                                     "SMSG_READ_ITEM_OK",
	OpcodeSMSG_REALM_SPLIT:                                      "SMSG_REALM_SPLIT",
	OpcodeSMSG_REAL_GROUP_UPDATE:                                "SMSG_REAL_GROUP_UPDATE",
	OpcodeSMSG_RECEIVED_MAIL:                                    "SMSG_RECEIVED_MAIL",
	OpcodeSMSG_REDIRECT_CLIENT:                                  "SMSG_REDIRECT_CLIENT",
	OpcodeSMSG_REFER_A_FRIEND_EXPIRED:                           "SMSG_REFER_A_FRIEND_EXPIRED",
	OpcodeSMSG_REFER_A_FRIEND_FAILURE:                           "SMSG_REFER_A_FRIEND_FAILURE",
	OpcodeSMSG_REMOVED_FROM_PVP_QUEUE:                           "SMSG_REMOVED_FROM_PVP_QUEUE",
	OpcodeSMSG_REMOVED_SPELL:                                    "SMSG_REMOVED_SPELL",
	OpcodeSMSG_REPORT_PVP_AFK_RESULT:                            "SMSG_REPORT_PVP_AFK_RESULT",
	OpcodeSMSG_RESET_FAILED_NOTIFY:                              "SMSG_RESET_FAILED_NOTIFY",
	OpcodeSMSG_RESET_RANGED_COMBAT_TIMER:                        "SMSG_RESET_RANGED_COMBAT_TIMER",
	OpcodeSMSG_RESISTLOG:                                        "SMSG_RESISTLOG",
	OpcodeSMSG_RESPOND_INSPECT_ACHIEVEMENTS:                     "SMSG_RESPOND_INSPECT_ACHIEVEMENTS",
	OpcodeSMSG_RESUME_CAST_BAR:                                  "SMSG_RESUME_CAST_BAR",
	OpcodeSMSG_RESURRECT_FAILED:                                 "SMSG_RESURRECT_FAILED",
	OpcodeSMSG_RESURRECT_REQUEST:                                "SMSG_RESURRECT_REQUEST",
	OpcodeSMSG_RESYNC_RUNES:                                     "SMSG_RESYNC_RUNES",
	OpcodeSMSG_RWHOIS:                                           "SMSG_RWHOIS",
	OpcodeSMSG_SCRIPT_MESSAGE:                                   "SMSG_SCRIPT_MESSAGE",
	OpcodeSMSG_SELL_ITEM:                                        "SMSG_SELL_ITEM",
	OpcodeSMSG_SEND_ALL_COMBAT_LOG:                              "SMSG_SEND_ALL_COMBAT_LOG",
	OpcodeSMSG_SEND_MAIL_RESULT:                                 "SMSG_SEND_MAIL_RESULT",
	OpcodeSMSG_SEND_UNLEARN_SPELLS:                              "SMSG_SEND_UNLEARN_SPELLS",
	OpcodeSMSG_SERVERINFO:                                       "SMSG_SERVERINFO",
	OpcodeSMSG_SERVERTIME:                                       "SMSG_SERVERTIME",
	OpcodeSMSG_SERVER_BUCK_DATA:                                 "SMSG_SERVER_BUCK_DATA",
	OpcodeSMSG_SERVER_BUCK_DATA_START:                           "SMSG_SERVER_BUCK_DATA_START",
	OpcodeSMSG_SERVER_FIRST_ACHIEVEMENT:                         "SMSG_SERVER_FIRST_ACHIEVEMENT",
	OpcodeSMSG_SERVER_INFO_RESPONSE:                             "SMSG_SERVER_INFO_RESPONSE",
	OpcodeSMSG_SET_EXTRA_AURA_INFO_NEED_UPDATE_OBSOLETE:         "SMSG_SET_EXTRA_AURA_INFO_NEED_UPDATE_OBSOLETE",
	OpcodeSMSG_SET_EXTRA_AURA_INFO_OBSOLETE:                     "SMSG_SET_EXTRA_AURA_INFO_OBSOLETE",
	OpcodeSMSG_SET_FACTION_ATWAR:                                "SMSG_SET_FACTION_ATWAR",
	OpcodeSMSG_SET_FACTION_STANDING:                             "SMSG_SET_FACTION_STANDING",
	OpcodeSMSG_SET_FACTION_VISIBLE:                              "SMSG_SET_FACTION_VISIBLE",
	OpcodeSMSG_SET_FLAT_SPELL_MODIFIER:                          "SMSG_SET_FLAT_SPELL_MODIFIER",
	OpcodeSMSG_SET_FORCED_REACTIONS:                             "SMSG_SET_FORCED_REACTIONS",
	OpcodeSMSG_SET_PCT_SPELL_MODIFIER:                           "SMSG_SET_PCT_SPELL_MODIFIER",
	OpcodeSMSG_SET_PHASE_SHIFT:                                  "SMSG_SET_PHASE_SHIFT",
	OpcodeSMSG_SET_PLAYER_DECLINED_NAMES_RESULT:                 "SMSG_SET_PLAYER_DECLINED_NAMES_RESULT",
	OpcodeSMSG_SET_PROFICIENCY:                                  "SMSG_SET_PROFICIENCY",
	OpcodeSMSG_SET_PROJECTILE_POSITION:                          "SMSG_SET_PROJECTILE_POSITION",
	OpcodeSMSG_SHOWTAXINODES:                                    "SMSG_SHOWTAXINODES",
	OpcodeSMSG_SHOW_BANK:                                        "SMSG_SHOW_BANK",
	OpcodeSMSG_SHOW_MAILBOX:                                     "SMSG_SHOW_MAILBOX",
	OpcodeSMSG_SOCKET_GEMS_RESULT:                               "SMSG_SOCKET_GEMS_RESULT",
	OpcodeSMSG_SPELLBREAKLOG:                                    "SMSG_SPELLBREAKLOG",
	OpcodeSMSG_SPELLDAMAGESHIELD:                                "SMSG_SPELLDAMAGESHIELD",
	OpcodeSMSG_SPELLDISPELLOG:                                   "SMSG_SPELLDISPELLOG",
	OpcodeSMSG_SPELLENERGIZELOG:                                 "SMSG_SPELLENERGIZELOG",
	OpcodeSMSG_SPELLHEALLOG:                                     "SMSG_SPELLHEALLOG",
	OpcodeSMSG_SPELLINSTAKILLLOG:                                "SMSG_SPELLINSTAKILLLOG",
	OpcodeSMSG_SPELLLOGEXECUTE:                                  "SMSG_SPELLLOGEXECUTE",
	OpcodeSMSG_SPELLLOGMISS:                                     "SMSG_SPELLLOGMISS",
	OpcodeSMSG_SPELLNONMELEEDAMAGELOG:                           "SMSG_SPELLNONMELEEDAMAGELOG",
	OpcodeSMSG_SPELLORDAMAGE_IMMUNE:                             "SMSG_SPELLORDAMAGE_IMMUNE",
	OpcodeSMSG_SPELLSTEALLOG:                                    "SMSG_SPELLSTEALLOG",
	OpcodeSMSG_SPELL_CHANCE_PROC_LOG:                            "SMSG_SPELL_CHANCE_PROC_LOG",
	OpcodeSMSG_SPELL_CHANCE_RESIST_PUSHBACK:                     "SMSG_SPELL_CHANCE_RESIST_PUSHBACK",
	OpcodeSMSG_SPELL_COOLDOWN:                                   "SMSG_SPELL_COOLDOWN",
	OpcodeSMSG_SPELL_DELAYED:                                    "SMSG_SPELL_DELAYED",
	OpcodeSMSG_SPELL_FAILED_OTHER:                               "SMSG_SPELL_FAILED_OTHER",
	OpcodeSMSG_SPELL_FAILURE:                                    "SMSG_SPELL_FAILURE",
	OpcodeSMSG_SPELL_GO:                                         "SMSG_SPELL_GO",
	OpcodeSMSG_SPELL_START:                                      "SMSG_SPELL_START",
	OpcodeSMSG_SPELL_UPDATE_CHAIN_TARGETS:                       "SMSG_SPELL_UPDATE_CHAIN_TARGETS",
	OpcodeSMSG_SPIRIT_HEALER_CONFIRM:                            "SMSG_SPIRIT_HEALER_CONFIRM",
	OpcodeSMSG_SPLINE_MOVE_FEATHER_FALL:                         "SMSG_SPLINE_MOVE_FEATHER_FALL",
	OpcodeSMSG_SPLINE_MOVE_GRAVITY_DISABLE:                      "SMSG_SPLINE_MOVE_GRAVITY_DISABLE",
	OpcodeSMSG_SPLINE_MOVE_GRAVITY_ENABLE:                       "SMSG_SPLINE_MOVE_GRAVITY_ENABLE",
	OpcodeSMSG_SPLINE_MOVE_LAND_WALK:                            "SMSG_SPLINE_MOVE_LAND_WALK",
	OpcodeSMSG_SPLINE_MOVE_NORMAL_FALL:                          "SMSG_SPLINE_MOVE_NORMAL_FALL",
	OpcodeSMSG_SPLINE_MOVE_ROOT:                                 "SMSG_SPLINE_MOVE_ROOT",
	OpcodeSMSG_SPLINE_MOVE_SET_FLYING:                           "SMSG_SPLINE_MOVE_SET_FLYING",
	OpcodeSMSG_SPLINE_MOVE_SET_HOVER:                            "SMSG_SPLINE_MOVE_SET_HOVER",
	OpcodeSMSG_SPLINE_MOVE_SET_RUN_MODE:                         "SMSG_SPLINE_MOVE_SET_RUN_MODE",
	OpcodeSMSG_SPLINE_MOVE_SET_WALK_MODE:                        "SMSG_SPLINE_MOVE_SET_WALK_MODE",
	OpcodeSMSG_SPLINE_MOVE_START_SWIM:                           "SMSG_SPLINE_MOVE_START_SWIM",
	OpcodeSMSG_SPLINE_MOVE_STOP_SWIM:                            "SMSG_SPLINE_MOVE_STOP_SWIM",
	OpcodeSMSG_SPLINE_MOVE_UNROOT:                               "SMSG_SPLINE_MOVE_UNROOT",
	OpcodeSMSG_SPLINE_MOVE_UNSET_FLYING:                         "SMSG_SPLINE_MOVE_UNSET_FLYING",
	OpcodeSMSG_SPLINE_MOVE_UNSET_HOVER:                          "SMSG_SPLINE_MOVE_UNSET_HOVER",
	OpcodeSMSG_SPLINE_MOVE_WATER_WALK:                           "SMSG_SPLINE_MOVE_WATER_WALK",
	OpcodeSMSG_SPLINE_SET_FLIGHT_BACK_SPEED:                     "SMSG_SPLINE_SET_FLIGHT_BACK_SPEED",
	OpcodeSMSG_SPLINE_SET_FLIGHT_SPEED:                          "SMSG_SPLINE_SET_FLIGHT_SPEED",
	OpcodeSMSG_SPLINE_SET_PITCH_RATE:                            "SMSG_SPLINE_SET_PITCH_RATE",
	OpcodeSMSG_SPLINE_SET_RUN_BACK_SPEED:                        "SMSG_SPLINE_SET_RUN_BACK_SPEED",
	OpcodeSMSG_SPLINE_SET_RUN_SPEED:                             "SMSG_SPLINE_SET_RUN_SPEED",
	OpcodeSMSG_SPLINE_SET_SWIM_BACK_SPEED:                       "SMSG_SPLINE_SET_SWIM_BACK_SPEED",
	OpcodeSMSG_SPLINE_SET_SWIM_SPEED:                            "SMSG_SPLINE_SET_SWIM_SPEED",
	OpcodeSMSG_SPLINE_SET_TURN_RATE:                             "SMSG_SPLINE_SET_TURN_RATE",
	OpcodeSMSG_SPLINE_SET_WALK_SPEED:                            "SMSG_SPLINE_SET_WALK_SPEED",
	OpcodeSMSG_STABLE_RESULT:                                    "SMSG_STABLE_RESULT",
	OpcodeSMSG_STANDSTATE_UPDATE:                                "SMSG_STANDSTATE_UPDATE",
	OpcodeSMSG_START_MIRROR_TIMER:                               "SMSG_START_MIRROR_TIMER",
	OpcodeSMSG_STOP_DANCE:                                       "SMSG_STOP_DANCE",
	OpcodeSMSG_STOP_MIRROR_TIMER:                                "SMSG_STOP_MIRROR_TIMER",
	OpcodeSMSG_SUMMON_CANCEL:                                    "SMSG_SUMMON_CANCEL",
	OpcodeSMSG_SUMMON_REQUEST:                                   "SMSG_SUMMON_REQUEST",
	OpcodeSMSG_SUPERCEDED_SPELL:                                 "SMSG_SUPERCEDED_SPELL",
	OpcodeSMSG_SUSPEND_COMMS:                                    "SMSG_SUSPEND_COMMS",
	OpcodeSMSG_TALENTS_INFO:                                     "SMSG_TALENTS_INFO",
	OpcodeSMSG_TALENTS_INVOLUNTARILY_RESET:                      "SMSG_TALENTS_INVOLUNTARILY_RESET",
	OpcodeSMSG_TAXINODE_STATUS:                                  "SMSG_TAXINODE_STATUS",
	OpcodeSMSG_TEST_DROP_RATE_RESULT:                            "SMSG_TEST_DROP_RATE_RESULT",
	OpcodeSMSG_TEXT_EMOTE:                                       "SMSG_TEXT_EMOTE",
	OpcodeSMSG_THREAT_CLEAR:                                     "SMSG_THREAT_CLEAR",
	OpcodeSMSG_THREAT_REMOVE:                                    "SMSG_THREAT_REMOVE",
	OpcodeSMSG_THREAT_UPDATE:                                    "SMSG_THREAT_UPDATE",
	OpcodeSMSG_TIME_SYNC_REQ:                                    "SMSG_TIME_SYNC_REQ",
	OpcodeSMSG_TITLE_EARNED:                                     "SMSG_TITLE_EARNED",
	OpcodeSMSG_TOGGLE_XP_GAIN:                                   "SMSG_TOGGLE_XP_GAIN",
	OpcodeSMSG_TOTEM_CREATED:                                    "SMSG_TOTEM_CREATED",
	OpcodeSMSG_TRADE_STATUS:                                     "SMSG_TRADE_STATUS",
	OpcodeSMSG_TRADE_STATUS_EXTENDED:                            "SMSG_TRADE_STATUS_EXTENDED",
	OpcodeSMSG_TRAINER_BUY_FAILED:                               "SMSG_TRAINER_BUY_FAILED",
	OpcodeSMSG_TRAINER_BUY_SUCCEEDED:                            "SMSG_TRAINER_BUY_SUCCEEDED",
	OpcodeSMSG_TRAINER_LIST:                                     "SMSG_TRAINER_LIST",
	OpcodeSMSG_TRANSFER_ABORTED:                                 "SMSG_TRANSFER_ABORTED",
	OpcodeSMSG_TRANSFER_PENDING:                                 "SMSG_TRANSFER_PENDING",
	OpcodeSMSG_TRIGGER_CINEMATIC:                                "SMSG_TRIGGER_CINEMATIC",
	OpcodeSMSG_TRIGGER_MOVIE:                                    "SMSG_TRIGGER_MOVIE",
	OpcodeSMSG_TURN_IN_PETITION_RESULTS:                         "SMSG_TURN_IN_PETITION_RESULTS",
	OpcodeSMSG_TUTORIAL_FLAGS:                                   "SMSG_TUTORIAL_FLAGS",
	OpcodeSMSG_UPDATE_ACCOUNT_DATA:                              "SMSG_UPDATE_ACCOUNT_DATA",
	OpcodeSMSG_UPDATE_ACCOUNT_DATA_COMPLETE:                     "SMSG_UPDATE_ACCOUNT_DATA_COMPLETE",
	OpcodeSMSG_UPDATE_COMBO_POINTS:                              "SMSG_UPDATE_COMBO_POINTS",
	OpcodeSMSG_UPDATE_INSTANCE_ENCOUNTER_UNIT:                   "SMSG_UPDATE_INSTANCE_ENCOUNTER_UNIT",
	OpcodeSMSG_UPDATE_INSTANCE_OWNERSHIP:                        "SMSG_UPDATE_INSTANCE_OWNERSHIP",
	OpcodeSMSG_UPDATE_LAST_INSTANCE:                             "SMSG_UPDATE_LAST_INSTANCE",
	OpcodeSMSG_UPDATE_LFG_LIST:                                  "SMSG_UPDATE_LFG_LIST",
	OpcodeSMSG_UPDATE_OBJECT:                                    "SMSG_UPDATE_OBJECT",
	OpcodeSMSG_UPDATE_WORLD_STATE:                               "SMSG_UPDATE_WORLD_STATE",
	OpcodeSMSG_USERLIST_ADD:                                     "SMSG_USERLIST_ADD",
	OpcodeSMSG_USERLIST_REMOVE:                                  "SMSG_USERLIST_REMOVE",
	OpcodeSMSG_USERLIST_UPDATE:                                  "SMSG_USERLIST_UPDATE",
	OpcodeSMSG_VOICESESSION_FULL:                                "SMSG_VOICESESSION_FULL",
	OpcodeSMSG_VOICE_CHAT_STATUS:                                "SMSG_VOICE_CHAT_STATUS",
	OpcodeSMSG_VOICE_PARENTAL_CONTROLS:                          "SMSG_VOICE_PARENTAL_CONTROLS",
	OpcodeSMSG_VOICE_SESSION_ADJUST_PRIORITY:                    "SMSG_VOICE_SESSION_ADJUST_PRIORITY",
	OpcodeSMSG_VOICE_SESSION_ENABLE:                             "SMSG_VOICE_SESSION_ENABLE",
	OpcodeSMSG_VOICE_SESSION_LEAVE:                              "SMSG_VOICE_SESSION_LEAVE",
	OpcodeSMSG_VOICE_SESSION_ROSTER_UPDATE:                      "SMSG_VOICE_SESSION_ROSTER_UPDATE",
	OpcodeSMSG_VOICE_SET_TALKER_MUTED:                           "SMSG_VOICE_SET_TALKER_MUTED",
	OpcodeSMSG_WARDEN_DATA:                                      "SMSG_WARDEN_DATA",
	OpcodeSMSG_WEATHER:                                          "SMSG_WEATHER",
	OpcodeSMSG_WHO:                                              "SMSG_WHO",
	OpcodeSMSG_WHOIS:                                            "SMSG_WHOIS",
	OpcodeSMSG_WORLD_STATE_UI_TIMER_UPDATE:                      "SMSG_WORLD_STATE_UI_TIMER_UPDATE",
	OpcodeSMSG_ZONE_MAP:                                         "SMSG_ZONE_MAP",
	OpcodeSMSG_ZONE_UNDER_ATTACK:                                "SMSG_ZONE_UNDER_ATTACK",
	OpcodeUMSG_DELETE_GUILD_CHARTER:                             "UMSG_DELETE_GUILD_CHARTER",
	OpcodeUMSG_UPDATE_GROUP_INFO:                                "UMSG_UPDATE_GROUP_INFO",
	OpcodeUMSG_UPDATE_GROUP_MEMBERS:                             "UMSG_UPDATE_GROUP_MEMBERS",
	OpcodeUMSG_UPDATE_GUILD:                                     "UMSG_UPDATE_GUILD",
}
