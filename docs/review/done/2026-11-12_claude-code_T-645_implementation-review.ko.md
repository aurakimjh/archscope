# T-645 PostgreSQL pg_stat_database·Linux ss -s 스냅샷 구현 독립 리뷰

- 작성: claude-code (독립 리뷰어; 구현·처리와 다른 AI)
- 대상: `b36d83e` (T-645 첫 묶음 + T-644 R21-01/R21-03 처리). 이 리뷰는 구현 코드를 수정하지 않았다.
- 방법: 대상 커밋의 scratch worktree에서 CLI를 새로 빌드하고 격리 `ARCHSCOPE_STATE_ROOT`로 실행했다. 이 환경의 Docker로 **제 소유의 일회용 PostgreSQL 16.14 서버(네트워크 없음)**를 띄워 작성자와 다른 워크로드·순서로 캡처했고, 별도의 네트워크 없는 컨테이너에서 iproute2-6.1.0 `ss -s`를 IPv6·UDP 소켓이 포함된 다른 시나리오로 수집했다. 파생 delta/rate와 `ss` 값은 Python으로 독립 계산·독립 파싱했다.

## 1. 결론

**CONDITIONAL — P3 3건, P1·P2 없음(차단 없음).**

- PostgreSQL 카운터 수명(reset/unknown)과 `ss -s` 게이지 의미는 독립 입력에서 정확하다. 파생 값 14개와 `ss` 값 84개가 독립 계산과 모두 일치했다.
- T-644 R21-01(원본 크기 한도·Nsight 부분 추출 도구)과 R21-03(무시 레코드가 지원 이벤트 한도를 소모하지 않음)의 로컬 수정도 실제 입력으로 확인했다.
- 남은 것은 해석 일관성과 사용성(R22-01~R22-03)이며 후속 큐로 충분하다. T-645 DONE 복귀에 필요한 선행 조건은 없다.

## 2. 확인한 계약 충족 항목

