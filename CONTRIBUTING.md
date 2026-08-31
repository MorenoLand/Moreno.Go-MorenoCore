Contributions must preserve reference behavior and include focused tests or differential fixtures for changed paths.

Do not commit database dumps, generated `*.db` files, credentials, logs, client data, private scripts, build output, or absolute local paths. Keep changes scoped to the Go project and document any source-provenance or license impact.

Run `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./...` before submitting a change. A successful build alone is not evidence of protocol or gameplay parity.
