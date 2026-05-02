"""Tests for demo_site_mapping and demo_site_runner modules."""

import json
from pathlib import Path

import pytest

from archscope_engine.demo_site_mapping import (
    AnalyzerTypeMapping,
    command_for_mapping,
    find_analyzer_type_mapping,
    input_option_for_mapping,
    load_analyzer_type_mappings,
)
from archscope_engine.demo_site_runner import (
    DemoAnalyzerRun,
    DemoScenarioRun,
    discover_demo_manifests,
    run_demo_site_manifest,
)


class TestAnalyzerTypeMapping:
    def test_load_mappings_from_json(self, tmp_path: Path) -> None:
        mapping_file = tmp_path / "analyzer_type_mapping.json"
        mapping_file.write_text(
            json.dumps(
                {
                    "mappings": {
                        "access_log": {
                            "command": ["access-log", "analyze"],
                            "input_option": "--file",
                        },
                        "profiler_collapsed": {
                            "command": ["profiler", "analyze-collapsed"],
                            "input_option": "--wall",
                            "format_overrides": {
                                "jennifer_csv": {
                                    "command": ["profiler", "analyze-jennifer-csv"],
                                    "input_option": "--file",
                                }
                            },
                        },
                        "reference_doc": {
                            "command": None,
                            "input_option": None,
                            "note": "Documentation file, not analyzed",
                        },
                    }
                }
            ),
            encoding="utf-8",
        )

        mappings = load_analyzer_type_mappings(tmp_path)

        assert "access_log" in mappings
        assert mappings["access_log"].command == ("access-log", "analyze")
        assert mappings["access_log"].input_option == "--file"

        assert "profiler_collapsed" in mappings
        assert "jennifer_csv" in mappings["profiler_collapsed"].format_overrides

        assert mappings["reference_doc"].command is None
        assert mappings["reference_doc"].note == "Documentation file, not analyzed"

    def test_load_mappings_raises_on_invalid_structure(self, tmp_path: Path) -> None:
        mapping_file = tmp_path / "analyzer_type_mapping.json"
        mapping_file.write_text(json.dumps({"wrong_key": {}}), encoding="utf-8")

        with pytest.raises(ValueError, match="Invalid analyzer type mapping"):
            load_analyzer_type_mappings(tmp_path)

    def test_find_mapping_walks_parent_directories(self, tmp_path: Path) -> None:
        mapping_file = tmp_path / "analyzer_type_mapping.json"
        mapping_file.write_text(
            json.dumps({"mappings": {"a": {"command": ["a"], "input_option": "--f"}}}),
            encoding="utf-8",
        )

        nested = tmp_path / "sub" / "dir"
        nested.mkdir(parents=True)

        found = find_analyzer_type_mapping(nested)
        assert found == mapping_file

    def test_find_mapping_raises_when_not_found(self, tmp_path: Path) -> None:
        with pytest.raises(FileNotFoundError):
            find_analyzer_type_mapping(tmp_path / "nonexistent")

    def test_command_for_mapping_uses_format_override(self) -> None:
        override = AnalyzerTypeMapping(
            analyzer_type="profiler",
            command=("profiler", "analyze-jennifer-csv"),
            input_option="--file",
        )
        mapping = AnalyzerTypeMapping(
            analyzer_type="profiler",
            command=("profiler", "analyze-collapsed"),
            input_option="--wall",
            format_overrides={"jennifer_csv": override},
        )

        assert command_for_mapping(mapping) == ("profiler", "analyze-collapsed")
        assert command_for_mapping(mapping, file_format="jennifer_csv") == (
            "profiler",
            "analyze-jennifer-csv",
        )

    def test_input_option_for_mapping_uses_format_override(self) -> None:
        override = AnalyzerTypeMapping(
            analyzer_type="profiler",
            command=("profiler", "analyze-jennifer-csv"),
            input_option="--file",
        )
        mapping = AnalyzerTypeMapping(
            analyzer_type="profiler",
            command=("profiler", "analyze-collapsed"),
            input_option="--wall",
            format_overrides={"jennifer_csv": override},
        )

        assert input_option_for_mapping(mapping) == "--wall"
        assert input_option_for_mapping(mapping, file_format="jennifer_csv") == "--file"


