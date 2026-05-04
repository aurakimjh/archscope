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
