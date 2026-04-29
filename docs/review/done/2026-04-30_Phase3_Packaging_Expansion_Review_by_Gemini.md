# [Report] Phase 3 Packaging and Runtime Expansion Implementation Review

**Date:** 2026-04-30
**Reviewer:** Gemini (Principal IT Software Architecture Diagnostic Specialist)
**Subject:** Phase 3 Packaging, Extensibility, and Security Implementations

---

## 1. Executive Summary

Phase 3의 기반 기능 안정화 이후 진행된 패키징 전략(Packaging Spike) 및 런타임 환경 확장에 대한 심층 아키텍처 리뷰를 진행하였습니다. 
전반적으로 OCP(개방 폐쇄 원칙)를 준수하는 스택 분류 설계와 저수지 샘플링의 재현성 확보는 훌륭하게 구현되었습니다. 반면, Python 빌드 메타데이터 통합 및 Nonce 기반 CSP 적용과 관련해서는 기술적 리스크를 고려하여 현실적이고 합리적인 '지연(Defer)' 및 '혼합(Hybrid)' 전략을 취한 점이 돋보입니다. 

---

## 2. 배포 안정성 및 결합도 (Deployment Stability & Integration)

### A. Electron + PyInstaller 패키징 전략 (T-022)
- **리뷰:** `docs/en/PACKAGING_PLAN.md`에 명시된 대로, Python 엔진을 PyInstaller 사이드카로 빌드하고 Electron의 `child_process.execFile`로 통신하는 방식은 로컬 진단 도구로서 가장 안전하고 독립적인 배포 모델입니다.
- **안정성 평가:** Electron 메인 프로세스가 Python 사이드카를 호출할 때, 각 분석 요청이 독립적인 프로세스로 실행되므로 엔진 크래시가 전체 애플리케이션 다운으로 이어지지 않는 강한 장애 격리(Fault Isolation)를 보장합니다.

### B. 빌드 메타데이터 통합 (setup.py vs pyproject.toml) (T-024, T-025)
- **리뷰:** `setuptools<64` 상한선을 `setuptools>=68,<81`로 현대화하면서도, 모든 메타데이터를 `pyproject.toml`로 완전 통합하는 대신 `setup.cfg`를 메타데이터 원천으로 유지하는 하이브리드 방식을 선택했습니다.
- **유지보수성 평가:** 이는 `pip install -e .` 형태의 Editable Install 및 PyInstaller와의 의존성 해석 호환성을 보장하기 위한 매우 영리한(Pragmatic) 엔지니어링 결정입니다. 배포 파이프라인의 복잡도를 낮추고 안정성을 우선시한 접근을 높게 평가합니다.

---

## 3. 확장성 및 유지보수성 (Architecture & Extensibility)

### A. 프로파일러 스택 분류 룰 외부화 (T-026, T-027)
- **리뷰:** `profiler_analyzer.py`에 하드코딩되어 있던 분류 로직을 `profile_classification.py` 모듈과 `StackClassificationRule` 데이터 클래스로 분리했습니다.
- **확장성 평가 (OCP 준수):** 
  분석기 함수(`analyze_collapsed_profile`)가 `classification_rules`를 의존성 주입(Dependency Injection) 형태로 받도록 변경하여 완벽한 OCP를 달성했습니다. 코드 수정 없이 규칙 튜플만 변경하여 JVM, Node.js, Python, Go, .NET 등 다중 런타임을 지원하게 되었습니다.
- **개선 권고 (Actionable Feedback):** 
  현재는 코드 레벨의 튜플로 정의되어 있으나, 궁극적인 "Configuration-driven" 목표를 달성하기 위해서는 PyInstaller 사이드카 패키징이 완료된 이후 JSON이나 YAML 형태의 외부 설정 파일에서 규칙을 동적으로 로드하는 메커니즘을 추가해야 합니다.

---

## 4. 보안 및 알고리즘 정확성 (Security & Correctness)

### A. Nonce 기반 CSP 적용 평가 (T-052)
- **리뷰:** `style-src 'unsafe-inline'` 제거를 위한 Nonce 기반 CSP 적용이 `PACKAGING_PLAN.md`에 의해 전략적으로 보류(Deferred)되었습니다.
- **보안 평가:** React/Vite 빌드 아웃풋과 ECharts의 인라인 스타일 주입(툴팁, 테마 렌더링) 메커니즘 특성상, 현 단계에서 무리하게 Nonce를 적용하면 렌더링 호환성 문제가 발생할 위험이 큽니다. `script-src`가 이미 안전하게 통제되고 있으므로, UI 안정화 이후로 우선순위를 조정한 것은 타당한 기술적 결정입니다.

### B. Bounded Percentile Sampling 재현성 (T-053)
- **리뷰:** `statistics.py`의 저수지 샘플링(Reservoir Sampling) 로직에 `seed` 파라문을 추가하여 결정론적 인덱싱(`_deterministic_reservoir_index`)을 구현했습니다.
- **정확성 평가:** `((count * 1_103_515_245) + seed) % count`라는 선형 합동 생성기(LCG) 기반의 접근은 메모리 제약을 지키면서도 동일 데이터에 대해 매번 동일한 백분위수(Percentile)를 반환함을 테스트(`test_statistics.py`)로 완벽히 입증했습니다. 대규모 로그 분석에서 분석 결과의 신뢰성(Reliability)을 담보하는 훌륭한 알고리즘 개선입니다.

---

## 5. Actionable Feedback & Next Steps

1. **외부 설정 로더(Config Loader) 구현 (Phase 4 연계):**
   `profile_classification.py`의 `DEFAULT_STACK_CLASSIFICATION_RULES`를 외부 `runtimes.json` 파일로 빼고, PyInstaller 빌드 시 해당 JSON 파일을 데이터 리소스로 포함(`--add-data`)시키는 파이프라인을 구축하십시오.
2. **사이드카 생명주기(Lifecycle) 및 고아 프로세스(Orphan Process) 관리:**
   `execFile`로 실행된 Python 프로세스가 Electron 앱의 비정상 종료 시 시스템에 좀비 프로세스로 남지 않도록, Node.js 단에서 `child.once('exit')` 및 `process.on('exit')` 이벤트를 통한 `kill()` 루틴을 명시적으로 추가하는 것을 권장합니다.
3. **ECharts Nonce 호환성 사전 연구:**
   Phase 4 이후 CSP 강화를 대비하여, ECharts 인스턴스 초기화 시 옵션으로 `csp: { nonce: '...' }`를 넘겨주는 기능이 ECharts 6에서 정상 작동하는지 소규모 Spike 테스트를 진행하십시오.