# Runtime Classification

Profiler stack classification is configuration-driven at the rule level. The analyzer maps collapsed stack text to a component family before building `series.component_breakdown`.

## Default Families

Current built-in families:

- Oracle JDBC
- Spring Batch
- Spring Framework
- HTTP / Network
- JVM
- Node.js
- Python
- Go
- ASP.NET / .NET
- Application fallback

Rules are ordered. The first matching rule wins, so specific framework rules should appear before broad runtime rules.

## Extension Direction

Future runtime expansion should load additional rules from packaged configuration after the PyInstaller sidecar path is stable. User-editable classification should remain bounded to simple `contains` tokens until the UI needs regular expressions.
