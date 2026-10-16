import { ArrowsClockwise, CheckCircle, GitCommit, Tag, WarningCircle, X } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { getVersion } from '../api/client'

export function VersionBadge() {
  const [isOpen, setIsOpen] = useState(false)

  const clientVersion = import.meta.env.VITE_APP_VERSION || '1.0.0'
  const clientCommit = import.meta.env.VITE_APP_COMMIT || ''

  const {
    data: serverVersion,
    isLoading,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ['system', 'version'],
    queryFn: getVersion,
    staleTime: 60_000,
  })

  const activeVersion = serverVersion?.version || clientVersion
  const activeCommit = serverVersion?.git_commit || clientCommit
  const isSynced = !serverVersion || serverVersion.version === clientVersion

  return (
    <>
      <button
        type="button"
        onClick={() => setIsOpen(true)}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono font-medium transition-all bg-slate-100 hover:bg-slate-200 border border-slate-200 text-slate-700 hover:text-cyan-800 cursor-pointer shadow-xs"
        title="Xem chi tiết phiên bản và bản build"
      >
        <Tag size={13} className="text-cyan-600" />
        <span>v{activeVersion}</span>
        {activeCommit ? (
          <span className="text-[10px] text-slate-500 opacity-90">({activeCommit.slice(0, 7)})</span>
        ) : null}
        <span
          className={`w-1.5 h-1.5 rounded-full ${
            isLoading ? 'bg-amber-400 animate-pulse' : isSynced ? 'bg-emerald-500' : 'bg-rose-500 animate-ping'
          }`}
        />
      </button>

      {isOpen && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-xs animate-in fade-in duration-150"
          onClick={() => setIsOpen(false)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setIsOpen(false)
          }}
        >
          {/* Modal Content */}
          <div
            role="document"
            className="w-full max-w-md bg-white border border-slate-200 rounded-2xl shadow-xl overflow-hidden p-6 text-slate-800"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between pb-4 border-b border-slate-200">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-cyan-50 border border-cyan-200 flex items-center justify-center text-cyan-600 font-bold">
                  <Tag size={18} />
                </div>
                <div>
                  <h3 className="text-base font-bold text-slate-900">Thông tin phiên bản & Bản build</h3>
                  <p className="text-xs text-slate-500">Đồng bộ giữa Backend và Giao diện Web</p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                className="text-slate-400 hover:text-slate-600 p-1.5 rounded-lg hover:bg-slate-100 transition-colors"
              >
                <X size={18} />
              </button>
            </div>

            <div className="space-y-4 py-4 text-xs">
              {/* Synchronization Status */}
              <div
                className={`p-3 rounded-xl border flex items-start gap-3 ${
                  isSynced
                    ? 'bg-emerald-50/80 border-emerald-200 text-emerald-800'
                    : 'bg-rose-50/80 border-rose-200 text-rose-800'
                }`}
              >
                {isSynced ? (
                  <CheckCircle size={20} className="text-emerald-600 shrink-0 mt-0.5" weight="fill" />
                ) : (
                  <WarningCircle size={20} className="text-rose-600 shrink-0 mt-0.5" weight="fill" />
                )}
                <div>
                  <div className="font-semibold text-sm">
                    {isSynced ? 'Hệ thống đã đồng bộ' : 'Cảnh báo không đồng bộ phiên bản'}
                  </div>
                  <div className="text-[11px] opacity-90 mt-0.5">
                    {isSynced
                      ? 'Giao diện Web và Backend API đang chạy trên cùng phiên bản build.'
                      : 'Phiên bản Web không khớp với Backend API. Vui lòng tải lại trang (Ctrl + F5).'}
                  </div>
                </div>
              </div>

              {/* Version Details Grid */}
              <div className="bg-slate-50 rounded-xl p-3.5 border border-slate-200 space-y-2.5 font-mono">
                <div className="flex justify-between items-center">
                  <span className="text-slate-600 font-sans">Phiên bản (Version):</span>
                  <span className="font-bold text-cyan-800 bg-cyan-50 px-2 py-0.5 rounded border border-cyan-200">
                    v{activeVersion}
                  </span>
                </div>

                <div className="flex justify-between items-center">
                  <span className="text-slate-600 font-sans flex items-center gap-1.5">
                    <GitCommit size={14} /> Git Commit:
                  </span>
                  <span className="text-slate-800 font-semibold">{activeCommit ? activeCommit.slice(0, 7) : '—'}</span>
                </div>

                <div className="flex justify-between items-center">
                  <span className="text-slate-600 font-sans">Ngày biên dịch (Build Date):</span>
                  <span className="text-slate-700 text-[11px]">
                    {serverVersion?.build_date || import.meta.env.VITE_APP_BUILD_TIME || '—'}
                  </span>
                </div>

                <div className="flex justify-between items-center pt-2 border-t border-slate-200 font-sans">
                  <span className="text-slate-600">Bản Web (Client):</span>
                  <span className="text-slate-800 font-mono text-[11px]">v{clientVersion}</span>
                </div>

                <div className="flex justify-between items-center font-sans">
                  <span className="text-slate-600">Bản Backend (API):</span>
                  <span className="text-slate-800 font-mono text-[11px]">
                    {isLoading ? 'Đang kiểm tra…' : `v${serverVersion?.version || 'Không xác định'}`}
                  </span>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-between pt-3 border-t border-slate-200 text-xs">
              <button
                type="button"
                onClick={() => refetch()}
                disabled={isFetching}
                className="flex items-center gap-1.5 text-slate-600 hover:text-cyan-700 px-3 py-1.5 rounded-lg hover:bg-slate-100 transition-colors disabled:opacity-50 cursor-pointer"
              >
                <ArrowsClockwise size={14} className={isFetching ? 'animate-spin' : ''} />
                <span>Kiểm tra lại</span>
              </button>
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                className="bg-slate-100 hover:bg-slate-200 text-slate-700 font-medium px-4 py-1.5 rounded-lg transition-colors cursor-pointer"
              >
                Đóng
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
