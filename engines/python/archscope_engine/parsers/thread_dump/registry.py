"""Parser plugin protocol + registry for multi-language thread dumps (T-189).

Each plugin handles a single source format (Java jstack, Go goroutine
dump, Python py-spy, etc.). The registry probes the first 4 KB of every
input file, asks plugins ``can_parse(head)``, and dispatches to the
first match — or to an explicitly requested format.

A bundle is one (file, snapshots) pair. The multi-dump pipeline rejects
inputs whose detected ``source_format`` values disagree because mixing
dumps from different runtimes makes the persistence findings (T-191)
meaningless.
"""
from __future__ import annotations

import inspect
from pathlib import Path
from typing import Any, Callable, Iterable, Protocol, runtime_checkable

from archscope_engine.common.file_utils import detect_text_encoding_from_bytes
from archscope_engine.models.thread_snapshot import ThreadDumpBundle

DETECT_HEAD_BYTES = 4096


class UnknownFormatError(ValueError):
    """No registered plugin claimed the input."""

    def __init__(self, source: str, head_preview: str) -> None:
        super().__init__(
            f"No thread-dump parser plugin recognized {source!r}. "
            f"Header preview: {head_preview[:200]!r}"
        )
        self.source = source
        self.head_preview = head_preview


class MixedFormatError(ValueError):
    """A multi-dump bundle resolved to more than one source format."""

    def __init__(self, formats: dict[str, str]) -> None:
        joined = ", ".join(f"{src}={fmt}" for src, fmt in formats.items())
        super().__init__(
            f"Multi-dump input mixes incompatible source formats: {joined}. "
            "Pass --format to force a single parser if you intentionally want "
            "to coerce one of them."
        )
        self.formats = formats


@runtime_checkable
class ThreadDumpParserPlugin(Protocol):
    """Protocol every concrete parser plugin must satisfy."""

    format_id: str
    language: str

    def can_parse(self, head: str) -> bool:  # pragma: no cover - protocol
        ...

    def parse(self, path: Path) -> ThreadDumpBundle:  # pragma: no cover - protocol
        ...


class ParserRegistry:
    """Collection of plugins with header-sniffing dispatch."""

    def __init__(self) -> None:
        self._plugins: list[ThreadDumpParserPlugin] = []

    def register(self, plugin: ThreadDumpParserPlugin) -> None:
        if not isinstance(plugin, ThreadDumpParserPlugin):
            raise TypeError(
                f"{type(plugin).__name__} does not satisfy ThreadDumpParserPlugin"
            )
        self._plugins.append(plugin)

    @property
    def plugins(self) -> tuple[ThreadDumpParserPlugin, ...]:
        return tuple(self._plugins)

    def get(self, format_id: str) -> ThreadDumpParserPlugin:
        for plugin in self._plugins:
            if plugin.format_id == format_id:
                return plugin
        raise UnknownFormatError(format_id, "")

    def detect_format(self, head: str) -> ThreadDumpParserPlugin | None:
        for plugin in self._plugins:
            if plugin.can_parse(head):
                return plugin
        return None

    def parse_one(
        self,
        path: Path,
        *,
        format_override: str | None = None,
        dump_index: int = 0,
        dump_label: str | None = None,
        parser_options: dict[str, Any] | None = None,
    ) -> ThreadDumpBundle:
        plugin = self._select_plugin(path, format_override)
        bundle = _call_plugin_method(plugin.parse, path, parser_options)
        bundle.dump_index = dump_index
        bundle.dump_label = dump_label or path.name
        return bundle

    def parse_many(
        self,
        paths: Iterable[Path],
        *,
        format_override: str | None = None,
        labels: dict[str, str] | None = None,
        parser_options: dict[str, Any] | None = None,
    ) -> list[ThreadDumpBundle]:
        """Parse multiple dumps and reject mixed source formats.

        ``format_override`` is honored uniformly — when the caller passes
        an explicit format every file is parsed with that plugin and the
        mixed-format check is skipped (the operator opted in).
        """
        path_list = [Path(p) for p in paths]
        labels = labels or {}
        bundles: list[ThreadDumpBundle] = []
        next_dump_index = 0
        for path in path_list:
            plugin = self._select_plugin(path, format_override)
            parsed = _parse_path_bundles(plugin, path, parser_options)
            base_label = labels.get(str(path)) or path.name
            multi_file = len(parsed) > 1
            for local_index, bundle in enumerate(parsed):
                bundle.dump_index = next_dump_index
                if bundle.dump_label is None or bundle.dump_label == path.name:
                    suffix = f"#{local_index + 1}" if multi_file else ""
                    bundle.dump_label = f"{base_label}{suffix}"
                next_dump_index += 1
                bundles.append(bundle)

        if format_override is None and len({b.source_format for b in bundles}) > 1:
            raise MixedFormatError(
                {b.source_file: b.source_format for b in bundles}
            )
        return bundles

    def _select_plugin(
        self,
        path: Path,
        format_override: str | None,
    ) -> ThreadDumpParserPlugin:
        if format_override:
            return self.get(format_override)
        head = _read_head(path)
        plugin = self.detect_format(head)
        if plugin is None:
            raise UnknownFormatError(str(path), head)
        return plugin


def _read_head(path: Path) -> str:
    with path.open("rb") as handle:
        raw = handle.read(DETECT_HEAD_BYTES)
    encoding = detect_text_encoding_from_bytes(raw)
    return raw.decode(encoding, errors="replace")


def _parse_path_bundles(
    plugin: ThreadDumpParserPlugin,
    path: Path,
    parser_options: dict[str, Any] | None = None,
) -> list[ThreadDumpBundle]:
    parse_all = getattr(plugin, "parse_all", None)
    if callable(parse_all):
        bundles = list(_call_plugin_method(parse_all, path, parser_options))
    else:
        bundles = [_call_plugin_method(plugin.parse, path, parser_options)]
    return bundles or [_call_plugin_method(plugin.parse, path, parser_options)]


def _call_plugin_method(
    method: Callable[..., Any],
    path: Path,
    parser_options: dict[str, Any] | None,
) -> Any:
    if not parser_options:
        return method(path)
    signature = inspect.signature(method)
    parameters = signature.parameters
    if any(param.kind is inspect.Parameter.VAR_KEYWORD for param in parameters.values()):
        return method(path, **parser_options)
    accepted = {
        key: value
        for key, value in parser_options.items()
        if key in parameters
    }
    if not accepted:
        return method(path)
    return method(path, **accepted)


# Importing the default registry separately avoids a circular import: the
# Java jstack plugin lives next door and registers itself on import.
DEFAULT_REGISTRY = ParserRegistry()
