Third-party components and translated behavior retain their original attribution and license obligations.

The reference source includes TrinityCore, MorenoLand modifications, Eluna Lua Engine, Lua 5.2.3, Boost, OpenSSL-related cryptographic behavior, MySQL client behavior, fmt, zlib, bzip2, Argon2, recastnavigation, libmpq, g3dlite, jemalloc, efsw, SFMT, utf8cpp, gsoap, readline, Catch2, and other components under their respective licenses.

The Go implementation does not copy the reference C or C++ dependency trees. Pure-Go replacements or Go ports must be recorded here with their exact module versions and license texts before publication.

github.com/JoshVarga/blast v0.0.0-20210808061142-eadad17358e8 is used by `tools/mpq` for PKWARE Data Compression Library (DCL) explode decoding. The upstream project attributes its implementation to Mark Adler's zlib blast decoder and Ladislav Zezula's StormLib implode implementation and distributes its source under the permissive notice reproduced in that module's LICENSE/README.

The adaptive MPQ Huffman decoder in `tools/mpq/huffman.go` and `tools/mpq/huffman_tables.go` is a Go port derived from `github.com/ldmonster/go-stormlib` v0.1.0, itself derived from StormLib's Huffman codec. It is distributed under Apache License 2.0; source: https://github.com/ldmonster/go-stormlib/tree/v0.1.0/internal/compress/huffman.
