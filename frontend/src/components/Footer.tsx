import { GitCommit } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { getVersion } from '../api/client'

export function Footer() {
  const clientVersion = import.meta.env.VITE_APP_VERSION || '1.0.0'
  const clientCommit = import.meta.env.VITE_APP_COMMIT || ''

  const { data: serverVersion } = useQuery({
    queryKey: ['system', 'version'],
    queryFn: getVersion,
    staleTime: 60_000,
  })

  const version = serverVersion?.version || clientVersion
  const commit = serverVersion?.git_commit || clientCommit

  return (
    <footer className="border-t border-slate-200 bg-white/80 py-4 px-4 sm:px-6 lg:px-8 text-xs text-slate-500 mt-auto">
      <div className="max-w-7xl mx-auto flex flex-col sm:flex-row items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-slate-700">NPMS</span>
          <span>— Nền tảng quản lý máy in mạng LAN</span>
        </div>
        <div className="flex items-center gap-2 font-mono text-[11px] text-slate-600">
          <span className="font-semibold">v{version}</span>
          {commit && (
            <span className="flex items-center gap-0.5 text-slate-500">
              <GitCommit size={12} />
              {commit.slice(0, 7)}
            </span>
          )}
          <span className="text-emerald-700 bg-emerald-50 border border-emerald-200 px-1.5 py-0.5 rounded text-[10px] font-sans">
            Đồng bộ
          </span>
        </div>
      </div>
    </footer>
  )
}
