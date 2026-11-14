# T-613 메모리 증가 추세 검증

기존2시간 실행의 root guard와 tree 합계만으로는 워밍업·캐시와 지속 누수를
구분할 수 없어 추가 검증했다. Windows production x64/x86의 고정 반복 부하·idle,
공통 엔진의 GC 후 잔존 힙·goroutine, Docker core race 검사를 수행했다.
이번에 정한 유한 검증 범위의 기준은 통과했다. 모든 입력·압력 조건의 누수 없음이나
전체 T-613 완료를 증명하지 않는다.

## 실제 production 앱의 반복 부하와 idle

앱은 이전2시간과 같은 source324a8d97 production 바이너리이며 해시도 일치한다.
2분 초기 idle 뒤 동일한512건 요청/128행 보관/Clear/3분 running-idle을10회
반복했다. 첫2회는 워밍업, 뒤8회는 측정 구간이다. 마지막 finalized 종료·128행
저장/재열기 뒤5분 idle도 관측했다. 양쪽 모두 총5,120건, 드롭0이며 최종 세대의
512건 캡처=저장과128행 전체 필드·주석 정책·선택 행 Raw200을 확인했다.

| 항목 | x64/amd64 | x86/386 |
|---|---:|---:|
| 실제 실행 시간 | 2,482.907초 | 2,483.728초 |
| 측정 idle의 tree private 중앙값 차이 | +9.125MiB | +13.125MiB |
| 루트 private 중앙값 차이 | +0.387MiB | +1.668MiB |
| 루트 핸들 중앙값 | 613→613 | 685→693 |
| 루트 스레드 중앙값 | 31→30 | 33→33 |
| finalized5분 idle 끝의 루트 핸들 | 609 | 689 |

측정 전에 고정한 기준은 첫3개/마지막3개 idle 중앙값의 차이와8개 지점의
모든 쌍 기울기 중앙값(Theil–Sen)을 함께 사용한다. tree private는 차이64MiB·
cycle당4MiB, 루트 private는16MiB·cycle당1MiB, 루트 핸들은24개·cycle당2개,
스레드는4개·cycle당0.5개 이하라는 screening budget이다. 0증가나 통계적
평탄성을 요구하는 기준이 아니며, 이 범위의 작은 누수를 배제하는 검정도 아니다.
결과 분류는 정확히 `BOUNDED_IN_THIS_RUN`이다. 회귀8개는 평탄/변동 데이터,
지속적인 메모리·핸들·스레드 증가, 큰 계단 증가와 누락/잘못된 표본을 확인한다.
CI에는 이 판정 회귀를 연결했으며 hosted CI 실행 결과는 주장하지 않는다.

같은 PID/생성 시각에서 유도한 run-local ordinal과 역할로 root/browser/renderer/
GPU/utility/crashpad를 추적했다. 측정8회 모두 동일한7개 프로세스 집합이어서
프로세스 재시작에 의한 메모리 감소를 plateau로 오인하지 않았다. 실제 PID,
명령줄·사용자 프로세스 목록은 공개하지 않는다. OS sampler는 root exe·시작
identity를 확인하고, .NET Process 객체도 매번 Dispose한다.

관측된 renderer private 중앙값 차이는 x64+7.906MiB/x86+12.910MiB, GPU는
+1.867/+4.668MiB이다. 역할별 중앙값은 따로 계산하므로 그 합이 tree 중앙값
차이와 같을 필요가 없다. 증분이 이 역할들에 관측됐다는 의미이며 특정 캐시나
객체가 원인이라고 확정하지 않는다. 원시 표본은 native-*.json, cycle별 값은
matched-idle-cycles.csv, 산술과 데이터의 별도 재계산은 readback-*.json에 있다.

두 앱은 서로 다른 capture/WebView 프로필·proxy와 producer PID/start를 사용하며
같은 Windows 호스트와 Docker loopback origin을 공유했다. 다른 검증 프로세스도
같은 호스트에서 실행됐으므로 독점 자원 benchmark가 아니다. sample 간15초를
기다리지만 실제 관측 간격에는 OS 조회 시간이 추가된다. 현재 살아 있는 자손만
측정하고 모든 순간의 peak나 JS heap/Go heap을 직접 측정한 자료는 아니다.

