# 엔진 ↔ UI 브릿지

이 문서는 원래 Electron-IPC 브릿지 실험을 기록했습니다. Phase 1
(T-206 … T-209)에서 그 브릿지를 FastAPI 엔진과 React UI 사이의 HTTP
경계로 교체했습니다. Electron 시대 노트는 historical context로
하단에 보존됩니다.

## 현재 브릿지 모델

```text
브라우저 (React)
   │  window.archscope.* — 레거시 IPC와 동일 표면
   │      (selectFile / analyzer / exporter / demo / settings)
   │  src/api/httpBridge.ts가 각 호출을 fetch('/api/...')에 매핑
   ▼
FastAPI 서버 (`archscope-engine serve`)
   • POST /api/upload                  multipart 업로드
                                       (~/.archscope/uploads/에 저장)
   • POST /api/analyzer/execute        단일 dispatcher (type별)
   • POST /api/analyzer/cancel         단일 프로세스 — no-op
   • POST /api/export/execute          html / pptx / diff
   • GET  /api/demo/list / POST /api/demo/run
   • GET  /api/files?path=…            로컬 artifact 스트리밍
   • GET/PUT /api/settings             ~/.archscope/settings.json
   • GET  /                            정적 React 빌드 (--static-dir)
   ▼
archscope_engine 패키지
   • 분석기를 in-process 호출 — 서브프로세스/Typer 라운드트립 없음
   • AnalysisResult JSON envelope을 그대로 반환
```

### 왜 HTTP인가

- 브라우저는 Electron IPC를 말할 수 없으므로 웹 전환 시 어차피
  언어 중립적인 경계가 필요했음.
- 로컬 UI와 향후 LAN 배포(`--host 0.0.0.0`) 모두에 단일 전송 계층.
- FastAPI 프로세스가 분석기 모듈을 직접 소유하므로 모든 호출이
  in-process로 머무르고, 이전 서브프로세스 분기를 회피.

### 파일 선택 contract

UI는 Electron 렌더러가 사용하던 그대로 `window.archscope.selectFile(...)`
을 노출합니다. 내부 동작:

1. 브릿지가 hidden `<input type="file">`을 생성하고, 사용자가 선택한
   `File`로 resolve(취소 시 `{ canceled: true }`).
2. 파일을 `multipart/form-data`로 `/api/upload`에 POST.
3. 서버가 `~/.archscope/uploads/<uuid>/<orig>`에 저장 후
   `{ filePath, originalName, size }` 반환.
4. UI는 서버 측 `filePath`를 필요한 분석기 요청
   (예: `/api/analyzer/execute`의 `params.filePath`)에 전달.

분석기 페이지가 `selectFile`을 우회하고 직접 `FileDock` 컴포넌트를
인스턴스화할 수도 있습니다. 두 경로 모두 같은 `/api/upload`로
귀결됩니다.

### 취소

`/api/analyzer/cancel`은 존재하지만 현재 단일 프로세스 엔진에서는
no-op — 모든 분석기는 FastAPI 요청 핸들러 안에서 동기 실행됩니다.
`archscope-engine serve --reload` 개발 루프는 uvicorn auto-reload에
의존하므로 코드 변경을 즉시 반영합니다. 긴 분석 실행은 다음 요청이
반환되기 전에 자연스럽게 완료됩니다.

### 오류

Dispatcher는 구조화된 오류(`code` + `message`, 선택 `detail`)를
반환합니다. UI가 사용하는 알려진 코드:

- `INVALID_OPTION` — 요청 body가 malformed.
- `FILE_NOT_FOUND` — `filePath`가 더 이상 존재하지 않음
  (`~/.archscope/uploads/` 정리 가능).
- `UNKNOWN_THREAD_DUMP_FORMAT` — thread dump의 첫 바이트가 어떤
  플러그인과도 매칭되지 않음.
- `MIXED_THREAD_DUMP_FORMATS` — 멀티 덤프 요청이 둘 이상의 포맷으로
  resolve. `format`으로 강제.
- `ENGINE_FAILED` — generic catch-all(분석기가 throw). 예외는
  `detail`에 보존.
- `ENGINE_OUTPUT_INVALID` (레거시 CLI 경로) — 이전 서브프로세스
  JSON contract와의 호환성 유지.

## Historical: Electron IPC 브릿지 (2026-Q1)

원래 PoC는 React 렌더러가 Electron main process로 IPC 호출을 보내고,
main process가 `archscope-engine`을 서브프로세스로 `execFile`해서
JSON 출력을 파싱하는 방식이었습니다. Phase 1에서 HTTP로 이동한 세
가지 이유:

1. Electron sandbox가 렌더러의 직접 파일 경로 접근을 차단해 모든
   파일 선택마다 IPC 핸드셰이크 필요.
2. 서브프로세스 분기가 엔진/IPC 채널/렌더러 로거 셋에 걸쳐 파서
   실패 컨텍스트를 중복.
3. 번들 크기 — Electron + PyInstaller 인스톨러는 사용자 데이터
   없이도 수백 MB. FastAPI + Vite 번들은 pip install 한 번 + Vite
   build 한 번.

폐기된 Electron main / preload 코드는 `apps/desktop/electron/`에
살았으며 Phase 1에서 삭제되었습니다.
