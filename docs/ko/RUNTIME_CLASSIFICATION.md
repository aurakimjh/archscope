# 런타임 분류

Profiler stack classification은 rule 수준에서 configuration-driven 구조로 둔다. Analyzer는 collapsed stack text를 component family로 매핑한 뒤 `series.component_breakdown`을 만든다.

## 기본 Family

현재 built-in family는 다음과 같다.

- Oracle JDBC
- Spring Batch
- Spring Framework
- HTTP / Network
- JVM
- Node.js
- Python
- Go
- ASP.NET / .NET
- Application fallback

Rule은 순서가 있다. 먼저 매칭된 rule이 적용되므로, 구체적인 framework rule은 넓은 runtime rule보다 앞에 있어야 한다.

## 확장 방향

향후 runtime 확장에서는 PyInstaller sidecar path가 안정화된 뒤 packaged configuration에서 추가 rule을 로드한다. 사용자 편집 classification은 UI가 정규식을 필요로 하기 전까지 단순 `contains` token 기반으로 제한한다.