native-cases.json의4개 화면은 실제 running/finalized 동작 확인이다. 동일 제목
창은 활성화 후 새 관측과 화면 proxy5698(x64)/5699(x86)로 대조했다. 한 running
관측 사이에 행 선택이 바뀌어 정확히 같은 renderer 선택 상태의 증거로 사용하지
않는다. 가려진/사용자 입력 감지 frame은 새로 관측해 제외했다. 본문은 꺼져 있고
강제 GC·working-set trim·브라우저 캐시 삭제·보안 UI 변경은 하지 않았다.

## 공통 엔진의 GC 후 잔존 메모리

Windows 양쪽에서 같은 서비스 객체로24회 실제 loopback proxy의 시작/512요청/
Clear/정지/포트 재사용/소유 finalized store 정리를 반복했다. 각 cycle 뒤 테스트
프로세스에서만 GC2회와50ms settle 후 heap을 읽었다. production 앱이나 WebView에
GC를 강제하지 않았다. 최초8회 제외 후 첫4개/마지막4개 중앙값을 비교한다.

| GC 후 측정 | x64/amd64 | x86/386 |
|---|---:|---:|
| 전체 요청 / cycle 수 | 12,288 / 24 | 12,288 / 24 |
| 잔존 힙 중앙값 차이 | +262,248B | +18,568B |
| heap objects 중앙값 차이 | +239 | +374 |
| goroutine 중앙값 차이 | 0 | 0 |

사전 기준은 잔존 힙 차이8MiB, 객체5,000개, goroutine8개 이하이며 모두 통과했다.
이것은 공통 엔진 반복 수명주기의 범위이고, 모든 finalizer 완료나 GUI heap의
누수 없음을 증명하지 않는다. Windows는 CGO=0이므로 Windows race 통과를
주장하지 않는다. 별도 Linux Docker Go1.26.10에서 capture24개 패키지의 race는
통과했다. 초기 직접386 go test는 기존 Wails ABI wrapper를 우회해 실패했으며,
node build/windows/test.mjs로 재실행해 통과했다. 최초 Docker 빌드는 noexec
tmpfs에서 toolchain 실행이 거절됐고, 격리된 executable build tmpfs로 보정했다.
이 준비 실패를 제품 누수나 성공 표본으로 사용하지 않았다.

재현(저장소 루트):

```powershell
node --test scripts/test-desktop-memory-checks.mjs
node scripts/verify-windows-desktop-memory.mjs --exe .tmp/p2-windows-20261114/archscope-p2-amd64.exe --out .tmp/new-memory-amd64 --origin http://127.0.0.1:11512/
node scripts/verify-windows-desktop-memory.mjs --exe .tmp/p2-windows-20261114/archscope-p2-386.exe --out .tmp/new-memory-386 --origin http://127.0.0.1:11512/
```

엔진 opt-in은 app 디렉터리에서 ARCHSCOPE_CAPTURE_MEMORY_CYCLES=24 및 절대
ARCHSCOPE_CAPTURE_MEMORY_REPORT 경로를 지정하고, GOARCH=amd64/386 별로
`node build/windows/test.mjs -tags production -run '^TestCaptureLifecyclePostGCMemory$' -count=1 -timeout 15m .`
를 실행한다. 일반 test 실행은 이 긴 측정을 건너뛴다.

사용한 runner·sampler·판정기·Go test·비교기와 해시를 보존했다. 평평한 snapshot
폴더를 직접 실행하지 않고 원 저장소의 상대 import 구조를 사용한다. 새 복구
초기화 수정8985736f의 native 오류/재시도 검증은 별도 실패 경계 영수증에 있다.
이 memory 실행이 새 renderer bundle 전체를 장시간 재검증했다고 주장하지 않는다.

cleanup.json에서 소유 앱·producer·WebView·창·컨테이너0개, 기존 Docker7개
유지를 확인했다. private 세션·credential은 공개하지 않는다. 이전 영수증과 원
독립 판정은 변경하지 않았으며 T-613 PENDING/T-627 REVIEW를 유지한다.
현재 추가 범위와 남은 인수는 remaining-acceptance.json에 구분했다.
