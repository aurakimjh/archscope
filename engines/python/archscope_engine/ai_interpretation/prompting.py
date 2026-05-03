from __future__ import annotations

from dataclasses import dataclass
from importlib import resources
import json
import re
from typing import Any

from archscope_engine.ai_interpretation.evidence import EvidenceItem, EvidenceRegistry
from archscope_engine.ai_interpretation.privacy import redact_sensitive_text
from archscope_engine.models.analysis_result import AnalysisResult

PROMPT_VERSION = "ai-interpretation-v1"
CONFIG_PACKAGE = "archscope_engine.config"
PROMPT_TEMPLATE_RESOURCE = "prompt_templates.json"
SUSPICIOUS_INSTRUCTION_PATTERN = re.compile(
    r"(?i)\b(ignore|forget|override|disregard)\b.{0,80}\b(instruction|prompt|system|previous)\b"
)


@dataclass(frozen=True)
class EvidenceSelection:
    items: list[EvidenceItem]
    omitted_count: int
    total_chars: int
    suspicious_instruction_refs: list[str]


@dataclass(frozen=True)
class PromptPayload:
    prompt_version: str
    system: str
    user: str
    evidence_refs: list[str]
    logging_allowed: bool = False


@dataclass(frozen=True)
class PromptTemplate:
    model_profile: str
    prompt_version: str
    languages: dict[str, dict[str, str]]


class EvidenceSelector:
    def __init__(
        self,
        *,
        max_items: int = 20,
        max_chars_per_item: int = 800,
        max_total_chars: int = 6000,
    ) -> None:
        self.max_items = max_items
        self.max_chars_per_item = max_chars_per_item
        self.max_total_chars = max_total_chars

    def select(self, registry: EvidenceRegistry) -> EvidenceSelection:
        selected: list[EvidenceItem] = []
        total_chars = 0
        suspicious_refs: list[str] = []

        for item in registry.items():
            if len(selected) >= self.max_items or total_chars >= self.max_total_chars:
                break

            text = redact_sensitive_text(item.text)
            text = text[: self.max_chars_per_item]
            remaining = self.max_total_chars - total_chars
            if len(text) > remaining:
                text = text[:remaining]

            if not text:
                continue

            if SUSPICIOUS_INSTRUCTION_PATTERN.search(text):
                suspicious_refs.append(item.ref)

            selected.append(
                EvidenceItem(
                    ref=item.ref,
                    source_type=item.source_type,
                    entity_type=item.entity_type,
                    identifier=item.identifier,
                    text=text,
                    source_file=item.source_file,
                    location=item.location,
                    payload=item.payload,
                )
            )
            total_chars += len(text)

        return EvidenceSelection(
            items=selected,
            omitted_count=max(len(registry.items()) - len(selected), 0),
            total_chars=total_chars,
            suspicious_instruction_refs=suspicious_refs,
        )


class PromptBuilder:
    def __init__(
        self,
        *,
        response_language: str = "en",
        model_profile: str = "default",
        templates: list[PromptTemplate] | None = None,
    ) -> None:
        self.response_language = response_language
        self.model_profile = model_profile
        self.templates = (
            templates if templates is not None else load_packaged_prompt_templates()
        )

    def build(
        self,
        result: AnalysisResult | dict[str, Any],
        selection: EvidenceSelection,
    ) -> PromptPayload:
        payload = result.to_dict() if isinstance(result, AnalysisResult) else result
        result_metadata = {
            "type": payload.get("type"),
            "created_at": payload.get("created_at"),
            "schema_version": (
                payload.get("metadata", {}).get("schema_version")
                if isinstance(payload.get("metadata"), dict)
                else None
            ),
            "summary": payload.get("summary"),
        }
        evidence_rows = [
            {
                "evidence_ref": item.ref,
                "source_file": item.source_file,
                "location": item.location,
                "text": item.text,
            }
            for item in selection.items
        ]
        user_data = {
            "result_metadata": result_metadata,
            "evidence": evidence_rows,
            "omitted_evidence_count": selection.omitted_count,
            "suspicious_instruction_refs": selection.suspicious_instruction_refs,
        }
        template = select_prompt_template(self.templates, self.model_profile)
        language_template = select_language_template(template, self.response_language)
        diagnostic_data = json.dumps(user_data, ensure_ascii=False, indent=2)
        return PromptPayload(
            prompt_version=template.prompt_version,
            system=language_template["system"],
            user=language_template["user"].format(diagnostic_data=diagnostic_data),
            evidence_refs=[item.ref for item in selection.items],
            logging_allowed=False,
        )


def load_packaged_prompt_templates() -> list[PromptTemplate]:
    config_file = resources.files(CONFIG_PACKAGE).joinpath(PROMPT_TEMPLATE_RESOURCE)
    with config_file.open("r", encoding="utf-8") as file:
        return parse_prompt_templates(json.load(file))


def parse_prompt_templates(value: object) -> list[PromptTemplate]:
    if not isinstance(value, list) or not value:
        raise ValueError("Prompt templates must be a non-empty JSON array.")

    templates: list[PromptTemplate] = []
    for index, item in enumerate(value):
        if not isinstance(item, dict):
            raise ValueError(f"Prompt template at index {index} must be an object.")
        model_profile = item.get("model_profile")
        prompt_version = item.get("prompt_version")
        languages = item.get("languages")
        if not isinstance(model_profile, str) or not model_profile.strip():
            raise ValueError(f"Prompt template at index {index} has invalid model_profile.")
        if not isinstance(prompt_version, str) or not prompt_version.strip():
            raise ValueError(f"Prompt template at index {index} has invalid prompt_version.")
        if not isinstance(languages, dict) or not languages:
            raise ValueError(f"Prompt template at index {index} must define languages.")

        parsed_languages: dict[str, dict[str, str]] = {}
        for language, language_template in languages.items():
            if not isinstance(language, str) or not isinstance(language_template, dict):
                raise ValueError(
                    f"Prompt template at index {index} has invalid language entry."
                )
            system = language_template.get("system")
            user = language_template.get("user")
            if not isinstance(system, str) or not system.strip():
                raise ValueError(
                    f"Prompt template {model_profile}/{language} has invalid system."
                )
            if (
                not isinstance(user, str)
                or not user.strip()
                or "{diagnostic_data}" not in user
            ):
                raise ValueError(
                    f"Prompt template {model_profile}/{language} has invalid user."
                )
            parsed_languages[language] = {"system": system, "user": user}

        templates.append(
            PromptTemplate(
                model_profile=model_profile,
                prompt_version=prompt_version,
                languages=parsed_languages,
            )
        )
    return templates


def select_prompt_template(
    templates: list[PromptTemplate],
    model_profile: str,
) -> PromptTemplate:
    for template in templates:
        if template.model_profile == model_profile:
            return template
    for template in templates:
        if template.model_profile == "default":
            return template
    raise ValueError("Prompt templates must include a default model_profile.")


def select_language_template(
    template: PromptTemplate,
    response_language: str,
) -> dict[str, str]:
    language = response_language.lower()
    if language in template.languages:
        return template.languages[language]
    if "en" in template.languages:
        return template.languages["en"]
    return next(iter(template.languages.values()))
