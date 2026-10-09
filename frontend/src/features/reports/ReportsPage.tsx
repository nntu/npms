import { ChartBar, CheckCircle, Printer as PrinterIcon, Pulse } from '@phosphor-icons/react'
import { usePrinters } from '../devices/usePrinters'

export function ReportsPage() {
  const printersQuery = usePrinters()
  const printers = printersQuery.data?.data ?? []

  const onlineCount = printers.filter((p) => p.status === 'online').length
  const offlineCount = printers.filter((p) => p.status === 'offline').length

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="glass-panel p-6 sm:p-8">
        <div className="flex items-center gap-3 mb-2">
          <ChartBar size={28} className="text-cyan-400" />
          <h1 className="text-2xl sm:text-3xl font-black text-slate-100">Báo cáo & Thống kê lưu lượng in</h1>
        </div>
        <p className="text-slate-400 text-sm max-w-2xl">
          Tổng hợp sản lượng in ấn theo ngày, theo tháng và đánh giá trạng thái hoạt động của toàn bộ thiết bị trong
          mạng LAN.
        </p>
      </div>

      {/* Summary KPI Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="glass-card p-5 flex items-center gap-4">
          <div className="w-12 h-12 rounded-2xl bg-cyan-950/80 border border-cyan-800/60 text-cyan-400 flex items-center justify-center text-2xl font-bold">
            <PrinterIcon size={24} />
          </div>
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Tổng số máy in</p>
            <p className="text-2xl font-black text-slate-100 mt-0.5">{printers.length} thiết bị</p>
          </div>
        </div>

        <div className="glass-card p-5 flex items-center gap-4">
          <div className="w-12 h-12 rounded-2xl bg-emerald-950/80 border border-emerald-800/60 text-emerald-400 flex items-center justify-center text-2xl font-bold">
            <CheckCircle size={24} />
          </div>
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Đang hoạt động (Online)</p>
            <p className="text-2xl font-black text-emerald-400 mt-0.5">{onlineCount} thiết bị</p>
          </div>
        </div>

        <div className="glass-card p-5 flex items-center gap-4">
          <div className="w-12 h-12 rounded-2xl bg-rose-950/80 border border-rose-800/60 text-rose-400 flex items-center justify-center text-2xl font-bold">
            <Pulse size={24} />
          </div>
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Mất kết nối (Offline)</p>
            <p className="text-2xl font-black text-rose-400 mt-0.5">{offlineCount} thiết bị</p>
          </div>
        </div>
      </div>

      {/* Printers Overview Table */}
      <div className="glass-panel overflow-hidden">
        <div className="p-6 border-b border-slate-800 flex items-center justify-between">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Thống kê từng máy in</p>
            <h3 className="text-lg font-bold text-slate-100">Danh sách theo dõi lưu lượng</h3>
          </div>
        </div>

        {printers.length === 0 ? (
          <div className="p-10 text-center text-slate-400 text-sm">Chưa có máy in nào để tạo báo cáo.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs sm:text-sm">
              <thead>
                <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                  <th className="p-4 pl-6">Tên máy in</th>
                  <th className="p-4">Nhà sản xuất</th>
                  <th className="p-4">Dòng máy (Model)</th>
                  <th className="p-4">Mã tài sản</th>
                  <th className="p-4 pr-6">Trạng thái</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {printers.map((item) => (
                  <tr key={item.id} className="hover:bg-slate-800/40 transition-colors">
                    <td className="p-4 pl-6 font-bold text-slate-200">{item.display_name}</td>
                    <td className="p-4 text-slate-300">{item.manufacturer || '—'}</td>
                    <td className="p-4 text-slate-300">{item.model || '—'}</td>
                    <td className="p-4 text-slate-400">{item.asset_code || '—'}</td>
                    <td className="p-4 pr-6">
                      <span className={`status-badge status-${item.status}`}>
                        {item.status === 'online' ? 'Hoạt động' : 'Ngoại tuyến'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
