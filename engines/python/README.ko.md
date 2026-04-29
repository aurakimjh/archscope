# ArchScope Python Engine

Python engine은 raw diagnostic file을 파싱하고, record를 정규화하며, 통계를 집계한 뒤 AnalysisResult 형식의 JSON 파일을 생성합니다.

## 설치

```bash
cd engines/python
python -m venv .venv
source .venv/bin/activate
pip install -e .
```

## CLI

설치 후에는 console script를 사용합니다.

```bash
archscope-engine --help
```

Access log 샘플:

```bash
archscope-engine access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json
```

Collapsed profiler 샘플:

```bash
archscope-engine profiler analyze-collapsed \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --wall-interval-ms 100 \
  --elapsed-sec 1336.559 \
  --out ../../examples/outputs/profiler-result.json
```

설치 전 source tree 개발에서는 `python -m archscope_engine.cli ...` 경로도 계속 지원합니다.

JFR command-bridge PoC 입력:

```bash
archscope-engine jfr analyze-json \
  --file ../../examples/jfr/sample-jfr-print.json \
  --out ../../examples/outputs/jfr-result.json
```
