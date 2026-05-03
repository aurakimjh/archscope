# Runtime Classification

Profiler stack classification is configuration-driven at the rule level. The analyzer maps collapsed stack text to a component family before building `series.component_breakdown`.

Rules can be loaded from the packaged JSON resource:

```text
archscope_engine/config/runtime_classification_rules.json
```

The Python API also supports loading a caller-provided JSON file through `load_stack_classification_rules(path)` for packaging spikes and future user-managed rule sets.

## Default Families

Current built-in families:

- Oracle JDBC
- Spring Batch
- Spring Framework
- Node.js
- Python
- Go
- ASP.NET / .NET
- HTTP / Network
- JVM
- Application fallback

Rules are ordered. The first matching rule wins, so specific framework rules should appear before broad runtime rules.

## Rule Format

```json
[
  {
    "label": "Spring Batch",
    "contains": ["springframework.batch"]
  }
]
```

Rule authoring constraints:

- `label` must be a non-empty component family name.
- `contains` must be a non-empty list of lowercase substring tokens.
- Prefer package-qualified tokens such as `java.net.http` over broad words such as `http`.
- Put narrow framework or library rules before broad runtime rules.
- Keep rules substring-only until the UI needs regular expressions.

## Extension Direction

The default configuration file should be included as PyInstaller sidecar data with the engine package. User-editable classification can later layer an external JSON file over the packaged default after packaged resource paths and update policy are stable.
