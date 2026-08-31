A Go-based reimplementation of a 3.3.5a authentication and world server with configurable MySQL/MariaDB and SQLite storage, gameplay systems, scripting, and extraction tooling.

## Status

This repository is being implemented against the MorenoCore4 reference at commit `dcdbc0c5d88eb96f412f69c34bd5b9de2eed5df6`. The product is known internally as MorenoCore. The parity manifest records translated, generated, and pending source areas. The project is not complete until every first-party server, script, and tool behavior has a verified Go implementation.

## Run locally

From the repository root, `go run .` uses SQLite and starts the authentication and world services together. It creates or migrates `auth.db`, `characters.db`, and `world.db` in the current directory when they are absent. Existing database files are opened after schema validation and migration; they are never reset automatically.

The individual services are available with `go run ./server/authserver` and `go run ./server/worldserver`. Use `--config`, `--backend`, and `--data-dir` to select configuration, database backend, and runtime data location. MySQL/MariaDB connection values are supplied through configuration or environment variables and are never embedded in the binaries.

## Database input

The public SQL files contain schema only. Use `go run ./tools/dbtool schema` to derive public schema templates from explicit dump paths, `go run ./tools/dbtool import-sql` to convert local SQL dumps into SQLite files, and `go run ./tools/dbtool verify` to check imported table and row totals. Local dumps and generated databases are ignored by Git.

## Build and test

```text
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

## Compatibility

The target is the wire, persistence, configuration, scripting, and runtime behavior of the checked-out reference. Compatibility work is tracked by subsystem and verified with differential fixtures rather than inferred from successful compilation alone.
