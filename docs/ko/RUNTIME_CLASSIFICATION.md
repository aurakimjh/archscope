# 런타임 분류

Profiler stack classification은 rule 수준에서 configuration-driven 구조로 둔다. Analyzer는 collapsed stack text를 component family로 매핑한 뒤 `series.component_breakdown`을 만든다.

Rule은 패키지에 포함된 JSON 리소스에서 로드할 수 있다.

```text
archscope_engine/config/runtime_classification_rules.json
```

Python API는 packaging spike와 향후 사용자 관리 rule set을 위해 `load_stack_classification_rules(path)`로 외부 JSON 파일도 로드할 수 있게 한다.

## 기본 Family

현재 built-in family는 다음과 같다.

- Oracle JDBC
- Spring Batch
- Spring Framework
- Node.js
- Python
- Go
- ASP.NET / .NET
- HTTP / Network
- JVM
- Application fallback

Rule은 순서가 있다. 먼저 매칭된 rule이 적용되므로, 구체적인 framework rule은 넓은 runtime rule보다 앞에 있어야 한다.

## Rule 형식

```json
[
  {
    "label": "Spring Batch",
    "contains": ["springframework.batch"]
  }
]
```

Rule 작성 제약:

- `label`은 비어 있지 않은 component family 이름이어야 한다.
- `contains`는 비어 있지 않은 lowercase substring token 목록이어야 한다.
- `http` 같은 넓은 단어보다 `java.net.http` 같은 package-qualified token을 우선한다.
- 좁은 framework/library rule을 넓은 runtime rule보다 앞에 둔다.
- UI가 정규식을 필요로 하기 전까지 substring-only rule로 제한한다.

## 확장 방향

기본 configuration 파일은 engine package와 함께 PyInstaller sidecar data로 포함해야 한다. Packaged resource path와 update policy가 안정화된 뒤 사용자 편집 classification은 packaged default 위에 외부 JSON 파일을 덧씌우는 방식으로 확장한다.