| 항목 | 확인 내용 |
|---|---|
| 독립 PostgreSQL 캡처 | `postgres:16`(server_version 16.14, num 160014, `track_counts=on`) 일회용 서버에서 번들 읽기 전용 SQL(`BEGIN; SET LOCAL stats_fetch_consistency=snapshot; …; COMMIT`)로 4개 스냅샷(reset 없음 → 적재 → `pg_stat_reset()` → 갱신/삭제) 수집. 가져오기 성공: 시리즈 15, 포인트 60(=15×4), 파생 구간 14. |
| 카운터 수명 | `stats_reset` null 구간은 delta/rate 없이 `start_time_unavailable`, reset 이후 에폭에서만 파생. 마지막 구간의 14개 카운터 delta가 스냅샷 차이와 모두 일치(`xact_commit` 8−2=6, `tup_updated` 999). rate는 정확한 유리수(`6 / (end₄−end₃)` = `250000/96429`)이며 독립 계산과 같다. |
| 변조 입력 18종 | reset이 읽기 구간 뒤(`POSTGRES_CAPTURE_RESET`), `track_counts=false`·`stats_fetch_consistency=none`, 버전 불일치, 겹치는 구간, 2^64 오버플로, 비정수 OID, 경로형 host·연결 문자열형 db 태그(`SNAPSHOT_CAPTURE_IDENTITY`), 알 수 없는 필드(`query`), start>end가 모두 코드와 함께 거부. 2^64−1은 정확히 보존. reset 없이 카운터가 감소하면 해당 카운터 1개만 파생이 막힌다(13/14). 단일 스냅샷은 파생 0으로 수용. |
| 수명 보수성 | unknown 시작을 보수적으로 억제하는 설계는 타당하다: 백엔드 crash recovery는 postmaster 재시작 없이 통계를 버릴 수 있어 "NULL reset + 같은 postmaster"가 연속성을 증명하지 못한다. |
| 독립 `ss -s` 캡처 | iproute2 6.1.0, Linux 7.0.14-linuxkit, 네트워크 없는 컨테이너에서 IPv4/IPv6 listener·UDP 소켓 4개·연결 5쌍(estab 10)을 만들어 4회 수집(0/10/4/0 종료 순서). 가져오기: 시리즈 21·포인트 84. 독립 정규식 파싱과 **84/84 값 일치**(Total/TCP estab·closed·orphaned·timewait/RAW·UDP·TCP·INET·FRAG × total·ip·ipv6). |
| `ss` 거부 | 열 추가, 비-C 로케일(`Gesamt:`), netstat 형식, 17 KB 요약, 다른 iproute2 버전, 빈 커널 문자열, 경로형 namespace 태그, 2^64 값이 모두 코드와 함께 거부. 음수 `closed`는 문서대로 보존, CRLF는 수용. |
| gzip 입력 | 압축한 postgres 캡처가 동일 결과(시리즈 15)로 가져와진다. 잘못된 형식 지정(`--format linux-ss-summary-json` + postgres 캡처)은 알 수 없는 필드로 거부. |
| T-644 R21-01 수정 | 제 겹침 변형 Nsight SQLite에 70 MiB 패딩을 더한 73,535,488바이트 파일을 번들 부분 추출 도구(`subset-nsight-sqlite.py`)로 창 `[80000000,80500000)` 추출 → 24,576바이트, 26행, 영수증에 원본/부분 바이트·SHA·창·테이블별 행 수 기록. 가져오면 14 kernel, 합 77,888 ns, 합집합 57,696 ns이고 `sqlite3` 독립 계산과 일치. 기존 출력 재사용은 `OUTPUT_EXISTS_OR_ALIAS`로 거부. |
| T-644 R21-03 수정 | 실제 PyTorch 2.15.0 트레이스(228 이벤트)에 무시되는 인스턴트 레코드 100,500개를 덧붙여도 지원 이벤트 228개로 가져와지고, 지원 이벤트 5.2만 개는 `GPU_TRACE_EVENT_BUDGET: at_least=50001 limit=50000`으로 거부된다. |

## 3. Finding 요약

| ID | 심각도 | 요약 |
|---|---|---|
| R22-01 | P3 | 알 수 없는 에폭에서 처음 알려진 reset으로 넘어간 구간은 시작이 알려졌는데도 rate가 없고 상태명이 `start_time_unavailable`; 알려진→알려진 reset 구간은 rate가 있어 일관되지 않음 |
| R22-02 | P3 | 단일 필드·시간순 불변식 미검증(`numbackends=0`, 거꾸로 움직이는 `stats_reset`) |
| R22-03 | P3 | 일반적인 reset 이력 없는 DB에서는 delta/rate를 전혀 못 얻는데 그 이유·대응이 문서에 없고, PostgreSQL 캡처 래퍼를 수동 조립해야 함 |

## 4. Finding 상세

### R22-01 (P3) 에폭 전이의 파생 규칙이 일관되지 않다

- 재현(제 캡처, `xact_commit`): 스냅샷3은 `stats_reset`이 처음 알려진 지점이다. 결과 행은 `start_ns=1794487808782265000`(= reset 시각), `value 2`인데 `delta/rate null`, `status start_time_unavailable`. 작성자 fixture에서 두 번째 reset(알려진→알려진)은 같은 정보(`start_ns` = reset 시각, `value 7`)에 `delta 7`, `rate 3500000/622943`, `status reset`이 붙는다. 첫 번째 reset 행(알려진 시작, 값 7)은 rate가 없다.
- 영향: 같은 형태의 "reset 이후 값과 reset 시각"이 이전 에폭이 unknown이냐에 따라 rate 유무가 갈린다. 시작이 알려졌는데 `start_time_unavailable`이라는 상태명은 읽는 사람에게 모순이다. 값 오류는 아니고 보수적 쪽 오차다.
- 권고: 정책을 문서화하고 상태명을 `epoch_transition_from_unknown`처럼 구분하거나, reset 시각이 알려진 새 에폭의 post-reset 값/rate를 동일하게 제공한다(이전 에폭은 계속 unknown).

