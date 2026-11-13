# 기존 결과의 백분위 계산 정책

새로 계산한 기존 계열 결과는 `metadata.percentile_method`에 버전 정책 ID·계산법·
모집단을 공개합니다(metadata extras에서 직렬화). 이 추가 메타데이터는 payload나
계산한 숫자를 바꾸지 않습니다. 필드가 없는 저장 결과는 원래 바이트와 미공개 정책을
유지합니다. 명시된 정책을 얻으려면 원본을 재분석하며 읽을 때 정책을 소급하지 않습니다.

| 계열 | 계산 | 모집단과 한계 |
|---|---|---|
| Access log | Reservoir 표본 후 선형 보간 | 집계별 알려진 응답시간; 용량 10,000·seed 12,345·Go PCG; 용량 초과는 추정 |
| GC log | 선형 보간 | 파싱한 pause; 소수 3자리 half-even 반올림 |
| HTTP capture | 선형 보간 | 보존한 유효 entry 시간; diff는 원본 요약값 사용; 과거 원본 정책은 미공개 |
| Trace import | 선형 보간 | 관측한 trace별 경과 시간 |
| Database slow query | Floor index | 입력 필터 아래 파싱한 query 시간 |
| Jennifer profile | Nearest rank | GUID group·signature 응답시간 |

선형 보간의 rank는 `(n-1)*p/100`, floor index는 `floor((n-1)*p/100)` 위치의
값이며 nearest rank는 `ceil(n*p/100)-1`을 표본 범위로 제한한 위치의 값입니다.
`[1,2,3,100]`의 p95는 각각 **85.45**, **3**, **100**입니다. 기존 계열의 의도된
차이입니다. Reservoir 보간은 보존한 표본에 대한 계산이며 용량 초과 시 전체 입력의
정확한 백분위는 아닙니다. PRNG 순서도 Python 구현과 다릅니다.

다른 모집단·분할 파일·시간 창의 백분위를 평균하거나 더하지 마세요. 통계 이름이
같아도 측정 의미가 같다는 근거가 아닙니다. 정규 실행 근거는 측정별 별도 통계 정책을
유지하며 MetricHistory histogram은 백분위를 추정하지 않고 bucket을 보존합니다.
