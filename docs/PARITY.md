## Parity contract

Parity means matching the reference's observable behavior: configuration, network framing and packet bytes, authentication, persistence, timers, world state, scripts, Lua hooks, commands, and tool output. It is not a textual translation of C++ syntax.

Every first-party reference source area must be mapped to a Go package or a generated equivalent. Unsupported paths are recorded as incomplete and cannot be counted as complete implementation.

## Current inventory

The reference contains 144 common files, 45 database files, 31 shared files, 610 game files, 707 script files, 8 authentication files, 9 worldserver files, 38 tool files, and 9 test files. Its database layer registers 84 login, 457 character, and 71 world prepared statements; the three enum `MAX` values are sentinels, not SQL statements.

## MorenoCore extensions

The pinned reference includes the following required custom behavior:

- SoloLFG enables the LFG manager on player login and bypasses full-party compatibility requirements when active.
- Character creation allows every reference-playable race and class unless expansion, source create data, or configured masks reject the request. The reference defaults also expose the custom character limits and always-max-skill setting.
- Learned 310% flying mounts set `PLAYER_EXTRA_HAS_310_FLYER`; mount selection, druid flight speed, and unlearning recalculate the flag from learned spell effects.
- NPC Bots load appearance, race/class, ownership, roles, specs, equipment, and disabled spells from `characters_npcbot` and the world NPCBot tables, then persist the same update operations.

The Go runtime currently has configuration, protocol, character-session, SoloLFG state, mount-state, and NPCBot persistence foundations for these areas. Full world gameplay, bot class AI, LFG packet handlers, and spell/data-store integration remain open parity work and are not counted as complete until differential tests cover them.
