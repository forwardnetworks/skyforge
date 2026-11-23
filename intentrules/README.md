# intentrules

A design-time rules layer for netlab topologies. It reads a topology file, checks it against an Infrahub-format schema plus a sidecar of
extra constraints, and reports findings before anything is deployed.

- `internal/intentrules`: the evaluator, the schema (`schema/`), the sidecar (`sidecar/`) and the generated rule reference (`docs/rules.md`).
- `internal/netlabref`, `internal/netlabcaps`, `internal/netlabcapsgen`, `internal/netlabdocs`: the netlab capability and documentation
  lookups the evaluator uses (the docs corpus is the netlab documentation, with the pages of features this repository does not ship removed).
- `cmd/intentrules`: a command line front end (`intentrules inspect FILE...`).

```
go build ./...
go test ./...
go run ./cmd/intentrules inspect topology.yml
```

Plain Go, no other tooling. Direct dependencies: `gopkg.in/yaml.v3`, `github.com/google/jsonschema-go`, `sigs.k8s.io/yaml`.

## Licence

Fair Core License 1.0, Apache-2.0 future licence ([LICENSE](LICENSE)): source-available, not open source. The vendored Infrahub JSON
schema in `internal/intentrules/jsonschema/` keeps its own Apache-2.0 licence and NOTICE.
