# Architecture Docs Examples

Generate an evidence-backed architecture documentation package from existing
result JSON files:

```bash
go run ./cmd/archscope-engine architecture-docs draft \
  --in ../../../examples/api-contract/access-result.json \
  --in ../../../examples/api-contract/broker-result.json \
  --out /tmp/architecture-docs.json
```

For richer output, include `api_contract_analysis`, `stitched_evidence`,
`kubernetes_evidence`, trace, database, broker, and profile result JSON files.
