# DB·OS 스냅샷 첫 검증 배치

`diagnostics.system-db-snapshots.v1`은 실제 PostgreSQL16.14와 Linux iproute2-6.1.0의 합성 실행에서 얻은 관측 자료입니다. 다른 DB·APM·Windows·프로파일·serving 어댑터의 지원 선언이 아닙니다.

## 실제 입력

- PostgreSQL: network-none 임시 서버의 단일 합성 DB에서 5번 관측했습니다. unknown stats_reset, 두 번의 명시적 초기화, insert/rollback 부하가 포함됩니다. 14개 누적 counter와 numbackends gauge를 보존합니다. 초기화는 전용 일회성 DB에서만 수행했으며 수집 SQL 자체는 읽기 전용입니다.
- Linux: 같은 kernel에서 격리 네임스페이스의 `LC_ALL=C ss -s`를 baseline/4개 loopback 연결 쌍/닫기 이후에 실행했습니다. established는 0/8/0입니다. 21개 값은 모두 gauge이며 rate를 만들지 않습니다. TCP total/closed/orphan 수치에는 네임스페이스 밖 공용 커널 집계가 포함될 수 있습니다.
- 각 원본의 15 series/75 point 및21 series/63 point 모델은 정확한 정수·원본 hash·읽기 시각·host/boot/instance/database/namespace 범위를 유지합니다. 식별자 태그는 정제된 선언이고 실제 호스트 신원 인증이 아닙니다.

## 수집 및 해석

PostgreSQL은 별도 psql 세션에서 `BEGIN; SET LOCAL stats_fetch_consistency=snapshot;` 뒤 postgres-snapshot.sql을 실행하고 COMMIT하세요. collector_version1.0.0, 실제 server_version_num160014/version16.14, track_counts=true, 정제 태그와 database_oid 및 결과 배열을 postgres-capture.json 형태로 저장합니다. 원본 DB명·SQL/쿼리 본문·사용자·접속 문자열은 저장하지 않습니다. stats_reset이 null/누락이면 unknown으로 남깁니다. pg_postmaster_start_time으로 대체하지 않습니다. 백엔드별 지연 갱신으로 정확한 transaction 실행 구간이나 query 통계가 되지 않습니다.

ss 수집에는 주소·포트·프로세스 정보가 없는 summary만 사용합니다. 수동 수집기로 각 호출 전후 time.time_ns와 실제 iproute2/kernel 버전, 정제된 host/boot/network_namespace를 기록하세요. generate-ss.py는 격리된 테스트 환경에서 합성 loopback 연결을 만들고 모두 닫은 뒤 예시 계약을 출력합니다. 다른 kernel/environment는 추가 검증 대상입니다.

읽기 구간은 순차 관측의 skew이며 동시 측정 구간이 아닙니다. PostgreSQL의 stats_reset은 counter 시작 시각이고 postmaster 시각은 프로세스 수명 범위입니다. blks_hit/read는 물리 디스크 read나 cache 비율의 증거가 아닙니다. 소켓 필드를 합산하거나 연결 처리량·손실·RTT·bandwidth·프로세스 소유권을 추정하지 않습니다. importer는 SQL·명령·네트워크 수집을 실행하지 않습니다.

## 경계

두 계약은 collector1.0.0, 원본4MiB,1..256개 순서가 겹치지 않는 읽기 snapshot, ss summary16KiB를 제한합니다. 모델은 기존 MetricHistory1.0.0 규약을 재사용합니다. native WebView/Windows 실행이나 모든 kernel·PostgreSQL 버전은 검증하지 않았습니다. producer-provenance.json과 manifest.json의 SHA-256을 확인하세요. raw source schema는 형태를, compiled importer는 숫자·시각·scope·layout의 의미도 검증합니다.

stats_reset NULL은 reset 이력 미확보이며 같은 postmaster도 crash recovery 중 통계 연속성을 증명하지 못한다. unknown counter의 delta/rate는 억제한다. 누적 관측만 보존하거나, DB 소유자가 별도 허가된 정비 창에 reset 영향을 검토하고 새 epoch를 만든 뒤 수집할 수 있다. 수집기는 reset을 하지 않는다. capture-postgres.py는 libpq 환경/pgpass를 사용하는 별도 명시적 psql 읽기 전용 wrapper 수집기이며 importer가 호출하지 않는다.

새 PostgreSQL 가져오기는 normalizer1.3.0: unknown→known 첫 전이는 epoch_transition_from_unknown으로 명시하고 delta/rate는 null, 다음 같은 epoch 관측부터 파생한다. 이전 unknown은 추정하지 않는다. 알려진→알려진 reset의 post-reset 파생은 유지하며 기존 저장 모델은 재해석하지 않는다.
