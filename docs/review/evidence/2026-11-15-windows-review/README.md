# Windows 수신 결과 검토 근거

2026-11-15, ca722025..fbf2ce2c의 코드·수신 증거를 검토했다.
원 증거는 수정하지 않고 이 디렉터리에 재계산과 검증 결과만 둔다.

- audit_received.py: 검토한 fbf2ce2c의 Git 바이트로 원 artifact 528개와 소스 34개를
  대조하고, case 분포·메모리 8건·차트 24파일의 공개 자료를 재계산한다.
- received-audit.json: 위 코드의 출력. 초기 메모리 실패를 그대로 보존한다.
- reproduce_units.mjs: 실제 Go nanoseconds 결과와 현재 production CSV helper로
  profiler 단위 오류를 재현한다. JFR inventory 수치 혼용도 원 공개 자료와 대조한다.
- reproduction.json: 두 발견 사항의 소스 결합 재현. native UI 실행은 아니다.
- local-validation.json: macOS 대상 테스트·race·vet·프런트엔드·자원 검사 결과와 로그 지문.

재계산은 저장소 루트에서 실행한다.

```sh
python3 docs/review/evidence/2026-11-15-windows-review/audit_received.py
```

단위 재현은 새 임시 파일에 Root;work 1000000 한 행을 쓰고, 아래처럼 결과를 만든 뒤 실행한다.
입력·출력은 본인이 소유하는 새 경로를 선택한다.

```sh
# apps/engine-native에서 실행
go run ./cmd/archscope-engine profiler analyze-collapsed --in /tmp/owned-weight.collapsed --profile-kind lock --value-unit nanoseconds --out /tmp/owned-weight.json
# 저장소 루트에서 Node 25 실행
node docs/review/evidence/2026-11-15-windows-review/reproduce_units.mjs /tmp/owned-weight.json
```

원 Windows store·바이너리·JFR·private log는 이 호스트에 없으며 수신된 해시/영수증으로만
대조했다. 공개 process ordinal은 실제 PID 인증이나 생성자 인증을 대신하지 않는다.
이번 검토는 새로운 Windows native 테스트나 기존 CONDITIONAL의 독립 PASS가 아니다.
