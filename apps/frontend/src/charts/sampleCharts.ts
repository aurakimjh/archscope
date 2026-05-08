// ─────────────────────────────────────────────────────────────────────
// [한글] sampleCharts.ts — 엔진 미연결 상태에서 대시보드/차트 스튜디오가
//   사용하는 정적인 샘플 분석 결과(dashboard_sample 타입).
//
// 책임/목적:
//   - 첫 진입 사용자가 "내 데이터가 없어도 차트 모양과 동작을 볼 수 있게"
//     하기 위한 데모 데이터.
//   - 키 구조는 실제 분석 결과(AnalysisResult) 와 호환되도록 의도적으로
//     동일한 컬럼 이름(snake_case) 을 사용합니다.
//
// 사용처:
//   - mockAnalyzerClient.loadDashboardSample, ChartStudioPage 의 기본 데이터.
// ─────────────────────────────────────────────────────────────────────
export const sampleAnalysisResult = {
  type: "dashboard_sample",
  summary: {
    total_requests: 6,
    avg_response_ms: 648.67,
    p95_response_ms: 2300,
    error_rate: 33.33,
    total_wall_samples: 32629,
  },
  series: {
    requests_per_minute: [
      { time: "10:00", value: 3 },
      { time: "10:01", value: 2 },
      { time: "10:02", value: 1 },
      { time: "10:03", value: 4 },
      { time: "10:04", value: 7 },
      { time: "10:05", value: 5 },
    ],
    p95_response_time_per_minute: [
      { time: "10:00", value: 2300 },
      { time: "10:01", value: 340 },
      { time: "10:02", value: 830 },
      { time: "10:03", value: 640 },
      { time: "10:04", value: 510 },
      { time: "10:05", value: 910 },
    ],
    status_code_distribution: [
      { status: "2xx", count: 4 },
      { status: "4xx", count: 1 },
      { status: "5xx", count: 1 },
    ],
    profiler_component_breakdown: [
      { component: "HTTP / Network", samples: 13522 },
      { component: "Oracle JDBC", samples: 12478 },
      { component: "Spring Batch", samples: 5971 },
      { component: "Spring Framework", samples: 658 },
    ],
  },
} as const;
