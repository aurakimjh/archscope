# T-630 사건 수명주기 구현 재리뷰 R2 — 2026-11-11

- 대상: 독립 리뷰(`29eec49`, 대상 `82bc6a8`)의 R6-01~R6-07을 처리한 수정 커밋 `e2c1c7d`(fix(cases): remediate T-630 independent review findings). 확인 기준 HEAD는 `e2c1c7d`다.
- 계약 기준: `docs/ko/DATA_MODEL.md` "T-630 사건·capture 저장 계약"(수정분 포함), `docs/ko/PARSER_DESIGN.md` "T-630 안전한 intake·adapter 경계", `work_status.md`의 R6-01~R6-07 처리 표
- 증거: `docs/review/evidence/2026-11-11-t630-r6/local.json` (`source_sha256` 32개 항목이 `e2c1c7d`의 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 HTTP 변경은 수정하지 않았다.
- 방법: 수정 diff 정독, 저장소 밖 scratch worktree(`e2c1c7d`)에서 이전 재현을 그대로 다시 실행(미커밋 throwaway probe), 이 커밋에서 빌드한 실제 CLI로 신규 명령 실행.
- 범위: R6-01~R6-07 처리분과 그 부작용. T-631(generic 결과 소비자)·T-635는 평가하지 않는다.

## 1 결론

**판정: 승인에 가까운 조건부.** R6-01~R6-07 일곱 건이 모두 재현 기준으로 해소되었다. 신규 P3가 1건(R7-01) 있고 P1·P2는 없다. 이전 리뷰가 T-630의 `DONE` 조건으로 둔 R6-01·R6-02가 닫혔으므로 **T-630 `DONE` 복귀가 가능하다.** R7-01은 종료를 막지 않는다.

- **T-635 착수:** R6-02가 닫혀 막히지 않는다. 다만 `work_status.md`가 이미 적었듯 원본 페이지 색인이 100만 행까지만 접두(prefix)를 덮으므로 T-635는 이 범위 표시(`indexed_lines`/`index_truncated`)를 반드시 따라야 한다.
- 실제 WebView IPC/UI(특히 새 "결과 캐시 관리" 버튼), Windows 네이티브 실행, hosted CI는 여전히 미검증이다(4절).

## 2 이전 finding 확인

| ID | 등급 | 결과 | 재현 근거 |
|---|---|---|---|
| R6-01 내보내기 보호 대소문자 우회 | P2 | **해소** | 대소문자 비구분 볼륨(macOS)에서 저장소 경로를 대문자로 바꾼 `HEAD.json`·새 파일·저장소 root 대상이 모두 `CASE_EXPORT_DESTINATION_UNSAFE`. 저장소 밖 하드링크는 `destination aliases managed case data`로 거절. 저장소 밖 정상 경로는 허용. 수정은 모든 존재 조상의 `os.SameFile`과 저장소 전체 파일의 identity 대조다. |
| R6-02 evidence-cache 정리·saved case 열기 | P2 | **해소** | 일반 캐시 1,024개를 채운 뒤 새 import는 `IMPORT_STORE_FULL`로 실패하지만 안내 문구가 `CleanupEvidenceCache`/`evidence cleanup-cache`를 가리킨다. saved case 열기(`LoadOwnedSnapshot`)는 별도 예비 영역(32개·64 MiB)으로 32개까지 성공하고 33번째만 `IMPORT_REOPEN_FULL`(조치 안내 포함). 같은 바이트는 동일 handle을 공유하고 lease가 따로 발급된다. 마지막 lease 해제 시 예비 파일이 삭제되고 `reopened/`는 재시작 시 비워진다. 소유 lease가 있는 항목은 `administrative Release`가 거절된다. 정리는 소유자 있는 32개를 보존하고 1,024개를 제거했으며 이후 새 import가 성공했다. |
| R6-03 같은 결과 재저장 중복 | P3 | **해소** | 동일 결과·source·selection·옵션·공유 승인 저장은 참조 1개(revision 2)로 유지. selection 키 순서가 달라도 같은 것으로 보고, `9007199254740993`과 `…992`는 서로 다른 참조로 구분하며, 공유 승인이 다르면 새 참조. |
| R6-04 손상 사건 삭제 불가 | P3 | **해소** | 읽을 수 있는 사건은 `CASE_NOT_CORRUPT`로 거절. HEAD를 손상시킨 사건의 `corrupt-delete-plan`이 파일 수·크기·제외 범위를 보이고, 트리가 바뀌면 이전 token과 빈 token이 거절되며, 현재 token으로만 `cases/<id>`가 제거된다. 사건 밖의 외부 원본은 그대로 남았다. 실제 CLI에서도 동일. |
| R6-05 이관 복구 경로 | P3 | **해소(R7-01 잔여)** | 빈 legacy 디렉터리와 `results`만 있는 디렉터리 모두 이관이 성공한다. 손상 snapshot은 보존한 채 `IMPORT_MIGRATION_RECOVERY_REQUIRED`와 조치(앱 종료·두 디렉터리 보존·legacy 폴더 이름 변경 후 재시작)를 알리며 완료 marker를 쓰지 않는다. |
| R6-06 원본 페이지 비용·100만 행 | P3 | **해소** | 60 MiB·786,432행: 첫 페이지 약 310 ms(이전 340 ms는 매 호출), 이후 페이지 5개 약 110 ms(페이지당 약 22 ms, 원본 전체를 스트리밍 hash로 재검증). 5,719,505행 파일은 이제 거절되지 않고 `total_lines`=5,719,505, `indexed_lines`=1,000,000, `index_truncated:true`로 표시한다. |
| R6-07 콜론 파일명 | P3 | **해소** | `jstack_10:30:01.txt`가 실제 CLI와 단위 probe에서 모두 intake되고 이름이 그대로 보존된다. 표시 이름은 제어 문자·길이·UTF-8만 제한하고 archive/capture 경로 규칙은 그대로다. |

## 3 신규 finding

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R7-01 | P3 | 이관 복구 오류가 실제 원인을 숨기고 단순 잠금 충돌에도 "legacy 폴더 이름 변경"을 안내함 | `analysis/migration.go` `fail`, `MigrationRecoveryError.Error` |

### R7-01 이관 복구 오류가 원인을 숨김 (P3)

**근거**: 수정된 `fail`은 컨텍스트 취소를 제외한 **모든** 오류를 `IMPORT_MIGRATION_RECOVERY_REQUIRED`로 감싸고, `Error()`는 `Cause`를 출력하지 않는다(`Cause`는 `json:"-"`). 따라서 읽을 수 없는 파일, 권한 오류, 손상된 JSON, 다른 프로세스의 lock이 모두 같은 문구가 된다.

**재현**(throwaway probe):
- legacy 폴더에 손상 snapshot이 있을 때: 복구 안내가 나온다(의도된 동작). 그러나 어떤 파일이 왜 실패했는지 메시지에 없다.
- 다른 소유자가 legacy 저장소 lock을 잡고 있을 때(`IMPORT_STORE_BUSY`가 원인): 같은 `RECOVERY_REQUIRED`와 "legacy 디렉터리 이름을 바꾸고 재시작하라"가 출력된다. 실제 원인은 잠깐 기다리면 풀리는 잠금 충돌이다.

**영향**: 사용자가 필요 없는 폴더 이름 변경을 하거나, 반대로 실제 손상 파일을 찾지 못한다. 데이터를 지우라는 안내는 아니고(보존을 명시) 오류도 fail-closed이므로 피해는 혼란에 그친다.

**수정 제안**: `Error()`에 `Cause`(어느 handle/파일, 잠금 여부)를 포함하고, `IMPORT_STORE_BUSY`는 복구 안내로 감싸지 말고 그대로 반환한다. 원인별로 다른 조치를 안내한다.

## 4 검증 범위

### 실행한 것 (Mac arm64)

| 항목 | 결과 |
|---|---|
| `go vet ./...` | 통과(출력 없음) |
| 전체 `go test -race -count=1 ./...` | 96개 패키지 통과, 실패 없음 |
| Windows amd64 교차 `go vet`(`internal/cases`, `internal/analysis`, `cmd`) | 통과 |
| 프런트엔드 `tsc --noEmit`, `npm run test:state` | 통과 |
| 이전 재현 probe 재실행(scratch worktree, 미커밋) | R6-01~R6-07 모두 해소 확인, R7-01 재현 |
| 실제 CLI | 콜론 파일명 intake, 손상 HEAD → `corrupt-delete-plan`→잘못된 token 거절→`purge-corrupt`, `evidence cleanup-cache --cache-dir` |
| 증거 `source_sha256` 32개 | 일치 |

### 코드·테스트 정독으로만 확인한 것

예비 영역 lease 해제의 `os.Remove` 실패 경로(이미 lease는 제거됨), `CleanupEvidenceCache`가 진행 중인 import와 `beginLoad` slot을 공유하는 점, 화면의 lease 획득과 import 완료 사이의 짧은 틈(`EvidenceResultPanel`이 비동기로 lease를 얻음)은 정독으로만 보았다. `1.10`과 `1.1` 같은 선택 값은 서로 다른 참조로 처리되는데(보수적), 수정 의도와 어긋나지 않아 finding으로 올리지 않았다.

### 실행하지 않은 것 (기존 미검증 항목과 구분)

- 실제 WebView IPC/UI: 새 "결과 캐시 관리" 버튼과 중복 저장 방지, 사건 열기 흐름을 눌러 보지 않았다. 손상 사건 삭제는 UI가 없고 CLI/Wails 메서드뿐이다.
- Windows 네이티브 실행: 교차 vet만 있다. 내보내기 보호의 `os.SameFile` 대조(Windows 파일 index), `LockFileEx`, 대소문자 경로를 Windows에서 실행하지 않았다.
- hosted CI, 실제 producer 입력(Q01~Q04), 서명 pack 검증(T-632).
- 캐시·저장소 상한 경계(4 GiB, 128 사건)와 프로세스 강제 종료 후 복구.

## 5 처리 권장 순서

1. R7-01: 이관 오류에 원인을 포함하고 잠금 충돌을 분리한다. T-630 종료를 막지 않는다.
2. T-635 설계에서 원본 페이지의 `index_truncated`를 소비자가 반드시 표시하게 한다(이전 리뷰 6.4 항목의 후속).
