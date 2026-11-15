# Windows 후속 테스트와 변경 사항 검토

- 검토일: 2026-11-15
- 검토자: Codex
- 범위: `ca722025..fbf2ce2c`, 40개 커밋, 585개 변경 파일
- 판정: **CONDITIONAL — 신규 P2 1건·P3 1건, 확인된 신규 P1 없음**
- T-613 `PENDING`, T-627 `REVIEW` 및 기존 독립 리뷰의 원 판정 유지.

## WREV2-01 · P2 · 차트 CSV가 시간·bytes 프로파일도 samples로 선언

위치: `apps/engine-native/cmd/archscope-app/frontend/src/pages/ChartStudioPage.tsx:87–92,172–176`.

`40091114`는 차트 CSV에 단위와 출처를 추가하면서 profiler.components의 단위를
항상 samples로 지정했다. 그러나 해당 template이 허용하는 profiler_collapsed는
summary.value_unit이 nanoseconds, microseconds, milliseconds, seconds, bytes,
unknown인 결과도 포함한다. 컴포넌트 행의 samples라는 필드명은 이 경우 원래
가중치를 담는 호환 필드이며 관측 개수라는 뜻이 아니다.

현재 CLI로 `Root;work 1000000`을 lock/nanoseconds로 분석하면 value_unit은
nanoseconds, estimated_seconds는 0.001이고 component weight는 1,000,000이다.
현재 페이지에서 선택하는 template 단위와 실제 production CSV helper를 사용하면
동일 값이 `1000000,samples`로 내보내진다. 1ms에 해당하는 비용을 백만 개의
샘플로 잘못 선언해 후속 보고서·스프레드시트에서 다른 측정량으로 해석할 수 있다.

새 Windows 24파일 검증의 profiler fixture는 원래 samples인 경우라 이 경로를
검증하지 않는다. 해당 fixture의 PASS를 취소하는 문제가 아니라 지원 입력의
단위 선택 누락이다. 이번 재현은 Go 결과와 production helper를 사용한 로컬
소스 결합 재현이며 새 Windows Save dialog 실행으로 보고하지 않는다.

수정 조건: 원본 결과의 측정 계약에서 실제 단위를 정하고 CSV와 차트 설명에
일관되게 전달한다. 누락·unknown을 임의로 samples로 승격하지 않는다.
samples/ns/us/ms/s/bytes/unknown 입력의 값·단위·출처 보존 회귀를 추가한다.
원본 수치와 interval 의미는 변경하지 않는다. T-627 후속으로 우선 처리한다.

## WREV2-02 · P3 · 남은 검증 목록의 JFR 이벤트와 스택 샘플 수 혼용

위치: `docs/review/evidence/2026-11-14-windows-final/remaining-acceptance.json:32`.

JFR 항목은 153 filtered events/29 stacks로 기록하지만, 같은 실행의
jfr-notable-dense-fixed.txt와 JPEG는 필터 이벤트 3,010, 전체 이벤트 3,010,
샘플 이벤트/스택 샘플 153, 고유 스택 29를 보여준다.
독립 JDK 집계 jfr-binary-counts.json의 총 이벤트도 3,010이다.
README의 상세 설명은 이를 구분하고 있다.

제품 계산 오류는 아니지만 현재 남은 작업 목록을 읽으면 검증한 모집단을
잘못 이해하게 된다. 원 수신 증거와 그 해시를 수정하지 않고, 다음 현행
inventory 또는 정정 부록에서 전체/필터 이벤트와 스택 샘플을 구분한다.
작업표에는 올바른 값을 함께 기록했다.

## 확인한 Windows 결과와 한계

