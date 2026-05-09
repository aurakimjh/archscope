// ─────────────────────────────────────────────────────────────────────
// [한글] MetricCard.tsx — KPI 한 칸을 표시하는 가장 단순한 표시 컴포넌트.
//   대시보드/분석 페이지의 상단 핵심 지표(예: 평균 응답시간) 한 줄에
//   여러 개를 그리드로 배치할 때 사용합니다.
//   .metric-card 클래스는 styles/global.css 에 정의(테두리/패딩/타이포).
// ─────────────────────────────────────────────────────────────────────
type MetricCardProps = {
  label: string;
  value: string | number;
};

// [한글] MetricCard — label + value 두 줄짜리 작은 카드. 의도적으로 매우
//   얇은 wrapper 로 두어 페이지마다 자유롭게 grid 에 배치 가능합니다.
export function MetricCard({ label, value }: MetricCardProps): JSX.Element {
  return (
    <div className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
