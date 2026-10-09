import { ArrowRight, MagnifyingGlass, Plus, Printer as PrinterIcon, ShieldCheck, Warning } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { registerPrinter } from '../../api/client'
import type { Printer, PrinterStatus } from '../../api/types'
import { showError, showSuccess } from '../../utils/sweetalert'
import { usePollJob } from './usePollJob'
import { usePrinters } from './usePrinters'
import { useProfiles } from './useProfiles'

const statusLabels: Record<PrinterStatus, string> = {
  online: 'Hoạt động',
  offline: 'Ngoại tuyến',
  unknown: 'Chưa xác định',
  unavailable: 'Không khả dụng',
}

function StatusBadge({ status }: { status: PrinterStatus }) {
  return <span className={`status-badge status-${status}`}>{statusLabels[status]}</span>
}

function PrinterRow({ printer, onSelect }: { printer: Printer; onSelect: (id: string) => void }) {
  return (
    <tr
      onClick={() => onSelect(printer.id)}
      className="hover:bg-slate-800/50 cursor-pointer transition-colors border-b border-slate-800/60"
    >
      <td className="p-4 pl-6">
        <button className="text-left group">
          <span className="font-bold text-slate-100 group-hover:text-cyan-400 transition-colors block text-sm">
            {printer.display_name}
          </span>
          <small className="text-slate-400 text-xs mt-0.5 block">{printer.asset_code ?? 'Chưa gán mã tài sản'}</small>
        </button>
      </td>
      <td className="p-4 text-slate-300 text-sm font-semibold">{printer.department || '—'}</td>
      <td className="p-4 text-slate-300 text-sm">{printer.manufacturer || '—'}</td>
      <td className="p-4 text-slate-300 text-sm">{printer.model || '—'}</td>
      <td className="p-4">
        <StatusBadge status={printer.status} />
      </td>
      <td className="p-4 pr-6 text-slate-400 text-xs">
        {printer.last_seen_at ? new Date(printer.last_seen_at).toLocaleString('vi-VN') : 'Chưa từng quét'}
      </td>
    </tr>
  )
}

