# WREV2-01·02 수정과 회귀 검증

2026-11-15 사용자 지시에 따라 P2 단위 오류와 P3 JFR 검증 목록을 수정했다.
원 검토 문서와 원 수신 증거는 변경하지 않는다. 두 항목은 로컬 수정·검증 완료이며
새 Windows 네이티브 실행이나 기존 CONDITIONAL의 독립 PASS를 의미하지 않는다.

## P2: 실제 프로파일 단위 유지

Chart Studio는 선택한 결과의 summary.value_unit에서 단위를 결정하고 같은 단위를
차트 축과 CSV에 사용한다. samples/ns/us/ms/s/bytes/unknown의 의미와 값은 유지한다.
누락·잘못된 단위를 samples로 바꾸지 않고 unknown으로 표시한다.
다른 차트의 고정 단위와 comparison의 metric-specific 의미는 그대로 유지한다.

- production state 회귀: 7단위의 값·0/null·전체 출처 보존, 단위 누락/미지원,
  다른 template 단위와 입력 불변성을 검사한다.
- 실제 Go CLI에서 생성한 7개 결과를 production CSV helper로 검증했다.
  fbf2ce2c 원 소스는 6단위에서 실패하며 samples만 맞는다.
  수정 후 Mac·격리 Docker는 7/7 통과한다.
- verify_units.mjs의 --baseline은 원 helper/template을 Git에서 읽어 별도 임시
  모듈로 실행한다. 원 수신 코드나 현재 소스를 바꾸지 않는다. --baseline의
  exit1은 의도한 수정 전 실패다.
- 원 검토 디렉터리의 reproduce_units.mjs/reproduction.json은 수정 전 기록이다.
  현재 회귀·전후 비교에는 이 디렉터리의 verify_units.mjs를 사용한다.

## P3: 현행 검증 목록 정정

[remaining-acceptance.current.json](remaining-acceptance.current.json)은 원 목록의
JFR 항목만 정정하고 정확한 모집단 수·원 목록과 근거 해시를 추가한다.
전체/필터 이벤트는 3,010, sample events/stack samples는 153, unique stacks는 29다.
다른 route·남은 작업·T-613 PENDING·T-627 REVIEW는 원 목록 그대로다.
통합 중 수신한 2026-11-15-windows-conditional의 추가 목록은 지문과 함께
relatedIncrementalInventory로 연결한다. 21개 route 정정과 추가 조건별 수용은
각각의 범위를 유지하며 함께 읽는다.

update_inventory.py는 원 artifact manifest에 기록된 바이트와 지문을 확인하고
native 텍스트의 각 표시값, 독립 JDK 이벤트 종류별 합계와 total을 대조한다.
검증은 Python -O에서도 유지한다. verify_inventory.py의 8건은 정상 파일과
잘못된 설명·계수·DONE 승격을 일반/최적화 Python에서 확인한다.
원 remaining-acceptance.json·manifest·JPEG·텍스트는 덮어쓰지 않는다.

## 결과와 범위

- 전체 프런트엔드 state regression: Mac 및 network-none/read-only Node22 Docker 통과.
- 프런트엔드 production TypeScript/Vite build 통과.
- 기존 Go profiler counter-duration, CLI explicit-unit, desktop service unit-flow 검사 통과.
- 고정 fbf2ce2c의 원 528개 artifact/34개 소스·메모리/차트 재검토 결과 재현.
- 현행 JFR 목록 검사와 Python3.12 Docker -O 검사 통과.
- 기존 offline resources/input assistance 자원 검사 통과.
- Docker는 새로 만든 격리 검증 컨테이너만 사용하고 자동 제거했다.

초기 Docker 단위 실행은 fixture 생성이 완료되기 전에 시작되어 unknown.json
미존재로 종료됐다. 생성 완료를 확인한 뒤 같은 검증을 다시 실행해 통과했다.
새 state 검증의 출처 문자열 예상값 오기도 수정했다. 제품 오류나 통과 증거로
해석하지 않는다. 최종 결과만 units-after-*.json과 regression-results.json에 기록한다.
실제 Windows Save dialog/화면의 새 단위 조합은 별도 수용 범위다.

원격 3f55342b까지 추가 7커밋을 통합하면서 양쪽 작업 이력을 보존했다.
통합 후 프런트엔드 state/build·Docker 검증을 다시 실행하고 추가된
requestbuilder/summary 패키지 테스트도 통과했다. 원 검토 범위는
ca722025..fbf2ce2c로 유지하며 새 7커밋 전체의 독립 리뷰를 주장하지 않는다.

## 재실행

먼저 frontend에서 npm run test:state로 production state 모듈을 컴파일한다.
입력은 본인 소유 임시 디렉터리의 한 행 Root;work 1000000이다.
각 단위로 profiler analyze-collapsed --profile-kind lock --value-unit UNIT을 실행해
UNIT.json을 만들고 저장소 루트에서 실행한다.

```sh
node docs/review/evidence/2026-11-15-windows-remediation/verify_units.mjs /path/to/owned-fixtures
node docs/review/evidence/2026-11-15-windows-remediation/verify_units.mjs /path/to/owned-fixtures --baseline
python3 docs/review/evidence/2026-11-15-windows-remediation/update_inventory.py --check
python3 docs/review/evidence/2026-11-15-windows-remediation/verify_inventory.py
```

상세 결과는 [regression-results.json](regression-results.json),
[수정 전](units-before.json), [Mac](units-after-mac.json),
[Docker](units-after-docker.json), [정정 회귀](inventory-regression.json)에 있다.
