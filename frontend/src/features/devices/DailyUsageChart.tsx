import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { DailyUsage } from '../../api/types'

export function DailyUsageChart({ usage }: { usage: DailyUsage[] }) {
  const key = usage[0]?.definition_key
  const series = usage.filter((item) => item.definition_key === key).slice(-14)
  if (series.length === 0) return null

  const unit = series[0].unit
  const chartData = series.map((item) => ({
    date: item.local_date,
    shortDate: item.local_date.slice(5),
    delta: item.delta,
    unit: item.unit,
    quality: item.quality,
  }))

  const getBarColor = (quality: string) => {
    switch (quality) {
      case 'valid':
        return '#10b981'
      case 'suspicious':
        return '#f59e0b'
      default:
        return '#06b6d4'
    }
  }

  return (
    <div className="glass-card p-5 my-6">
      <div className="flex flex-wrap items-center justify-between gap-4 mb-4 pb-3 border-b border-slate-200">
        <div>
          <p className="text-xs uppercase font-bold tracking-wider text-emerald-700">
            Lưu lượng in theo ngày (14 ngày gần nhất)
          </p>
          <h4 className="text-lg font-bold text-slate-900">{key}</h4>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-slate-600 bg-slate-100 px-3 py-1 rounded-full border border-slate-200">
            Đơn vị tính: <strong className="text-slate-900">{unit}</strong>
          </span>
        </div>
      </div>

      <div className="h-60 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={chartData} margin={{ top: 10, right: 20, left: 10, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" opacity={0.8} />
            <XAxis dataKey="shortDate" stroke="#64748b" fontSize={11} tickLine={false} />
            <YAxis stroke="#64748b" fontSize={11} tickLine={false} tickFormatter={(v: number) => v.toLocaleString()} />
            <Tooltip
              content={(props: any) => {
                if (!props.active || !props.payload?.length) return null
                const data = props.payload[0].payload
                return (
                  <div className="bg-white border border-slate-200 p-3 rounded-xl shadow-lg text-xs space-y-1">
                    <p className="font-semibold text-emerald-700">Ngày: {data.date}</p>
                    <p className="text-slate-900 font-bold text-sm">
                      Sử dụng: {data.delta.toLocaleString()} {data.unit}
                    </p>
                    <p className="text-slate-500">
                      Trạng thái dữ liệu: <span className={`quality-tag quality-${data.quality}`}>{data.quality}</span>
                    </p>
                  </div>
                )
              }}
            />
            <Bar dataKey="delta" radius={[6, 6, 0, 0]}>
              {chartData.map((entry) => (
                <Cell key={`cell-${entry.date}`} fill={getBarColor(entry.quality)} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
