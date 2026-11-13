# 수동 DB·소켓 스냅샷

DB/OS 스냅샷의 첫 검증 배치는 PostgreSQL16.14 `pg_stat_database`와 Linux7.0.14-linuxkit의
iproute2-6.1.0 `ss -s`입니다. 기존 MetricHistory1.0.0, `metrics.history-json`,
`metrics_timeseries`0.2.0 경로를 재사용합니다. 다른 DB·커널·APM·프로파일·serving
어댑터를 일괄 지원하지 않습니다. 가져오기 중 DB 접속·SQL·도구 실행·수집을 하지 않습니다.

## 실제 입력과 범위

| 형식 | 실제 검증 | 보존하는 관측 |
|---|---|---|
| postgres-stat-database-json | PostgreSQL16.14/server_version_num160014, collector1.0.0, network-none 일회성 서버의 합성 DB | unknown reset, reset A, 부하, reset B, 부하의5 snapshot. 누적 counter14개와 numbackends gauge:15 series/75 points. |
| linux-ss-summary-json | iproute2-6.1.0, collector1.0.0, Linux7.0.14-linuxkit, 격리된 network-none namespace | baseline, loopback4쌍, 소유 endpoint 닫기 이후. established0/8/0,21 gauge/63 points. |

내장 `system-db-*` 자료는 원본·모델·스키마 hash, producer 출처, 읽기 전용 수집 SQL,
loopback 표본 생성기를 포함합니다. 임시 PostgreSQL 서버는 수집 후 중지·제거했습니다.
운영 데이터·DB SQL 본문·접속 문자열·주소·포트·프로세스명은 저장하지 않습니다.
host/boot/instance/database/namespace 태그는 정제된 선언이며 원본 신원 인증이 아닙니다.
기존 저장 모델·결과를 변경하거나 새 결과 envelope·범용 importer를 추가하지 않습니다.

## 수집과 정규화

별도 psql에서 `BEGIN; SET LOCAL stats_fetch_consistency=snapshot;` 후
`postgres-snapshot.sql`을 실행하고 COMMIT하세요. 실제 버전, track_counts=true,
database OID와 정제된 수집 맥락 및 결과 배열을 `postgres-stat-database.v1` 계약으로
저장합니다. 단일 DB만 선택하며 공용 OID0 행은 제외합니다. 이것이 전체 cluster의
관측 목록은 아닙니다. 수집 쿼리도 backend/transaction 활동에 포함됩니다. 검증용 reset과
쓰기는 일회성 DB에서만 수행했고, 일반 수집 SQL은 통계나 애플리케이션 데이터를 바꾸지 않습니다.

Linux는 `LC_ALL=C ss -s` 전후 time.time_ns와 실제 ss -V/kernel release,
정제된 host/boot/network_namespace를 `linux-ss-summary.v1` 계약으로 기록합니다.
빈 줄을 제외한8행의 검증된 summary만 받습니다. 임의 ss/netstat 목록·추가 열은 거절합니다.
패킷 캡처나 앱 내 권한 상승 수집기는 필요하지 않습니다. 다른 kernel/environment는 추가 입력 검증 대상입니다.

```sh
archscope-engine metrics history-inspect --in capture.json
archscope-engine metrics history-normalize --in capture.json --out model.json
archscope-engine metrics history-import --in capture.json --out result.json
archscope-engine metrics history-query --in result.json --query '{"selection":{"series_ids":["metric-..."]},"limit":25}'
```

닫힌 contract로 자동 감지하며 명시적 format도 위 표의 값을 사용합니다. 파일의
producer/version/context를 query·namespace·export-ID 옵션으로 덮어쓰지 못합니다.
기존 출력 파일은 거절합니다. Workspace의 메트릭 이력에서 형식 선택/auto → 원본 확인 →
시계열 선택 → 새 모델 저장 → 분석으로 진행합니다. 확인한 원본 SHA가 저장 시에도 같아야 합니다.
기존 정확한 조회·JSON/CSV/HTML/report pack·이미지 템플릿·바이트 일치 case를 사용합니다.
범용 CSV는 요약·관측 가능성 표를 유지하고 HTML은 제한된 정확한 구간 값/rate·bucket 상세를
추가합니다. 더 큰 선택은 history-query JSON이나 구간 CSV를 사용하세요. 원본 merge 자체는 시계 정렬이나 비교 가능한 scope를 증명하지 않습니다.

## 의미 보존

PostgreSQL counter 시작은 실제 stats_reset이며 null/누락은 unknown입니다. Postmaster
시각은 프로세스 수명 범위이고 counter 시작으로 대체하지 않습니다. 정상 재시작 시 더 오래된
통계 epoch가 유지될 수 있습니다. unknown start에서는 rate를 만들지 않습니다. 예고되지 않은
감소도 계산 불가 상태입니다. 알려진 reset은 새 epoch를 보존하고 reset 이후 값/rate만
표시하며, 누락된 이전 구간을 복구하지 않습니다. 읽기 구간은 순차 관측 skew이고 통계는
비동기로 갱신되므로 query 실행 구간이 아닙니다. numbackends는 수집기까지 포함한 연결
backend gauge이며 active SQL·pool queue가 아닙니다. blks_hit/read는 물리 디스크 I/O나
OS cache miss·cache 비율·query percentile의 증거가 아닙니다.

