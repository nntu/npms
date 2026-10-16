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
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono font-medium transition-all bg-slate-800/80 hover:bg-slate-700/80 border border-slate-700/60 text-slate-300 hover:text-cyan-300 cursor-pointer shadow-sm"
        title="Xem chi tiết phiên bản và bản build"
      >
        <Tag size={13} className="text-cyan-400" />
        <span>v{activeVersion}</span>
        {activeCommit ? (
          <span className="text-[10px] text-slate-400 opacity-80">({activeCommit.slice(0, 7)})</span>
        ) : null}
        <span
          className={`w-1.5 h-1.5 rounded-full ${
            isLoading ? 'bg-amber-400 animate-pulse' : isSynced ? 'bg-emerald-400' : 'bg-rose-400 animate-ping'
          }`}
        />
      </button>

      {isOpen && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-150"
          onClick={() => setIsOpen(false)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setIsOpen(false)
          }}
        >
          {/* Modal Content */}
          <div
            role="document"
            className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden p-6 text-slate-200"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between pb-4 border-b border-slate-800">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-cyan-950 border border-cyan-800/50 flex items-center justify-center text-cyan-400 font-bold">
                  <Tag size={18} />
                </div>
                <div>
                  <h3 className="text-base font-bold text-slate-100">Thông tin phiên bản & Bản build</h3>
                  <p className="text-xs text-slate-400">Đồng bộ giữa Backend và Giao diện Web</p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                className="text-slate-400 hover:text-slate-200 p-1.5 rounded-lg hover:bg-slate-800 transition-colors"
              >
                <X size={18} />
              </button>
            </div>

            <div className="space-y-4 py-4 text-xs">
              {/* Synchronization Status */}
              <div
                className={`p-3 rounded-xl border flex items-start gap-3 ${
                  isSynced
                    ? 'bg-emerald-950/30 border-emerald-800/50 text-emerald-300'
                    : 'bg-rose-950/30 border-rose-800/50 text-rose-300'
                }`}
              >
                {isSynced ? (
                  <CheckCircle size={20} className="text-emerald-400 shrink-0 mt-0.5" weight="fill" />
                ) : (
                  <WarningCircle size={20} className="text-rose-400 shrink-0 mt-0.5" weight="fill" />
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
              <div className="bg-slate-950/60 rounded-xl p-3.5 border border-slate-800/80 space-y-2.5 font-mono">
                <div className="flex justify-between items-center">
                  <span className="text-slate-400 font-sans">Phiên bản (Version):</span>
                  <span className="font-bold text-cyan-400 bg-cyan-950/60 px-2 py-0.5 rounded border border-cyan-800/40">
                    v{activeVersion}
                  </span>
                </div>

                <div className="flex justify-between items-center">
                  <span className="text-slate-400 font-sans flex items-center gap-1.5">
                    <GitCommit size={14} /> Git Commit:
                  </span>
                  <span className="text-slate-200">{activeCommit ? activeCommit.slice(0, 7) : '—'}</span>
                </div>

                <div className="flex justify-between items-center">
                  <span className="text-slate-400 font-sans">Ngày biên dịch (Build Date):</span>
                  <span className="text-slate-300 text-[11px]">
                    {serverVersion?.build_date || import.meta.env.VITE_APP_BUILD_TIME || '—'}
                  </span>
                </div>

                <div className="flex justify-between items-center pt-2 border-t border-slate-800/60 font-sans">
                  <span className="text-slate-400">Bản Web (Client):</span>
                  <span className="text-slate-300 font-mono text-[11px]">v{clientVersion}</span>
                </div>

                <div className="flex justify-between items-center font-sans">
                  <span className="text-slate-400">Bản Backend (API):</span>
                  <span className="text-slate-300 font-mono text-[11px]">
                    {isLoading ? 'Đang kiểm tra…' : `v${serverVersion?.version || 'Không xác định'}`}
                  </span>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-between pt-3 border-t border-slate-800 text-xs">
              <button
                type="button"
                onClick={() => refetch()}
                disabled={isFetching}
                className="flex items-center gap-1.5 text-slate-400 hover:text-cyan-400 px-3 py-1.5 rounded-lg hover:bg-slate-800 transition-colors disabled:opacity-50"
              >
                <ArrowsClockwise size={14} className={isFetching ? 'animate-spin' : ''} />
                <span>Kiểm tra lại</span>
              </button>
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                className="bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium px-4 py-1.5 rounded-lg transition-colors"
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
