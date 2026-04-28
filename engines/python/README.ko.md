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

```bash
python -m archscope_engine.cli --help
```

Access log 샘플:

```bash
python -m archscope_engine.cli access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json
```

Collapsed profiler 샘플:

```bash
python -m archscope_engine.cli profiler analyze-collapsed \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --wall-interval-ms 100 \
  --elapsed-sec 1336.559 \
  --out ../../examples/outputs/profiler-result.json
```