### R22-02 (P3) 불변식 검증 공백

- 재현: `numbackends = "0"`이 수용된다(문서는 수집기 자신이 포함된다고 설명). `stats_reset_ns`를 이전 스냅샷의 reset보다 1초 이른 값으로 바꿔도 에폭 변경으로 수용되고 파생 구간은 0이 된다(조용히 파생만 막힘). 반면 reset이 읽기 구간보다 늦으면 거부된다.
- 영향: 수동 조립 입력의 오류가 증거로 남는다. 파생을 막으므로 오판 경로는 좁다.
- 권고: `numbackends ≥ 1`, `stats_reset_ns`가 스냅샷 사이에 비감소임을 검증하고 위반 시 명시적 사유로 거부하거나 표시한다.

### R22-03 (P3) reset 이력이 없는 일반 DB는 delta/rate를 얻을 수 없다

- 설명: 한 번도 reset하지 않은 DB는 `pg_stat_database.stats_reset`이 NULL이다(제 서버에서 확인). 어댑터는 안전하게 모든 파생을 막으므로(R22 §2 "수명 보수성") 이런 서버의 캡처는 누적값 포인트만 남는다. 문서는 "Unknown starts suppress derived rates"라고만 적고 이유(crash recovery)나 사용자가 얻을 수 있는 경로(정비 창에 `pg_stat_reset()`으로 알려진 에폭을 만든 뒤 수집)를 설명하지 않는다. 또한 Postgres 캡처는 `psql` 출력으로 래퍼 JSON을 직접 조립해야 하며 Apple 경로와 달리 배포된 수집 도구가 없다(`ss`용 스크립트는 fixture 생성기).
- 권고: 문서에 NULL reset의 의미와 대응을 한 단락으로 추가하고, 읽기 전용 수집 보조 스크립트(래퍼 조립 포함)를 Apple 수집기처럼 제공한다.

## 5. 실행한 검증과 미검증 항목

**실행함**
- 대상 커밋 scratch worktree 빌드, `go vet ./...`(host)·`GOOS=windows GOARCH=386 go vet ./...` 통과(링커 SDK 경고 외 없음).
- `go test -race -count=1`: `parsers/metrichistory`, `parsers/gputrace`, `catalog`, `cmd/archscope-engine`, `cmd/archscope-app`, `distribution` 모두 `ok`.
- 실제 PostgreSQL 16.14 일회용 서버 캡처 1종 + 변조 18종, 실제 `ss -s` 캡처 1종 + 변조 12종(84개 값 독립 일치), Nsight 부분 추출·R21 수정 확인. 서버 컨테이너는 검증 후 제거했다(남은 컨테이너 0).

**실행하지 않음**
- 전체 118개 패키지 race 스위트, Wails WebView·Playwright 화면, 네이티브 Windows, hosted CI, 오프라인 syscall 70건 재실행, Mac 패키징·서명.
- 다른 PostgreSQL 마이너 버전·다중 DB 클러스터·복제 환경, 다른 iproute2·커널 버전, 실제 운영 서버의 캡처. CLI 옵션 덮어쓰기(producer/namespace) 거부는 단위 테스트에 의존했고 제 입력으로 재현하지 않았다.
- T-644 R21-02·R21-04, T-643 R20-02~R20-04, T-642 R19-02~R19-04 후속 큐는 다시 판정하지 않았다.

## 6. 권고 처리 순서

1. R22-03 — 문서(NULL reset 의미·대응)와 수집 보조 스크립트.
2. R22-01 — 에폭 전이 상태명·rate 정책 일관화.
3. R22-02 — 불변식 검증.
