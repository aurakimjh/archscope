"""격리 venv에서 SDK 1.38.0으로 합성 OTLP JSON을 생성한다. 시각은 실행 때 달라진다."""
import json
from pathlib import Path
from google.protobuf.json_format import MessageToDict
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import InMemoryMetricReader
from opentelemetry.sdk.metrics.view import View, ExplicitBucketHistogramAggregation, ExponentialBucketHistogramAggregation
from opentelemetry.exporter.otlp.proto.common.metrics_encoder import encode_metrics

reader = InMemoryMetricReader()
provider = MeterProvider(metric_readers=[reader], resource=Resource({'service.name': 'synthetic-checkout', 'deployment.environment.name': 'fixture'}), views=[View(instrument_name='request.duration', aggregation=ExplicitBucketHistogramAggregation(boundaries=[1, 5, 10])), View(instrument_name='payload.size', aggregation=ExponentialBucketHistogramAggregation(max_size=32, max_scale=4))])
meter = provider.get_meter('synthetic.instrumentation', '1.0.0', schema_url='https://opentelemetry.io/schemas/1.37.0')
counter = meter.create_counter('request.count', unit='{request}')
active = meter.create_up_down_counter('request.active', unit='{request}')
hist = meter.create_histogram('request.duration', unit='ms')
exp = meter.create_histogram('payload.size', unit='By')
attrs = {'service.role': 'server', 'http.request.method': 'GET'}
exports = []
for n in [1, 2]:
    counter.add(9007199254740993 if n == 1 else 7, attrs)
    active.add(3 if n == 1 else -1, attrs)
    for value in [0, 2, 8]:
        hist.record(value, attrs)
        exp.record(value, attrs)
    message = encode_metrics(reader.get_metrics_data())
    exports.extend(MessageToDict(message, preserving_proto_field_name=False, use_integers_for_enums=True)['resourceMetrics'])
Path(__file__).with_name('sdk-python-1.38.0.otlp.json').write_text(json.dumps({'resourceMetrics': exports}, indent=2)+'\n')
provider.shutdown()
