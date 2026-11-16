# Windows 추가 검증과 메모리 실패 검토

- 검토일: 2026-11-17, Codex.
- 범위: `635512cf..acdd1d49`, 8커밋·128파일.
- 판정: CONDITIONAL. 새로 확정한 제품 P1/P2 없음. 일반 body-rich 메모리 FAIL 두 건은
  미해결 수용 조건이며 정상화하거나 계측 결과로 대체하지 않는다.
- 기존 8개 미완료 상태와 이전 독립 CONDITIONAL 유지. 이전 검토의 WREV3-01은
  이번 배치 3번에서 별도 수정할 예정이며 이 검토 커밋에서는 미수정이다.

## 재검토 결과

수신 원본 111개 파일의 SHA-256·길이와 manifest 전체 파일 집합이 일치한다.
별도 진단 코드 4개의 원 지문과 Git 지문도 대조했다. CRLF 차이는 양쪽 지문을
기록하며 코드 불일치로 해석하지 않는다. 원문·실패·취소·검사기 수정 기록은 보존했다.

- Windows 양쪽 7단위 Canvas→SVG 및 SVG→PNG의 총 28개 실제 저장 파일을 확인했다.
  SVG의 축·값·실제 bar와 PNG CRC·압축 해제 크기·2배 이미지 크기가 일치한다.
  PNG 7쌍은 양쪽 바이트가 같다. 초기 x64 toolbar SVG를 측정한 메타데이터는
  신뢰하지 않고 저장 파일 크기를 사용했다. 최초 x86 취소는 PASS가 아니다.
- 일반 production 고유 파일 검사: 양쪽 36주기·108열기, 기존 28준비·8측정과
  예산을 유지해 재계산 PASS. 본문 파일 검사는 양쪽 FAIL을 그대로 재현했다.
- Windows x64/x86 및 Linux headless의 216개 projection 입력/결과/행 fingerprint
  기록이 전부 일치한다. private EXE·전체 원본 파일을 여기서 다시 열지는 않았다.
- 별도 계측의 181개 작은 관측 중 마지막 8개 process 표본에 선행하는 JS 관측을
  다시 매칭했다. 최대 지연 5.328초이며 사전 기준 10초 이내다. 카드80·배열80·
  행10240은 유지된다. JS used heap의 기울기 약0.004MiB/주기와 renderer private의
  약4.458MiB/주기는 특정 native allocator를 증명하지 않는다.

| 일반 production 부하 | x64 tree 기울기 MiB/주기 | x86 tree 기울기 MiB/주기 | 결과 |
|---|---:|---:|---|
| 고유512행/108파일 | 0.052 | -0.393 | 둘 다 PASS |
| 본문128행/108파일 | 4.801 | 8.764 | 둘 다 FAIL; 원 기준4초과 |

Root·handle·thread 예산은 통과한다. 증가 역할은 renderer에 집중되지만, 공개
수치만으로 DOM·native buffer·V8 commitment·할당기 중 어느 원인인지 확정할 수 없다.
계측 run의 tree 기울기3.755 통과는 일반 run의 실패를 취소하지 않는다.

## 다음 단계

1. 2번 작업에서 renderer 역할·JS/DOM/native allocation 관측 범위와 변동성을
   분석하고, payload 없는 후속 관측·고정 프로토콜의 Windows 재검증을 준비한다.
   근거 없이 보관량을 줄이거나 메모리 예산을 완화하지 않는다.
2. 3번 작업에서 WREV3-01 결과 JSON 발행/읽기를 수정하고 race·종료·손상 경계를
   회귀 검사한다. 수신 결과 원본이나 과거 실패 판정은 변경하지 않는다.
3. 공식 서명 팩·실제 설치/업그레이드, driver/vendor/full reference/OS-DPI/IME,
   실제 GPU·자연 압력 및 독립 검토 조건은 기존 열린 상태로 유지한다.

재현 가능한 읽기 전용 검증:
[감사 코드](evidence/2026-11-17-windows-additional-review/audit_received.py),
[계산 결과](evidence/2026-11-17-windows-additional-review/received-audit.json),
[진단 소스 지문](evidence/2026-11-17-windows-additional-review/source-bindings.json).
이 검토는 새로운 Windows 실행이나 이전 Codex 리뷰의 독립 종결이 아니다.
