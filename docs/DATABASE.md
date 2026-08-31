## Backends

The server supports MySQL/MariaDB and SQLite through one database contract. The logical databases are auth, characters, and world. The default backend is SQLite and the default files are `auth.db`, `characters.db`, and `world.db` relative to the configured data directory.

Queries use prepared statements and transactions. Dialect-specific SQL is selected by named statement rather than by rewriting arbitrary SQL at runtime. Database updates run in-process and preserve ordered application, version tracking, redundancy checks, archive handling, and failure reporting.

## Local dumps

Use explicit input paths with `tools/dbtool`. No Desktop path, credential, live row, generated database, or dump is embedded in the repository.
