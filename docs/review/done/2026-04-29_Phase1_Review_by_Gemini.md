# [Report] Phase 1 Architecture Diagnosis & Code Review

**Date:** 2026-04-29  
**Reviewer:** Gemini (Principal IT Software Architecture Diagnostic Specialist)  
**Subject:** Phase 1 Foundation Stabilization & Phase 2 Scalability Assessment

---

## 1. Executive Summary

Phase 1의 기반 기능 개발(Foundation Stabilization)은 매우 견고한 데이터 컨트랙트(`AnalysisResult`)와 명확한 관심사 분리(UI vs. Engine)를 바탕으로 성공적으로 마무리되었습니다. 특히 **Privacy-first local professional diagnostic workbench**라는 제품 포지셔닝에 걸맞게, 로컬 실행 모델과 IPC 기반의 통신 구조가 안전하게 설계되었습니다.

본 진단에서는 Phase 2의 본격적인 확장(신규 분석기 추가, 대용량 파일 처리, 리포트 고도화)을 앞두고, 현재 아키텍처의 잠재적 병목 지점과 코드 스멜을 식별하고 이에 대한 전략적 리팩토링 방안을 제안합니다.

---

## 2. 아키텍처 진단 (Architecture Diagnostics)

### A. 안정성 (Stability)
- **현황:** `child_process.execFile`을 사용하여 쉘 인젝션을 방지하고, 타임아웃(60s) 및 버퍼 제한(4MB)을 적용하고 있음.
- **진단:** 
    - **타임아웃 리스크:** 수백 MB 이상의 대용량 로그 분석 시 60초는 부족할 수 있습니다. 분석기별로 기대 실행 시간이 다르므로, 이를 동적으로 설정하거나 엔진 측에서 프로그레스 이벤트를 전송하는 구조가 필요합니다.
    - **엔진 비정상 종료 대응:** 현재 `execEngine`에서 프로세스 종료 코드를 확인하고 있으나, 특정 시그널(SIGSEGV 등)에 의한 종료 시 사용자에게 제공되는 피드백이 'ENGINE_EXITED'로 다소 포괄적입니다.
- **개선 제안:** 분석기 요청 객체에 `timeout` 옵션을 추가하고, 엔진의 `stderr`를 실시간 파악하여 상세 에러를 UI에 노출해야 합니다.

### B. 효율성 (Efficiency)
- **현황:** Python 엔진은 `iter_access_log_records_with_diagnostics`를 통해 스트리밍 방식으로 읽지만, `build_access_log_result`에서 통계 계산을 위해 데이터를 메모리에 적재함.
- **진단:** 
    - **메모리 압박:** `response_times` 리스트와 같이 원시 데이터를 모두 들고 있는 구조는 대용량 파일에서 메모리 부족(OOM)을 유발할 수 있습니다.
    - **I/O 오버헤드:** 현재 임시 JSON 파일을 생성하고 읽는 방식은 안전하지만, 분석 결과가 수십 MB를 넘어서면 UI 응답성이 저하될 수 있습니다.
- **개선 제안:** `statistics.py`에 온라인 알고리즘(T-Digest 등)을 도입하여 메모리 사용량을 O-constant로 제한하는 리팩토링이 필요합니다 (RD-009 연계).

### C. 유지보수성 및 확장성 (Maintainability & Scalability)
- **현황:** 신규 분석기 추가 시 `main.ts`, `analyzerContract.ts`, `cli.py` 등 여러 파일을 수정해야 함.
- **진단:** 
    - **IPC Handler Bloat:** 현재 분석기마다 개별 IPC 채널(`analyzer:access-log:analyze`)을 생성하고 있습니다. 이는 분석기가 10개 이상으로 늘어날 경우 `main.ts`의 비대화를 초래합니다.
    - **데이터 모델 동기화:** Python `TypedDict`와 TypeScript `interface`가 수동으로 관리되고 있어, 필드 추가 시 누락될 가능성이 높습니다.
