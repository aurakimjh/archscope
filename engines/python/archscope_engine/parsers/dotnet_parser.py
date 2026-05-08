"""ASP.NET / IIS exception + W3C access log parser."""
# ─────────────────────────────────────────────────────────────────────
# [한글] dotnet_parser — .NET 예외 스택 + IIS W3C 액세스 로그 파서.
#
# 책임/목적
#   하나의 텍스트 파일에 (1) ASP.NET 예외 스택과 (2) IIS W3C 형식
#   액세스 로그가 섞여 있을 수 있다는 가정으로 두 종류 record 를
#   동시 추출. RuntimeStackRecord, IisAccessRecord 의 두 리스트 반환.
#
# 알고리즘
#   1) 라인 단위 dispatch.
#   2) `*Exception:` 으로 시작하는 라인이 만나면 새 exception 시작.
#      이후 들여쓰기 라인들은 스택 프레임으로 누적, 빈 라인 만나면 flush.
#   3) `#Fields: ...` 헤더 라인을 만나면 IIS 필드 컬럼명 캡처. 그 후
#      공백 구분 토큰 라인을 IisAccessRecord 로 매핑.
#   4) skip 사유는 diagnostics 에 기록.
#
# parity 주의사항: DOTNET_EXCEPTION_RE 패턴, IIS #Fields prefix,
# 컬럼 이름 정규화가 Go 측 internal/parsers/runtimestack/dotnet.go 와 동일.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.runtime_stack import IisAccessRecord, RuntimeStackRecord


DOTNET_EXCEPTION_RE = re.compile(
    r"^(?P<type>[A-Za-z_][\w.]*Exception)(?::\s*(?P<message>.*))?$"
)
IIS_FIELDS_PREFIX = "#Fields:"


def parse_dotnet_exception_and_iis(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
    debug_log: DebugLogCollector | None = None,
) -> tuple[list[RuntimeStackRecord], list[IisAccessRecord]]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    exceptions: list[RuntimeStackRecord] = []
    iis_records: list[IisAccessRecord] = []
    current_exception: list[str] = []
    fields: list[str] = []

    def flush_exception() -> None:
        if not current_exception:
            return
        record = _parse_exception_block(current_exception)
        if record is not None:
            exceptions.append(record)
            own_diagnostics.parsed_records += 1

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        own_diagnostics.total_lines += 1
        stripped = line.strip()
        if not stripped:
            flush_exception()
            current_exception = []
            continue
        if stripped.startswith(IIS_FIELDS_PREFIX):
            flush_exception()
            current_exception = []
            fields = stripped[len(IIS_FIELDS_PREFIX) :].strip().split()
            continue
        if stripped.startswith("#"):
            continue
        if fields:
            record = _parse_iis_line(stripped, fields)
            if record is not None:
                iis_records.append(record)
                own_diagnostics.parsed_records += 1
                continue
        if DOTNET_EXCEPTION_RE.match(stripped):
            flush_exception()
            current_exception = [stripped]
            continue
        if current_exception and stripped.startswith("at "):
            current_exception.append(stripped)
            continue

        own_diagnostics.add_skipped(
            line_number=line_number,
            reason="UNSUPPORTED_DOTNET_OR_IIS_LINE",
            message="Line did not match .NET exception or IIS W3C access fields.",
            raw_line=line,
        )
        if debug_log is not None:
            debug_log.add_parse_error(
                line_number=line_number,
                reason="UNSUPPORTED_DOTNET_OR_IIS_LINE",
                message="Line did not match .NET exception or IIS W3C access fields.",
                raw_context={"before": None, "target": line, "after": None},
                failed_pattern="DOTNET_EXCEPTION_OR_IIS_W3C",
                field_shapes=infer_field_shapes(stripped),
            )

    flush_exception()
    return exceptions, iis_records


def _parse_exception_block(block: list[str]) -> RuntimeStackRecord | None:
    header = DOTNET_EXCEPTION_RE.match(block[0])
    if header is None:
        return None
    stack = [line[3:] for line in block[1:] if line.startswith("at ")]
    error_type = header.group("type")
    top_frame = stack[0] if stack else "(no-frame)"
    return RuntimeStackRecord(
        runtime="dotnet",
        record_type=error_type,
        headline=error_type,
        message=header.group("message"),
        stack=stack,
        signature=f"{error_type}|{top_frame}",
        raw_block="\n".join(block),
    )


def _parse_iis_line(line: str, fields: list[str]) -> IisAccessRecord | None:
    values = line.split()
    if len(values) < len(fields):
        return None
    row = dict(zip(fields, values))
    method = row.get("cs-method")
    uri = row.get("cs-uri-stem")
    status = _int(row.get("sc-status"))
    if method is None or uri is None or status is None:
        return None
    return IisAccessRecord(
        method=method,
        uri=uri,
        status=status,
        time_taken_ms=_int(row.get("time-taken")),
        raw_line=line,
    )


def _int(value: str | None) -> int | None:
    if value is None or value == "-":
        return None
    try:
        return int(value)
    except ValueError:
        return None
