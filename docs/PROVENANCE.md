## Reference

The behavioral authority is the read-only checkout at `G:\Development\Rust\Warcraft\References\Servers\Moreno.TrinityCore`, branch `morenocore4`, commit `dcdbc0c5d88eb96f412f69c34bd5b9de2eed5df6`.

The reference is a 3.3.5a MorenoLand/TrinityCore server tree. The Go repository is independent and does not contain the reference C++ checkout.

## Data boundaries

The local `auth.sql`, `characters.sql`, and `world.sql` dumps are conversion inputs only. Their live rows remain outside the public repository. Public SQL is schema-only and generated databases remain ignored runtime state.
