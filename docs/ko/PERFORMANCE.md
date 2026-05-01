# 성능 측정

ArchScope의 성능 변경은 측정 우선으로 관리한다. 핵심 분석기 시간은
추가 의존성 없이 로컬에서 확인할 수 있다.

```bash
cd engines/python
python3 benchmarks/core_benchmark.py --rows 10000 --repeat 5
python3 benchmarks/core_benchmark.py --rows 10000 --repeat 5 --json
```

기본 벤치마크는 임시 synthetic access log와 collapsed profiler 입력을
생성한 뒤 다음 경로를 측정한다.

- `access_log_analyzer`
- `profiler_collapsed_analyzer`

자동화나 before/after 비교에는 JSON 출력을 사용한다. 첫 번째 실행은
warm-up으로 처리하며 보고 시간에는 포함하지 않는다.

## 프로파일링

Python call graph를 먼저 볼 때는 `cProfile`을 사용한다.

```bash
cd engines/python
python3 -m cProfile -o /tmp/archscope-core.prof benchmarks/core_benchmark.py --rows 100000 --repeat 1
```

대용량 실행 중 sampling profile이 필요하면 사용 가능한 환경에서
`py-spy`를 사용한다.

```bash
py-spy record -o /tmp/archscope.svg -- python3 benchmarks/core_benchmark.py --rows 100000 --repeat 3
```

향후 CI 작업은 작은 timing noise로 실패시키기보다 benchmark JSON을
게시하고 큰 회귀를 알리는 방향으로 확장한다.