export function DevicesPage() {
  const navigate = useNavigate()
  const query = usePrinters()
  const queryClient = useQueryClient()
  const profiles = useProfiles()

  const [showCreate, setShowCreate] = useState(false)
  const [displayName, setDisplayName] = useState('')
  const [address, setAddress] = useState('')
  const [community, setCommunity] = useState('')
  const [department, setDepartment] = useState('')

  const [registrationJobID, setRegistrationJobID] = useState<string | null>(null)
  const registrationJob = usePollJob(registrationJobID)

  const create = useMutation({
    mutationFn: () =>
      registerPrinter({
        display_name: displayName.trim(),
        address: address.trim(),
        version: '2c',
        community,
        department: department.trim(),
      }),
    onSuccess: (result) => {
      setRegistrationJobID(result.poll_job_id ?? null)
      setDisplayName('')
      setAddress('')
      setCommunity('')
      setDepartment('')
      setShowCreate(false)
      void query.refetch()
      void showSuccess('Đã thêm máy in mới!', 'Kết nối đã được lưu và tiến trình thu thập ban đầu đã khởi chạy.')
    },
    onError: (err) => {
      void showError(
        'Không thể đăng ký máy in',
        err instanceof Error ? err.message : 'Kiểm tra lại thông tin IP và SNMP Community.',
      )
    },
  })

  useEffect(() => {
    if (registrationJob.data?.status === 'success' || registrationJob.data?.status === 'failed') {
      void queryClient.invalidateQueries({ queryKey: ['printers'] })
    }
  }, [registrationJob.data?.status, queryClient])

  if (query.isLoading) {
    return (
      <div className="glass-panel p-12 text-center my-8">
        <div className="w-10 h-10 border-4 border-cyan-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
        <p className="text-slate-400 text-sm">Đang nạp danh sách máy in trong mạng...</p>
      </div>
    )
  }

  if (query.isError) {
    return (
      <div className="glass-panel p-10 text-center my-8 border-rose-900/50">
        <Warning size={48} className="text-rose-400 mx-auto mb-3" />
        <h3 className="text-xl font-bold text-slate-100 mb-2">Không thể kết nối dịch vụ API</h3>
        <p className="text-slate-400 text-sm max-w-md mx-auto mb-6">
          {query.error instanceof Error ? query.error.message : 'Hệ thống API backend không phản hồi.'}
        </p>
        <button
          onClick={() => void query.refetch()}
          className="bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all"
        >
          Thử lại
        </button>
      </div>
    )
  }

  const printers = query.data?.data ?? []
  const verifiedProfilesCount = profiles.data?.data.filter((p) => p.verification_status === 'verified').length ?? 0

  return (
    <div className="space-y-6">
      {/* Intro Workspace Banner */}
      <div className="glass-panel p-6 sm:p-8 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6 relative overflow-hidden">
        <div className="space-y-2 max-w-2xl z-10">
          <p className="text-xs uppercase font-extrabold tracking-widest text-cyan-400">
            Không gian quản lý máy in LAN
          </p>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-100 tracking-tight">
            Quản lý lưu lượng in ấn tập trung
          </h1>
          <p className="text-slate-400 text-sm leading-relaxed">
            Giám sát chỉ số bộ đếm máy in qua SNMP v2c/v3, chuẩn hóa theo múi giờ IANA và tính toán trang in tự động.
          </p>
        </div>
        <div className="flex items-center gap-3 z-10 w-full sm:w-auto">
          <button
            onClick={() => navigate('/discovery')}
            className="flex-1 sm:flex-initial bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm border border-slate-700/80 transition-all flex items-center justify-center gap-2"
          >
            <MagnifyingGlass size={18} />
            <span>Dò tìm SNMP</span>
          </button>
          <button
            onClick={() => setShowCreate((v) => !v)}
            className="flex-1 sm:flex-initial bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm shadow-lg shadow-cyan-950/50 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <Plus size={18} weight="bold" />
            <span>{showCreate ? 'Hủy bỏ' : 'Thêm máy in'}</span>
          </button>
        </div>
      </div>

      {/* Profile Library Summary Banner */}
      <div className="glass-card p-4 flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <ShieldCheck size={24} className="text-emerald-400" />
          <div>
            <p className="text-xs text-slate-400 font-medium">Thư viện Profile nhận diện</p>
            <p className="text-sm font-bold text-slate-200">
              {profiles.isLoading ? 'Đang nạp profile...' : `${profiles.data?.total ?? 0} profile mẫu có sẵn`}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-emerald-400 bg-emerald-950/60 border border-emerald-800/60 px-3 py-1 rounded-full font-semibold">
            {verifiedProfilesCount} profile đã kiểm thực
          </span>
          <button
            onClick={() => navigate('/profiles')}
            className="text-xs text-cyan-400 hover:text-cyan-300 font-semibold px-3 py-1 flex items-center gap-1"
          >
            Xem tất cả <ArrowRight size={14} />
          </button>
        </div>
      </div>

      {/* Quick Add Printer Form */}
      {showCreate && (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (displayName.trim() && address.trim() && community) create.mutate()
          }}
          className="glass-panel p-6 space-y-4 border-cyan-800/50"
        >
          <div className="flex items-center justify-between pb-3 border-b border-slate-800">
            <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
              <PrinterIcon size={20} className="text-cyan-400" /> Đăng ký kết nối máy in mới
            </h3>
            <span className="text-xs text-slate-400">Bảo mật AES-256 GCM</span>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div>
              <label htmlFor="display-name" className="block text-xs font-bold text-slate-300 mb-1.5">
                Tên hiển thị
              </label>
              <input
                id="display-name"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="vd: Máy in Phòng Kế Toán"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
                required
              />
            </div>

            <div>
              <label htmlFor="department" className="block text-xs font-bold text-slate-300 mb-1.5">
                Phòng ban
              </label>
              <input
                id="department"
                value={department}
                onChange={(e) => setDepartment(e.target.value)}
                placeholder="vd: Phòng IT / Kế toán"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
              />
            </div>

            <div>
              <label htmlFor="printer-address" className="block text-xs font-bold text-slate-300 mb-1.5">
                Địa chỉ IP hoặc Hostname
              </label>
              <input
                id="printer-address"
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                placeholder="vd: 192.168.1.50"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={253}
                required
              />
            </div>

            <div>
              <label htmlFor="snmp-community" className="block text-xs font-bold text-slate-300 mb-1.5">
                Chuỗi SNMP v2c Community
              </label>
              <input
                id="snmp-community"
                type="password"
                value={community}
                onChange={(e) => setCommunity(e.target.value)}
                placeholder="vd: public"
                autoComplete="new-password"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                required
              />
            </div>
          </div>

          <div className="flex items-center justify-between pt-2">
            <p className="text-xs text-slate-400">
              🔒 Chuỗi Community string sẽ được mã hóa an toàn trước khi lưu vào SQLite.
            </p>
            <button
              type="submit"
              disabled={create.isPending}
              className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all shadow-md cursor-pointer"
            >
              {create.isPending ? 'Đang tạo...' : 'Lưu và kết nối'}
            </button>
          </div>
        </form>
      )}

      {/* Printers Main Table */}
      <div className="glass-panel overflow-hidden">
        <div className="p-6 border-b border-slate-800 flex items-center justify-between">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Danh sách thiết bị</p>
            <h2 className="text-xl font-bold text-slate-100">Máy in hiện có</h2>
          </div>
          <span className="text-xs font-semibold bg-slate-800 text-slate-300 px-3 py-1 rounded-full">
            {printers.length} máy in
          </span>
        </div>

        {printers.length === 0 ? (
          <div className="p-12 text-center text-slate-400 space-y-3">
            <div className="w-12 h-12 rounded-2xl bg-slate-800/80 text-cyan-400 flex items-center justify-center mx-auto text-xl">
              ⌁
            </div>
            <h3 className="text-base font-bold text-slate-200">Chưa có máy in nào được đăng ký</h3>
            <p className="text-xs text-slate-400 max-w-sm mx-auto">
              Nhấn nút "Thêm máy in" ở trên hoặc dùng tính năng Dò tìm SNMP để kết nối máy in trong mạng LAN.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs sm:text-sm">
              <thead>
                <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                  <th className="p-4 pl-6">Tên máy in / Mã tài sản</th>
                  <th className="p-4">Phòng ban</th>
                  <th className="p-4">Nhà sản xuất</th>
                  <th className="p-4">Dòng máy (Model)</th>
                  <th className="p-4">Trạng thái</th>
                  <th className="p-4 pr-6">Lần thấy cuối</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {printers.map((printer) => (
                  <PrinterRow key={printer.id} printer={printer} onSelect={(id) => navigate(`/printers/${id}`)} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
