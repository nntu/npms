import { ArrowRight, CheckCircle, MagnifyingGlass, Shield } from '@phosphor-icons/react'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { probeDiscovery } from '../../api/client'
import { showError, showSuccess } from '../../utils/sweetalert'

export function DiscoveryPage() {
  const navigate = useNavigate()
  const [probeAddress, setProbeAddress] = useState('')
  const [probeCommunity, setProbeCommunity] = useState('')

  const discovery = useMutation({
    mutationFn: () => probeDiscovery({ address: probeAddress.trim(), version: '2c', community: probeCommunity }),
    onSuccess: () => {
      void showSuccess('Đã đọc thông tin máy in!', 'Hệ thống đã kết nối thành công tới thiết bị.')
    },
    onError: (err) => {
      void showError(
        'Dò tìm thất bại',
        err instanceof Error ? err.message : 'Không nhận được phản hồi SNMP từ địa chỉ IP.',
      )
    },
  })

  const handleRegister = () => {
    if (!discovery.data) return
    navigate('/printers')
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="glass-panel p-6 sm:p-8">
        <div className="flex items-center gap-3 mb-2">
          <MagnifyingGlass size={28} className="text-cyan-400" />
          <h1 className="text-2xl sm:text-3xl font-black text-slate-100">Dò tìm SNMP (Single-IP Probe)</h1>
        </div>
        <p className="text-slate-400 text-sm max-w-2xl">
          Kiểm tra thông tin định danh (Identity), sysObjectID và chỉ số bộ đếm tức thì của một máy in trong mạng LAN mà
          không lưu trữ thông tin kết nối.
        </p>
      </div>

      {/* Form Section */}
      <div className="glass-panel p-6">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (probeAddress.trim() && probeCommunity) discovery.mutate()
          }}
          className="space-y-4 max-w-xl"
        >
          <div>
            <label htmlFor="probe-address" className="block text-xs font-bold text-slate-300 mb-1.5">
              Địa chỉ IP máy in
            </label>
            <input
              id="probe-address"
              value={probeAddress}
              onChange={(e) => setProbeAddress(e.target.value)}
              placeholder="vd: 192.168.1.50"
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
              required
            />
          </div>

          <div>
            <label htmlFor="probe-community" className="block text-xs font-bold text-slate-300 mb-1.5">
              SNMP v2c Community String
            </label>
            <input
              id="probe-community"
              type="password"
              value={probeCommunity}
              onChange={(e) => setProbeCommunity(e.target.value)}
              placeholder="vd: public"
              autoComplete="new-password"
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
              required
            />
          </div>

          <div className="flex items-center justify-between pt-2">
            <span className="text-xs text-slate-400 flex items-center gap-1">
              <Shield size={14} className="text-emerald-400" /> Tiến trình thử nghiệm không lưu credential.
            </span>
            <button
              type="submit"
              disabled={discovery.isPending}
              className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-6 py-2.5 rounded-xl text-sm transition-all shadow-lg shadow-cyan-950/50 cursor-pointer flex items-center gap-2"
            >
              <MagnifyingGlass size={16} className={discovery.isPending ? 'animate-spin' : ''} />
              <span>{discovery.isPending ? 'Đang đọc SNMP...' : 'Đọc thông tin'}</span>
            </button>
          </div>
        </form>

        {/* Discovery Results Display */}
        {discovery.data && (
          <div className="mt-8 pt-6 border-t border-slate-800 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-100 flex items-center gap-2">
                <CheckCircle size={20} className="text-emerald-400" /> Kết quả phản hồi SNMP
              </h3>
              <button
                onClick={handleRegister}
                className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold px-4 py-2 rounded-xl text-xs sm:text-sm transition-all flex items-center gap-1.5 cursor-pointer shadow-lg shadow-emerald-950/50"
              >
                <span>Chuyển sang Đăng ký máy in này</span>
                <ArrowRight size={14} />
              </button>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                <dt className="text-[11px] font-bold uppercase text-slate-400">Địa chỉ IP</dt>
                <dd className="text-sm font-semibold text-slate-200 mt-1">{discovery.data.address}</dd>
              </div>

              <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                <dt className="text-[11px] font-bold uppercase text-slate-400">Tên hệ thống (sysName)</dt>
                <dd className="text-sm font-semibold text-slate-200 mt-1">{discovery.data.name || '—'}</dd>
              </div>

              <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                <dt className="text-[11px] font-bold uppercase text-slate-400">Mô tả thiết bị (sysDescr)</dt>
                <dd className="text-xs text-slate-300 mt-1 line-clamp-2">{discovery.data.description || '—'}</dd>
              </div>

              <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                <dt className="text-[11px] font-bold uppercase text-slate-400">System Object ID (sysObjectID)</dt>
                <dd className="text-xs font-mono text-cyan-400 mt-1">{discovery.data.sys_object_id || '—'}</dd>
              </div>

              <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                <dt className="text-[11px] font-bold uppercase text-slate-400">Số sê-ri</dt>
                <dd className="text-sm font-semibold text-slate-200 mt-1">{discovery.data.serial || '—'}</dd>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
