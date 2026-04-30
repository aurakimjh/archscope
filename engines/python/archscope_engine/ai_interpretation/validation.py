from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from archscope_engine.ai_interpretation.evidence import (
    EvidenceRegistry,
    is_allowed_namespace,
    parse_evidence_ref,
)

VALID_SEVERITIES = {"info", "warning", "critical"}


@dataclass(frozen=True)
class ValidationIssue:
    code: str
    message: str
    finding_id: str | None = None
    evidence_ref: str | None = None


class AiFindingValidationError(ValueError):
    def __init__(self, issues: list[ValidationIssue]) -> None:
        self.issues = issues
        super().__init__("AI interpretation output failed evidence validation.")


class AiFindingValidator:
    def __init__(
        self,
        registry: EvidenceRegistry,
        *,
        min_confidence: float = 0.3,
    ) -> None:
        self.registry = registry
        self.min_confidence = min_confidence

    def validate_interpretation(self, payload: dict[str, Any]) -> dict[str, Any]:
        issues: list[ValidationIssue] = []

        if payload.get("schema_version") != "0.1.0":
            issues.append(ValidationIssue("SCHEMA_VERSION", "schema_version must be 0.1.0."))

        findings = payload.get("findings")
        if not isinstance(findings, list):
            issues.append(ValidationIssue("FINDINGS_REQUIRED", "findings must be a list."))
            raise AiFindingValidationError(issues)

        valid_findings = []
        for finding in findings:
            if not isinstance(finding, dict):
                issues.append(ValidationIssue("FINDING_SHAPE", "Each finding must be an object."))
                continue
            finding_issues = self.validate_finding(finding)
            if finding_issues:
                issues.extend(finding_issues)
            else:
                valid_findings.append(finding)

        if issues:
            raise AiFindingValidationError(issues)

        return {**payload, "findings": valid_findings}

    def validate_finding(self, finding: dict[str, Any]) -> list[ValidationIssue]:
        finding_id = finding.get("id") if isinstance(finding.get("id"), str) else None
        issues: list[ValidationIssue] = []

        for field in ("id", "label", "model", "summary", "reasoning"):
            if not _non_empty_string(finding.get(field)):
                issues.append(
                    ValidationIssue(
                        f"{field.upper()}_REQUIRED",
                        f"{field} must be a non-empty string.",
                        finding_id,
                    )
                )

        if finding.get("generated_by") != "ai":
            issues.append(
                ValidationIssue(
                    "GENERATED_BY_REQUIRED",
                    'generated_by must be "ai".',
                    finding_id,
                )
            )

        if finding.get("severity") not in VALID_SEVERITIES:
            issues.append(
                ValidationIssue(
                    "SEVERITY_INVALID",
                    "severity must be info, warning, or critical.",
                    finding_id,
                )
            )

        confidence = finding.get("confidence")
        if not isinstance(confidence, (int, float)) or not 0 <= confidence <= 1:
            issues.append(
                ValidationIssue(
                    "CONFIDENCE_INVALID",
                    "confidence must be a number between 0 and 1.",
                    finding_id,
                )
            )
        elif confidence < self.min_confidence:
            issues.append(
                ValidationIssue(
                    "CONFIDENCE_TOO_LOW",
                    f"confidence must be at least {self.min_confidence}.",
                    finding_id,
                )
            )

        limitations = finding.get("limitations")
        if not isinstance(limitations, list) or not all(
            isinstance(item, str) for item in limitations
        ):
            issues.append(
                ValidationIssue(
                    "LIMITATIONS_INVALID",
                    "limitations must be a list of strings.",
                    finding_id,
                )
            )

        evidence_refs = finding.get("evidence_refs")
        if not isinstance(evidence_refs, list) or not evidence_refs:
            issues.append(
                ValidationIssue(
                    "EVIDENCE_REFS_REQUIRED",
                    "evidence_refs must be a non-empty list.",
                    finding_id,
                )
            )
            return issues

        evidence_quotes = finding.get("evidence_quotes", {})
        if evidence_quotes is not None and not isinstance(evidence_quotes, dict):
            issues.append(
                ValidationIssue(
                    "EVIDENCE_QUOTES_INVALID",
                    "evidence_quotes must be an object when provided.",
                    finding_id,
                )
            )
            evidence_quotes = {}

        for raw_ref in evidence_refs:
            issues.extend(
                self._validate_evidence_ref(
                    raw_ref,
                    finding_id=finding_id,
                    evidence_quotes=evidence_quotes,
                )
            )

        return issues

    def _validate_evidence_ref(
        self,
        raw_ref: Any,
        *,
        finding_id: str | None,
        evidence_quotes: dict[str, Any],
    ) -> list[ValidationIssue]:
        if not _non_empty_string(raw_ref):
            return [
                ValidationIssue(
                    "EVIDENCE_REF_BLANK",
                    "evidence_ref values must not be blank.",
                    finding_id,
                )
            ]

        evidence_ref = raw_ref.strip()
        parsed = parse_evidence_ref(evidence_ref)
        if parsed is None:
            return [
                ValidationIssue(
                    "EVIDENCE_REF_GRAMMAR",
                    "evidence_ref must follow source:entity:identifier grammar.",
                    finding_id,
                    evidence_ref,
                )
            ]

        if not is_allowed_namespace(parsed):
            return [
                ValidationIssue(
                    "EVIDENCE_REF_NAMESPACE",
                    "evidence_ref namespace is not registered.",
                    finding_id,
                    evidence_ref,
                )
            ]

        item = self.registry.get(evidence_ref)
        if item is None:
            return [
                ValidationIssue(
                    "EVIDENCE_REF_UNKNOWN",
                    "evidence_ref is not present in the input evidence set.",
                    finding_id,
                    evidence_ref,
                )
            ]

        quote = evidence_quotes.get(evidence_ref)
        if isinstance(quote, str) and quote.strip():
            normalized_quote = _normalize(quote)
            normalized_text = _normalize(item.text)
            if normalized_quote not in normalized_text:
                return [
                    ValidationIssue(
                        "EVIDENCE_QUOTE_MISMATCH",
                        "evidence quote is not present in the original evidence text.",
                        finding_id,
                        evidence_ref,
                    )
                ]

        return []


def _non_empty_string(value: Any) -> bool:
    return isinstance(value, str) and bool(value.strip())


def _normalize(value: str) -> str:
    return " ".join(value.split()).casefold()
