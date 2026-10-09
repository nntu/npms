import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { CounterReading } from '../../api/types'

function formatDate(value: string) {
  const d = new Date(value)
  return `${d.getDate()}/${d.getMonth() + 1} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

export function CounterChart({ readings }: { readings: CounterReading[] }) {
  const firstKey = readings[0]?.definition_key
  const series = readings
    .filter(
      (reading) =>
        reading.definition_key === firstKey && reading.quality !== 'unsupported' && reading.quality !== 'unavailable',
    )
    .slice()
    .sort((a, b) => new Date(a.collected_at).getTime() - new Date(b.collected_at).getTime())

  if (series.length === 0) return null

  const unit = series[0].unit
  const chartData = series.map((item) => ({
    time: formatDate(item.collected_at),
    rawTime: item.collected_at,
    value: item.raw_value,
    quality: item.quality,
    unit: item.unit,
  }))

  return (
    <div className="glass-card p-5 my-6">
      <div className="flex flex-wrap items-center justify-between gap-4 mb-4 pb-3 border-b border-slate-800">
        <div>
          <p className="text-xs uppercase font-bold tracking-wider text-cyan-400">Xu hướng lịch sử bộ đếm</p>
          <h4 className="text-lg font-bold text-slate-100">{series[0].definition_key}</h4>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-slate-400 bg-slate-800/80 px-3 py-1 rounded-full border border-slate-700/60">
            Đơn vị: <strong className="text-slate-200">{unit}</strong>
          </span>
          <span className="text-xs text-slate-400 bg-slate-800/80 px-3 py-1 rounded-full border border-slate-700/60">
            Số lượng: <strong className="text-slate-200">{series.length} mẫu</strong>
          </span>
        </div>
      </div>

      <div className="h-64 w-full">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData} margin={{ top: 10, right: 20, left: 10, bottom: 0 }}>
            <defs>
              <linearGradient id="counterGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#06b6d4" stopOpacity={0.4} />
                <stop offset="95%" stopColor="#06b6d4" stopOpacity={0.0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#334155" opacity={0.5} />
            <XAxis dataKey="time" stroke="#94a3b8" fontSize={11} tickLine={false} />
            <YAxis
              stroke="#94a3b8"
              fontSize={11}
              tickLine={false}
              domain={['dataMin', 'dataMax']}
              tickFormatter={(v: number) => v.toLocaleString()}
            />
            <Tooltip
              content={(props: any) => {
                if (!props.active || !props.payload?.length) return null
                const data = props.payload[0].payload
                return (
                  <div className="bg-slate-900 border border-slate-700 p-3 rounded-xl shadow-xl text-xs space-y-1">
                    <p className="font-semibold text-cyan-400">{new Date(data.rawTime).toLocaleString('vi-VN')}</p>
                    <p className="text-slate-200 font-bold text-sm">
                      Chỉ số: {data.value.toLocaleString()} {data.unit}
                    </p>
                    <p className="text-slate-400">
                      Chất lượng: <span className={`quality-tag quality-${data.quality}`}>{data.quality}</span>
                    </p>
                  </div>
                )
              }}
            />
            <Area
              type="monotone"
              dataKey="value"
              stroke="#06b6d4"
              strokeWidth={2.5}
              fillOpacity={1}
              fill="url(#counterGradient)"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      {series.some((reading) => reading.quality !== 'valid') && (
        <p className="text-xs text-amber-400/90 bg-amber-950/40 border border-amber-900/50 p-2.5 rounded-lg mt-3">
          ⚠️ Chú ý: Dữ liệu chứa các mẫu ghi chưa được xác thực chuẩn (unverified/suspicious). Vui lòng đối chiếu kỹ
          trước khi lập báo cáo.
        </p>
      )}
    </div>
  )
}