| 영역 | 확인한 내용 | 남은 범위 |
| --- | --- | --- |
| 기존 WREV-01 | 양쪽 query 4행·short soak 128행·live 640행, 총 6파일의 원 해시/전체 필드 비교 영수증 | 원본 private store/export를 여기서 다시 열지는 않았음. 과거 boolean/원 판정 소급 변경 없음 |
| 저장 실패/재시도 | 손상 행 거절, backup 복원, export 거절/재시도, capability 초기화 분리 | 실제 Windows 디스크 고갈·전원 손실·서비스 복구와 구분 |
| 주석·본문 | post-mask 4096-byte 제한, 기존 redacted flag 유지, Unicode 저장, x86 실제 gate 허용 body fixture | x64 body는 실제 LocalDumps gate에서 거절. 실제 IME·drag 및 전체 body/format 조합 미완료 |
| 화면·보고서 | 21개 route의 173개 native case에 보존된 before FAIL/부분/불확실 결과가 포함됨. 최종 Diff·JFR·본문 화면 3개 대조 | 140 PASS / 22 FAIL / 7 PARTIAL / 3 INCONCLUSIVE / 1 OBSERVED는 역사적 결과 분포이며 전부 최종 실패·전부 성공으로 해석하지 않음 |
| 산출물 | 차트 8종 × CSV/PNG/SVG의 값·단위·출처/벡터 구조/PNG CRC·zlib·크기 확인 | 단일 profiler samples fixture를 다른 단위로 확대하지 않음. Office/모든 renderer 조합 미실행 |
| 메모리 | 기존 capture 2건, import 5건, body 1건의 8-point median/Theil–Sen·동일 7 process identity 재계산 | 유한 screening 기준이며 모든 workload의 누수 없음 판정이 아님 |

Import 초기 두 실행의 tree delta/slope는 각각 +147.398/+29.840 MiB,
+157.152/+28.051 MiB/cycle로 실패한다. 36-cycle x64도
+32.008 MiB/+5.943 MiB/cycle로 실패한다. x86 history 36-cycle과
x64의 별도 continuation 마지막 창은 통과하지만 기존 실패를 없애지 않는다.
현행 고정 프로토콜의 반복 관측과 증가 원인 진단은 T-613에 남긴다.

Body-enabled x86는 2,470.27초·5,120요청·10회 Clear·8개 측정 idle 지점에서
고정 기준을 통과한다. 보관 128행의 262,144/300,020-byte prefix 검사는
수신 private-file 영수증이며 이 호스트에서 직접 재실행한 검사는 아니다.
이번 재계산 tree delta +15.715 MiB, slope +2.874 MiB/cycle은 원 기록과 일치한다.

## 로컬 검증

- 원 수신 artifact manifest 4개, 총 528파일 SHA-256·길이 일치.
- 최종 변경 제품 소스 34개의 Git 바이트 지문 일치.
- 173 case 결과 분포와 21-route inventory 대조.
- 메모리 결과 8건의 측정값·고정 budget·분류·동일 process 집합 독립 재계산.
- 차트 CSV 8개 값/단위/출처 수, SVG 8개 vector 구조,
  PNG 8개 CRC·zlib·geometry 검사. 새 native/Office 렌더 검증과 구분.
- macOS arm64: 관련 Go 테스트 27패키지, race 26패키지, 같은 대상 vet 통과.
- 프런트엔드 state regression·production build, memory/soak helper 49테스트 통과.
- offline resources/input assistance 자원 검사 통과.
- 새 Windows 실행·driver 설치·보안 정책 변경·강제 메모리 부하는 수행하지 않음.

재계산 코드는 [audit_received.py](evidence/2026-11-15-windows-review/audit_received.py),
결과는 [received-audit.json](evidence/2026-11-15-windows-review/received-audit.json),
발견 사항 재현은 [reproduction.json](evidence/2026-11-15-windows-review/reproduction.json),
로컬 검증은 [local-validation.json](evidence/2026-11-15-windows-review/local-validation.json)에 있다.

## 남은 작업과 처리

현재 호스트에서 WREV2-01 → WREV2-02를 처리할 수 있다.
Windows에서는 T-627의 원본/reference·조건·실제 OS DPI·언어 조합,
T-613의 고정 프로토콜 메모리 재관측·IME/drag·원본 V7.6.4 동등성,
실제 gate 허용 시에만 x64 body 조합이 남는다.
T-601 driver/service/타 세션 reach와 T-611 vendor COM/cookie/clean-host 등록은
해당 격리 환경에서 검증한다. T-654 실제 CUDA/NCCL과 T-662 자연 swap/pressure는
각각 하드웨어·자연 관측 조건이 필요한 TODO로 유지한다.

작업표의 우선순위 큐·Open Work Snapshot·T-613/T-627 registry 및 검토 이력에
반영했다. 기존 2026-11-14 Codex 원 리뷰와 이번 문서는 작성자와 다른 AI의
독립 처리 대상으로 미처리 위치에 둔다. 원 수신 증거와 판정을 보존했으며
이번 작업은 제품 수정·커밋·push를 포함하지 않는다.
