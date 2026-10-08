import type { CounterReading } from '../../api/types'

const chartWidth = 760
const chartHeight = 250
const padding = { top: 20, right: 24, bottom: 42, left: 58 }

function formatDate(value: string) {
  return new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function CounterChart({ readings }: { readings: CounterReading[] }) {
  const firstKey = readings[0]?.definition_key
  const series = readings
    .filter((reading) => reading.definition_key === firstKey && reading.quality !== 'unsupported' && reading.quality !== 'unavailable')
    .slice()
    .sort((a, b) => new Date(a.collected_at).getTime() - new Date(b.collected_at).getTime())
  if (series.length === 0) return null

  const plotWidth = chartWidth - padding.left - padding.right
  const plotHeight = chartHeight - padding.top - padding.bottom
  const values = series.map((reading) => reading.raw_value)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  const x = (index: number) => padding.left + (series.length === 1 ? plotWidth / 2 : (index / (series.length - 1)) * plotWidth)
  const y = (value: number) => padding.top + plotHeight - ((value - min) / range) * plotHeight
  const points = series.map((reading, index) => `${x(index)},${y(reading.raw_value)}`).join(' ')
  const unit = series[0].unit

  return <div className="counter-chart" role="img" aria-label={`Raw ${series[0].definition_key} counter history`}>
    <div className="counter-chart-heading"><div><p className="eyebrow">Trend</p><h4>{series[0].definition_key}</h4></div><span>{unit} · {series.length} readings</span></div>
    <svg viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="none">
      <title>{`Raw ${series[0].definition_key} values over time`}</title>
      <line x1={padding.left} y1={padding.top + plotHeight} x2={chartWidth - padding.right} y2={padding.top + plotHeight} className="chart-axis" />
      <line x1={padding.left} y1={padding.top} x2={padding.left} y2={padding.top + plotHeight} className="chart-axis" />
      <text x={padding.left - 10} y={padding.top + 4} textAnchor="end" className="chart-label">{max.toLocaleString()}</text>
      <text x={padding.left - 10} y={padding.top + plotHeight + 4} textAnchor="end" className="chart-label">{min.toLocaleString()}</text>
      <polyline points={points} className="chart-line" />
      {series.map((reading, index) => <circle key={reading.id} cx={x(index)} cy={y(reading.raw_value)} r="5" className={`chart-point chart-point-${reading.quality}`}><title>{`${reading.raw_value.toLocaleString()} ${reading.unit} · ${new Date(reading.collected_at).toLocaleString()} · ${reading.quality}`}</title></circle>)}
      <text x={padding.left} y={chartHeight - 12} className="chart-label">{formatDate(series[0].collected_at)}</text>
      <text x={chartWidth - padding.right} y={chartHeight - 12} textAnchor="end" className="chart-label">{formatDate(series[series.length - 1].collected_at)}</text>
    </svg>
    {series.some((reading) => reading.quality !== 'valid') && <p className="chart-note">Raw values include readings that are not marked valid; review quality before using them in reports.</p>}
  </div>
}
