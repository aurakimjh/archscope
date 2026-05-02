# 패키징 계획

ArchScope 패키징은 desktop shell과 Python engine을 독립적으로 검증 가능하게 유지하면서, 사용자에게는 하나의 desktop application으로 제공하는 것을 목표로 한다.

## Packaging Spike 범위

Phase 3 spike의 목표 구조는 다음과 같다.

- Electron이 renderer와 main process를 패키징한다.
- Python engine은 PyInstaller sidecar executable로 빌드한다.
- Electron은 개발 환경과 같은 CLI contract로 sidecar를 호출한다. 즉 analyzer command argument와 `--out` 경로를 사용한다.
- 패키징 후에도 UI와 engine 사이의 경계는 `AnalysisResult` JSON으로 유지한다.

## Spike 체크리스트

1. macOS에서 PyInstaller로 Python engine sidecar를 먼저 빌드한다.
2. packaged app resource path에 sidecar 위치를 연결한다.
3. packaged sidecar 경로로 `access-log analyze`와 `profiler analyze-collapsed`를 검증한다.
4. malformed-line diagnostics와 engine stderr detail이 UI까지 전달되는지 검증한다.
5. macOS path와 signing 가정이 정리된 뒤 Windows에서 반복 검증한다.

Linux packaging은 macOS와 Windows sidecar path가 검증될 때까지 defer한다.

## PyInstaller Sidecar 빌드 방법

### macOS

```bash
cd engines/python
pip install pyinstaller

pyinstaller \
  --name archscope-engine \
  --onedir \
  --noconfirm \
  --add-data "archscope_engine/config:archscope_engine/config" \
  --hidden-import archscope_engine.parsers \
  --hidden-import archscope_engine.analyzers \
  --hidden-import archscope_engine.exporters \
  archscope_engine/cli.py

# 결과: dist/archscope-engine/archscope-engine (실행 파일)
```

### Windows

```powershell
cd engines\python
pip install pyinstaller

pyinstaller `
  --name archscope-engine `
  --onedir `
  --noconfirm `
  --add-data "archscope_engine\config;archscope_engine\config" `
  --hidden-import archscope_engine.parsers `
  --hidden-import archscope_engine.analyzers `
  --hidden-import archscope_engine.exporters `
  archscope_engine\cli.py

# 결과: dist\archscope-engine\archscope-engine.exe
```

### 빌드 결과 검증

```bash
# sidecar가 정상 동작하는지 확인
./dist/archscope-engine/archscope-engine --help
./dist/archscope-engine/archscope-engine access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out /tmp/test-result.json
```

## macOS 코드 서명 및 공증 (Notarization)

배포용 빌드에서는 macOS Gatekeeper를 통과하기 위해 코드 서명과 공증이 필요하다.

```bash
# 1. Electron app 서명 (electron-builder가 처리)
# package.json의 build.mac 설정에 identity 지정

# 2. PyInstaller sidecar 서명
codesign --deep --force --options runtime \
  --sign "Developer ID Application: Your Name (TEAMID)" \
  dist/archscope-engine/archscope-engine

# 3. 공증 (notarization)
# electron-builder의 afterSign hook에서 app 전체를 notarize
```

Windows에서는 Authenticode 코드 서명 인증서를 사용한다. 구체적인 CI 파이프라인 구성은 signing infrastructure가 확보된 뒤 결정한다.

## Metadata 결정

낮은 `setuptools<64` 상한은 현대적인 bounded range로 올린다. 전체 metadata를 `pyproject.toml`로 통합하는 작업은 PyInstaller, editable development install, 향후 wheel publishing 요구가 packaging spike에서 확인된 뒤 진행한다.

현재 결정:

- `setup.cfg`는 package metadata, dependency, extra, entry point의 source로 유지한다.
- `setup.py`는 최소 compatibility shim으로 유지한다.
- `pyproject.toml`은 build-system requirement와 tool configuration을 담당한다.

## CSP Nonce 평가

Production CSP는 이미 unsafe script execution을 차단한다. `style-src 'unsafe-inline'` 제거에는 style injection nonce 전파와 React, Vite output, ECharts tooltip/theme 호환성 검증이 필요하다.

결정: Phase 3 packaging 중에는 현재 style policy를 유지한다. Packaged renderer 동작과 chart export flow가 안정화된 뒤 nonce 기반 style CSP를 재검토한다.

Phase 3 follow-up spike 결과: 로컬 ECharts 6 및 zrender package source에서는 문서화된 `nonce` 또는 `csp.nonce` 초기화 옵션을 확인하지 못했다. 다음 CSP hardening 시도는 policy 변경 전에 packaged renderer smoke test를 수행하고, chart tooltip/theme rendering이 유지되는 경우에만 unsafe inline style 제거를 우선한다.
