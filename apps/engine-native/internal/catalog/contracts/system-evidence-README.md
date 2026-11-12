# 시스템 성능 근거 계약 표본

민감 정보 없는 합성 fixture입니다. perf-script.txt는 자체 생성한 작은 perf.data를 Linux perf 6.1.190으로 실제 변환했습니다. 호스트 workload 수집이나 권한 변경은 하지 않았습니다. generate_perf.py와 perf-capture.json이 원본 hash·fields·명령을 보존합니다. Binary는 이 변환 재현용이며 제품 입력은 text입니다.

events.jsonl은 명시적 system-evidence.v1 event schema의 샘플·scheduler·block I/O·syscall·network 경계입니다. 양/음 PMU 변환이나 원격 latency 추론을 하지 않습니다.

bpftrace-hist.jsonl은 bpftrace v0.22.0의 hist/lhist JSON shape와 비교한 합성 계약 표본입니다. 실제 BPF attach/수집을 수행했다는 근거는 아닙니다. script hash·tool version·단위·모집단·집계 구간·reset을 sidecar에서 선언해야 합니다. bucket min/max는 정수 양끝 포함이고 생략된 끝은 열린 tail입니다. 빈 bucket도 보존하며 event timeline이나 quantile을 만들지 않습니다.

정규 모델은 system-evidence-set.schema.json과 compiled 검사로 검증합니다. Source hash·line/byte 좌표·loss/clock/entity lifetime을 보존하고, 불완전 구간은 null입니다. system-evidence-set.json은 이벤트 원본을 정규화한 transport fixture입니다. 집계와 원본 event를 혼합하지 않습니다.

기준 문서: https://raw.githubusercontent.com/torvalds/linux/v6.12/tools/perf/Documentation/perf-script.txt 및 https://github.com/bpftrace/bpftrace/tree/v0.22.0/tests/runtime/outputs .

1.1.0 저장 모델은 frame/stack을 한 번만 저장하고 record는 정수 stack_id로 참조합니다. 1.0.0/결과 0.1.0 reader는 유지합니다. keyed histogram 합성 출력은 bpftrace 0.22.0 src/output.cpp의 JsonOutput::map_hist/key formatter와 대조했으며 실제 BPF attach는 하지 않았습니다. Original_sha256은 수집 binary 등 변환 전 원본의 선언 provenance이며 source_sha256(가져온 text/gzip 실제 byte hash)과 다를 수 있습니다. 두 항목을 같은 assertion으로 사용하지 않습니다.