class TestDemoSiteRunner:
    def test_discover_manifests_finds_nested_json(self, tmp_path: Path) -> None:
        scenario_dir = tmp_path / "synthetic" / "test-scenario"
        scenario_dir.mkdir(parents=True)
        manifest = scenario_dir / "manifest.json"
        manifest.write_text("{}", encoding="utf-8")

        found = discover_demo_manifests(tmp_path)
        assert manifest in found

    def test_discover_manifests_returns_single_file(self, tmp_path: Path) -> None:
        manifest = tmp_path / "manifest.json"
        manifest.write_text("{}", encoding="utf-8")

        found = discover_demo_manifests(manifest)
        assert found == [manifest]

    def test_run_demo_manifest_with_access_log(self, tmp_path: Path) -> None:
        sample_log = Path(__file__).parents[3] / "examples/access-logs/sample-nginx-access.log"
        if not sample_log.exists():
            pytest.skip("sample data not available")

        mapping_file = tmp_path / "analyzer_type_mapping.json"
        mapping_file.write_text(
            json.dumps(
                {
                    "mappings": {
                        "access_log": {
                            "command": ["access-log", "analyze"],
                            "input_option": "--file",
                        }
                    }
                }
            ),
            encoding="utf-8",
        )

        scenario_dir = tmp_path / "synthetic" / "smoke"
        scenario_dir.mkdir(parents=True)

        import shutil

        shutil.copy(sample_log, scenario_dir / "access.log")

        manifest = scenario_dir / "manifest.json"
        manifest.write_text(
            json.dumps(
                {
                    "scenario": "smoke",
                    "data_source": "synthetic",
                    "files": [
                        {
                            "file": "access.log",
                            "analyzer_type": "access_log",
                            "format": "nginx",
                        }
                    ],
                }
            ),
            encoding="utf-8",
        )

        output_dir = tmp_path / "output"
        result = run_demo_site_manifest(manifest, output_dir, write_pptx=False)

        assert isinstance(result, DemoScenarioRun)
        assert result.scenario == "smoke"
        assert len(result.runs) == 1
        assert result.runs[0].failed is False
        assert result.runs[0].json_path is not None
        assert result.runs[0].json_path.exists()

        payload = json.loads(result.runs[0].json_path.read_text(encoding="utf-8"))
        assert payload["type"] == "access_log"
        assert payload["summary"]["total_requests"] == 6

    def test_run_demo_manifest_skips_missing_analyzer_type(self, tmp_path: Path) -> None:
        mapping_file = tmp_path / "analyzer_type_mapping.json"
        mapping_file.write_text(
            json.dumps({"mappings": {"access_log": {"command": ["a"], "input_option": "--f"}}}),
            encoding="utf-8",
        )

        scenario_dir = tmp_path / "synthetic" / "bad"
        scenario_dir.mkdir(parents=True)
        manifest = scenario_dir / "manifest.json"
        manifest.write_text(
            json.dumps(
                {
                    "scenario": "bad",
                    "files": [
                        {"file": "x.log", "analyzer_type": "unknown_type"},
                        {"file": "", "analyzer_type": "access_log"},
                    ],
                }
            ),
            encoding="utf-8",
        )

        output_dir = tmp_path / "output"
        result = run_demo_site_manifest(manifest, output_dir, write_pptx=False)

        assert len(result.runs) == 0
        assert len(result.skipped_files) == 2

    def test_demo_scenario_run_properties(self) -> None:
        run_ok = DemoAnalyzerRun(
            analyzer_type="access_log",
            file="a.log",
            command=["access-log", "analyze"],
            json_path=Path("/tmp/a.json"),
        )
        run_fail = DemoAnalyzerRun(
            analyzer_type="gc_log",
            file="b.log",
            command=["gc-log", "analyze"],
            failed=True,
            error="file not found",
        )
        scenario = DemoScenarioRun(
            scenario="test",
            data_source="synthetic",
            manifest_path=Path("/m.json"),
            output_dir=Path("/out"),
            runs=[run_ok, run_fail],
        )

        assert scenario.failed_runs == [run_fail]
        assert scenario.json_paths == [Path("/tmp/a.json")]
