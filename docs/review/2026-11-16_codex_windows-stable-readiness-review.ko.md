# Windows 수신 결과와 정식 v0.4.0 준비 검토

- 검토일: 2026-11-16, 검토자: Codex.
- 범위: `98eeaf3e..635512cf`, 8커밋·90파일. 리본·언어의 기존 구현은 변경의
  관련 문맥으로 확인했으며 전체 번역의 독립 언어 검수로 확대하지 않는다.
- 판정: **CONDITIONAL — 새 제품 P1/P2 없음, 검증 러너 P3 1건**.
- T-601/T-627/T-663/T-664/T-611 REVIEW, T-613 PENDING, T-654/T-662 TODO 유지.
  기존 Codex 작성 리뷰의 독립 처리·최종 PASS는 주장하지 않는다.

## WREV3-01 · P3 · 기록 중인 결과 JSON을 읽으면 재시도 없이 검증 실패

위치: `scripts/verify-windows-ribbon.mjs:32`.
생산자는 `apps/engine-native/cmd/archscope-app/layout_audit.go:83`의
`os.WriteFile`로 최종 경로에 직접 기록한다. 파일 생성·truncate와 완성 사이에
러너가 읽으면 `JSON.parse`가 SyntaxError를 낸다. 현재 러너는 ENOENT만 재시도해
정상 결과가 곧 완성되는 경우에도 즉시 첫 화면 검사를 중단한다.

실제 새 러너의 소스를 VM에서 통제된 프로세스·파일 I/O와 함께 실행했다.
완성 결과와 ENOENT 후 완성 결과는 통과했다. 부분 JSON 후 완성될 입력은
첫 읽기에서 SyntaxError로 실패해 다음 정상 읽기까지 도달하지 않았다.
실제 Windows에서 새로 관측한 실패나 제품 데이터 문제는 아니다. 이미 완료된
Windows 6회 리본 검사와 14개 CSV 저장 결과는 그대로 유지한다.

수정 조건: 결과를 임시 파일에 기록한 뒤 원자적으로 발행하거나, 기록 중인 JSON과
종료 후 손상된 JSON을 구분해 제한된 재시도를 적용한다. 완성·미생성·부분→완성·
영구 손상·프로세스 종료·시간 초과를 검사하고 실제 report.passed=false는 거절한다.
T-663의 비차단 후속으로 작업표에 등록했으며 이 검토에서 제품·러너를 수정하지 않는다.

재현과 출력: [재현 코드](evidence/2026-11-16-windows-review/reproduce_receipt_read.mjs),
[재현 결과](evidence/2026-11-16-windows-review/reproduction.json).

## 확인한 결과와 한계

| 영역 | 재검토 결과 | 남은 범위 |
|---|---|---|
| 증거·소스 | 수신 69파일 원 Git 바이트와 해시 일치. 소스 17개는 Git 원형6개·Windows CRLF11개로 모두 해소 | private 원본 모델·EXE·로그를 이 호스트에서 다시 열거나 실행하지 않음 |
| 차트 | 양쪽 7단위 CSV 14개 값·단위·형식·출처와 native report 지문 일치. 양쪽 저장 파일 바이트 동일 | 모든 PNG/SVG 내보내기·Office·실제 OS drag/DPI·원본 도구 전체 조합과 별도 |
| 리본·언어 | 180언어/탭 조합·18테마 기록, 대비66개 독립 재계산(최소4.904:1). 대표 번체 dark 화면 가독성 확인 | 실제 OS 배율은100%; WebView125%를 OS125%로 계산하지 않음. 유창한 사용자 언어 검수·IME·전체 키보드 조합 필요 |
| x86 메모리 | 일반 production36주기/108열기, 측정8지점·동일7프로세스의 기존 예산 재계산 통과 | 한 번의 유한 고정 부하 결과. 과거 FAIL·고유/본문/장기 부하·OS private와 JS heap 차이는 남음 |
| 화면 이력 | 13기록: 과거 FAIL2·PASS10·PARTIAL1 보존 | native80카드 관측은 접근성 제한으로 PARTIAL; headless80개 검증으로 치환하지 않음 |
| 워크스페이스·팩 | 수신 Windows양쪽/Linux headless 각3케이스/9exports, 모델219개 및 준비용 팩 행렬 기록 확인 | 실제 공식 서명 팩, embedded startup, 설치·업그레이드·복구는 미완료 |

