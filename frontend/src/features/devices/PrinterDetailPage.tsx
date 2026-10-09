import { ArrowClockwise, ArrowLeft, CheckCircle, Clock, Drop, Warning } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { listCartridges, replacePrinterCartridge, startPrinterPoll } from '../../api/client'
import type { DailyUsage, PrinterStatus } from '../../api/types'
import { showConfirm, showError, showSuccess } from '../../utils/sweetalert'
import { CounterChart } from './CounterChart'
import { DailyUsageChart } from './DailyUsageChart'
import { usePollingRuns } from './usePollingRuns'
import { usePollJob } from './usePollJob'
import { usePrinter } from './usePrinter'
import { usePrinterCounters } from './usePrinterCounters'
import { usePrinterUsage } from './usePrinterUsage'

const statusLabels: Record<PrinterStatus, string> = {
  online: 'Hoạt động',
  offline: 'Ngoại tuyến',
  unknown: 'Chưa xác định',
  unavailable: 'Không khả dụng',
}

function StatusBadge({ status }: { status: PrinterStatus }) {
  return <span className={`status-badge status-${status}`}>{statusLabels[status]}</span>
}

type MonthlyUsage = Omit<DailyUsage, 'local_date'> & { month: string }

function aggregateMonthlyUsage(items: DailyUsage[]): MonthlyUsage[] {
  const grouped = new Map<string, MonthlyUsage>()
  for (const item of items) {
    const month = item.local_date.slice(0, 7)
    const key = `${item.definition_key}-${month}`
    const current = grouped.get(key)
    if (current) {
      current.delta += item.delta
      if (current.quality === 'valid' && item.quality !== 'valid') current.quality = item.quality
      continue
    }
    grouped.set(key, { ...item, month })
  }
  return [...grouped.values()].sort(
    (a, b) => a.month.localeCompare(b.month) || a.definition_key.localeCompare(b.definition_key),
  )
}

