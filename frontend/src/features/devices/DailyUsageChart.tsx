import type { DailyUsage } from '../../api/types'

export function DailyUsageChart({ usage }: { usage: DailyUsage[] }) {
  const key = usage[0]?.definition_key
  const series = usage.filter((item) => item.definition_key === key).slice(-14)
  if (series.length === 0) return null
  const width = 760
  const height = 220
  const left = 48
  const bottom = 42
  const top = 18
  const plotHeight = height - top - bottom
  const plotWidth = width - left - 20
  const max = Math.max(...series.map((item) => item.delta), 1)
  const slot = plotWidth / series.length
  return <div className="counter-chart usage-chart" role="img" aria-label={`Daily ${key} usage`}>
    <div className="counter-chart-heading"><div><p className="eyebrow">Daily delta</p><h4>{key}</h4></div><span>{series[0].unit} · last {series.length} days</span></div>
    <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none">
      <title>{`Daily ${key} usage`}</title>
      <line x1={left} y1={top + plotHeight} x2={width - 20} y2={top + plotHeight} className="chart-axis" />
      <text x={left - 8} y={top + 4} textAnchor="end" className="chart-label">{max.toLocaleString()}</text>
      {series.map((item, index) => {
        const barHeight = (item.delta / max) * plotHeight
        const barWidth = Math.max(slot - 8, 4)
        const x = left + index * slot + 4
        const y = top + plotHeight - barHeight
        return <g key={`${item.definition_key}-${item.local_date}`}><rect x={x} y={y} width={barWidth} height={barHeight} rx="3" className={`usage-bar usage-bar-${item.quality}`}><title>{`${item.local_date}: ${item.delta.toLocaleString()} ${item.unit} · ${item.quality}`}</title></rect><text x={x + barWidth / 2} y={height - 12} textAnchor="middle" className="chart-label">{item.local_date.slice(5)}</text></g>
      })}
    </svg>
  </div>
}
