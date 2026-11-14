# T-613 / WREV-01 Windows private-file 재검증

기존 Windows x64/x86 실제 캡처에서 보존한 private rolling store·export를
maintained 비교 정책1.0.0으로 다시 읽었다. 새 앱 실행·주석 편집·UI 관측은
아니며, 원래 파일과 과거 증거는 수정하지 않았다.

| 기존 실행 | x64 | x86 |
|---|---:|---:|
| 중첩 query | 4행 PASS | 4행 PASS |
| 130초 schema4 | 128행 PASS | 128행 PASS |
| live grid 및 비어 있지 않은 별/주석 | 640행 PASS | 640행 PASS |

6개 파일 모두 ID 집합·전체 transaction 필드가 명시적 정책과 일치한다.
expected host/query는 source 보관 URL에서 유도해 export 값과 비교하며,
export 값을 덮어쓰지 않는다. 별/주석은 store annotations.json의 ID별 값을
권위로 사용한다. 생략된 필드의 Go 기본값 false/빈 문자열만 보완하고 제외하는
transaction 필드는 없다. 두 live640행 파일은 선택된 요청50의 별과 비어 있지
않은 Unicode 주석을 포함하고 이를 정확히 비교했다. private 주석 내용·ID는
출력하지 않는다. 원본 export 길이·SHA-256은 과거 공개 영수증과도 일치한다.

`query-*.json`, `soak-*.json`은 maintained CLI의 schema2 출력이다.
`live640-*.json`은 같은 compare_transactions 코어를 호출하는 별도 wrapper의
출력이다. wrapper는 원래 producer의 index별200/404/503 상태 규칙도 대조한다.
최초 probe가 모든 응답200을 잘못 가정해 거절했으나, 원래 fixture가 의도한
혼합 상태임을 확인하고 producer 규칙으로 보정했다. 제품 오류가 아니며
실패한 probe는 성공 보고서를 쓰지 않았다.

사용한 verifier·live wrapper를 함께 보존하고 source/artifact 해시를 기록했다.
원본 파일은 이미 종료된 이전 테스트의 격리 저장소에서 읽기만 했다.
이 결과는 WREV-01의 native private-file 비교 후속을 해당6파일 범위에서
충족한다. 이전 boolean을 소급해서 바꾸거나 원 독립 리뷰의 CONDITIONAL을
PASS로 바꾸지 않는다. T-613 전체 인수와 새로운 주석 편집 UI 테스트는 별도다.