export function PrinterDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const printerId = id ?? ''

  const query = usePrinter(printerId)
  const counters = usePrinterCounters(printerId)
  const usage = usePrinterUsage(printerId)
  const monthlyUsage = useMemo(() => aggregateMonthlyUsage(usage.data?.data ?? []), [usage.data?.data])
  const _pollingRuns = usePollingRuns(printerId)
  const queryClient = useQueryClient()

  const [jobID, setJobID] = useState<string | null>(null)
  const job = usePollJob(jobID)

  const cartridgesQuery = useQuery({ queryKey: ['cartridges'], queryFn: () => listCartridges() })
  const [showReplaceModal, setShowReplaceModal] = useState(false)
  const [selectedCartridgeId, setSelectedCartridgeId] = useState('')
  const [sourceType, setSourceType] = useState<'new' | 'refilled'>('new')
  const [replaceNotes, setReplaceNotes] = useState('')

  const replaceMutation = useMutation({
    mutationFn: () =>
      replacePrinterCartridge({
        cartridge_id: selectedCartridgeId,
        device_id: printerId,
        source_type: sourceType,
        notes: replaceNotes.trim(),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['cartridges'] })
      void queryClient.invalidateQueries({ queryKey: ['cartridge-logs'] })
      setShowReplaceModal(false)
      setSelectedCartridgeId('')
      setReplaceNotes('')
      void showSuccess('Đã thay mực thành công!', 'Hệ thống đã ghi nhận việc thay mực và cập nhật kho.')
    },
    onError: (err) => {
      void showError('Không thể thay mực', err instanceof Error ? err.message : 'Kiểm tra lại số lượng tồn kho.')
    },
  })

  const poll = useMutation({
    mutationFn: () => startPrinterPoll(printerId),
    onSuccess: (created) => {
      setJobID(created.id)
      void showSuccess('Đã tạo tiến trình thu thập!', 'Yêu cầu quét dữ liệu SNMP đã được gửi tới worker.')
    },
    onError: (err) => {
      void showError('Không thể khởi tạo', err instanceof Error ? err.message : 'Lỗi kết nối tới máy in.')
    },
  })

  useEffect(() => {
    if (job.data?.status === 'success') {
      void queryClient.invalidateQueries({ queryKey: ['printer', printerId] })
      void queryClient.invalidateQueries({ queryKey: ['printer-counters', printerId] })
      void queryClient.invalidateQueries({ queryKey: ['printer-polling-runs', printerId] })
    }
  }, [job.data?.status, queryClient, printerId])

  const handleManualPoll = async () => {
    const res = await showConfirm(
      'Xác nhận thu thập SNMP?',
      'Hệ thống sẽ kết nối tới máy in và đọc chỉ số bộ đếm tức thì.',
    )
    if (res.isConfirmed) {
      setJobID(null)
      poll.mutate()
    }
  }

  if (query.isLoading) {
    return (
      <div className="glass-panel p-12 text-center my-8">
        <div className="w-10 h-10 border-4 border-cyan-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
        <p className="text-slate-400 text-sm">Đang tải thông tin chi tiết máy in...</p>
      </div>
    )
  }

  if (query.isError || !query.data) {
    return (
      <div className="glass-panel p-10 text-center my-8 border-rose-900/50">
        <Warning size={48} className="text-rose-400 mx-auto mb-3" />
        <h3 className="text-xl font-bold text-slate-100 mb-2">Không thể tải thông tin máy in</h3>
        <p className="text-slate-400 text-sm max-w-md mx-auto mb-6">
          {query.error instanceof Error ? query.error.message : 'Thiết bị không tồn tại hoặc đã bị xóa khỏi hệ thống.'}
        </p>
        <button
          onClick={() => navigate('/printers')}
          className="bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold px-5 py-2.5 rounded-xl text-sm transition-all"
        >
          Trở về danh sách
        </button>
      </div>
    )
  }

  const printer = query.data

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="glass-panel p-6 sm:p-8">
        <button
          onClick={() => navigate('/printers')}
          className="inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-cyan-400 hover:text-cyan-300 transition-colors mb-4"
        >
          <ArrowLeft size={16} weight="bold" /> Trở về danh sách máy in
        </button>

        <div className="flex flex-wrap items-center justify-between gap-6 pb-6 border-b border-slate-800">
          <div>
            <div className="flex items-center gap-3 mb-1">
              <h1 className="text-2xl sm:text-3xl font-extrabold text-slate-100">{printer.display_name}</h1>
              <StatusBadge status={printer.status} />
            </div>
            <p className="text-slate-400 text-sm">
              Mã tài sản: <strong className="text-slate-200">{printer.asset_code ?? 'Chưa gán'}</strong>
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => setShowReplaceModal(true)}
              className="bg-amber-600 hover:bg-amber-500 text-white font-semibold px-4 py-2.5 rounded-xl text-sm transition-all flex items-center gap-2 shadow-lg shadow-amber-950/50 cursor-pointer"
            >
              <Drop size={18} weight="bold" />
              <span>Thay mực máy in</span>
            </button>
            <button
              disabled={poll.isPending || job.data?.status === 'queued'}
              onClick={handleManualPoll}
              className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all flex items-center gap-2 shadow-lg shadow-cyan-950/50 cursor-pointer"
            >
              <ArrowClockwise size={18} className={poll.isPending ? 'animate-spin' : ''} />
              <span>{poll.isPending ? 'Đang gửi...' : 'Thu thập ngay'}</span>
            </button>
          </div>
        </div>

        {/* Polling Job Status Alert */}
        {job.data && (
          <div
            className={`mt-4 p-3.5 rounded-xl text-xs font-semibold flex items-center justify-between border ${
              job.data.status === 'queued'
                ? 'bg-cyan-950/50 text-cyan-300 border-cyan-800/60'
                : job.data.status === 'success'
                  ? 'bg-emerald-950/50 text-emerald-300 border-emerald-800/60'
                  : 'bg-rose-950/50 text-rose-300 border-rose-800/60'
            }`}
          >
            <div className="flex items-center gap-2">
              {job.data.status === 'queued' ? (
                <Clock size={16} className="animate-spin" />
              ) : job.data.status === 'success' ? (
                <CheckCircle size={16} />
              ) : (
                <Warning size={16} />
              )}
              <span>
                {job.data.status === 'queued'
                  ? 'Tiến trình thu thập SNMP đang chờ xử lý trong hàng đợi...'
                  : job.data.status === 'success'
                    ? 'Thu thập dữ liệu SNMP thành công!'
                    : `Thu thập thất bại: ${job.data.error_code ?? 'Lỗi không xác định'}`}
              </span>
            </div>
          </div>
        )}

        {/* Specifications Grid */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4 mt-6">
          <div className="bg-slate-950/50 p-4 rounded-xl border border-slate-800/60">
            <dt className="text-[11px] uppercase font-bold text-slate-400">Phòng ban</dt>
            <dd className="text-sm font-semibold text-cyan-400 mt-1">{printer.department ?? '—'}</dd>
          </div>
          <div className="bg-slate-950/50 p-4 rounded-xl border border-slate-800/60">
            <dt className="text-[11px] uppercase font-bold text-slate-400">Nhà sản xuất</dt>
            <dd className="text-sm font-semibold text-slate-200 mt-1">{printer.manufacturer ?? '—'}</dd>
          </div>
          <div className="bg-slate-950/50 p-4 rounded-xl border border-slate-800/60">
            <dt className="text-[11px] uppercase font-bold text-slate-400">Dòng máy (Model)</dt>
            <dd className="text-sm font-semibold text-slate-200 mt-1">{printer.model ?? '—'}</dd>
          </div>
          <div className="bg-slate-950/50 p-4 rounded-xl border border-slate-800/60">
            <dt className="text-[11px] uppercase font-bold text-slate-400">Số sê-ri (Serial)</dt>
            <dd className="text-sm font-semibold text-slate-200 mt-1">{printer.serial ?? '—'}</dd>
          </div>
          <div className="bg-slate-950/50 p-4 rounded-xl border border-slate-800/60">
            <dt className="text-[11px] uppercase font-bold text-slate-400">Lần cuối thấy</dt>
            <dd className="text-xs font-semibold text-slate-200 mt-1">
              {printer.last_seen_at ? new Date(printer.last_seen_at).toLocaleString('vi-VN') : 'Chưa thu thập'}
            </dd>
          </div>
          <div className="bg-slate-950/50 p-4 rounded-xl border border-slate-800/60">
            <dt className="text-[11px] uppercase font-bold text-slate-400">Mã thiết bị (UUID)</dt>
            <dd className="text-xs font-mono text-cyan-400 mt-1 truncate">{printer.id}</dd>
          </div>
        </div>
      </div>

      {/* Recharts Charts Section */}
      {!counters.isLoading && !counters.isError && (counters.data?.data.length ?? 0) > 0 && (
        <CounterChart readings={counters.data?.data ?? []} />
      )}

      {!usage.isLoading && !usage.isError && (usage.data?.data.length ?? 0) > 0 && (
        <DailyUsageChart usage={usage.data?.data ?? []} />
      )}

      {/* Raw Counter Readings Table */}
      <div className="glass-panel overflow-hidden">
        <div className="p-6 border-b border-slate-800 flex items-center justify-between">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Dữ liệu gốc bất biến</p>
            <h3 className="text-lg font-bold text-slate-100">Lịch sử bộ đếm (Counter Readings)</h3>
          </div>
          <span className="text-xs font-semibold bg-slate-800 text-slate-300 px-3 py-1 rounded-full">
            {counters.data?.total ?? 0} bản ghi
          </span>
        </div>

        {counters.isLoading ? (
          <div className="p-8 text-center text-slate-400 text-sm">Đang tải lịch sử chỉ số bộ đếm...</div>
        ) : (counters.data?.data.length ?? 0) === 0 ? (
          <div className="p-8 text-center text-slate-400 text-sm">Chưa có chỉ số bộ đếm nào được ghi nhận.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs sm:text-sm">
              <thead>
                <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                  <th className="p-4 pl-6">Bộ đếm</th>
                  <th className="p-4">Giá trị gốc</th>
                  <th className="p-4">Đơn vị</th>
                  <th className="p-4">Chất lượng</th>
                  <th className="p-4 pr-6">Thời gian thu thập</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {counters.data?.data.map((item) => (
                  <tr key={item.id} className="hover:bg-slate-800/40 transition-colors">
                    <td className="p-4 pl-6 font-semibold text-slate-200">{item.definition_key}</td>
                    <td className="p-4 font-bold text-cyan-400">{item.raw_value.toLocaleString()}</td>
                    <td className="p-4 text-slate-300">{item.unit}</td>
                    <td className="p-4">
                      <span className={`quality-tag quality-${item.quality}`}>{item.quality}</span>
                    </td>
                    <td className="p-4 pr-6 text-slate-400">{new Date(item.collected_at).toLocaleString('vi-VN')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Monthly Usage Aggregation */}
      <div className="glass-panel overflow-hidden">
        <div className="p-6 border-b border-slate-800 flex items-center justify-between">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-emerald-400">Tổng hợp theo tháng</p>
            <h3 className="text-lg font-bold text-slate-100">Sản lượng in hàng tháng</h3>
          </div>
          <span className="text-xs font-semibold bg-slate-800 text-slate-300 px-3 py-1 rounded-full">
            {monthlyUsage.length} tháng
          </span>
        </div>

        {monthlyUsage.length === 0 ? (
          <div className="p-8 text-center text-slate-400 text-sm">Chưa có dữ liệu sản lượng in theo tháng.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs sm:text-sm">
              <thead>
                <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                  <th className="p-4 pl-6">Tháng</th>
                  <th className="p-4">Bộ đếm</th>
                  <th className="p-4">Lưu lượng in</th>
                  <th className="p-4 pr-6">Trạng thái dữ liệu</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {monthlyUsage.map((item) => (
                  <tr key={`${item.definition_key}-${item.month}`} className="hover:bg-slate-800/40 transition-colors">
                    <td className="p-4 pl-6 font-bold text-slate-200">{item.month}</td>
                    <td className="p-4 text-slate-300">{item.definition_key}</td>
                    <td className="p-4 font-bold text-emerald-400">
                      {item.delta.toLocaleString()} {item.unit}
                    </td>
                    <td className="p-4 pr-6">
                      <span className={`quality-tag quality-${item.quality}`}>{item.quality}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal Replace Toner */}
      {showReplaceModal && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-amber-800">
            <h3 className="text-lg font-bold text-slate-100 flex items-center gap-2">
              <Drop size={20} className="text-amber-400" /> Thay mực cho máy in {printer.display_name}
            </h3>
            <div className="space-y-3">
              <div>
                <label htmlFor="cartridge-select" className="block text-xs font-bold text-slate-300 mb-1">
                  Chọn chủng loại mực
                </label>
                <select
                  id="cartridge-select"
                  value={selectedCartridgeId}
                  onChange={(e) => setSelectedCartridgeId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                >
                  <option value="">-- Chọn loại mực --</option>
                  {(cartridgesQuery.data?.data ?? []).map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name} ({c.sku_code}) - Mới: {c.stock_new} | Bơm: {c.stock_refilled}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <span className="block text-xs font-bold text-slate-300 mb-1">Loại mực sử dụng</span>
                <div className="flex items-center gap-4 pt-1">
                  <label className="inline-flex items-center gap-2 cursor-pointer text-sm text-slate-200">
                    <input
                      type="radio"
                      name="sourceType"
                      value="new"
                      checked={sourceType === 'new'}
                      onChange={() => setSourceType('new')}
                      className="text-amber-500 focus:ring-amber-500"
                    />
                    <span>🟢 Bình MỚI 100%</span>
                  </label>
                  <label className="inline-flex items-center gap-2 cursor-pointer text-sm text-slate-200">
                    <input
                      type="radio"
                      name="sourceType"
                      value="refilled"
                      checked={sourceType === 'refilled'}
                      onChange={() => setSourceType('refilled')}
                      className="text-amber-500 focus:ring-amber-500"
                    />
                    <span>🔵 Bình ĐÃ BƠM</span>
                  </label>
                </div>
              </div>

              <div>
                <label htmlFor="replace-notes" className="block text-xs font-bold text-slate-300 mb-1">
                  Ghi chú thay mực
                </label>
                <input
                  id="replace-notes"
                  value={replaceNotes}
                  onChange={(e) => setReplaceNotes(e.target.value)}
                  placeholder="vd: Kỹ thuật viên Nguyễn Văn A thay mực"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowReplaceModal(false)}
                className="bg-slate-800 text-slate-300 px-4 py-2 rounded-xl text-sm"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={replaceMutation.isPending || !selectedCartridgeId}
                onClick={() => replaceMutation.mutate()}
                className="bg-amber-600 hover:bg-amber-500 text-white font-semibold px-4 py-2 rounded-xl text-sm cursor-pointer"
              >
                {replaceMutation.isPending ? 'Đang thực hiện...' : 'Xác nhận thay mực'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
