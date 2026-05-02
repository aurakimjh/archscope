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

## 커스텀 Rule 작성 가이드

### 예시: 사내 프레임워크 분류 추가

사내에서 `com.mycompany.framework`라는 패키지를 사용하고, 이를 별도 분류로 표시하려면:

```json
[
  {
    "label": "MyCompany Framework",
    "contains": ["com.mycompany.framework"]
  },
  {
    "label": "MyCompany Data Layer",
    "contains": ["com.mycompany.data", "com.mycompany.repository"]
  }
]
```

### Rule 우선순위 가이드

Rule은 배열의 앞에서부터 순서대로 매칭된다. 아래 예시에서 `oracle.jdbc`가 `oracle` 보다 먼저 있어야 JDBC 호출이 정확히 분류된다.

```json
[
  {"label": "Oracle JDBC", "contains": ["oracle.jdbc"]},
  {"label": "Oracle Misc", "contains": ["oracle."]}
]
```

잘못된 순서 (넓은 rule이 먼저):
```json
[
  {"label": "Oracle Misc", "contains": ["oracle."]},
  {"label": "Oracle JDBC", "contains": ["oracle.jdbc"]}
]
```
이 경우 `oracle.jdbc` 스택도 "Oracle Misc"로 분류되어 의도와 다른 결과가 나온다.

### Rule 테스트 방법

CLI의 `profiler breakdown` 명령으로 분류 결과를 빠르게 확인할 수 있다:

```bash
archscope-engine profiler breakdown \
  --wall sample.collapsed \
  --out /tmp/breakdown.json

# breakdown.json의 series.execution_breakdown에서 category별 분류 확인
python3 -c "
import json
with open('/tmp/breakdown.json') as f:
    result = json.load(f)
for row in result['series']['execution_breakdown']:
    print(f\"{row['category']:30s} {row['samples']:>8d} samples ({row['total_ratio']*100:.1f}%)\")
"
```

### 흔한 분류 문제

| 증상 | 원인 | 해결 |
|------|------|------|
| 대부분 "Application"으로 분류 | rule에 해당 framework 없음 | 누락된 package prefix rule 추가 |
| JDBC가 "JVM"으로 분류 | JDBC rule이 JVM rule 뒤에 위치 | JDBC rule을 앞으로 이동 |
| Spring 내부가 "HTTP"로 분류 | `http` 같은 넓은 token 사용 | `springframework.web` 등 구체적 token으로 변경 |

## 확장 방향

기본 configuration 파일은 engine package와 함께 PyInstaller sidecar data로 포함해야 한다. Packaged resource path와 update policy가 안정화된 뒤 사용자 편집 classification은 packaged default 위에 외부 JSON 파일을 덧씌우는 방식으로 확장한다.
