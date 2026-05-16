# Evidence Stitching Example

Run the second-pass stitching analyzer over existing result JSON files:

```bash
archscope-engine stitch analyze \
  --in examples/stitching/access-result.json \
  --in examples/stitching/trace-result.json \
  --in examples/stitching/database-result.json \
  --in examples/stitching/profile-result.json \
  --time-window-seconds 60
```

The sample demonstrates exact `trace_id` stitching, trace/profile linkage from
profile labels, and timestamp-window stitching between service aliases such as
`order-service` and `order-deployment`.
