from __future__ import annotations

from dataclasses import dataclass
from typing import Sequence


@dataclass(frozen=True)
class StackClassificationRule:
    label: str
    contains: tuple[str, ...]


DEFAULT_STACK_CLASSIFICATION_RULES: tuple[StackClassificationRule, ...] = (
    StackClassificationRule("Oracle JDBC", ("oracle.jdbc",)),
    StackClassificationRule("Spring Batch", ("springframework.batch",)),
    StackClassificationRule("Spring Framework", ("springframework",)),
    StackClassificationRule("HTTP / Network", ("socket", "http")),
    StackClassificationRule("Node.js", ("node:", "node_modules", "v8::", "uv_")),
    StackClassificationRule("Python", ("python", "site-packages", ".py:")),
    StackClassificationRule("Go", ("runtime.", "goroutine", ".go:")),
    StackClassificationRule("ASP.NET / .NET", ("system.", "microsoft.", ".dll")),
    StackClassificationRule("JVM", ("java.", "javax.", "jdk.", "sun.")),
)


def classify_stack(
    stack: str,
    rules: Sequence[StackClassificationRule] = DEFAULT_STACK_CLASSIFICATION_RULES,
) -> str:
    lowered = stack.lower()
    for rule in rules:
        if any(token in lowered for token in rule.contains):
            return rule.label
    return "Application"