- **개선 제안:** 제네릭 IPC 핸들러(`analyzer:run`)를 도입하여 분석기 타입과 옵션을 파라미터로 전달받는 구조로 전환하고, JSON Schema를 통한 컨트랙트 자동 검증 도입을 권고합니다.

---

## 3. 코드 리뷰 및 엔지니어링 피드백 (Code Quality)

### A. Python Engine (`archscope_engine`)
- **Code Smell:** `access_log_analyzer.py`의 `build_access_log_result` 함수가 100라인이 넘는 비대한 로직을 가지고 있습니다.
    - **문제:** 통계 계산, 시계열 데이터 생성, Finding 도출 로직이 한 곳에 섞여 있어 테스트와 재사용이 어렵습니다.
    - **리팩토링:** `Aggregator` 클래스 또는 전용 함수군(e.g., `calculate_metrics`, `generate_time_series`, `derive_findings`)으로 분리하십시오.
- **Contract Enforcement:** `AnalysisResult`가 `dict[str, Any]`를 필드로 가져 타입 안정성이 떨어집니다. `Pydantic` 도입을 통해 엔진 내부에서도 강한 타입 검증을 수행할 것을 권장합니다 (RD-007 재검토 필요).

### B. Desktop UI (`apps/desktop`)
- **UI State Management:** `AccessLogAnalyzerPage.tsx`에서 모든 상태(filePath, format, result 등)를 `useState`로 수동 관리하고 있습니다.
    - **문제:** 분석 옵션이 복잡해지거나 여러 페이지 간에 분석 결과를 공유해야 할 경우 상태 불일치 리스크가 큽니다.
    - **리팩토링:** `Zustand` 또는 `React Context`를 사용하여 분석 세션(Analysis Session) 상태를 중앙 집중화하십시오.
- **Chart Logic Coupling:** 차트 옵션 빌더(`buildRequestChartOption`)가 페이지 컴포넌트 내부에 존재하거나 밀접하게 결합되어 있습니다.
    - **개선:** `charts/chartOptions.ts`로 로직을 이관하고, `AnalysisResult` 타입별로 표준 차트 옵션을 생성하는 Factory 패턴 도입을 제안합니다.

---

## 4. Phase 2 확장을 위한 구체적 권고 사항 (Actionable Items)

### 1) Generic IPC Bridge 도입 (P1)
```typescript
// main.ts 개선 예시
ipcMain.handle("analyzer:execute", async (_event, { type, params }) => {
  // type에 따라 적절한 args 생성 및 runAnalyzer 호출
  // 새로운 분석기 추가 시 main.ts 수정 최소화
});
```

### 2) Streaming Aggregator 패턴 (P2)
Python 엔진에서 모든 데이터를 리스트에 넣지 말고, 이터레이터를 순회하며 즉시 집계하는 구조로 전환하십시오.
```python
# 리팩토링 방향
class AccessLogAggregator:
    def add_record(self, record: AccessLogRecord):
        self.total += 1
        self.status_counter[record.status_family] += 1
        # 리스트에 넣지 않고 누적 합산 등 수행
```

### 3) Contract Validation 강화 (P2)
Zod(TypeScript) 또는 Pydantic(Python)을 사용하여 IPC 경계에서 데이터 정합성을 즉시 검증하십시오. 이는 엔진 버전 업그레이드 시 UI 하위 호환성 문제를 조기에 발견하게 해줍니다.

---

## 5. 결론

ArchScope Phase 1은 실무적인 도구로서의 기틀을 훌륭히 닦았습니다. 제안된 **Generic IPC Bridge**와 **Streaming Aggregator** 도입은 Phase 2에서 대용량 데이터를 처리하고 다양한 런타임(JVM, Node.js 등)을 신속하게 지원하는 핵심 동력이 될 것입니다.

이 리뷰의 결과 중 즉시 실행 가능한 항목들은 `work_status.md`에 반영하여 우선순위를 조정해 주시기 바랍니다.
