import {
  ArrowClockwise,
  CheckCircle,
  Clock,
  Cpu,
  Printer as PrinterIcon,
  Warning,
  XCircle,
} from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { listJobs } from '../../api/client'
import type { JobItem } from '../../api/types'
import { DataTable } from '../../components/DataTable'

function formatDuration(ms: number): string {
  if (!ms || ms <= 0) return '0ms'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const mins = Math.floor(ms / 60000)
  const secs = Math.round((ms % 60000) / 1000)
  return `${mins}m ${secs}s`
}

function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const date = new Date(iso)
  return date.toLocaleString('vi-VN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  })
}

const kindLabels: Record<string, { label: string; color: string }> = {
  status: { label: 'Đồng bộ trạng thái', color: 'bg-sky-50 text-sky-800 border-sky-200' },
  counter: { label: 'Đọc số trang in', color: 'bg-emerald-50 text-emerald-800 border-emerald-200' },
  discovery: { label: 'Dò quét mạng', color: 'bg-purple-50 text-purple-800 border-purple-200' },
}

export function JobsPage() {
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [autoRefresh, setAutoRefresh] = useState<boolean>(true)

  const { data, isLoading, isRefetching, refetch } = useQuery({
    queryKey: ['jobs', statusFilter],
    queryFn: () => listJobs(statusFilter, 500, 0),
    refetchInterval: autoRefresh ? 4000 : false,
  })

  const jobs = data?.items ?? []

  // Metrics summary
  const metrics = useMemo(() => {
    let running = 0
    let failed = 0
    let success = 0
    let queued = 0
    for (const j of jobs) {
      if (j.status === 'running') running++
      else if (j.status === 'failed') failed++
      else if (j.status === 'success') success++
      else if (j.status === 'queued') queued++
    }
    return { total: jobs.length, running, failed, success, queued }
  }, [jobs])

  const columns = useMemo<ColumnDef<JobItem>[]>(
    () => [
      {
        accessorKey: 'id',
        header: 'Mã Job',
        cell: ({ row }) => (
          <span
            className="font-mono text-xs px-2 py-1 rounded bg-slate-100 border border-slate-200 text-slate-700"
            title={row.original.id}
          >
            {row.original.id.slice(0, 8)}…
          </span>
        ),
      },
      {
        accessorKey: 'device_name',
        header: 'Máy in / Địa chỉ',
        cell: ({ row }) => {
          const item = row.original
          return (
            <div className="flex flex-col">
              {item.device_id ? (
                <Link
                  to={`/printers/${item.device_id}`}
                  className="font-medium text-cyan-700 hover:text-cyan-800 hover:underline flex items-center gap-1.5"
                >
                  <PrinterIcon size={14} />
                  {item.device_name || 'Thiết bị không tên'}
                </Link>
              ) : (
                <span className="font-medium text-slate-800">{item.device_name || 'Hệ thống'}</span>
              )}
              {item.device_ip && <span className="font-mono text-xs text-slate-500">{item.device_ip}</span>}
            </div>
          )
        },
      },
      {
        accessorKey: 'kind',
        header: 'Loại tiến trình',
        cell: ({ row }) => {
          const k = kindLabels[row.original.kind] || {
            label: row.original.kind,
            color: 'bg-slate-100 text-slate-700 border-slate-200',
          }
          return (
            <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${k.color}`}>
              {k.label}
            </span>
          )
        },
      },
      {
        accessorKey: 'status',
        header: 'Trạng thái',
        cell: ({ row }) => {
          const status = row.original.status
          if (status === 'running') {
            return (
              <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-sky-50 text-sky-800 border border-sky-200 animate-pulse">
                <span className="w-1.5 h-1.5 rounded-full bg-sky-600 animate-ping" />
                Đang chạy
              </span>
            )
          }
          if (status === 'failed') {
            return (
              <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-rose-50 text-rose-800 border border-rose-200">
                <XCircle size={14} weight="fill" />
                Lỗi / Ngừng
              </span>
            )
          }
          if (status === 'success') {
            return (
              <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-800 border border-emerald-200">
                <CheckCircle size={14} weight="fill" />
                Hoàn tất
              </span>
            )
          }
          return (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-50 text-amber-800 border border-amber-200">
              <Clock size={14} />
              Đang chờ
            </span>
          )
        },
      },
      {
        accessorKey: 'error_code',
        header: 'Chi tiết / Lỗi',
        cell: ({ row }) => {
          const err = row.original.error_code
          if (!err) return <span className="text-slate-400">—</span>
          return (
            <div className="flex items-center gap-1 text-rose-700 font-mono text-xs max-w-xs truncate" title={err}>
              <Warning size={14} className="shrink-0" />
              <span className="truncate">{err}</span>
            </div>
          )
        },
      },
      {
        accessorKey: 'duration_ms',
        header: 'Thời lượng',
        cell: ({ row }) => (
          <span className="font-mono text-xs text-slate-700">{formatDuration(row.original.duration_ms)}</span>
        ),
      },
      {
        accessorKey: 'created_at',
        header: 'Thời điểm tạo',
        cell: ({ row }) => <span className="text-xs text-slate-500">{formatDateTime(row.original.created_at)}</span>,
      },
    ],
    [],
  )

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 flex items-center gap-2">
            <Cpu size={28} className="text-cyan-600" />
            Theo dõi Tiến trình Polling (Jobs)
          </h1>
          <p className="text-sm text-slate-600 mt-1">
            Quản lý và giám sát các tiến trình đọc counter, thăm dò SNMP và phát hiện thiết bị.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2 text-xs font-medium text-slate-700 bg-white px-3 py-1.5 rounded-lg border border-slate-200 cursor-pointer select-none shadow-2xs">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              className="rounded border-slate-300 text-cyan-600 focus:ring-0"
            />
            Tự động làm mới (4s)
          </label>

          <button
            onClick={() => refetch()}
            disabled={isLoading || isRefetching}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-cyan-50 text-cyan-800 border border-cyan-200 hover:bg-cyan-100 transition-all disabled:opacity-50 shadow-2xs cursor-pointer"
          >
            <ArrowClockwise size={14} className={isRefetching ? 'animate-spin' : ''} />
            Làm mới
          </button>
        </div>
      </div>

      {/* Metrics overview */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div className="glass-panel p-4 rounded-xl border border-slate-200 flex flex-col shadow-2xs">
          <span className="text-xs font-medium text-slate-500">Tổng số Jobs</span>
          <span className="text-2xl font-bold text-slate-900 mt-1">{metrics.total}</span>
        </div>

        <div className="glass-panel p-4 rounded-xl border border-sky-200 bg-sky-50/70 flex flex-col shadow-2xs">
          <span className="text-xs font-medium text-sky-800 flex items-center gap-1.5">
            <span className="w-2 h-2 rounded-full bg-sky-600 animate-ping" />
            Đang chạy
          </span>
          <span className="text-2xl font-bold text-sky-800 mt-1">{metrics.running}</span>
        </div>

        <div className="glass-panel p-4 rounded-xl border border-rose-200 bg-rose-50/70 flex flex-col shadow-2xs">
          <span className="text-xs font-medium text-rose-800 flex items-center gap-1.5">
            <Warning size={14} />
            Lỗi / Bị gián đoạn
          </span>
          <span className="text-2xl font-bold text-rose-800 mt-1">{metrics.failed}</span>
        </div>

        <div className="glass-panel p-4 rounded-xl border border-emerald-200 bg-emerald-50/70 flex flex-col shadow-2xs">
          <span className="text-xs font-medium text-emerald-800 flex items-center gap-1.5">
            <CheckCircle size={14} />
            Thành công
          </span>
          <span className="text-2xl font-bold text-emerald-800 mt-1">{metrics.success}</span>
        </div>
      </div>

      {/* Main Table */}
      <div className="glass-panel rounded-xl border border-slate-200 p-6 shadow-2xs">
        <DataTable
          columns={columns}
          data={jobs}
          searchPlaceholder="Tìm kiếm mã job, tên máy in, địa chỉ IP, mã lỗi..."
          initialPageSize={15}
          filterSlot={
            <div className="flex items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0">
              {[
                { id: 'all', label: 'Tất cả' },
                { id: 'running', label: 'Đang chạy' },
                { id: 'failed', label: 'Lỗi / Ngừng' },
                { id: 'queued', label: 'Chờ chạy' },
                { id: 'success', label: 'Thành công' },
              ].map((f) => (
                <button
                  key={f.id}
                  onClick={() => setStatusFilter(f.id)}
                  className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer shadow-2xs ${
                    statusFilter === f.id
                      ? 'bg-cyan-50 text-cyan-800 border border-cyan-300 font-semibold'
                      : 'bg-white text-slate-600 border border-slate-200 hover:bg-slate-50 hover:text-slate-900'
                  }`}
                >
                  {f.label}
                </button>
              ))}
            </div>
          }
        />
      </div>
    </div>
  )
}
