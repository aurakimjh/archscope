from __future__ import annotations

from abc import ABC, abstractmethod
import asyncio
from dataclasses import dataclass
from datetime import datetime, timezone
import json
from typing import Any
from urllib.error import URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen

from archscope_engine.ai_interpretation.evidence import EvidenceRegistry
from archscope_engine.ai_interpretation.prompting import PromptPayload
from archscope_engine.ai_interpretation.validation import AiFindingValidator
from archscope_engine.models.analysis_result import AnalysisResult


class LocalLlmPolicyError(ValueError):
    pass


class LocalLlmExecutionError(RuntimeError):
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
    request_options: dict[str, Any] | None = None


@dataclass(frozen=True)
class LocalLlmAvailability:
    available: bool
    reason: str | None = None
    models: list[str] | None = None


def validate_local_llm_config(config: LocalLlmConfig) -> None:
    if not config.enabled:
        return

    if config.provider != "ollama":
        raise LocalLlmPolicyError("Only the ollama local LLM provider is supported.")
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


class LocalLlmClient(ABC):
    @abstractmethod
    def execute(
        self,
        prompt: PromptPayload,
        *,
        source_result: AnalysisResult | dict[str, Any],
        registry: EvidenceRegistry,
    ) -> dict[str, Any]:
        """Execute the local model and return a validated InterpretationResult."""

    async def execute_async(
        self,
        prompt: PromptPayload,
        *,
        source_result: AnalysisResult | dict[str, Any],
        registry: EvidenceRegistry,
    ) -> dict[str, Any]:
        return await asyncio.to_thread(
            self.execute,
            prompt,
            source_result=source_result,
            registry=registry,
        )


class OllamaClient(LocalLlmClient):
    def __init__(self, config: LocalLlmConfig) -> None:
        validate_local_llm_config(config)
        self.config = config

    def execute(
        self,
        prompt: PromptPayload,
        *,
        source_result: AnalysisResult | dict[str, Any],
        registry: EvidenceRegistry,
    ) -> dict[str, Any]:
        if not self.config.enabled:
            return _disabled_interpretation_result(
                self.config,
                prompt,
                source_result,
                reason="AI interpretation is disabled.",
            )

        body: dict[str, Any] = {
            "model": self.config.model,
            "system": prompt.system,
            "prompt": prompt.user,
            "stream": False,
            "format": "json",
        }
        if self.config.request_options:
            body["options"] = self.config.request_options

        request = Request(
            f"{self.config.base_url.rstrip('/')}/api/generate",
            data=json.dumps(body).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            method="POST",
        )

        try:
            with urlopen(request, timeout=self.config.timeout_seconds) as response:
                response_payload = json.loads(response.read().decode("utf-8"))
        except (OSError, URLError, json.JSONDecodeError) as exc:
            raise LocalLlmExecutionError(str(exc)) from exc

        raw_text = response_payload.get("response")
        if not isinstance(raw_text, str) or not raw_text.strip():
            raise LocalLlmExecutionError("Ollama response did not include a JSON response body.")

        interpretation = _load_interpretation_json(raw_text)
        interpretation = _with_interpretation_envelope(
            interpretation,
            self.config,
            prompt,
            source_result,
        )
        return AiFindingValidator(registry).validate_interpretation(interpretation)


def _load_interpretation_json(value: str) -> dict[str, Any]:
    text = value.strip()
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        start = text.find("{")
        end = text.rfind("}")
        if start < 0 or end <= start:
            raise LocalLlmExecutionError("Ollama response was not valid JSON.")
        try:
            payload = json.loads(text[start : end + 1])
        except json.JSONDecodeError as exc:
            raise LocalLlmExecutionError("Ollama response was not valid JSON.") from exc

    if not isinstance(payload, dict):
        raise LocalLlmExecutionError("Ollama interpretation JSON must be an object.")
    return payload


def _with_interpretation_envelope(
    payload: dict[str, Any],
    config: LocalLlmConfig,
    prompt: PromptPayload,
    source_result: AnalysisResult | dict[str, Any],
) -> dict[str, Any]:
    result_payload = (
        source_result.to_dict() if isinstance(source_result, AnalysisResult) else source_result
    )
    metadata = result_payload.get("metadata")
    source_schema_version = (
        metadata.get("schema_version") if isinstance(metadata, dict) else None
    )
    return {
        **payload,
        "schema_version": "0.1.0",
        "provider": config.provider,
        "model": config.model,
        "prompt_version": prompt.prompt_version,
        "source_result_type": result_payload.get("type", "unknown"),
        "source_schema_version": source_schema_version or "unknown",
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "disabled": False,
    }


def _disabled_interpretation_result(
    config: LocalLlmConfig,
    prompt: PromptPayload,
    source_result: AnalysisResult | dict[str, Any],
    *,
    reason: str,
) -> dict[str, Any]:
    result = _with_interpretation_envelope({}, config, prompt, source_result)
    return {
        **result,
        "disabled": True,
        "findings": [],
        "disabled_reason": reason,
    }
