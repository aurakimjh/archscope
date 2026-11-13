# 로컬 진단 독립 재검증 요청

이 문서는 재검증 요청과 재현 입력 목록이며 독립 판정이 아닙니다. 원 종합 리뷰의
CONDITIONAL을 유지합니다. 리뷰 대상 커밋의 새 checkout에서 전체 SHA를 기록하세요.
구현자 측 테스트·합성 corpus metadata로 독립 PASS, Windows/WebView, CUDA/NCCL,
추가 실제 생산자 버전 또는 배포 수용을 주장하지 않습니다.

## 입력과 범위

공유 자산 인덱스의 `diagnostics.parser-adversarial.v1`이 원본이며, 저장소의 테스트용
해시 고정 미러는 `apps/engine-native/internal/modeltest/testdata/adversarial`입니다.
재생 전 manifest의 모든 해시를 대조하세요. 모델 JSON 12개는 미지원 버전·끊어진
참조로 전체 거절해야 합니다. Go `go test fuzz v1` 파일은 안전한 거부 또는 유효한
정규 모델 저장/재열기를 기대하며 모두 음성 입력인 것은 아닙니다. 5초 fuzz 4회는
짧은 smoke이며 전체 공격 탐색을 의미하지 않습니다.

| 범위 | 독립 확인 내용 | 보존 입력·실행 probe |
|---|---|---|
| R13 메트릭 이력 | 정수 overflow 좌표, 알려진 epoch의 정확한 차분 7·1, 선행/reset 의미, 10만 point warm 조회, owner/cache 격리, 구간 CSV·페이지 선택 | `metric-signed-int64-overflow.otlp.json`, `metric-known-epochs.snapshots.json`, modeltest 재생, analysis metric cache/history·metric CSV·프런트 상태 테스트 |
| R14 보고서 자료 | 페이지 처리 전 diff 정렬/offset 고정, 이미지 5개 초과 거부, 공격자가 hash를 맞춘 뒤에도 frozen view/spec/font로 SVG 재생성 대조, 실제 ZIP/확장 byte 한도, presentation이 정규 JSON을 바꾸지 않음 | analysis visual-view, visuals artifact/protection/version 테스트. 보관 SVG hash만 신뢰하지 말고 고정 font로 새 artifact를 생성 |
| R15 perf/BPF | 상위 binary 출처와 실제 text/gzip hash 구분, 제외 lifetime 원본 보존, waker header로 관측 entity를 만들지 않음, sleeping/runnable 분리 | `system-perf-source.txt`·`system-imported-hash-mismatch.options.json` 쌍과 parser/analyzer R15 회귀 |
| R17 토큰 시각 | 현재 정책에서 첫 토큰이 span 시작보다 앞서면 TTFT·TPOT 모두 null, last 역전/범위 초과/경계 누락, 과거 저장 정책의 기존 투영 보존 | `llm-token-before-span.otlp.json`과 기존 neutral options, modeltest 재생, `TestR1705ContradictingSuppliedTokenBoundsInvalidateBoth` |
| 직접 모델·SVG 공백 | 6종 모델의 정확 byte 한도, 미지원 필드/참조/ID 거절, SVG XML 이스케이프와 선택 전체 한도 거부 | 각 `TestDirectModelBoundaries`, modeltest corpus, exporters/gputrace SVG 테스트 |

## 명령

리뷰 checkout의 `apps/engine-native`에서 실행합니다.

```sh
go test ./internal/modeltest ./internal/executionmodel ./internal/gputracemodel ./internal/inferencemodel ./internal/llmmodel ./internal/runtimemodel ./internal/systemmodel ./internal/exporters/gputrace -count=1
go test ./internal/analysis ./internal/visuals ./internal/parsers/metrichistory ./internal/parsers/systemtrace ./internal/analyzers/systemtrace ./internal/analyzers/traceimport -run 'Test(R13|R14|R15|R17)' -count=1 -timeout 10m
go test ./internal/exporters/metriccsv -count=1
```

fuzz는 해당 parser 패키지에서 각각 `-run '^$' -fuzz '^TARGET$' -fuzztime=30s
-parallel=2`로 실행하세요. 대상은 execution/FuzzExecutionTextCSVJSON,
gputrace/FuzzGPUJSONSQLite, metrichistory/FuzzSocketSummaryText,
runtimeevidence/FuzzRuntimeThreadText입니다. 새 실패 입력은 반드시 보존합니다.
프런트 폴더에서 `npm run test:state`와 `npm run build`도 실행합니다.

같은 normalizer/analyzer를 기대값 계산기로 쓰지 말고 수치를 별도 계산하고 새로운
변조도 시도하세요. 원본 사실·로컬 테스트·독립 확인·미실행 runtime 범위를 구분합니다.
리뷰는 `docs/review/`에 작성하고, 다른 AI가 work_status.md 반영 후 보관하도록 합니다.
요청서·테스트 실행·hash 일치를 승인으로 기록하지 않습니다.
