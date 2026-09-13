## Backends

The server supports MySQL/MariaDB and SQLite through one database contract. The logical databases are auth, characters, and world. The default backend is SQLite and the default files are `auth.db`, `characters.db`, and `world.db` relative to the configured data directory.

Queries use prepared statements and transactions. Dialect-specific SQL is selected by named statement rather than by rewriting arbitrary SQL at runtime. Database updates run in-process and preserve ordered application, version tracking, redundancy checks, archive handling, and failure reporting.

## Local dumps

Use explicit input paths with `tools/dbtool`. No Desktop path, credential, live row, generated database, or dump is embedded in the repository.

Run `go run ./tools/dbtool statement-audit --input-dir bin` after importing or migrating local databases. The command prepares every generated statement against its logical SQLite database and reports each failure deterministically. A successful `612/612` preparation result proves SQL/schema compatibility only; exact parameter binding, result shapes, NULL handling, transaction failures, and MySQL/MariaDB behavior remain separate parity work.
