import {
  Archive,
  ArrowRight,
  CheckCircle,
  MagnifyingGlass,
  PencilSimple,
  Plus,
  Printer as PrinterIcon,
  ShieldCheck,
  Trash,
  Warning,
  X,
} from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { deletePrinter, probeDiscovery, registerPrinter, updatePrinter } from '../../api/client'
import type { DiscoveryResult, Printer, PrinterStatus } from '../../api/types'
import { DataTable } from '../../components/DataTable'
import { showConfirm, showError, showSuccess } from '../../utils/sweetalert'
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

interface LocationDiscoveryState {
  prefillFromDiscovery?: {
    result: DiscoveryResult
    community: string
  }
}

export function DevicesPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const query = usePrinters()
  const queryClient = useQueryClient()
  const profiles = useProfiles()

  // Create form state
  const [showCreate, setShowCreate] = useState(false)
  const [displayName, setDisplayName] = useState('')
  const [assetCode, setAssetCode] = useState('')
  const [department, setDepartment] = useState('')
  const [address, setAddress] = useState('')
  const [port, setPort] = useState('161')
  const [community, setCommunity] = useState('public')
  const [manufacturer, setManufacturer] = useState('')
  const [model, setModel] = useState('')
  const [serial, setSerial] = useState('')
  const [sysObjectId, setSysObjectId] = useState('')
  const [profileId, setProfileId] = useState('')
  const [probePreview, setProbePreview] = useState<DiscoveryResult | null>(null)

  // Edit modal state
  const [editingPrinter, setEditingPrinter] = useState<Printer | null>(null)
  const [editDisplayName, setEditDisplayName] = useState('')
  const [editAssetCode, setEditAssetCode] = useState('')
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

  const [registrationJobID, setRegistrationJobID] = useState<string | null>(null)
  const registrationJob = usePollJob(registrationJobID)

  // Prefill from DiscoveryPage if navigated with state
  useEffect(() => {
    const state = location.state as LocationDiscoveryState | null
    if (state?.prefillFromDiscovery) {
      const { result, community: discCommunity } = state.prefillFromDiscovery
      setShowCreate(true)
      setAddress(result.address || '')
      setCommunity(discCommunity || 'public')
      setManufacturer(result.manufacturer || '')
      setModel(result.model || '')
      setSerial(result.serial || '')
      setSysObjectId(result.sys_object_id || '')
      setProfileId(result.matched_profile_id || '')
      setDisplayName(
        [result.model, result.name].filter(Boolean).join(' - ') || result.name || `Máy in ${result.address}`,
      )
      if (result.location) {
        setDepartment(result.location)
      }
      setProbePreview(result)
      window.history.replaceState({}, document.title)
    }
  }, [location.state])

  const resetCreateForm = () => {
    setDisplayName('')
    setAssetCode('')
    setDepartment('')
    setAddress('')
    setPort('161')
    setCommunity('public')
    setManufacturer('')
    setModel('')
    setSerial('')
    setSysObjectId('')
    setProfileId('')
    setProbePreview(null)
  }

  const probeCreateMutation = useMutation({
    mutationFn: () =>
      probeDiscovery({
        address: address.trim(),
        port: Number(port) || 161,
        version: '2c',
        community: community || 'public',
      }),
    onSuccess: (res) => {
      setProbePreview(res)
      if (res.manufacturer) setManufacturer(res.manufacturer)
      if (res.model) setModel(res.model)
      if (res.serial) setSerial(res.serial)
      if (res.sys_object_id) setSysObjectId(res.sys_object_id)
      if (res.matched_profile_id) setProfileId(res.matched_profile_id)
      if (res.location && !department.trim()) setDepartment(res.location)
      if (!displayName.trim()) {
        setDisplayName([res.model, res.name].filter(Boolean).join(' - ') || res.name || `Máy in ${res.address}`)
      }
      void showSuccess(
        'Nhận diện SNMP thành công!',
        `Phát hiện: ${[res.manufacturer, res.model].filter(Boolean).join(' ') || res.name || res.address} → Profile: ${res.matched_profile_id || 'generic-printer-mib'}`,
      )
    },
    onError: (err) => {
      void showError(
        'Không đọc được SNMP từ máy in',
        err instanceof Error ? err.message : 'Kiểm tra lại địa chỉ IP, cổng 161 và SNMP Community.',
      )
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
      if (res.location && !editDepartment.trim()) setEditDepartment(res.location)
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

  const create = useMutation({
    mutationFn: () =>
      registerPrinter({
        display_name: displayName.trim() || `Máy in ${address.trim()}`,
        asset_code: assetCode.trim() || undefined,
        department: department.trim() || undefined,
        manufacturer: manufacturer.trim() || undefined,
        model: model.trim() || undefined,
        serial: serial.trim() || undefined,
        sys_object_id: sysObjectId.trim() || undefined,
        profile_id: profileId || undefined,
        address: address.trim(),
        port: Number(port) || 161,
        version: '2c',
        community: community || 'public',
      }),
    onSuccess: (result) => {
      setRegistrationJobID(result.poll_job_id ?? null)
      resetCreateForm()
      setShowCreate(false)
      void query.refetch()
      void showSuccess(
        'Đã thêm máy in mới!',
        `Máy in "${result.printer.display_name}" đã được gán profile "${result.printer.profile_id ?? result.profile_id ?? 'generic-printer-mib'}" và bắt đầu thu thập bộ đếm.`,
      )
    },
    onError: (err) => {
      void showError(
        'Không thể đăng ký máy in',
        err instanceof Error ? err.message : 'Kiểm tra lại thông tin IP, mã tài sản và SNMP Community.',
      )
    },
  })

  const updateMut = useMutation({
    mutationFn: () => {
      if (!editingPrinter) throw new Error('Chưa chọn máy in cần sửa')
      return updatePrinter(editingPrinter.id, {
        display_name: editDisplayName.trim(),
        asset_code: editAssetCode.trim(),
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
      })
    },
    onSuccess: (updated) => {
      setEditingPrinter(null)
      setEditProbePreview(null)
      void queryClient.invalidateQueries({ queryKey: ['printers'] })
      void queryClient.invalidateQueries({ queryKey: ['printer', updated.id] })
      void showSuccess('Đã cập nhật máy in!', `Thông tin máy in "${updated.display_name}" đã được lưu.`)
    },
    onError: (err) => {
      void showError('Không thể cập nhật máy in', err instanceof Error ? err.message : 'Kiểm tra lại dữ liệu nhập.')
    },
  })

  const deleteMut = useMutation({
    mutationFn: (id: string) => deletePrinter(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['printers'] })
      void showSuccess(
        'Đã xóa máy in!',
        'Máy in đã được gỡ khỏi danh sách hoạt động (lịch sử bộ đếm vẫn được bảo toàn).',
      )
    },
    onError: (err) => {
      void showError('Không thể xóa máy in', err instanceof Error ? err.message : 'Đã xảy ra lỗi khi xóa máy in.')
    },
  })

  const handleOpenEdit = (printer: Printer) => {
    setEditingPrinter(printer)
    setEditDisplayName(printer.display_name)
    setEditAssetCode(printer.asset_code ?? '')
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
  }

  const handleDeletePrinter = async (printer: Printer) => {
    const confirmed = await showConfirm(
      `Xóa máy in "${printer.display_name}"?`,
      'Máy in sẽ được ẩn khỏi danh sách quản lý. Lịch sử trang in và nhật ký thay mực trước đó vẫn được lưu trữ an toàn.',
    )
    if (confirmed.isConfirmed) {
      deleteMut.mutate(printer.id)
    }
  }

  useEffect(() => {
    if (registrationJob.data?.status === 'success' || registrationJob.data?.status === 'failed') {
      void queryClient.invalidateQueries({ queryKey: ['printers'] })
    }
  }, [registrationJob.data?.status, queryClient])

  const printers = query.data?.data ?? []
  const availableProfiles = profiles.data?.data ?? []
  const verifiedProfilesCount = availableProfiles.filter((p) => p.verification_status === 'verified').length

  const departmentOptions = useMemo(() => {
    const set = new Set<string>()
    for (const p of printers) {
      if (p.department) set.add(p.department)
    }
    return Array.from(set).sort()
  }, [printers])

  const printerColumns = useMemo<ColumnDef<Printer>[]>(
    () => [
      {
        id: 'display_name',
        accessorFn: (row) => `${row.display_name} ${row.asset_code ?? ''} ${row.serial ?? ''}`,
        header: 'Tên máy in / Mã tài sản',
        cell: ({ row }) => {
          const printer = row.original
          return (
            <div className="group text-left">
              <span className="font-bold text-slate-100 group-hover:text-cyan-400 transition-colors block text-sm">
                {printer.display_name}
              </span>
              <small className="text-slate-400 text-xs mt-0.5 block">
                {printer.asset_code ?? 'Chưa gán mã tài sản'}
                {printer.serial ? ` · SN: ${printer.serial}` : ''}
              </small>
            </div>
          )
        },
      },
      {
        id: 'address',
        accessorKey: 'address',
        header: 'Địa chỉ IP',
        cell: ({ row }) => {
          const printer = row.original
          return (
            <span className="text-slate-300 text-xs font-mono">
              {printer.address
                ? `${printer.address}${printer.port && printer.port !== 161 ? `:${printer.port}` : ''}`
                : '—'}
            </span>
          )
        },
      },
      {
        id: 'department',
        accessorKey: 'department',
        header: 'Phòng ban',
        cell: ({ row }) => (
          <span className="text-slate-300 text-sm font-semibold">{row.original.department || '—'}</span>
        ),
      },
      {
        id: 'model',
        accessorFn: (row) => `${row.manufacturer ?? ''} ${row.model ?? ''}`,
        header: 'Hãng / Dòng máy',
        cell: ({ row }) => {
          const printer = row.original
          return (
            <div className="text-sm">
              <span className="font-semibold text-slate-200">{printer.manufacturer || '—'}</span>
              {printer.model ? <span className="block text-xs text-slate-400 mt-0.5">{printer.model}</span> : null}
            </div>
          )
        },
      },
      {
        id: 'profile_id',
        accessorKey: 'profile_id',
        header: 'Profile SNMP',
        cell: ({ row }) => (
          <span className="text-xs font-mono bg-slate-900 border border-slate-800 text-cyan-300 px-2.5 py-1 rounded-lg">
            {row.original.profile_id || 'generic-printer-mib'}
          </span>
        ),
      },
      {
        id: 'status',
        accessorKey: 'status',
        header: 'Trạng thái',
        cell: ({ row }) => <StatusBadge status={row.original.status} />,
        filterFn: (row, id, value) => {
          if (!value || value === 'all') return true
          return row.getValue(id) === value
        },
      },
      {
        id: 'last_seen_at',
        accessorKey: 'last_seen_at',
        header: 'Lần thấy cuối',
        cell: ({ row }) => (
          <span className="text-slate-400 text-xs">
            {row.original.last_seen_at ? new Date(row.original.last_seen_at).toLocaleString('vi-VN') : 'Chưa từng quét'}
          </span>
        ),
      },
      {
        id: 'actions',
        header: () => <span className="block text-right">Thao tác</span>,
        enableSorting: false,
        cell: ({ row }) => {
          const printer = row.original
          return (
            <div className="flex items-center justify-end gap-1.5">
              <button
                type="button"
                title="Sửa thông tin & cấu hình SNMP"
                onClick={(e) => {
                  e.stopPropagation()
                  handleOpenEdit(printer)
                }}
                className="p-2 rounded-lg bg-slate-800/80 hover:bg-cyan-950/80 text-slate-300 hover:text-cyan-300 border border-slate-700/70 hover:border-cyan-700/70 transition-colors cursor-pointer"
              >
                <PencilSimple size={16} weight="bold" />
              </button>
              <button
                type="button"
                title="Xóa máy in"
                onClick={(e) => {
                  e.stopPropagation()
                  void handleDeletePrinter(printer)
                }}
                className="p-2 rounded-lg bg-slate-800/80 hover:bg-rose-950/80 text-slate-300 hover:text-rose-300 border border-slate-700/70 hover:border-rose-700/70 transition-colors cursor-pointer"
              >
                <Trash size={16} weight="bold" />
              </button>
            </div>
          )
        },
      },
    ],
    [],
  )

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
          type="button"
          onClick={() => void query.refetch()}
          className="bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all"
        >
          Thử lại
        </button>
      </div>
    )
  }

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
            Giám sát chỉ số bộ đếm máy in qua SNMP v2c/v3, tự động nhận diện dòng máy & gán Profile Printer-MIB chuẩn.
          </p>
        </div>
        <div className="flex items-center gap-3 z-10 w-full sm:w-auto">
          <button
            type="button"
            onClick={() => navigate('/backup')}
            className="flex-1 sm:flex-initial bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm border border-slate-700/80 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <Archive size={18} />
            <span>Sao lưu & Khôi phục</span>
          </button>
          <button
            type="button"
            onClick={() => navigate('/discovery')}
            className="flex-1 sm:flex-initial bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm border border-slate-700/80 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <MagnifyingGlass size={18} />
            <span>Dò tìm SNMP</span>
          </button>
          <button
            type="button"
            onClick={() => setShowCreate((v) => !v)}
            className="flex-1 sm:flex-initial bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm shadow-lg shadow-cyan-950/50 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <Plus size={18} weight="bold" />
            <span>{showCreate ? 'Đóng biểu mẫu' : 'Thêm máy in'}</span>
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
            type="button"
            onClick={() => navigate('/profiles')}
            className="text-xs text-cyan-400 hover:text-cyan-300 font-semibold px-3 py-1 flex items-center gap-1 cursor-pointer"
          >
            Xem tất cả <ArrowRight size={14} />
          </button>
        </div>
      </div>

      {/* Add Printer Form with SNMP Auto-Detection */}
      {showCreate && (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (address.trim() && community) create.mutate()
          }}
          className="glass-panel p-6 space-y-5 border-cyan-800/50"
        >
          <div className="flex flex-wrap items-center justify-between gap-2 pb-3 border-b border-slate-800">
            <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
              <PrinterIcon size={20} className="text-cyan-400" /> Thêm máy in mới & Nhận diện SNMP tự động
            </h3>
            <span className="text-xs text-slate-400">
              Nhập IP + Community rồi nhấn &ldquo;Kiểm tra SNMP & Nhận diện&rdquo; để tự động điền thông tin & Profile
            </span>
          </div>

          {/* Step 1: Network & SNMP Probe */}
          <div className="grid grid-cols-1 sm:grid-cols-12 gap-4 items-end bg-slate-950/50 p-4 rounded-xl border border-slate-800/80">
            <div className="sm:col-span-4">
              <label htmlFor="printer-address" className="block text-xs font-bold text-slate-300 mb-1.5">
                Địa chỉ IP máy in <span className="text-rose-400">*</span>
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

            <div className="sm:col-span-2">
              <label htmlFor="printer-port" className="block text-xs font-bold text-slate-300 mb-1.5">
                Cổng SNMP
              </label>
              <input
                id="printer-port"
                type="number"
                min={1}
                max={65535}
                value={port}
                onChange={(e) => setPort(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 focus:outline-none focus:border-cyan-500 transition-colors"
              />
            </div>

            <div className="sm:col-span-3">
              <label htmlFor="snmp-community" className="block text-xs font-bold text-slate-300 mb-1.5">
                SNMP v2c Community <span className="text-rose-400">*</span>
              </label>
              <input
                id="snmp-community"
                type="text"
                value={community}
                onChange={(e) => setCommunity(e.target.value)}
                placeholder="vd: public"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                required
              />
            </div>

            <div className="sm:col-span-3">
              <button
                type="button"
                disabled={probeCreateMutation.isPending || !address.trim()}
                onClick={() => probeCreateMutation.mutate()}
                className="w-full bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm transition-all flex items-center justify-center gap-2 shadow-md cursor-pointer"
              >
                <MagnifyingGlass size={16} className={probeCreateMutation.isPending ? 'animate-spin' : ''} />
                <span>{probeCreateMutation.isPending ? 'Đang kiểm tra...' : 'Kiểm tra SNMP & Nhận diện'}</span>
              </button>
            </div>
          </div>

          {/* SNMP Detection Banner */}
          {probePreview && (
            <div className="bg-emerald-950/30 border border-emerald-800/60 rounded-xl p-4 space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-emerald-300 text-xs font-bold">
                  <CheckCircle size={18} weight="fill" className="text-emerald-400" />
                  <span>
                    Đã nhận diện thiết bị qua SNMP: {probePreview.manufacturer || 'Chưa rõ hãng'}{' '}
                    {probePreview.model || probePreview.name}
                  </span>
                </div>
                <span className="text-xs font-mono bg-emerald-900/60 text-emerald-200 px-2.5 py-0.5 rounded-full border border-emerald-700/60">
                  Profile gợi ý: {probePreview.matched_profile_id || 'generic-printer-mib'}
                </span>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 text-xs text-slate-300 pt-1">
                <div>
                  <span className="text-slate-400">sysName:</span>{' '}
                  <strong className="text-slate-200">{probePreview.name || '—'}</strong>
                </div>
                <div>
                  <span className="text-slate-400">Serial:</span>{' '}
                  <strong className="text-slate-200">{probePreview.serial || '—'}</strong>
                </div>
                <div>
                  <span className="text-slate-400">Counter hiện tại:</span>{' '}
                  <strong className="text-cyan-300 font-mono">
                    {probePreview.markers?.length
                      ? probePreview.markers
                          .map((m) => `${m.life_count.toLocaleString('vi-VN')} (${m.counter_unit_name || m.instance})`)
                          .join(', ')
                      : 'Chưa đọc được marker'}
                  </strong>
                </div>
              </div>
            </div>
          )}

          {/* Step 2: Printer Metadata & Profile Selection */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div>
              <label htmlFor="display-name" className="block text-xs font-bold text-slate-300 mb-1.5">
                Tên hiển thị <span className="text-rose-400">*</span>
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
              <label htmlFor="asset-code" className="block text-xs font-bold text-slate-300 mb-1.5">
                Mã tài sản
              </label>
              <input
                id="asset-code"
                value={assetCode}
                onChange={(e) => setAssetCode(e.target.value)}
                placeholder="vd: TS-PRN-001"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
              />
            </div>

            <div>
              <label htmlFor="department" className="block text-xs font-bold text-slate-300 mb-1.5">
                Phòng ban / Vị trí
              </label>
              <input
                id="department"
                value={department}
                onChange={(e) => setDepartment(e.target.value)}
                placeholder="vd: Phòng Kế toán - Tầng 2"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
              />
            </div>

            <div>
              <label htmlFor="profile-select" className="block text-xs font-bold text-slate-300 mb-1.5">
                Profile SNMP (Tự động hoặc Chọn)
              </label>
              <select
                id="profile-select"
                value={profileId}
                onChange={(e) => setProfileId(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 focus:outline-none focus:border-cyan-500 transition-colors"
              >
                <option value="">-- Tự động chọn theo dòng máy --</option>
                {availableProfiles.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.id} ({p.manufacturer} · {p.verification_status})
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label htmlFor="manufacturer" className="block text-xs font-bold text-slate-300 mb-1.5">
                Nhà sản xuất (Tự động từ SNMP)
              </label>
              <input
                id="manufacturer"
                value={manufacturer}
                onChange={(e) => setManufacturer(e.target.value)}
                placeholder="vd: HP, Brother, Ricoh..."
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
              />
            </div>

            <div>
              <label htmlFor="model" className="block text-xs font-bold text-slate-300 mb-1.5">
                Dòng máy / Model (Tự động từ SNMP)
              </label>
              <input
                id="model"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="vd: HP LaserJet Enterprise M501dn"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
              />
            </div>

            <div>
              <label htmlFor="serial" className="block text-xs font-bold text-slate-300 mb-1.5">
                Số Serial (Tự động từ SNMP)
              </label>
              <input
                id="serial"
                value={serial}
                onChange={(e) => setSerial(e.target.value)}
                placeholder="vd: VNB3R12345"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
              />
            </div>

            <div>
              <label htmlFor="sys-object-id" className="block text-xs font-bold text-slate-300 mb-1.5">
                sysObjectID (Tự động từ SNMP)
              </label>
              <input
                id="sys-object-id"
                value={sysObjectId}
                onChange={(e) => setSysObjectId(e.target.value)}
                placeholder="vd: 1.3.6.1.4.1.11..."
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm font-mono text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 transition-colors"
                maxLength={200}
              />
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 pt-2">
            <p className="text-xs text-slate-400">
              💡 Nếu bạn bấm &ldquo;Lưu và kết nối&rdquo; ngay, hệ thống vẫn tự động dò SNMP để nhận diện dòng máy và
              gán Profile tương ứng.
            </p>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => {
                  resetCreateForm()
                  setShowCreate(false)
                }}
                className="bg-slate-800 hover:bg-slate-700 text-slate-300 font-semibold px-4 py-2.5 rounded-xl text-sm transition-all cursor-pointer"
              >
                Hủy
              </button>
              <button
                type="submit"
                disabled={create.isPending}
                className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all shadow-md cursor-pointer"
              >
                {create.isPending ? 'Đang kiểm tra & lưu...' : 'Lưu và kết nối'}
              </button>
            </div>
          </div>
        </form>
      )}

      {/* Printers Main Table */}
      <div className="glass-panel p-6 sm:p-8 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-1">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Danh sách thiết bị</p>
            <h2 className="text-xl font-bold text-slate-100">Máy in hiện có</h2>
          </div>
          <span className="text-xs font-semibold bg-slate-800 text-slate-300 px-3 py-1 rounded-full">
            {printers.length} máy in
          </span>
        </div>

        <DataTable
          columns={printerColumns}
          data={printers}
          searchPlaceholder="Tìm kiếm máy in theo tên, IP, phòng ban, model, serial..."
          initialPageSize={10}
          onRowClick={(printer) => navigate(`/printers/${printer.id}`)}
          emptyMessage="Chưa có máy in nào phù hợp"
          emptySubMessage="Thử điều chỉnh từ khóa tìm kiếm hoặc bấm Thêm máy in để đăng ký mới."
          renderCustomFilter={(table) => (
            <div className="flex items-center gap-2 flex-wrap">
              <select
                value={(table.getColumn('status')?.getFilterValue() as string) ?? 'all'}
                onChange={(e) =>
                  table.getColumn('status')?.setFilterValue(e.target.value === 'all' ? undefined : e.target.value)
                }
                className="bg-slate-950/80 border border-slate-800 rounded-xl px-3 py-2 text-xs sm:text-sm text-slate-200 focus:outline-none focus:border-cyan-500 cursor-pointer"
              >
                <option value="all">Tất cả trạng thái</option>
                <option value="online">🟢 Hoạt động</option>
                <option value="offline">🔴 Ngoại tuyến</option>
                <option value="unknown">⚪ Chưa xác định</option>
                <option value="unavailable">⚫ Không khả dụng</option>
              </select>

              {departmentOptions.length > 0 && (
                <select
                  value={(table.getColumn('department')?.getFilterValue() as string) ?? 'all'}
                  onChange={(e) =>
                    table.getColumn('department')?.setFilterValue(e.target.value === 'all' ? undefined : e.target.value)
                  }
                  className="bg-slate-950/80 border border-slate-800 rounded-xl px-3 py-2 text-xs sm:text-sm text-slate-200 focus:outline-none focus:border-cyan-500 cursor-pointer max-w-[200px]"
                >
                  <option value="all">Tất cả phòng ban</option>
                  {departmentOptions.map((dept) => (
                    <option key={dept} value={dept}>
                      {dept}
                    </option>
                  ))}
                </select>
              )}
            </div>
          )}
        />
      </div>

      {/* Edit Printer Modal */}
      {editingPrinter && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4 overflow-y-auto">
          <div className="glass-panel p-6 w-full max-w-3xl space-y-5 border-cyan-800/60 my-8">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <div>
                <h3 className="text-lg font-bold text-slate-100 flex items-center gap-2">
                  <PencilSimple size={20} className="text-cyan-400" /> Chỉnh sửa máy in: {editingPrinter.display_name}
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Cập nhật thông tin định danh, địa chỉ IP, cấu hình SNMP và Profile thu thập bộ đếm.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setEditingPrinter(null)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 cursor-pointer"
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
              {/* Endpoint & SNMP Re-probe */}
              <div className="grid grid-cols-1 sm:grid-cols-12 gap-3 items-end bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                <div className="sm:col-span-4">
                  <label htmlFor="edit-address" className="block text-xs font-bold text-slate-300 mb-1">
                    Địa chỉ IP
                  </label>
                  <input
                    id="edit-address"
                    value={editAddress}
                    onChange={(e) => setEditAddress(e.target.value)}
                    placeholder="vd: 192.168.1.50"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-slate-100"
                  />
                </div>
                <div className="sm:col-span-2">
                  <label htmlFor="edit-port" className="block text-xs font-bold text-slate-300 mb-1">
                    Cổng SNMP
                  </label>
                  <input
                    id="edit-port"
                    type="number"
                    min={1}
                    max={65535}
                    value={editPort}
                    onChange={(e) => setEditPort(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-slate-100"
                  />
                </div>
                <div className="sm:col-span-3">
                  <label htmlFor="edit-community" className="block text-xs font-bold text-slate-300 mb-1">
                    Community mới (nếu đổi)
                  </label>
                  <input
                    id="edit-community"
                    type="text"
                    value={editCommunity}
                    onChange={(e) => setEditCommunity(e.target.value)}
                    placeholder="Để trống nếu giữ nguyên"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-slate-100"
                  />
                </div>
                <div className="sm:col-span-3">
                  <button
                    type="button"
                    disabled={probeEditMutation.isPending || !editAddress.trim()}
                    onClick={() => probeEditMutation.mutate()}
                    className="w-full bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold px-3 py-2 rounded-xl text-xs transition-all flex items-center justify-center gap-1.5 cursor-pointer"
                  >
                    <MagnifyingGlass size={15} className={probeEditMutation.isPending ? 'animate-spin' : ''} />
                    <span>{probeEditMutation.isPending ? 'Đang đọc...' : 'Kiểm tra SNMP'}</span>
                  </button>
                </div>
              </div>

              {editProbePreview && (
                <div className="bg-emerald-950/30 border border-emerald-800/60 rounded-xl p-3 text-xs text-emerald-200 flex flex-wrap items-center justify-between gap-2">
                  <span>
                    ✔ Phản hồi SNMP: <strong>{editProbePreview.manufacturer}</strong>{' '}
                    <strong>{editProbePreview.model || editProbePreview.name}</strong> (SN:{' '}
                    {editProbePreview.serial || '—'})
                  </span>
                  <span className="font-mono bg-emerald-900/60 px-2 py-0.5 rounded">
                    Profile: {editProbePreview.matched_profile_id || 'generic-printer-mib'}
                  </span>
                </div>
              )}

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label htmlFor="edit-display-name" className="block text-xs font-bold text-slate-300 mb-1">
                    Tên hiển thị <span className="text-rose-400">*</span>
                  </label>
                  <input
                    id="edit-display-name"
                    value={editDisplayName}
                    onChange={(e) => setEditDisplayName(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                    required
                  />
                </div>
                <div>
                  <label htmlFor="edit-asset-code" className="block text-xs font-bold text-slate-300 mb-1">
                    Mã tài sản
                  </label>
                  <input
                    id="edit-asset-code"
                    value={editAssetCode}
                    onChange={(e) => setEditAssetCode(e.target.value)}
                    placeholder="vd: TS-PRN-001"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                  />
                </div>
                <div>
                  <label htmlFor="edit-department" className="block text-xs font-bold text-slate-300 mb-1">
                    Phòng ban
                  </label>
                  <input
                    id="edit-department"
                    value={editDepartment}
                    onChange={(e) => setEditDepartment(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                  />
                </div>
                <div>
                  <label htmlFor="edit-profile" className="block text-xs font-bold text-slate-300 mb-1">
                    Profile SNMP
                  </label>
                  <select
                    id="edit-profile"
                    value={editProfileId}
                    onChange={(e) => setEditProfileId(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
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
                  <label htmlFor="edit-manufacturer" className="block text-xs font-bold text-slate-300 mb-1">
                    Nhà sản xuất
                  </label>
                  <input
                    id="edit-manufacturer"
                    value={editManufacturer}
                    onChange={(e) => setEditManufacturer(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                  />
                </div>
                <div>
                  <label htmlFor="edit-model" className="block text-xs font-bold text-slate-300 mb-1">
                    Dòng máy (Model)
                  </label>
                  <input
                    id="edit-model"
                    value={editModel}
                    onChange={(e) => setEditModel(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                  />
                </div>
                <div>
                  <label htmlFor="edit-serial" className="block text-xs font-bold text-slate-300 mb-1">
                    Số Serial
                  </label>
                  <input
                    id="edit-serial"
                    value={editSerial}
                    onChange={(e) => setEditSerial(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                  />
                </div>
                <div>
                  <label htmlFor="edit-sysoid" className="block text-xs font-bold text-slate-300 mb-1">
                    sysObjectID
                  </label>
                  <input
                    id="edit-sysoid"
                    value={editSysObjectId}
                    onChange={(e) => setEditSysObjectId(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm font-mono text-slate-100"
                  />
                </div>
              </div>

              <div className="flex justify-end gap-2 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setEditingPrinter(null)}
                  className="bg-slate-800 hover:bg-slate-700 text-slate-300 font-semibold px-4 py-2 rounded-xl text-sm cursor-pointer"
                >
                  Hủy
                </button>
                <button
                  type="submit"
                  disabled={updateMut.isPending}
                  className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2 rounded-xl text-sm cursor-pointer"
                >
                  {updateMut.isPending ? 'Đang lưu...' : 'Lưu thay đổi'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
