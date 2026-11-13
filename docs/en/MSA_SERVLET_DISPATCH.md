# Servlet dispatch in MSA response time

The MSA response-time breakdown separates servlet dispatch overhead from ordinary
method time. Servlet Dispatch belongs to the `internal` group. The callee's
`HttpServlet.service()` frame wraps controller work; only its self-time enters
this category.

## Classification

Built-in patterns match these case-insensitive substrings:

- `jakarta.servlet.http.HttpServlet.service`
- `javax.servlet.http.HttpServlet.service`

`servletDispatchPatterns` adds patterns to these defaults. Empty entries and
case-insensitive duplicates are removed. Built-in SQL, fetch and external-call
classifications take priority. Dispatch matching applies only to remaining
`METHOD`/`UNKNOWN` events, after Network Prep matching.

## Accounting

```text
Servlet Dispatch = service elapsed − union(all direct child intervals)
Method Time = root response time − covered categories
```

All direct children participate, including ordinary controller methods, SQL and
external calls. Their overlapping intervals are counted once. Controller work
therefore remains ordinary method time; SQL and external calls retain their own
categories. Network Prep uses a different rule: it deducts external-call children
from a caller wrapper. An ordinary custom breakdown rule does not substitute for
dispatch self-time because it does not deduct plain method children.

For example, a 100 ms service frame with controller children covering a union of
80 ms contributes 20 ms of Servlet Dispatch. Valid covered categories plus
remaining method time equal the root duration. If covered time exceeds the root,
existing `NegativeMethodTime` handling clamps remaining method time to zero and
reports the inconsistency; the result does not establish exclusive accounting
for malformed input.

## Stored fields and presentation

| Surface | Identifier |
|---|---|
| Parsed event | `JenniferEventServletDispatch` / `SERVLET_DISPATCH_METHOD` |
| Response-time breakdown | `ServletDispatchMs` / `servlet_dispatch_ms` |
| Body aggregate | `ServletDispatchCumMs`, `ServletDispatchCount` |
| Parser option | `ServletDispatchPatterns` |
| Desktop option | `servletDispatchPatterns` |
| Chart group | `internal` |

The MSA chart displays Servlet Dispatch as a separate slice. Dispatch events are
excluded from ordinary method-hotspot ranking but remain in the containment tree
for child subtraction. Custom breakdown accounting recognizes their dedicated
bucket. Existing results retain their stored values; changing patterns requires
reanalyzing the original profile.

The parser classifier and Jennifer profile aggregator own these semantics. See
[the Korean guide](../ko/MSA_SERVLET_DISPATCH.md) for the equivalent explanation.
