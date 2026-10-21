import {
  ArrowClockwise,
  ArrowLeft,
  CheckCircle,
  Clock,
  Drop,
  MagnifyingGlass,
  PencilSimple,
  Printer as PrinterIcon,
  Trash,
  Warning,
  X,
} from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  deletePrinter,
  listCartridges,
  probeDiscovery,
  replacePrinterCartridge,
  startPrinterPoll,
  updatePrinter,
} from '../../api/client'
import type { CounterReading, DailyUsage, DiscoveryResult, PrinterStatus } from '../../api/types'
import { DataTable } from '../../components/DataTable'
import { showConfirm, showError, showSuccess } from '../../utils/sweetalert'
import { useDepartments } from '../departments/useDepartments'
import { CounterChart } from './CounterChart'
import { DailyUsageChart } from './DailyUsageChart'
import { usePollingRuns } from './usePollingRuns'
import { usePollJob } from './usePollJob'
import { usePrinter } from './usePrinter'
import { usePrinterCounters } from './usePrinterCounters'
import { usePrinterUsage } from './usePrinterUsage'
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
  const profiles = useProfiles()
  const departmentsQuery = useDepartments()
  const departmentsList = useMemo(() => departmentsQuery.data?.data ?? [], [departmentsQuery.data?.data])
  const monthlyUsage = useMemo(() => aggregateMonthlyUsage(usage.data?.data ?? []), [usage.data?.data])
  const _pollingRuns = usePollingRuns(printerId)
  const queryClient = useQueryClient()

  const counterColumns = useMemo<ColumnDef<CounterReading>[]>(
    () => [
      {
        id: 'definition_key',
        accessorKey: 'definition_key',
        header: 'Bộ đếm',
        cell: ({ row }) => <span className="font-semibold text-slate-800">{row.original.definition_key}</span>,
      },
      {
        id: 'raw_value',
        accessorKey: 'raw_value',
        header: 'Giá trị gốc',
        cell: ({ row }) => (
          <span className="font-bold text-cyan-700 font-mono">{row.original.raw_value.toLocaleString()}</span>
        ),
      },
      {
        id: 'unit',
        accessorKey: 'unit',
        header: 'Đơn vị',
        cell: ({ row }) => <span className="text-slate-600">{row.original.unit}</span>,
      },
      {
        id: 'quality',
        accessorKey: 'quality',
        header: 'Chất lượng',
        cell: ({ row }) => (
          <span className={`quality-tag quality-${row.original.quality}`}>{row.original.quality}</span>
        ),
      },
      {
        id: 'collected_at',
        accessorKey: 'collected_at',
        header: 'Thời gian thu thập',
        cell: ({ row }) => (
          <span className="text-slate-500">{new Date(row.original.collected_at).toLocaleString('vi-VN')}</span>
        ),
      },
    ],
    [],
  )

  const monthlyUsageColumns = useMemo<ColumnDef<MonthlyUsage>[]>(
    () => [
      {
        id: 'month',
        accessorKey: 'month',
        header: 'Tháng',
        cell: ({ row }) => <span className="font-bold text-slate-800">{row.original.month}</span>,
      },
      {
        id: 'definition_key',
        accessorKey: 'definition_key',
        header: 'Bộ đếm',
        cell: ({ row }) => <span className="text-slate-600">{row.original.definition_key}</span>,
      },
      {
        id: 'delta',
        accessorKey: 'delta',
        header: 'Lưu lượng in',
        cell: ({ row }) => (
          <span className="font-bold text-emerald-700 font-mono">
            {row.original.delta.toLocaleString()} {row.original.unit}
          </span>
        ),
      },
      {
        id: 'quality',
        accessorKey: 'quality',
        header: 'Trạng thái dữ liệu',
        cell: ({ row }) => (
          <span className={`quality-tag quality-${row.original.quality}`}>{row.original.quality}</span>
        ),
      },
    ],
    [],
  )

  const [jobID, setJobID] = useState<string | null>(null)
  const job = usePollJob(jobID)

  const cartridgesQuery = useQuery({ queryKey: ['cartridges'], queryFn: () => listCartridges() })
  const [showReplaceModal, setShowReplaceModal] = useState(false)
  const [selectedCartridgeId, setSelectedCartridgeId] = useState('')
  const [sourceType, setSourceType] = useState<'new' | 'refilled'>('new')
  const [replaceNotes, setReplaceNotes] = useState('')

  // Edit Modal State
  const [showEditModal, setShowEditModal] = useState(false)
  const [editDisplayName, setEditDisplayName] = useState('')
  const [editAssetCode, setEditAssetCode] = useState('')
  const [editDepartmentId, setEditDepartmentId] = useState('')
  const [editDepartment, setEditDepartment] = useState('')
  const [editAddress, setEditAddress] = useState('')
  const [editPort, setEditPort] = useState('161')
  const [editCommunity, setEditCommunity] = useState('')
  const [editManufacturer, setEditManufacturer] = useState('')
  const [editModel, setEditModel] = useState('')
  const [editSerial, setEditSerial] = useState('')
  const [editSysObjectId, setEditSysObjectId] = useState('')
  const [editProfileId, setEditProfileId] = useState('')
  const [editProbePreview, setEditProbePreview] = useState<DiscoveryResult | null>(null)

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

  const probeEditMutation = useMutation({
    mutationFn: () =>
      probeDiscovery({
        address: editAddress.trim(),
        port: Number(editPort) || 161,
        version: '2c',
        community: editCommunity || 'public',
      }),
    onSuccess: (res) => {
      setEditProbePreview(res)
      if (res.manufacturer) setEditManufacturer(res.manufacturer)
      if (res.model) setEditModel(res.model)
      if (res.serial) setEditSerial(res.serial)
      if (res.sys_object_id) setEditSysObjectId(res.sys_object_id)
      if (res.matched_profile_id) setEditProfileId(res.matched_profile_id)
      void showSuccess(
        'Nhận diện SNMP thành công!',
        `Phát hiện: ${[res.manufacturer, res.model].filter(Boolean).join(' ') || res.name || res.address} → Profile: ${res.matched_profile_id || 'generic-printer-mib'}`,
      )
    },
    onError: (err) => {
      void showError(
        'Không đọc được SNMP từ máy in',
        err instanceof Error ? err.message : 'Kiểm tra lại địa chỉ IP và SNMP Community.',
      )
    },
  })

  const updateMut = useMutation({
    mutationFn: () =>
      updatePrinter(printerId, {
        display_name: editDisplayName.trim(),
        asset_code: editAssetCode.trim(),
        department_id: editDepartmentId || undefined,
        department: editDepartment.trim(),
        manufacturer: editManufacturer.trim(),
        model: editModel.trim(),
        serial: editSerial.trim(),
        sys_object_id: editSysObjectId.trim(),
        profile_id: editProfileId || undefined,
        address: editAddress.trim() || undefined,
        port: Number(editPort) || 161,
        version: '2c',
        community: editCommunity.trim() || undefined,
      }),
    onSuccess: (updated) => {
      setShowEditModal(false)
      setEditProbePreview(null)
      void queryClient.invalidateQueries({ queryKey: ['printer', printerId] })
      void queryClient.invalidateQueries({ queryKey: ['printers'] })
      void showSuccess('Đã cập nhật máy in!', `Thông tin máy in "${updated.display_name}" đã được lưu.`)
    },
    onError: (err) => {
      void showError('Không thể cập nhật máy in', err instanceof Error ? err.message : 'Kiểm tra lại dữ liệu nhập.')
    },
  })

  const deleteMut = useMutation({
    mutationFn: () => deletePrinter(printerId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['printers'] })
      void showSuccess('Đã xóa máy in!', 'Máy in đã được gỡ khỏi danh sách hoạt động.')
      navigate('/printers')
    },
    onError: (err) => {
      void showError('Không thể xóa máy in', err instanceof Error ? err.message : 'Đã xảy ra lỗi khi xóa máy in.')
    },
  })

  useEffect(() => {
    if (job.data?.status === 'success') {
      void queryClient.invalidateQueries({ queryKey: ['printer', printerId] })
      void queryClient.invalidateQueries({ queryKey: ['printer-counters', printerId] })
      void queryClient.invalidateQueries({ queryKey: ['printer-usage', printerId] })
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
      <div className="glass-panel p-10 text-center my-8 border-rose-200">
        <Warning size={48} className="text-rose-600 mx-auto mb-3" />
        <h3 className="text-xl font-bold text-slate-900 mb-2">Không thể tải thông tin máy in</h3>
        <p className="text-slate-600 text-sm max-w-md mx-auto mb-6">
          {query.error instanceof Error ? query.error.message : 'Thiết bị không tồn tại hoặc đã bị xóa khỏi hệ thống.'}
        </p>
        <button
          type="button"
          onClick={() => navigate('/printers')}
          className="bg-white hover:bg-slate-50 text-slate-700 font-semibold px-5 py-2.5 rounded-xl text-sm border border-slate-200 transition-all cursor-pointer shadow-2xs"
        >
          Trở về danh sách
        </button>
      </div>
    )
  }

  const printer = query.data
  const availableProfiles = profiles.data?.data ?? []

  const openEditModal = () => {
    setEditDisplayName(printer.display_name)
    setEditAssetCode(printer.asset_code ?? '')
    setEditDepartmentId((printer as unknown as { department_id?: string }).department_id ?? '')
    setEditDepartment(printer.department ?? '')
    setEditAddress(printer.address ?? '')
    setEditPort(String(printer.port ?? 161))
    setEditCommunity('')
    setEditManufacturer(printer.manufacturer ?? '')
    setEditModel(printer.model ?? '')
    setEditSerial(printer.serial ?? '')
    setEditSysObjectId(printer.sys_object_id ?? '')
    setEditProfileId(printer.profile_id ?? '')
    setEditProbePreview(null)
    setShowEditModal(true)
  }

  const handleDelete = async () => {
    const confirmed = await showConfirm(
      `Xóa máy in "${printer.display_name}"?`,
      'Máy in sẽ được ẩn khỏi danh sách quản lý. Lịch sử trang in và nhật ký thay mực trước đó vẫn được lưu trữ an toàn.',
    )
    if (confirmed.isConfirmed) {
      deleteMut.mutate()
    }
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="glass-panel p-6 sm:p-8">
        <button
          type="button"
          onClick={() => navigate('/printers')}
          className="inline-flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-cyan-700 hover:text-cyan-800 transition-colors mb-4 cursor-pointer"
        >
          <ArrowLeft size={16} weight="bold" /> Trở về danh sách máy in
        </button>

        <div className="flex flex-wrap items-center justify-between gap-6 pb-6 border-b border-slate-200">
          <div>
            <div className="flex items-center gap-3 mb-1">
              <h1 className="text-2xl sm:text-3xl font-extrabold text-slate-900">{printer.display_name}</h1>
              <StatusBadge status={printer.status} />
            </div>
            <p className="text-slate-600 text-sm">
              Mã tài sản: <strong className="text-slate-800">{printer.asset_code ?? 'Chưa gán'}</strong>
              {printer.address ? (
                <span className="ml-3">
                  · Địa chỉ IP:{' '}
                  <strong className="text-cyan-700 font-mono">
                    {printer.address}
                    {printer.port && printer.port !== 161 ? `:${printer.port}` : ''}
                  </strong>
                </span>
              ) : null}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2.5">
            <button
              type="button"
              onClick={openEditModal}
              className="bg-white hover:bg-slate-50 text-slate-700 font-semibold px-3.5 py-2.5 rounded-xl text-sm border border-slate-200 transition-all flex items-center gap-2 cursor-pointer shadow-2xs"
            >
              <PencilSimple size={18} weight="bold" />
              <span>Sửa</span>
            </button>
            <button
              type="button"
              onClick={() => void handleDelete()}
              disabled={deleteMut.isPending}
              className="bg-rose-50 hover:bg-rose-100 text-rose-700 font-semibold px-3.5 py-2.5 rounded-xl text-sm border border-rose-200 transition-all flex items-center gap-2 cursor-pointer"
            >
              <Trash size={18} weight="bold" />
              <span>Xóa</span>
            </button>
            <button
              type="button"
              onClick={() => setShowReplaceModal(true)}
              className="bg-amber-600 hover:bg-amber-500 text-white font-semibold px-4 py-2.5 rounded-xl text-sm transition-all flex items-center gap-2 shadow-xs cursor-pointer"
            >
              <Drop size={18} weight="bold" />
              <span>Thay mực máy in</span>
            </button>
            <button
              type="button"
              disabled={poll.isPending || job.data?.status === 'queued'}
              onClick={handleManualPoll}
              className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all flex items-center gap-2 shadow-xs cursor-pointer"
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
                ? 'bg-cyan-50 text-cyan-800 border-cyan-200'
                : job.data.status === 'success'
                  ? 'bg-emerald-50 text-emerald-800 border-emerald-200'
                  : 'bg-rose-50 text-rose-800 border-rose-200'
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
          <div className="bg-slate-50 p-4 rounded-xl border border-slate-200">
            <dt className="text-[11px] uppercase font-bold text-slate-500">Phòng ban</dt>
            <dd className="text-sm font-semibold text-cyan-700 mt-1">{printer.department ?? '—'}</dd>
          </div>
          <div className="bg-slate-50 p-4 rounded-xl border border-slate-200">
            <dt className="text-[11px] uppercase font-bold text-slate-500">Nhà sản xuất</dt>
            <dd className="text-sm font-semibold text-slate-800 mt-1">{printer.manufacturer ?? '—'}</dd>
          </div>
          <div className="bg-slate-50 p-4 rounded-xl border border-slate-200">
            <dt className="text-[11px] uppercase font-bold text-slate-500">Dòng máy (Model)</dt>
            <dd className="text-sm font-semibold text-slate-800 mt-1">{printer.model ?? '—'}</dd>
          </div>
          <div className="bg-slate-50 p-4 rounded-xl border border-slate-200">
            <dt className="text-[11px] uppercase font-bold text-slate-500">Số sê-ri (Serial)</dt>
            <dd className="text-sm font-semibold text-slate-800 mt-1">{printer.serial ?? '—'}</dd>
          </div>
          <div className="bg-slate-50 p-4 rounded-xl border border-slate-200">
            <dt className="text-[11px] uppercase font-bold text-slate-500">Profile SNMP</dt>
            <dd className="text-xs font-mono font-semibold text-cyan-700 mt-1 truncate">
              {printer.profile_id ?? 'generic-printer-mib'}
            </dd>
          </div>
          <div className="bg-slate-50 p-4 rounded-xl border border-slate-200">
            <dt className="text-[11px] uppercase font-bold text-slate-500">Lần cuối thấy</dt>
            <dd className="text-xs font-semibold text-slate-800 mt-1">
              {printer.last_seen_at ? new Date(printer.last_seen_at).toLocaleString('vi-VN') : 'Chưa thu thập'}
            </dd>
          </div>
        </div>

        {/* Counter Highlight Banner */}
        {(() => {
          const latestReading = counters.data?.data?.[0]
          return (
            <div className="mt-6 p-4 sm:p-5 rounded-2xl bg-gradient-to-r from-cyan-50 via-white to-slate-50 border border-cyan-200 flex flex-wrap items-center justify-between gap-4 shadow-xs">
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 rounded-xl bg-cyan-100 border border-cyan-200 flex items-center justify-center text-cyan-700 shrink-0">
                  <PrinterIcon size={24} weight="bold" />
                </div>
                <div>
                  <span className="text-xs uppercase font-bold tracking-wider text-cyan-700">
                    Tổng số trang in (Bộ đếm hiện tại)
                  </span>
                  <div className="flex items-baseline gap-2 mt-0.5">
                    <span className="text-2xl sm:text-3xl font-extrabold text-slate-900">
                      {latestReading ? latestReading.raw_value.toLocaleString('vi-VN') : '—'}
                    </span>
                    <span className="text-xs font-semibold text-slate-500">
                      {latestReading
                        ? latestReading.unit === 'impressions'
                          ? 'trang (impressions)'
                          : 'tờ (sheets)'
                        : 'Chưa có bản ghi số trang'}
                    </span>
                  </div>
                </div>
              </div>
              {latestReading ? (
                <div className="text-left sm:text-right">
                  <span className="text-[11px] text-slate-500 block">
                    Thu thập lúc: {new Date(latestReading.collected_at).toLocaleString('vi-VN')}
                  </span>
                  <span
                    className={`inline-block mt-1 text-[11px] font-semibold px-2.5 py-0.5 rounded-full ${
                      latestReading.quality === 'valid'
                        ? 'bg-emerald-50 text-emerald-800 border border-emerald-200'
                        : 'bg-amber-50 text-amber-800 border border-amber-200'
                    }`}
                  >
                    {latestReading.quality === 'valid' ? 'Dữ liệu chuẩn' : 'Chưa kiểm chứng'}
                  </span>
                </div>
              ) : (
                <div className="text-xs text-slate-500">
                  Bấm nút <strong className="text-cyan-700">"Thu thập ngay"</strong> ở góc trên để quét chỉ số bộ đếm
                  tức thì từ máy in qua SNMP.
                </div>
              )}
            </div>
          )
        })()}
      </div>

      {/* Recharts Charts Section */}
      {!counters.isLoading && !counters.isError && (counters.data?.data.length ?? 0) > 0 && (
        <CounterChart readings={counters.data?.data ?? []} />
      )}

      {!usage.isLoading && !usage.isError && (usage.data?.data.length ?? 0) > 0 && (
        <DailyUsageChart usage={usage.data?.data ?? []} />
      )}

      {/* Raw Counter Readings Table */}
      <div className="glass-panel p-6 sm:p-8 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-1">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-700">Dữ liệu gốc bất biến</p>
            <h3 className="text-lg font-bold text-slate-900">Lịch sử bộ đếm (Counter Readings)</h3>
          </div>
          <span className="text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200 px-3 py-1 rounded-full">
            {counters.data?.total ?? 0} bản ghi
          </span>
        </div>

        {counters.isLoading ? (
          <div className="p-8 text-center text-slate-500 text-sm">Đang tải lịch sử chỉ số bộ đếm...</div>
        ) : (
          <DataTable
            columns={counterColumns}
            data={counters.data?.data ?? []}
            searchPlaceholder="Tìm kiếm theo bộ đếm, đơn vị, chất lượng..."
            initialPageSize={10}
            emptyMessage="Chưa có chỉ số bộ đếm nào được ghi nhận"
            emptySubMessage="Bấm nút 'Thu thập tức thì' ở trên để tiến hành đọc số trang từ máy in qua SNMP."
          />
        )}
      </div>

      {/* Monthly Usage Aggregation */}
      <div className="glass-panel p-6 sm:p-8 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-1">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-emerald-700">Tổng hợp theo tháng</p>
            <h3 className="text-lg font-bold text-slate-900">Sản lượng in hàng tháng</h3>
          </div>
          <span className="text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200 px-3 py-1 rounded-full">
            {monthlyUsage.length} tháng
          </span>
        </div>

        <DataTable
          columns={monthlyUsageColumns}
          data={monthlyUsage}
          searchPlaceholder="Tìm kiếm theo tháng (YYYY-MM), bộ đếm..."
          initialPageSize={10}
          emptyMessage="Chưa có dữ liệu sản lượng in theo tháng"
          emptySubMessage="Dữ liệu tổng hợp theo tháng sẽ tự động hiển thị sau khi ghi nhận các chu kỳ đọc bộ đếm."
        />
      </div>

      {/* Modal Edit Printer */}
      {showEditModal && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs z-50 flex items-center justify-center p-4 overflow-y-auto">
          <div className="glass-panel p-6 w-full max-w-3xl space-y-5 border-slate-200 shadow-xl my-8">
            <div className="flex items-center justify-between pb-3 border-b border-slate-200">
              <div>
                <h3 className="text-lg font-bold text-slate-900 flex items-center gap-2">
                  <PencilSimple size={20} className="text-cyan-600" /> Chỉnh sửa máy in: {printer.display_name}
                </h3>
                <p className="text-xs text-slate-500 mt-0.5">
                  Cập nhật thông tin định danh, địa chỉ IP, cấu hình SNMP và Profile thu thập bộ đếm.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setShowEditModal(false)}
                className="p-1.5 rounded-lg text-slate-500 hover:text-slate-800 hover:bg-slate-100 cursor-pointer"
              >
                <X size={18} />
              </button>
            </div>

            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (editDisplayName.trim()) updateMut.mutate()
              }}
              className="space-y-4"
            >
              <div className="grid grid-cols-1 sm:grid-cols-12 gap-3 items-end bg-slate-50 p-4 rounded-xl border border-slate-200">
                <div className="sm:col-span-4">
                  <label htmlFor="detail-edit-address" className="block text-xs font-bold text-slate-700 mb-1">
                    Địa chỉ IP
                  </label>
                  <input
                    id="detail-edit-address"
                    value={editAddress}
                    onChange={(e) => setEditAddress(e.target.value)}
                    placeholder="vd: 192.168.1.50"
                    className="w-full bg-white border border-slate-200 rounded-xl px-3 py-2 text-sm text-slate-900 shadow-2xs"
                  />
                </div>
                <div className="sm:col-span-2">
                  <label htmlFor="detail-edit-port" className="block text-xs font-bold text-slate-700 mb-1">
                    Cổng SNMP
                  </label>
                  <input
                    id="detail-edit-port"
                    type="number"
                    min={1}
                    max={65535}
                    value={editPort}
                    onChange={(e) => setEditPort(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3 py-2 text-sm text-slate-900 shadow-2xs"
                  />
                </div>
                <div className="sm:col-span-3">
                  <label htmlFor="detail-edit-community" className="block text-xs font-bold text-slate-700 mb-1">
                    Community mới (nếu đổi)
                  </label>
                  <input
                    id="detail-edit-community"
                    type="text"
                    value={editCommunity}
                    onChange={(e) => setEditCommunity(e.target.value)}
                    placeholder="Để trống nếu giữ nguyên"
                    className="w-full bg-white border border-slate-200 rounded-xl px-3 py-2 text-sm text-slate-900 shadow-2xs"
                  />
                </div>
                <div className="sm:col-span-3">
                  <button
                    type="button"
                    disabled={probeEditMutation.isPending || !editAddress.trim()}
                    onClick={() => probeEditMutation.mutate()}
                    className="w-full bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold px-3 py-2 rounded-xl text-xs transition-all flex items-center justify-center gap-1.5 cursor-pointer shadow-xs"
                  >
                    <MagnifyingGlass size={15} className={probeEditMutation.isPending ? 'animate-spin' : ''} />
                    <span>{probeEditMutation.isPending ? 'Đang đọc...' : 'Kiểm tra SNMP'}</span>
                  </button>
                </div>
              </div>

              {editProbePreview && (
                <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-3 text-xs text-emerald-800 flex flex-wrap items-center justify-between gap-2">
                  <span>
                    ✔ Phản hồi SNMP: <strong>{editProbePreview.manufacturer}</strong>{' '}
                    <strong>{editProbePreview.model || editProbePreview.name}</strong> (SN:{' '}
                    {editProbePreview.serial || '—'})
                  </span>
                  <span className="font-mono bg-emerald-100 text-emerald-800 px-2 py-0.5 rounded border border-emerald-200">
                    Profile: {editProbePreview.matched_profile_id || 'generic-printer-mib'}
                  </span>
                </div>
              )}

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label htmlFor="detail-edit-display-name" className="block text-xs font-bold text-slate-700 mb-1">
                    Tên hiển thị <span className="text-rose-500">*</span>
                  </label>
                  <input
                    id="detail-edit-display-name"
                    value={editDisplayName}
                    onChange={(e) => setEditDisplayName(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                    required
                  />
                </div>
                <div>
                  <label htmlFor="detail-edit-asset-code" className="block text-xs font-bold text-slate-700 mb-1">
                    Mã tài sản
                  </label>
                  <input
                    id="detail-edit-asset-code"
                    value={editAssetCode}
                    onChange={(e) => setEditAssetCode(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                  />
                </div>
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label htmlFor="detail-edit-department" className="block text-xs font-bold text-slate-700">
                      Phòng ban
                    </label>
                    <Link
                      to="/departments"
                      target="_blank"
                      className="text-[11px] text-cyan-600 hover:text-cyan-700 hover:underline font-semibold"
                    >
                      + Quản lý phòng ban
                    </Link>
                  </div>
                  <select
                    id="detail-edit-department"
                    value={editDepartmentId || editDepartment}
                    onChange={(e) => {
                      const val = e.target.value
                      const found = departmentsList.find((d) => d.id === val || d.name === val)
                      if (found) {
                        setEditDepartmentId(found.id)
                        setEditDepartment(found.name)
                      } else {
                        setEditDepartmentId('')
                        setEditDepartment(val)
                      }
                    }}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs cursor-pointer"
                  >
                    <option value="">-- Chưa phân phòng --</option>
                    {departmentsList.map((d) => (
                      <option key={d.id} value={d.id}>
                        {d.name} ({d.code})
                      </option>
                    ))}
                    {editDepartment &&
                      !departmentsList.some((d) => d.id === editDepartmentId || d.name === editDepartment) && (
                        <option value={editDepartment}>{editDepartment} (Chưa chuẩn hóa)</option>
                      )}
                  </select>
                </div>
                <div>
                  <label htmlFor="detail-edit-profile" className="block text-xs font-bold text-slate-700 mb-1">
                    Profile SNMP
                  </label>
                  <select
                    id="detail-edit-profile"
                    value={editProfileId}
                    onChange={(e) => setEditProfileId(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                  >
                    <option value="">-- Tự động nhận diện theo dòng máy --</option>
                    {availableProfiles.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.id} ({p.manufacturer} · {p.verification_status})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label htmlFor="detail-edit-manufacturer" className="block text-xs font-bold text-slate-700 mb-1">
                    Nhà sản xuất
                  </label>
                  <input
                    id="detail-edit-manufacturer"
                    value={editManufacturer}
                    onChange={(e) => setEditManufacturer(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                  />
                </div>
                <div>
                  <label htmlFor="detail-edit-model" className="block text-xs font-bold text-slate-700 mb-1">
                    Dòng máy (Model)
                  </label>
                  <input
                    id="detail-edit-model"
                    value={editModel}
                    onChange={(e) => setEditModel(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                  />
                </div>
                <div>
                  <label htmlFor="detail-edit-serial" className="block text-xs font-bold text-slate-700 mb-1">
                    Số Serial
                  </label>
                  <input
                    id="detail-edit-serial"
                    value={editSerial}
                    onChange={(e) => setEditSerial(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                  />
                </div>
                <div>
                  <label htmlFor="detail-edit-sysoid" className="block text-xs font-bold text-slate-700 mb-1">
                    sysObjectID
                  </label>
                  <input
                    id="detail-edit-sysoid"
                    value={editSysObjectId}
                    onChange={(e) => setEditSysObjectId(e.target.value)}
                    className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm font-mono text-slate-900 shadow-2xs"
                  />
                </div>
              </div>

              <div className="flex justify-end gap-2 pt-3 border-t border-slate-200">
                <button
                  type="button"
                  onClick={() => setShowEditModal(false)}
                  className="bg-slate-100 hover:bg-slate-200 text-slate-700 font-semibold px-4 py-2 rounded-xl text-sm border border-slate-200 cursor-pointer"
                >
                  Hủy
                </button>
                <button
                  type="submit"
                  disabled={updateMut.isPending}
                  className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2 rounded-xl text-sm cursor-pointer shadow-xs"
                >
                  {updateMut.isPending ? 'Đang lưu...' : 'Lưu thay đổi'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Replace Toner */}
      {showReplaceModal && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-amber-200 shadow-xl">
            <h3 className="text-lg font-bold text-slate-900 flex items-center gap-2">
              <Drop size={20} className="text-amber-600" /> Thay mực cho máy in {printer.display_name}
            </h3>
            <div className="space-y-3">
              <div>
                <label htmlFor="cartridge-select" className="block text-xs font-bold text-slate-700 mb-1">
                  Chọn chủng loại mực
                </label>
                <select
                  id="cartridge-select"
                  value={selectedCartridgeId}
                  onChange={(e) => setSelectedCartridgeId(e.target.value)}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
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
                <span className="block text-xs font-bold text-slate-700 mb-1">Loại mực sử dụng</span>
                <div className="flex items-center gap-4 pt-1">
                  <label className="inline-flex items-center gap-2 cursor-pointer text-sm text-slate-700">
                    <input
                      type="radio"
                      name="sourceType"
                      value="new"
                      checked={sourceType === 'new'}
                      onChange={() => setSourceType('new')}
                      className="text-amber-600 focus:ring-amber-500"
                    />
                    <span>🟢 Bình MỚI 100%</span>
                  </label>
                  <label className="inline-flex items-center gap-2 cursor-pointer text-sm text-slate-700">
                    <input
                      type="radio"
                      name="sourceType"
                      value="refilled"
                      checked={sourceType === 'refilled'}
                      onChange={() => setSourceType('refilled')}
                      className="text-amber-600 focus:ring-amber-500"
                    />
                    <span>🔵 Bình ĐÃ BƠM</span>
                  </label>
                </div>
              </div>

              <div>
                <label htmlFor="replace-notes" className="block text-xs font-bold text-slate-700 mb-1">
                  Ghi chú thay mực
                </label>
                <input
                  id="replace-notes"
                  value={replaceNotes}
                  onChange={(e) => setReplaceNotes(e.target.value)}
                  placeholder="vd: Kỹ thuật viên Nguyễn Văn A thay mực"
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                />
              </div>
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowReplaceModal(false)}
                className="bg-slate-100 hover:bg-slate-200 text-slate-700 px-4 py-2 rounded-xl text-sm border border-slate-200 cursor-pointer"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={replaceMutation.isPending || !selectedCartridgeId}
                onClick={() => replaceMutation.mutate()}
                className="bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-sm cursor-pointer shadow-xs"
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
