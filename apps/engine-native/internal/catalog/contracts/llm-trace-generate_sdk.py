"""격리 SDK 1.38.0으로 합성 trace를 생성한다. 네트워크 exporter는 사용하지 않는다."""
from pathlib import Path
import json
import base64
from importlib.metadata import version
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.id_generator import IdGenerator
from opentelemetry.sdk.trace.export import SimpleSpanProcessor, SpanExporter, SpanExportResult
from opentelemetry.sdk.resources import Resource
from opentelemetry.trace import SpanKind, Status, StatusCode, Link, SpanContext, TraceFlags
from opentelemetry.exporter.otlp.proto.common.trace_encoder import encode_spans
from google.protobuf.json_format import MessageToDict

assert version("opentelemetry-sdk") == "1.38.0"
class IDs(IdGenerator):
    def __init__(self): self.trace, self.span = 100, 0
    def generate_trace_id(self): self.trace += 1; return self.trace
    def generate_span_id(self): self.span += 1; return self.span
class Collect(SpanExporter):
    def __init__(self): self.spans = []
    def export(self, spans): self.spans.extend(spans); return SpanExportResult.SUCCESS

collector = Collect()
provider = TracerProvider(id_generator=IDs(), resource=Resource({
    "service.name": "synthetic-llm-app", "service.instance.id": "synthetic-instance-a",
    "host.id": "synthetic-host", "private.resource": "SYNTHETIC_BODY_MARKER"
}))
provider.add_span_processor(SimpleSpanProcessor(collector))
tracer = provider.get_tracer("synthetic-llm-sdk-fixture", "1.0.0", schema_url="https://opentelemetry.io/schemas/1.37.0")
def attrs(role, **extra):
    return {"diag.llm.role": role, "diag.llm.population": "warm",
            "diag.gateway.request_namespace": "synthetic-gateway-a",
            "diag.gateway.logical_request_id": "synthetic-logical-a", **extra}

with tracer.start_as_current_span("SYNTHETIC_BODY_MARKER raw span name", kind=SpanKind.SERVER,
    start_time=1_000_000_000, end_on_exit=False, attributes=attrs("agent")) as root:
    root.add_event("diag.lifecycle.warmup_end", timestamp=1_000_000_000)
    for role, start, end in (("retriever",1_050_000_000,1_200_000_000),("reranker",1_150_000_000,1_250_000_000),("embedding",1_250_000_000,1_300_000_000)):
        child=tracer.start_span("SYNTHETIC_BODY_MARKER",start_time=start,attributes=attrs(role))
        child.set_attribute("input.value","SYNTHETIC_BODY_MARKER")
        child.end(end_time=end)
    external=SpanContext(trace_id=999,span_id=999,is_remote=True,trace_flags=TraceFlags(1))
    client=tracer.start_span("SYNTHETIC_BODY_MARKER",kind=SpanKind.CLIENT,start_time=1_300_000_000,
        attributes=attrs("llm",**{"diag.llm.model":"synthetic-model-a","diag.llm.provider":"synthetic-provider",
          "diag.llm.input_tokens":9007199254740993,"diag.llm.output_tokens":3,
          "diag.llm.token_counting_method":"source_declared","diag.gateway.attempt_id":"attempt-a",
          "diag.gateway.attempt_kind":"initial","diag.gateway.token_authority":"client",
          "gen_ai.input.messages":"SYNTHETIC_BODY_MARKER","tool.arguments":"SYNTHETIC_BODY_MARKER"}),
        links=[Link(external,{"private.link":"SYNTHETIC_BODY_MARKER"})])
    client.add_event("diag.llm.first_token",timestamp=1_500_000_000)
    client.add_event("diag.llm.last_token",timestamp=1_900_000_000)
    client.add_event("gen_ai.content.completion",{"content":"SYNTHETIC_BODY_MARKER"},timestamp=1_900_000_000)
    client.set_status(Status(StatusCode.OK))
    client.end(end_time=2_000_000_000)
    tool=tracer.start_span("SYNTHETIC_BODY_MARKER",start_time=2_000_000_000,attributes=attrs("tool"))
    tool.set_status(Status(StatusCode.ERROR,"SYNTHETIC_BODY_MARKER"))
    tool.end(end_time=2_050_000_000)
    root.end(end_time=2_100_000_000)

body=MessageToDict(encode_spans(collector.spans),preserving_proto_field_name=False)
# OTLP/JSON trace/span identifiers use hexadecimal, unlike generic protobuf JSON bytes.
def hex_ids(value):
    if isinstance(value, dict):
        for key, item in value.items():
            if key in ("traceId", "spanId", "parentSpanId") and isinstance(item,str):
                value[key] = base64.b64decode(item).hex()
            else: hex_ids(item)
    elif isinstance(value,list):
        for item in value: hex_ids(item)
hex_ids(body)
Path(__file__).with_name("sdk-python-1.38.0.otlp.json").write_text(json.dumps(body,indent=2)+"\n")
provider.shutdown()
