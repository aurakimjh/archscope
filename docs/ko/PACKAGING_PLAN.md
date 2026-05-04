# 패키징 계획

ArchScope는 데스크톱 바이너리가 아니라 **로컬 웹 애플리케이션**으로
배포됩니다. 현재 모델에는 Electron 셸도, PyInstaller sidecar도
없습니다. Electron 시대의 원래 계획은 historical note로 문서 하단에
보존됩니다.

## 현재 배포 모델

엔드 유저는 엔진을 설치하고 단일 Python 프로세스로 React UI를
서빙합니다.

```bash
cd engines/python
python -m venv .venv && source .venv/bin/activate
pip install -e .                       # `archscope-engine` 등록

cd ../..
./scripts/serve-web.sh                 # apps/desktop/dist 빌드 + 서빙
# 브라우저에서 http://127.0.0.1:8765
```

구성:

- **Python 엔진** — `archscope-engine` 배포(엔트리는
  `engines/python/setup.cfg`). Typer + FastAPI + uvicorn +
  defusedxml + python-multipart를 설치하고 `archscope-engine` 콘솔
  스크립트를 노출. PyPI 휠 발행 + `pip install archscope-engine`은
  다음 단계(아래 Open items 참고).
- **React UI** — Vite가 `apps/desktop/dist`로 정적 번들 생성.
  `archscope-engine serve --static-dir apps/desktop/dist`(또는
  헬퍼 스크립트)가 `/`에서 번들 서빙.
- **플랫폼 바이너리 없음** — `.dmg` / `.msi` / `.deb` 없음. 사용자는
  Python ≥ 3.9이 있는 환경에서 엔진을 실행. 과거 세 가지 우려사항
  (Electron 버전 업그레이드, electron-builder 서명, PyInstaller
  sidecar 경로)을 단일 공급망으로 통합.

사용자 데이터는 `~/.archscope/`(uploads, settings)에 저장되며 로컬
머신을 떠나지 않음. 엔진은 기본적으로 `127.0.0.1`에 바인딩.

## CSP 정책

이제 잠가야 할 Electron 렌더러가 없습니다. 브라우저는 FastAPI에서
React 번들을 로드하고 같은 origin의 `fetch('/api/...')`로 다시 통신.
프로덕션 빌드에서는 `unsafe-eval` 불필요. 인라인 스타일은 ECharts
tooltip 테마에서만 발생.

## Open items

다음 패키징 단계 — 아직 미구현:

1. **PyPI에 버전된 wheel 발행** — 사용자가 저장소 클론 없이
   `pip install archscope-engine`으로 실행 가능하게.
2. **wheel에 React `dist/` 번들 포함** — Node.js 툴체인 없이 설치
   가능하게. wheel이 정적 파일을 함께 배포하고, `--static-dir`
   생략 시 엔진이 패키지 디렉토리에서 자동 해석.
3. **선택적 standalone 런타임** — `uv tool install
   archscope-engine` 레시피로 virtualenv 관리 없이 단일 명령으로
   CLI + 웹 서버 획득.
4. **Docker 이미지** — 신뢰할 수 있는 사내 호스트에서 팀 전체가
   `archscope-engine serve --host 0.0.0.0`로 사용.

## Historical: Electron + PyInstaller spike (2026-Q1)

원래 패키징 계획은 Electron 위에 PyInstaller sidecar를 쌓는 형태
였습니다. **Phase 1 (Web pivot, T-206 … T-209)**에서 세 가지 이유로
폐기됨: Electron 번들 인스톨러가 운영 사용자에게 너무 컸고,
PyInstaller sidecar가 디버깅 표면을 중복으로 만들었으며, Electron
IPC contract가 FastAPI HTTP 경계로 깔끔하게 처리되는 오버헤드를
추가했음. 위의 현재 형태가 그 계획을 전면 교체. 폐기된 구현은
`apps/desktop/electron/`(Phase 1에서 삭제)에 살았으며 spike
artifact는 더 이상 빌드 파이프라인에서 도달 불가.
