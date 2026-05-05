from __future__ import annotations

import json
from dataclasses import dataclass
from importlib import resources
from pathlib import Path
from typing import Sequence


@dataclass(frozen=True)
class StackClassificationRule:
    label: str
    contains: tuple[str, ...]


BUILTIN_STACK_CLASSIFICATION_RULES: tuple[StackClassificationRule, ...] = (
    StackClassificationRule("Oracle JDBC", ("oracle.jdbc",)),
    StackClassificationRule("Spring Batch", ("springframework.batch",)),
    StackClassificationRule("Spring Framework", ("springframework",)),
    StackClassificationRule("Node.js", ("node:", "node_modules", "v8::", "uv_")),
    StackClassificationRule("Python", ("python", "site-packages", ".py:")),
    StackClassificationRule("Go", ("runtime.", "goroutine", ".go:")),
    StackClassificationRule(
        "ASP.NET / .NET",
        ("system.web", "system.net", "microsoft.", ".dll"),
    ),
    StackClassificationRule(
        "HTTP / Network",
        (
            "java.net.socket",
            "java.net.http",
            "sun.nio.ch.socketchannel",
            "okhttp3.",
            "org.apache.http.",
            "http.client",
            "urllib3.",
            "requests.sessions",
            "net/http",
            "system.net.http",
        ),
    ),
    StackClassificationRule("JVM", ("java.", "javax.", "jdk.", "sun.")),
)

CONFIG_PACKAGE = "archscope_engine.config"
CONFIG_RESOURCE = "runtime_classification_rules.json"


def classify_stack(
    stack: str,
    rules: Sequence[StackClassificationRule] | None = None,
) -> str:
    active_rules = rules if rules is not None else DEFAULT_STACK_CLASSIFICATION_RULES
    lowered = stack.casefold()
    for rule in active_rules:
        if any(token.casefold() in lowered for token in rule.contains):
            return rule.label
    return "Application"


def load_stack_classification_rules(path: Path) -> tuple[StackClassificationRule, ...]:
    with path.open("r", encoding="utf-8") as file:
        return parse_stack_classification_rules(json.load(file))


def load_packaged_stack_classification_rules() -> tuple[StackClassificationRule, ...]:
    config_file = resources.files(CONFIG_PACKAGE).joinpath(CONFIG_RESOURCE)
    with config_file.open("r", encoding="utf-8") as file:
        return parse_stack_classification_rules(json.load(file))


def parse_stack_classification_rules(
    value: object,
) -> tuple[StackClassificationRule, ...]:
    if not isinstance(value, list):
        raise ValueError("Runtime classification rules must be a JSON array.")

    rules: list[StackClassificationRule] = []
    for index, item in enumerate(value):
        if not isinstance(item, dict):
            raise ValueError(f"Rule at index {index} must be an object.")

        label = item.get("label")
        contains = item.get("contains")
        if not isinstance(label, str) or not label.strip():
            raise ValueError(f"Rule at index {index} must include a non-empty label.")

        if not isinstance(contains, list) or not contains:
            raise ValueError(f"Rule at index {index} must include contains tokens.")

        tokens: list[str] = []
        for token in contains:
            if not isinstance(token, str) or not token.strip():
                raise ValueError(
                    f"Rule at index {index} contains an invalid token."
                )
            tokens.append(token.lower())

        rules.append(StackClassificationRule(label=label, contains=tuple(tokens)))

    return tuple(rules)


DEFAULT_STACK_CLASSIFICATION_RULES = load_packaged_stack_classification_rules()
