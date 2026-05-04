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

from pathlib import Path
from typing import Iterable, Protocol, runtime_checkable

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
    ) -> ThreadDumpBundle:
        plugin = self._select_plugin(path, format_override)
        bundle = plugin.parse(path)
        bundle.dump_index = dump_index
        bundle.dump_label = dump_label or path.name
        return bundle

    def parse_many(
        self,
        paths: Iterable[Path],
        *,
        format_override: str | None = None,
        labels: dict[str, str] | None = None,
    ) -> list[ThreadDumpBundle]:
        """Parse multiple dumps and reject mixed source formats.

        ``format_override`` is honored uniformly — when the caller passes
        an explicit format every file is parsed with that plugin and the
        mixed-format check is skipped (the operator opted in).
        """
        path_list = [Path(p) for p in paths]
        labels = labels or {}
        bundles: list[ThreadDumpBundle] = []
        for index, path in enumerate(path_list):
            bundle = self.parse_one(
                path,
                format_override=format_override,
                dump_index=index,
                dump_label=labels.get(str(path)) or path.name,
            )
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
    return raw.decode("utf-8", errors="replace")


# Importing the default registry separately avoids a circular import: the
# Java jstack plugin lives next door and registers itself on import.
DEFAULT_REGISTRY = ParserRegistry()
