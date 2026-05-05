# 성능 측정

ArchScope의 성능 변경은 측정 우선으로 관리한다. 핵심 분석기 시간은
추가 의존성 없이 로컬에서 확인할 수 있다.

```bash
cd engines/python
python3 benchmarks/core_benchmark.py --rows 10000 --repeat 5
python3 benchmarks/core_benchmark.py --rows 10000 --repeat 5 --json
```

기본 벤치마크는 임시 synthetic access log, collapsed profiler, Jennifer CSV
입력을 생성한 뒤 다음 경로를 측정한다.

- `access_log_analyzer`
- `profiler_collapsed_analyzer`
- `jennifer_csv_analyzer`
- `execution_breakdown_classifier`

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

## 성능 기준값

참고: 아래 값은 Apple M2 16GB 기준 측정값이며, 환경에 따라 달라질 수 있다.

| Analyzer | 입력 크기 | 기대 처리 시간 | 비고 |
|----------|-----------|----------------|------|
| access_log | 10,000 rows | < 500ms | streaming aggregation |
| access_log | 100,000 rows | < 3s | percentile sampling 포함 |
| profiler_collapsed | 10,000 stacks | < 300ms | flamegraph tree 구축 포함 |
| profiler_collapsed | 100,000 stacks | < 2s | drilldown + breakdown |

처리 시간이 이 기준의 2배를 넘으면 성능 회귀를 의심한다.

## 메모리 프로파일링

메모리 사용량 측정이 필요한 경우 `tracemalloc`을 사용한다.

```bash
cd engines/python
python3 -c "
import tracemalloc
tracemalloc.start()

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
result = analyze_access_log('../../examples/access-logs/sample-nginx-access.log', 'nginx')

current, peak = tracemalloc.get_traced_memory()
print(f'Current: {current / 1024 / 1024:.1f} MB')
print(f'Peak:    {peak / 1024 / 1024:.1f} MB')
tracemalloc.stop()
"
```

대용량 파일 분석 시 메모리 사용 지침:

- `max_lines` 없이 100만 줄 이상: peak 메모리 < 500MB 목표
- Reservoir sampling 사용으로 percentile 계산 메모리는 고정 (기본 10,000 samples)
- Flamegraph tree는 unique stack 수에 비례하므로, 매우 다양한 스택이 있는 경우 메모리 증가 가능

## Desktop UI 성능 고려사항

- ECharts 렌더링: 10,000 data points 이하에서는 Canvas renderer 권장
- 대량 data points (50,000+): SVG renderer 사용 시 렌더링 지연 발생 가능
- Flamegraph: 깊은 tree (depth > 100)에서는 초기 렌더링에 수초 소요 가능
- 차트 리사이즈: debounce 적용으로 연속 리사이즈 시 과도한 재렌더링 방지
