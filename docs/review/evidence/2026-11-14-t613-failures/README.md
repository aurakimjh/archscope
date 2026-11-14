# T-613 저장·복구 실패와 재시도

Windows production x64/x86와 제한된 Docker tmpfs에서 현재 가능한 실패 경계를
검증했다. 본문은 두 아키텍처 모두 비활성이다. 원래 사용자 저장소나 호스트
디스크를 채우거나 손상시키지 않았다.

기준 앱324a8d97에서 과거 저장소의 커밋된 행 하나를 격리 fixture 안에서만
잘라 복구 실패를 만들었다. 데이터 읽기는 올바르게 거절했으나, renderer의
초기 Promise.all도 함께 거절되어 정상 Windows capability·백엔드·CA 상태가
적용되지 않았다. 네이티브 화면은 빈 백엔드와 ‘사용 가능한 모드 없음’, 잘못된
Windows-only 안내를 표시했다. 첫 사용 확인이 미선택이므로 Start 비활성 자체를
이 결함의 근거로 삼지 않는다. 기준 FAIL 화면과 접근성 원문을 보존했다.

8985736f는 과거 저장소 복구 오류를 core capability 로딩과 분리한다. 경고는
hydrate 후에도 남기며, 손상 행을 무시·절단·자동 복구하지 않는다. core capability
자체의 오류는 계속 전체 초기화를 거절한다. 회귀·프런트엔드 production build와
양쪽 production 빌드가 통과했다. 수정된 두 네이티브 앱은 corrupt 경고와 함께
정상 explicit proxy·proxy_h1_mitm·실제 CA/본문 정책을 표시한다. x64는 body gate가
닫혔고 새 x86 exe는 정상 SEC-10 preflight에서 capability가 열렸지만, 본문·첫
사용 체크박스는 그대로 꺼 두었다. 앱의 정상 per-executable preflight가 실행됐으며
검증 도구가 수동으로 보안 정책이나 CA 설정을 바꾸지 않았다.

| 실패/재시도 | x64 | x86 |
|---|---|---|
| 기존 디렉터리로 export 거절, sentinel·커밋된6행 보존 | PASS | PASS |
| 기존 디렉터리를 소유 fixture 안에서 보존 이동 후 같은 경로 저장 재시도·선택 복원 | PASS | PASS |
| 손상된 커밋 행 읽기 거절·나머지5행 byte 보존 | PASS | PASS |
| 손상 과거 저장소가 있는 동안 별도 새4행 캡처·finalized 종료 | PASS | PASS |
| 정지 후 정확한 backup 복원·원래6행 byte/ID 보존·재열기 | PASS | PASS |
| 복원 export 전체 필드·주석 정책 비교와 선택 행 Raw200 native 화면 | PASS | PASS |

손상은 이미 finalized된 합성 저장소에 대한 제어된 변조다. 실제 전원 손실이나
실행 중 torn-write 전체 조합을 재현하지 않는다. backup 복원도 검증 도구가
소유 앱을 종료한 뒤 직접 수행한 것이며 제품의 자동 수리 능력이 아니다.
export 실패 경로의 기존 디렉터리와 내용은 삭제하지 않고 보존했다.

Windows 양쪽의 TestReserveRefusalPreservesCommittedRowsAndAllowsRetry는
주입한 free-space 값으로 실패 전후 manifest·기존 record bytes·물리적 행 수가
변하지 않음을 확인하고 공간 회복 후 재시도·재열기 ID를 검증한다.
이는 Windows 디스크를 실제로 채운 테스트가 아니다.

Docker의600MiB tmpfs는 실제 OS 여유 공간을629,141,504B에서535,822,336B로
낮췄다. 기본536,870,912B reserve를 유지했으며 append가 거절되어0행을 공개했다.
소유 임시 파일93,319,168B를 제거하자 여유 공간이 회복됐고 동일 요청1행 저장이
통과했다. 호스트 디스크를 채우지 않았고 실제 ENOSPC까지 밀어붙이지 않았다.
이 두 reserve 테스트는 Docker Go1.26.10 race에서도 통과했다.

새 native 원본 결과, 비교기·runner·회귀 소스 snapshot, 로그 및 해시를 함께
보존한다. 실제 controller credential·세션·record 내용은 공개하지 않는다.
동일 제목 창은 exe 경로와 별도 활성화/새 관측으로 구분하고, 최종6행 화면은
private export의 producer PID와도 대조했다. 가려진 화면은 공개 증거에서 제외했다.

초기 probe는 강제 종료 후 남은 O_EXCL descriptor를 보존 이동하지 않아 재기동
준비에 실패했다. 소유 PID/token의 descriptor만 이동하도록 보정한 뒤 다시
실행한 결과가 baseline-data.json과 fixed-*.json이다. 지원하지 않는 recover
명령도 준비 과정에서 제거했고 실제 제품 list/read로 손상 거절을 검사한다.
실패한 최초 probe를 제품 실패나 PASS 증거로 사용하지 않는다.

전원 손실·Windows 실제 디스크 고갈·service/driver/타세션 reach와 전체 원본
동등성은 남아 있다. 이 후속은 T-613 전체 완료나 원 독립 리뷰 PASS 판정이 아니다.