소켓 값은 모두 reported gauge입니다. TCP total/closed/orphan은 namespace 밖의
공용 커널 집계가 포함될 수 있으므로 context만으로 전체 값의 namespace locality를
보장하지 않습니다. Total/INET/protocol/IPv4/IPv6는 중복되어 합산하지 않습니다.
FRAG 단위는 socket이 아닌 fragment queue이며, producer의 음수 closed 계산도
그대로 보존합니다. 연결 처리량·손실·RTT·bandwidth·프로세스 소유권·인과성을 추정하지 않습니다.
순차 관측이므로 모든 값에 동시 측정 합계 관계를 강제하지 않습니다.

## 한계와 검증

원본4MiB,1..256개의 엄격히 증가하고 겹치지 않는 읽기 snapshot, ss summary16KiB를
제한합니다. uint64 값/ns는 정확한 문자열이며 JavaScript 부동소수점으로 바꾸지 않습니다.
기존 모델32MiB·1만 series/10만 point, compact result32MiB, 조회1..200행/256KiB
제한을 적용합니다. 잘못되거나 누락된 값·미지원 버전/필드·옵션 충돌·겹치는 시간은 명시적으로
거절하며 임의 잘라내기·0 채우기를 하지 않습니다.

실제 원본·모델·스키마, reset/unknown epoch, 구조적 큰 정수·음수 closed·프로세스 수명 분리,
원본 SHA 고정, 공통 소비자·기존 저장 reader·취소를 검증합니다. 브라우저 확인은 실제
React/CLI DTO와 mock IPC의 범위이며 native WebView·Windows 실행 증거가 아닙니다.
독립 리뷰는 별도 완료 조건입니다.

참고: [PostgreSQL16 통계](https://www.postgresql.org/docs/16/monitoring-stats.html),
[iproute2-6.1.0 summary 구현](https://github.com/iproute2/iproute2/blob/v6.1.0/misc/ss.c).

`stats_reset=NULL`인 reset 이력 없는 DB는 누적값만 남으며 delta/rate가 없습니다. 같은 postmaster 수명도 backend crash recovery 중 통계 연속성을 보증하지 않아 시작을 추정하지 않습니다. 누적 관측을 유지하거나 DB 소유자가 허가된 정비 창에 reset의 다른 모니터링 영향까지 검토한 뒤 새 epoch에서 수집할 수 있습니다. 자동 reset은 없습니다. 별도 `capture-postgres.py --out capture.json --host-id host-a --boot-id boot-a --instance-id db-a --database-tag sample --samples 3`는 libpq 환경/pgpass 연결을 사용해 `BEGIN READ ONLY`로 고정 SQL을 실행하고 wrapper를 조립합니다. SQL/접속 문자열/DB명은 출력하지 않으며 importer는 DB에 접속하지 않습니다. 수집 자체의 backend/transaction 관측 영향은 남습니다.

새 PostgreSQL 가져오기는 normalizer1.3.0을 사용합니다. unknown→known 첫 전이는 알려진 현재 시작을 유지하고 `epoch_transition_from_unknown`으로 표시하며 delta/rate는 null입니다. 이전 unknown 연속성을 가정하지 않고 다음 같은 epoch 관측부터 파생합니다. 알려진→알려진 reset의 post-reset 파생은 유지합니다. 저장된 1.0/1.1/1.2는 원래 상태/계산을 보존하며 명시적 merge는 최신 입력 normalizer를 선택합니다.

입력 검증은 `numbackends≥1`(수집 backend 포함), 알려진 `stats_reset`의 비감소를 요구합니다. 중간 unknown 이후에도 이전 알려진 reset보다 역행하면 `POSTGRES_CAPTURE_RESET_BACKWARDS`로 거절합니다. zero backend는 `POSTGRES_CAPTURE_BACKENDS`이며 시각/값을 자동 보정하지 않습니다.

검증된 버전이 기본값입니다. 다른 버전의 명시적 구조 검증 선택, 실제/미상 버전 보존과 제한은 [미검증 버전 정책](INPUT_ASSISTANCE.md#미검증-생산자-버전의-명시적-선택)을 참조하세요.

## SQLite 읽기 전용 PRAGMA 스냅샷

`sqlite-pragma-snapshots-json`은 SQLite 3.53.0의 수동 `sqlite-pragma-snapshots.v1` wrapper를 받습니다. `python3 scripts/capture-sqlite.py --database owned.sqlite --instance-tag sanitized-instance --out new-capture.json`을 명시 실행한 뒤 메트릭 이력 원본 검사/가져오기를 사용합니다. 수집기는 한 읽기 전용 연결과 고정 PRAGMA 네 개만 사용하며 DB 경로·SQL 본문·행 데이터를 내보내지 않습니다. Import는 DB를 열지 않습니다.

page_count·freelist_count·page_size·연결 로컬 data_version은 정확한 gauge이며 transaction counter나 rate로 바꾸지 않습니다. 순차 조회이므로 원자적 DB snapshot이 아닙니다. 새 연결·파일 재생성은 새 원본으로 구분합니다. 기존 MetricHistory 조회·케이스·export를 사용합니다. 실제 검증은 SQLite 3.53.0의 합성 파일을 소유한 writer가 변경하는 동안 세 번 읽은 범위이며 다른 버전/형식은 거절합니다.