이전 WREV2-01의 실제 Windows 7단위 차트/CSV 저장 경로는 이번 수신 범위에서
양쪽 아키텍처 확인이 추가됐다. T-627 전체 완료나 원 리뷰의 독립 PASS는 아니다.
본문 x64는 실제 SEC-10 preflight 허용 조건을 유지하며 정책 우회는 수행하지 않는다.

## 로컬 재검증

- stream progress 가상 시간 테스트 race100회, stream/app 전체 대상 race 통과.
- 같은 대상 vet, 프런트엔드 state·production build 통과.
- 번역 누락·placeholder 검사: 추가 언어별2197문구, 일반/최적화 Python 모두 통과.
- offline assets/input assistance 검사 통과.
- 새 읽기 전용 수신 감사 코드는 일반/최적화 Python 결과가 같다.
- 현재 Mac에서 새 Windows 실행·드라이버·보안 정책 변경·실제 GPU·자연 압력
  수용 검증은 수행하지 않았다.

계산 코드·정규화 지문·수치·명령 로그 지문:
[수신 감사](evidence/2026-11-16-windows-review/received-audit.json),
[로컬 검증](evidence/2026-11-16-windows-review/local-validation.json).

## 정식 릴리스까지 남은 작업

1. 공식 기존 signer로 정식 호환 팩을 발행하고, 새 설치의 embedded 초기화와
   RC→정식 전환·owner 활성 상태·복구를 검증한다. 기존 팩의 정식0.4.0 거절과
   준비된 저장소의 manager 재열기를 실제 설치 성공으로 확대하지 않는다.
2. T-601의 격리 WinDivert·서비스·다른 세션·프록시 무시 client 범위를 검증한다.
3. T-627/T-663/T-664: 남은 원본/reference·renderer/Office·실제 OS DPI·언어·
   키보드/IME/drag와 독립 언어 검수. WREV3-01은 T-663 후속으로 처리한다.
4. T-611 후 T-613: vendor/외부 early-bound COM·cookie·clean-host 등록, 원본16-case
   동등성, 실제 gate 허용 x64 본문, 격리 디스크 고갈·전원 손실·장기/고유/본문 부하.
5. T-654 실제2-NVIDIA-GPU CUDA/NCCL, T-662 승인된 자연 swap/압력 근거는 TODO.
6. 현재 v0.4.0 지원 범위에서 완료할 항목과 명시적으로 유예할 항목을 확정하고
   다른 검토자의 수용 판정을 받는다. 과거 CONDITIONAL을 임의로 바꾸지 않는다.
7. 현재 소스의 리본·5언어를 정식 CHANGELOG·사용자 문서·배포 범위에 반영하고,
   버전/플랫폼 숫자 빌드/라이선스·번들 자원을 갱신한다. 숫자 빌드는 마지막 RC보다
   증가시킨다. 지원 OS의 새 설치·RC 업그레이드와 오프라인 런타임을 확인한다.
8. 최종 동일 커밋에 전체 회귀·3플랫폼 패키징 preflight·첨부 파일 검증을 적용한다.
   OS 코드서명/공증 정책은 필수 팩 서명과 구분해 결정한다.

위 확인 사항과 후속을 작업표에 반영한 뒤 이 리뷰를 미처리 위치에 둔다.
기존 수신 원본·manifest·판정은 수정하지 않으며 작성자와 다른 처리자의 재검토가 남는다.
이번 요청의 결과는 로컬 문서 변경이며 커밋·push는 포함하지 않는다.
