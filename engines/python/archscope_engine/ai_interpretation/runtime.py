from __future__ import annotations

from dataclasses import dataclass
import json
from urllib.error import URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen


class LocalLlmPolicyError(ValueError):
    pass


@dataclass(frozen=True)
class LocalLlmConfig:
    enabled: bool = False
    provider: str = "ollama"
    base_url: str = "http://localhost:11434"
    model: str = "qwen2.5-coder:7b"
    timeout_seconds: float = 30.0
    max_concurrency: int = 1
    log_prompts: bool = False
    log_responses: bool = False


@dataclass(frozen=True)
class LocalLlmAvailability:
    available: bool
    reason: str | None = None
    models: list[str] | None = None


def validate_local_llm_config(config: LocalLlmConfig) -> None:
    if not config.enabled:
        return

    parsed = urlparse(config.base_url)
    if parsed.scheme != "http":
        raise LocalLlmPolicyError("Local LLM base_url must use http.")
    if parsed.hostname not in {"localhost", "127.0.0.1", "::1"}:
        raise LocalLlmPolicyError("Local LLM base_url must resolve to localhost.")
    if config.timeout_seconds <= 0:
        raise LocalLlmPolicyError("Local LLM timeout_seconds must be positive.")
    if config.max_concurrency != 1:
        raise LocalLlmPolicyError("Local LLM max_concurrency is limited to 1 initially.")
    if config.log_prompts or config.log_responses:
        raise LocalLlmPolicyError("Prompt and response logging is disabled by default policy.")


def check_ollama_availability(config: LocalLlmConfig) -> LocalLlmAvailability:
    try:
        validate_local_llm_config(config)
    except LocalLlmPolicyError as exc:
        return LocalLlmAvailability(False, str(exc), [])

    if not config.enabled:
        return LocalLlmAvailability(False, "AI interpretation is disabled.", [])

    request = Request(f"{config.base_url.rstrip('/')}/api/tags")
    try:
        with urlopen(request, timeout=config.timeout_seconds) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except (OSError, URLError, json.JSONDecodeError) as exc:
        return LocalLlmAvailability(False, str(exc), [])

    models = [
        model.get("name")
        for model in payload.get("models", [])
        if isinstance(model, dict) and isinstance(model.get("name"), str)
    ]
    if config.model not in models:
        return LocalLlmAvailability(
            False,
            f"Configured model '{config.model}' is not available.",
            models,
        )

    return LocalLlmAvailability(True, None, models)
