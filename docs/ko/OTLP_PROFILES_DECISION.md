# OTLP Profiles 결정 노트

상태: 추적은 유지하되, 아직 active ingestion release blocker로 삼지 않는다.

ArchScope는 현재 `profile_evidence`를 통해 pprof, async-profiler,
speedscope, py-spy/rbspy, StackProf, PHP/Xdebug, Swift/generic async stack,
Pyroscope/Phlare, Parca 계열 snapshot 같은 offline profile artifact를
가져온다. Collector를 실행하지 않고 고객이 전달하는 파일을 분석하는 범위는
이 경로가 우선이다.

다음 조건 중 하나가 만족되면 OTLP Profiles를 radar에서 active ingestion으로
올린다.

- OpenTelemetry Profiles signal이 cross-vendor stable file contract에 도달한다.
- incident evidence가 pprof/speedscope/Pyroscope/Parca snapshot보다 OTLP profile
  export file로 더 자주 전달된다.
- exported OTLP profile sample에 trace-to-profile correlation metadata가
  일관되게 포함된다.

그 전까지 OTLP Profiles는 profile-schema compatibility target으로 유지한다.
통합 frame/sample schema는 future OTLP importer에 필요한 runtime, language,
native, managed, async, labels, correlation field를 이미 담을 수 있다.
