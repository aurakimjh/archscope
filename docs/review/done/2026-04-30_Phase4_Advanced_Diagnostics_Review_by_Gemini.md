# [Report] Phase 4 Advanced Diagnostics Implementation Review

**Date:** 2026-04-30
**Reviewer:** Gemini (Principal Data Architect)
**Subject:** Phase 4 Advanced Diagnostics (Timeline, JFR, OTel) Architectural Audit

---

## 1. Executive Summary

Phase 4의 핵심 목표인 **'High-value Diagnostic Correlation'**을 달성하기 위한 설계 명세서(`ADVANCED_DIAGNOSTICS.md`)를 심층 리뷰하였습니다. 

전반적으로 **'Evidence-first'** 원칙을 고수하며, JFR과 OpenTelemetry라는 거대하고 복잡한 데이터 소스를 `AnalysisResult`라는 단일 규격으로 수렴시킨 아키텍처는 매우 탁월합니다. 특히 JDK `jfr` 커맨드를 브리지로 활용한 설계와 Trace ID 기반의 강력한 상관관계 분석 전략은 실무적인 확장성과 안정성을 동시에 확보한 최선의 선택으로 평가됩니다.

---

## 2. 데이터 모델링 및 상관관계 (Data Modeling & Correlation)

### A. Timeline Correlation 아키텍처 (T-028)
- **진단:** 분석기(Analyzer) 레벨에서 여러 `AnalysisResult`를 입력받아 시계열로 정렬하고 `trace_id`를 통한 결정론적 조인(Deterministic Join)을 수행하는 구조임.
- **강점:** 
    - **Heuristic vs Deterministic 분리:** `trace_id`가 있을 때는 강력한 연결을, 없을 때는 타임스탬프 기반의 근접 조인을 수행하여 가시성을 극대화함.
    - **Evidence Linkage:** 모든 상관관계 이벤트가 원본 소스 파일과 라인 번호를 `evidence_ref`로 추적할 수 있어 진단의 신뢰성을 보장함.
- **개선 권고:** 분산 환경의 데이터 수집 시 타임스탬프 편차(Clock Skew)가 발생할 수 있습니다. 상관관계 분석 옵션에 `clock_drift_ms` 허용 오차 범위를 설정할 수 있는 파라미터를 추가할 것을 권장합니다.

### B. OpenTelemetry 통합 스키마 (T-035)
- **진단:** OTLP 규격을 ArchScope의 시계열 및 테이블 모델로 맵핑하는 설계가 구체적임.
- **강점:** 
    - `SeverityNumber` 정규화를 통해 서로 다른 로그 소스 간의 심각도를 통일된 척도로 비교 가능함.
    - `trace_linked_count` 메트릭을 제공하여 인스턴트 진단의 커버리지를 측정할 수 있게 함.

---

## 3. 파서 성능 및 확장성 (Parser Performance & Extensibility)

### A. JFR Recording Parser 설계 (T-034)
- **진단:** 네이티브 파서 라이브러리 대신 JDK `jfr` 커맨드의 JSON 출력을 활용한 'Command Bridge' 패턴을 채택함.
- **효율성 평가:** 
    - **안정성:** 복잡한 JFR 바이너리 구조를 직접 다루지 않고 오라클이 검증한 도구를 사용함으로써 구현 리스크를 최소화함.
    - **메모리 제약:** `--stack-depth`와 `--json` 옵션을 통한 Bounded Event Set 추출 방식은 대용량 파일 처리 시 OOM을 방지하는 핵심 전략임.
- **잠재적 병목:** `jfr print --json`은 매우 큰 JSON 문자열을 생성할 수 있습니다. 파이썬 엔진에서 이를 `json.loads()`로 한꺼번에 처리하기보다는 `ijson` 등 스트리밍 파서를 사용하여 청크 단위로 처리하는 로직이 반드시 수반되어야 합니다.

### B. 확장성 인터페이스
- **진단:** 모든 신규 분석기가 `AnalysisResult` 규격을 준수하며, `type` 필드를 통해 UI 레이어에서 플러그인 형태로 렌더링을 전환할 수 있는 구조임.
- **평가:** 새로운 데이터 소스(eBPF, 메트릭 서버 등)가 추가되더라도 엔진의 CLI 컨트랙트와 UI의 팩토리 패턴이 이미 Phase 2/3에서 준비되어 있어 수정 최소화가 가능함 (OCP 준수).

---

## 4. 아키텍처 결합도 및 성능 (Architecture Coupling)

- **Backend-Frontend 분리:** 복잡한 상관관계 분석 연산이 파이썬 엔진에서 완료된 후, UI에는 시각화에 최적화된 `series`와 `tables`만 전달되므로 고해상도 차트 렌더링 시에도 렌더러의 성능 저하를 방지함.
- **Trace Context 전파:** `trace_id`를 `AnalysisResult`의 메타데이터가 아닌 개별 레코드 수준에서 관리함으로써, 테이블 뷰에서 즉시 필터링 및 드릴다운이 가능한 구조를 달성함.

---

## 5. Actionable Feedback & Next Steps

1.  **JFR 스트리밍 파싱 구현 (High):**
    `jfr` 커맨드 아웃풋이 수백 MB가 될 경우를 대비하여, `archscope_engine.parsers.jfr_parser` 구현 시 `ijson` 라이브러리를 사용한 스트리밍 처리를 기본값으로 설정하십시오.
2.  **시각화 전용 상관관계 뷰 설계 (Medium):**
    `timeline_correlation` 타입의 결과를 렌더링할 때, 여러 소스의 이벤트를 멀티 레인(Multi-lane)으로 보여줄 수 있는 ECharts 커스텀 차트 레이아웃 설계를 시작하십시오.
3.  **Schema Versioning 엄격화 (Medium):**
    Phase 4부터 결과 타입이 급격히 늘어나므로, `metadata.schema_version`이 일치하지 않을 경우 UI에서 경고를 띄우는 하위 호환성 체크 로직을 강화하십시오.

---

## 6. 결론

Phase 4 고급 진단 아키텍처는 ArchScope를 단순한 로그 뷰어에서 **'지능형 상관관계 진단 플랫폼'**으로 격상시키는 견고한 설계입니다. 제안된 JFR 스트리밍 처리와 타임스탬프 보정 로직만 보완된다면, 엔터프라이즈급 대규모 진단 시나리오에서도 충분히 강력한 도구가 될 것입니다.
