import {
  ArrowClockwise,
  ArrowRight,
  Broadcast,
  Buildings,
  CheckCircle,
  CheckSquare,
  ListChecks,
  MagnifyingGlass,
  PlusCircle,
  Shield,
  Square,
  Warning,
  X,
} from '@phosphor-icons/react'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { bulkRegisterPrinters, probeDiscovery, relocatePrinterIP, scanSubnet } from '../../api/client'
import type { BulkRegisterPrinterItem, DiscoveredPrinter, SubnetScanResult } from '../../api/types'
import { showError, showSuccess } from '../../utils/sweetalert'

export function DiscoveryPage() {
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<'subnet' | 'single'>('subnet')

  // Subnet scan state
  const [scanMode, setScanMode] = useState<'cidr' | 'range'>('cidr')
  const [scanCIDR, setScanCIDR] = useState('192.168.1.0/24')
  const [startIP, setStartIP] = useState('192.168.1.1')
  const [endIP, setEndIP] = useState('192.168.1.254')
  const [subnetCommunity, setSubnetCommunity] = useState('public')
  const [subnetPort, setSubnetPort] = useState('161')
  const [subnetTimeout, setSubnetTimeout] = useState('1000')
  const [subnetConcurrency, setSubnetConcurrency] = useState('25')
  const [scanResult, setScanResult] = useState<SubnetScanResult | null>(null)

  // Bulk selection and registration state
  const [selectedIPs, setSelectedIPs] = useState<Set<string>>(new Set())
  const [bulkModalOpen, setBulkModalOpen] = useState(false)
  const [bulkDefaultDepartment, setBulkDefaultDepartment] = useState('')
  const [bulkRegistering, setBulkRegistering] = useState(false)

  // Single-IP probe state
  const [probeAddress, setProbeAddress] = useState('')
  const [probeCommunity, setProbeCommunity] = useState('public')

  const singleDiscovery = useMutation({
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

  const subnetDiscovery = useMutation({
    mutationFn: () =>
      scanSubnet({
        cidr: scanMode === 'cidr' ? scanCIDR.trim() : undefined,
        start_ip: scanMode === 'range' ? startIP.trim() : undefined,
        end_ip: scanMode === 'range' ? endIP.trim() : undefined,
        port: Number(subnetPort) || 161,
        version: '2c',
        community: subnetCommunity.trim() || 'public',
        timeout_ms: Number(subnetTimeout) || 1000,
        concurrency: Number(subnetConcurrency) || 25,
      }),
    onSuccess: (data) => {
      setScanResult(data)
      setSelectedIPs(new Set())
      const count = (data.discovered_printers ?? []).length
      if (count > 0) {
        void showSuccess(
          'Quét mạng hoàn tất!',
          `Đã quét ${data.scanned_count} địa chỉ IP, tìm thấy ${count} máy in trong mạng LAN.`,
        )
      } else {
        void showSuccess(
          'Quét mạng hoàn tất!',
          `Đã quét ${data.scanned_count} địa chỉ IP nhưng không phát hiện máy in nào phản hồi SNMP. Kiểm tra lại Community String hoặc dải IP.`,
        )
      }
    },
    onError: (err) => {
      void showError(
        'Quét dải mạng thất bại',
        err instanceof Error ? err.message : 'Đã xảy ra lỗi khi quét dải mạng IP.',
      )
    },
  })

  const toggleSelect = (ip: string) => {
    setSelectedIPs((prev) => {
      const next = new Set(prev)
      if (next.has(ip)) next.delete(ip)
      else next.add(ip)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (!scanResult?.discovered_printers) return
    const unregistered = scanResult.discovered_printers.filter((p) => !p.already_registered)
    const allUnregisteredSelected = unregistered.length > 0 && unregistered.every((p) => selectedIPs.has(p.address))

    if (allUnregisteredSelected) {
      setSelectedIPs(new Set())
    } else {
      setSelectedIPs(new Set(unregistered.map((p) => p.address)))
    }
  }

  const handleBulkRegister = async () => {
    if (!scanResult?.discovered_printers) return
    const targetPrinters = scanResult.discovered_printers.filter((p) => selectedIPs.has(p.address))
    if (targetPrinters.length === 0) return

    setBulkRegistering(true)
    try {
      const items: BulkRegisterPrinterItem[] = targetPrinters.map((p) => ({
        address: p.address,
        port: p.port || 161,
        version: '2c',
        community: subnetCommunity.trim() || 'public',
        display_name: [p.model, p.name].filter(Boolean).join(' - ') || p.name || `Máy in ${p.address}`,
        manufacturer: p.manufacturer || undefined,
        model: p.model || undefined,
        serial: p.serial || undefined,
        sys_object_id: p.sys_object_id || undefined,
        profile_id: p.matched_profile_id || undefined,
        department: bulkDefaultDepartment.trim() || p.location || undefined,
      }))

      const res = await bulkRegisterPrinters({ printers: items })

      const successAddresses = new Set((res.results ?? []).filter((r) => r.status === 'success').map((r) => r.address))
      setScanResult((prev) => {
        if (!prev?.discovered_printers) return prev
        return {
          ...prev,
          discovered_printers: prev.discovered_printers.map((p) =>
            successAddresses.has(p.address) ? { ...p, already_registered: true } : p,
          ),
        }
      })

      setSelectedIPs((prev) => {
        const next = new Set(prev)
        for (const addr of successAddresses) {
          next.delete(addr)
        }
        return next
      })

      setBulkModalOpen(false)

      if (res.failed_count === 0) {
        void showSuccess(
          'Đăng ký hàng loạt thành công!',
          `Đã thêm ${res.registered_count} máy in vào hệ thống và kích hoạt cấu hình tự động.`,
        )
      } else {
        void showSuccess(
          'Hoàn tất đăng ký hàng loạt',
          `Thành công: ${res.registered_count} máy in. Thất bại: ${res.failed_count} máy in.`,
        )
      }
    } catch (err) {
      void showError(
        'Đăng ký hàng loạt thất bại',
        err instanceof Error ? err.message : 'Đã xảy ra lỗi trong quá trình thêm máy in hàng loạt.',
      )
    } finally {
      setBulkRegistering(false)
    }
  }

  const handleRelocateIP = async (device: DiscoveredPrinter) => {
    if (!device.existing_device_id) return
    const confirmed = window.confirm(
      `Phát hiện máy in "${device.existing_display_name || device.name}" (Serial: ${device.serial || '—'}) đã đổi từ IP cũ ${device.old_address} sang IP mới ${device.address}.\n\nBạn có muốn cập nhật IP cho máy in này không?\n(Toàn bộ lịch sử số trang và cấu hình phòng ban được giữ nguyên)`,
    )
    if (!confirmed) return

    try {
      const res = await relocatePrinterIP(device.existing_device_id, device.address)
      void showSuccess('Đã cập nhật IP máy in thành công!', res.message)
      if (scanResult?.discovered_printers) {
        setScanResult({
          ...scanResult,
          discovered_printers: scanResult.discovered_printers.map((p) =>
            p.address === device.address ? { ...p, is_ip_changed: false, already_registered: true } : p,
          ),
        })
      }
    } catch (err) {
      void showError('Không thể cập nhật IP máy in', err instanceof Error ? err.message : String(err))
    }
  }

  const handleRegisterDiscovered = (device: DiscoveredPrinter) => {
    navigate('/printers', {
      state: {
        prefillFromDiscovery: {
          result: {
            address: device.address,
            name: device.name,
            description: device.description,
            sys_object_id: device.sys_object_id,
            serial: device.serial,
            manufacturer: device.manufacturer,
            model: device.model,
            matched_profile_id: device.matched_profile_id,
            markers: device.markers,
            location: device.location,
          },
          community: subnetCommunity || 'public',
        },
      },
    })
  }

  const handleRegisterSingle = () => {
    if (!singleDiscovery.data) return
    navigate('/printers', {
      state: {
        prefillFromDiscovery: {
          result: singleDiscovery.data,
          community: probeCommunity || 'public',
        },
      },
    })
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="glass-panel p-6 sm:p-8">
        <div className="flex items-center gap-3 mb-2">
          <Broadcast size={28} className="text-cyan-400" />
          <h1 className="text-2xl sm:text-3xl font-black text-slate-100">Dò tìm & Quét mạng SNMP</h1>
        </div>
        <p className="text-slate-400 text-sm max-w-2xl">
          Tự động dò tìm toàn bộ dải IP mạng LAN để phát hiện các dòng máy in đang hoạt động, nhận diện Hãng sản xuất,
          Model, Serial và gán Profile SNMP phù hợp.
        </p>

        {/* Tab switch */}
        <div className="flex items-center gap-2 mt-6 p-1 bg-slate-950/80 rounded-xl border border-slate-800 w-fit">
          <button
            type="button"
            onClick={() => setActiveTab('subnet')}
            className={`px-4 py-2 rounded-lg text-xs sm:text-sm font-semibold transition-all cursor-pointer flex items-center gap-2 ${
              activeTab === 'subnet'
                ? 'bg-cyan-600 text-white shadow-md shadow-cyan-950/50'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Broadcast size={16} weight="bold" />
            <span>Quét toàn dải IP (Subnet Scan)</span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('single')}
            className={`px-4 py-2 rounded-lg text-xs sm:text-sm font-semibold transition-all cursor-pointer flex items-center gap-2 ${
              activeTab === 'single'
                ? 'bg-cyan-600 text-white shadow-md shadow-cyan-950/50'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <MagnifyingGlass size={16} weight="bold" />
            <span>Dò 1 địa chỉ IP (Single Probe)</span>
          </button>
        </div>
      </div>

      {/* TAB 1: SUBNET SCAN */}
      {activeTab === 'subnet' && (
        <div className="space-y-6">
          <div className="glass-panel p-6">
            <form
              onSubmit={(e) => {
                e.preventDefault()
                subnetDiscovery.mutate()
              }}
              className="space-y-5"
            >
              <div className="flex flex-wrap items-center gap-4 border-b border-slate-800 pb-4">
                <span className="text-xs font-bold text-slate-300">Phương thức chỉ định dải mạng:</span>
                <label className="inline-flex items-center gap-2 text-xs font-semibold text-slate-200 cursor-pointer">
                  <input
                    type="radio"
                    name="scanMode"
                    checked={scanMode === 'cidr'}
                    onChange={() => setScanMode('cidr')}
                    className="accent-cyan-500"
                  />
                  <span>Ký pháp CIDR (vd: 192.168.1.0/24)</span>
                </label>
                <label className="inline-flex items-center gap-2 text-xs font-semibold text-slate-200 cursor-pointer">
                  <input
                    type="radio"
                    name="scanMode"
                    checked={scanMode === 'range'}
                    onChange={() => setScanMode('range')}
                    className="accent-cyan-500"
                  />
                  <span>Khoảng IP từ đầu đến cuối</span>
                </label>
              </div>

              {scanMode === 'cidr' ? (
                <div className="max-w-md">
                  <label htmlFor="scan-cidr" className="block text-xs font-bold text-slate-300 mb-1.5">
                    Dải mạng CIDR (tối đa /23 hoặc /24)
                  </label>
                  <input
                    id="scan-cidr"
                    value={scanCIDR}
                    onChange={(e) => setScanCIDR(e.target.value)}
                    placeholder="192.168.1.0/24 hoặc 10.0.0.0/24"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
                    required
                  />
                  <small className="text-slate-500 text-[11px] mt-1 block">
                    Chỉ hỗ trợ dải mạng cục bộ/nội bộ (RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16).
                  </small>
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 max-w-xl">
                  <div>
                    <label htmlFor="start-ip" className="block text-xs font-bold text-slate-300 mb-1.5">
                      Địa chỉ IP bắt đầu
                    </label>
                    <input
                      id="start-ip"
                      value={startIP}
                      onChange={(e) => setStartIP(e.target.value)}
                      placeholder="192.168.1.1"
                      className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
                      required
                    />
                  </div>
                  <div>
                    <label htmlFor="end-ip" className="block text-xs font-bold text-slate-300 mb-1.5">
                      Địa chỉ IP kết thúc
                    </label>
                    <input
                      id="end-ip"
                      value={endIP}
                      onChange={(e) => setEndIP(e.target.value)}
                      placeholder="192.168.1.254"
                      className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
                      required
                    />
                  </div>
                </div>
              )}

              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 max-w-3xl">
                <div>
                  <label htmlFor="subnet-community" className="block text-xs font-bold text-slate-300 mb-1.5">
                    SNMP Community String
                  </label>
                  <input
                    id="subnet-community"
                    type="password"
                    value={subnetCommunity}
                    onChange={(e) => setSubnetCommunity(e.target.value)}
                    placeholder="public"
                    autoComplete="new-password"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                    required
                  />
                </div>

                <div>
                  <label htmlFor="subnet-port" className="block text-xs font-bold text-slate-300 mb-1.5">
                    Cổng SNMP (Port)
                  </label>
                  <input
                    id="subnet-port"
                    value={subnetPort}
                    onChange={(e) => setSubnetPort(e.target.value)}
                    placeholder="161"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
                  />
                </div>

                <div>
                  <label htmlFor="subnet-timeout" className="block text-xs font-bold text-slate-300 mb-1.5">
                    Timeout mỗi IP (ms)
                  </label>
                  <input
                    id="subnet-timeout"
                    value={subnetTimeout}
                    onChange={(e) => setSubnetTimeout(e.target.value)}
                    placeholder="1000"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
                  />
                </div>

                <div>
                  <label htmlFor="subnet-concurrency" className="block text-xs font-bold text-slate-300 mb-1.5">
                    Luồng quét đồng thời
                  </label>
                  <input
                    id="subnet-concurrency"
                    value={subnetConcurrency}
                    onChange={(e) => setSubnetConcurrency(e.target.value)}
                    placeholder="25"
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
                  />
                </div>
              </div>

              <div className="flex flex-wrap items-center justify-between gap-4 pt-2">
                <span className="text-xs text-slate-400 flex items-center gap-1.5">
                  <Shield size={16} className="text-emerald-400" />
                  Quét song song có giới hạn (bounded concurrency), tự động phân tích model & profile.
                </span>

                <button
                  type="submit"
                  disabled={subnetDiscovery.isPending}
                  className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-6 py-2.5 rounded-xl text-sm transition-all shadow-lg shadow-cyan-950/50 cursor-pointer flex items-center gap-2"
                >
                  {subnetDiscovery.isPending ? (
                    <>
                      <ArrowClockwise size={18} className="animate-spin" />
                      <span>Đang quét dải mạng (vui lòng đợi 5-15 giây)...</span>
                    </>
                  ) : (
                    <>
                      <Broadcast size={18} weight="bold" />
                      <span>Bắt đầu quét mạng</span>
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>

          {/* Subnet Scan Results Table */}
          {scanResult && (
            <div className="glass-panel overflow-hidden space-y-4">
              <div className="p-6 border-b border-slate-800 flex flex-wrap items-center justify-between gap-4">
                <div>
                  <div className="flex items-center gap-2">
                    <CheckCircle size={20} className="text-emerald-400" />
                    <h3 className="text-lg font-bold text-slate-100">Kết quả quét dải mạng</h3>
                  </div>
                  <p className="text-slate-400 text-xs mt-1">
                    Đã quét <strong className="text-slate-200">{scanResult.scanned_count}</strong> địa chỉ IP · Phát
                    hiện <strong className="text-cyan-400">{(scanResult.discovered_printers ?? []).length}</strong> máy
                    in phản hồi SNMP
                  </p>
                </div>

                {/* Quick select unregistered button */}
                {(scanResult.discovered_printers ?? []).some((p) => !p.already_registered) && (
                  <button
                    type="button"
                    onClick={toggleSelectAll}
                    className="inline-flex items-center gap-1.5 text-xs text-cyan-300 hover:text-cyan-200 bg-cyan-950/50 hover:bg-cyan-900/50 border border-cyan-800/60 px-3 py-1.5 rounded-lg transition-colors cursor-pointer"
                  >
                    <ListChecks size={16} weight="bold" />
                    <span>
                      {(scanResult.discovered_printers ?? [])
                        .filter((p) => !p.already_registered)
                        .every((p) => selectedIPs.has(p.address))
                        ? 'Bỏ chọn tất cả'
                        : `Chọn tất cả chưa đăng ký (${(scanResult.discovered_printers ?? []).filter((p) => !p.already_registered).length})`}
                    </span>
                  </button>
                )}
              </div>

              {/* Bulk Action Bar */}
              {selectedIPs.size > 0 && (
                <div className="mx-6 p-4 rounded-xl bg-gradient-to-r from-cyan-950/60 via-slate-900 to-cyan-950/40 border border-cyan-700/60 flex flex-wrap items-center justify-between gap-4 shadow-lg animate-in fade-in duration-200">
                  <div className="flex items-center gap-3">
                    <span className="bg-cyan-500 text-slate-950 font-black text-xs px-2.5 py-1 rounded-full shadow-sm">
                      {selectedIPs.size}
                    </span>
                    <span className="text-sm font-semibold text-slate-100">
                      máy in đã được chọn để đăng ký hàng loạt
                    </span>
                  </div>
                  <div className="flex items-center gap-3">
                    <button
                      type="button"
                      onClick={() => setSelectedIPs(new Set())}
                      className="text-xs text-slate-400 hover:text-slate-200 px-3 py-1.5 rounded-lg border border-slate-800 hover:bg-slate-800 transition-colors cursor-pointer"
                    >
                      Bỏ chọn
                    </button>
                    <button
                      type="button"
                      onClick={() => setBulkModalOpen(true)}
                      className="inline-flex items-center gap-1.5 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-4 py-2 rounded-xl text-xs sm:text-sm transition-all shadow-lg shadow-cyan-950/50 cursor-pointer"
                    >
                      <ListChecks size={18} weight="bold" />
                      <span>Đăng ký hàng loạt ({selectedIPs.size} máy in)</span>
                    </button>
                  </div>
                </div>
              )}

              {(scanResult.discovered_printers ?? []).length === 0 ? (
                <div className="p-10 text-center text-slate-400 text-sm">
                  <Warning size={40} className="text-amber-400 mx-auto mb-2 opacity-80" />
                  Không tìm thấy máy in nào trong dải mạng đã chọn phản hồi SNMP.
                  <p className="text-xs text-slate-500 mt-1">
                    Gợi ý: Kiểm tra xem máy in đã bật SNMP v2c chưa, và community string có phải là "public" không.
                  </p>
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left border-collapse text-xs sm:text-sm">
                    <thead>
                      <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                        <th className="p-4 pl-6 w-10">
                          <button
                            type="button"
                            onClick={toggleSelectAll}
                            disabled={!(scanResult.discovered_printers ?? []).some((p) => !p.already_registered)}
                            title="Chọn tất cả chưa đăng ký"
                            className="text-slate-400 hover:text-cyan-400 disabled:opacity-30 cursor-pointer flex items-center"
                          >
                            {(scanResult.discovered_printers ?? []).some((p) => !p.already_registered) &&
                            (scanResult.discovered_printers ?? [])
                              .filter((p) => !p.already_registered)
                              .every((p) => selectedIPs.has(p.address)) ? (
                              <CheckSquare size={18} weight="fill" className="text-cyan-400" />
                            ) : (
                              <Square size={18} />
                            )}
                          </button>
                        </th>
                        <th className="p-4">Địa chỉ IP</th>
                        <th className="p-4">Hãng & Model</th>
                        <th className="p-4">Vị trí (SNMP)</th>
                        <th className="p-4">Số Serial</th>
                        <th className="p-4">Profile đề xuất</th>
                        <th className="p-4">Bộ đếm (Markers)</th>
                        <th className="p-4">Trạng thái</th>
                        <th className="p-4 pr-6 text-right">Thao tác</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60">
                      {(scanResult.discovered_printers ?? []).map((printer) => {
                        const markerText = printer.markers?.length
                          ? printer.markers
                              .map(
                                (m) => `${m.life_count.toLocaleString('vi-VN')} (${m.counter_unit_name || m.instance})`,
                              )
                              .join(' · ')
                          : '—'
                        const isSelected = selectedIPs.has(printer.address)

                        return (
                          <tr
                            key={printer.address}
                            className={`transition-colors ${isSelected ? 'bg-cyan-950/30' : 'hover:bg-slate-800/40'}`}
                          >
                            <td className="p-4 pl-6 w-10">
                              <button
                                type="button"
                                onClick={() => toggleSelect(printer.address)}
                                disabled={printer.already_registered}
                                className={`cursor-pointer transition-colors flex items-center ${
                                  printer.already_registered
                                    ? 'opacity-30 cursor-not-allowed text-slate-600'
                                    : isSelected
                                      ? 'text-cyan-400'
                                      : 'text-slate-500 hover:text-slate-300'
                                }`}
                              >
                                {isSelected ? <CheckSquare size={18} weight="fill" /> : <Square size={18} />}
                              </button>
                            </td>
                            <td className="p-4 font-mono text-cyan-400 font-semibold">{printer.address}</td>
                            <td className="p-4">
                              <span className="font-bold text-slate-100 block">
                                {[printer.manufacturer, printer.model].filter(Boolean).join(' ') || printer.name || '—'}
                              </span>
                              {printer.description ? (
                                <span className="text-slate-400 text-xs block line-clamp-1 mt-0.5">
                                  {printer.description}
                                </span>
                              ) : null}
                            </td>
                            <td className="p-4">
                              {printer.location ? (
                                <span className="inline-flex items-center gap-1 text-slate-200 bg-slate-900 border border-slate-800 px-2 py-0.5 rounded text-xs">
                                  <Buildings size={13} className="text-cyan-400 shrink-0" />
                                  <span className="truncate max-w-[130px]" title={printer.location}>
                                    {printer.location}
                                  </span>
                                </span>
                              ) : (
                                <span className="text-slate-600 text-xs">—</span>
                              )}
                            </td>
                            <td className="p-4 text-slate-300 font-mono text-xs">
                              {printer.serial ? <span className="block font-semibold">{printer.serial}</span> : '—'}
                              {printer.mac ? (
                                <span className="text-[10px] text-slate-500 font-mono block">MAC: {printer.mac}</span>
                              ) : null}
                            </td>
                            <td className="p-4">
                              <span className="text-xs font-mono bg-slate-900 border border-slate-800 text-cyan-300 px-2 py-0.5 rounded-lg">
                                {printer.matched_profile_id || 'generic-printer-mib'}
                              </span>
                            </td>
                            <td className="p-4 text-slate-300 text-xs font-mono">{markerText}</td>
                            <td className="p-4">
                              {printer.is_ip_changed ? (
                                <span className="bg-amber-950/80 text-amber-300 text-xs font-semibold px-2.5 py-1 rounded-full border border-amber-800/60 inline-flex items-center gap-1.5 shadow-sm shadow-amber-950/40">
                                  <ArrowClockwise size={12} weight="bold" />
                                  <span>Đổi IP (Cũ: {printer.old_address})</span>
                                </span>
                              ) : printer.already_registered ? (
                                <span className="bg-slate-800 text-slate-400 text-xs font-medium px-2.5 py-1 rounded-full border border-slate-700">
                                  Đã có trong hệ thống
                                </span>
                              ) : (
                                <span className="bg-emerald-950/80 text-emerald-300 text-xs font-medium px-2.5 py-1 rounded-full border border-emerald-800/60">
                                  Chưa đăng ký
                                </span>
                              )}
                            </td>
                            <td className="p-4 pr-6 text-right">
                              {printer.is_ip_changed ? (
                                <button
                                  type="button"
                                  onClick={() => void handleRelocateIP(printer)}
                                  className="inline-flex items-center gap-1.5 bg-amber-600 hover:bg-amber-500 text-white font-semibold px-3 py-1.5 rounded-lg text-xs transition-colors cursor-pointer shadow-md shadow-amber-950/40"
                                  title={`Cập nhật IP cho ${printer.existing_display_name || printer.name} từ ${printer.old_address} sang ${printer.address}`}
                                >
                                  <ArrowClockwise size={14} weight="bold" />
                                  <span>Cập nhật IP</span>
                                </button>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => handleRegisterDiscovered(printer)}
                                  className="inline-flex items-center gap-1.5 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-3 py-1.5 rounded-lg text-xs transition-colors cursor-pointer shadow-md shadow-cyan-950/40"
                                >
                                  <PlusCircle size={14} weight="bold" />
                                  <span>{printer.already_registered ? 'Cập nhật' : 'Thêm máy in'}</span>
                                </button>
                              )}
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {/* Bulk Register Confirmation Modal */}
          {bulkModalOpen && (
            <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-150">
              <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full p-6 shadow-2xl space-y-5">
                <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                  <div className="flex items-center gap-2.5">
                    <ListChecks size={22} className="text-cyan-400" />
                    <h3 className="text-lg font-bold text-slate-100">
                      Đăng ký hàng loạt máy in ({selectedIPs.size} thiết bị)
                    </h3>
                  </div>
                  <button
                    type="button"
                    onClick={() => setBulkModalOpen(false)}
                    className="text-slate-400 hover:text-slate-200 p-1 rounded-lg hover:bg-slate-800 cursor-pointer"
                  >
                    <X size={18} />
                  </button>
                </div>

                <div className="space-y-4 text-xs sm:text-sm">
                  <div>
                    <label htmlFor="bulk-dept-input" className="block text-xs font-bold text-slate-300 mb-1.5">
                      Phòng ban / Vị trí mặc định (Tùy chọn)
                    </label>
                    <input
                      id="bulk-dept-input"
                      type="text"
                      value={bulkDefaultDepartment}
                      onChange={(e) => setBulkDefaultDepartment(e.target.value)}
                      placeholder="Để trống sẽ tự động lấy vị trí từ SNMP sysLocation của từng máy"
                      className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                    />
                    <p className="text-[11px] text-slate-400 mt-1">
                      Gợi ý: Nếu để trống, hệ thống sẽ ưu tiên đọc thông tin vị trí cài đặt sẵn trong máy in qua SNMP
                      (OID sysLocation).
                    </p>
                  </div>

                  <div>
                    <span className="block text-xs font-bold text-slate-300 mb-2">
                      Danh sách máy in sẽ thêm vào hệ thống ({selectedIPs.size}):
                    </span>
                    <div className="max-h-56 overflow-y-auto border border-slate-800 rounded-xl divide-y divide-slate-800/70 bg-slate-950/60">
                      {(scanResult?.discovered_printers ?? [])
                        .filter((p) => selectedIPs.has(p.address))
                        .map((p) => (
                          <div key={p.address} className="p-3 flex items-center justify-between gap-4 text-xs">
                            <div>
                              <span className="font-mono font-bold text-cyan-400 block">{p.address}</span>
                              <span className="text-slate-200">
                                {[p.manufacturer, p.model].filter(Boolean).join(' ') || p.name || 'Máy in'}
                              </span>
                            </div>
                            <div className="text-right">
                              <span className="text-slate-400 block">
                                Vị trí SNMP:{' '}
                                <strong className="text-slate-300">{p.location || 'Chưa thiết lập'}</strong>
                              </span>
                              <span className="text-slate-500 text-[11px]">
                                Profile: {p.matched_profile_id || 'generic-printer-mib'}
                              </span>
                            </div>
                          </div>
                        ))}
                    </div>
                  </div>
                </div>

                <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
                  <button
                    type="button"
                    onClick={() => setBulkModalOpen(false)}
                    disabled={bulkRegistering}
                    className="px-4 py-2 rounded-xl text-xs font-semibold text-slate-300 hover:bg-slate-800 transition-colors cursor-pointer"
                  >
                    Hủy
                  </button>
                  <button
                    type="button"
                    onClick={handleBulkRegister}
                    disabled={bulkRegistering}
                    className="inline-flex items-center gap-2 bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2.5 rounded-xl text-xs sm:text-sm transition-all shadow-lg shadow-cyan-950/50 cursor-pointer"
                  >
                    {bulkRegistering ? (
                      <>
                        <ArrowClockwise size={16} className="animate-spin" />
                        <span>Đang xử lý đăng ký...</span>
                      </>
                    ) : (
                      <>
                        <CheckCircle size={16} weight="bold" />
                        <span>Xác nhận thêm hàng loạt</span>
                      </>
                    )}
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* TAB 2: SINGLE-IP PROBE */}
      {activeTab === 'single' && (
        <div className="glass-panel p-6 space-y-6">
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (probeAddress.trim() && probeCommunity) singleDiscovery.mutate()
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
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
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
                placeholder="public"
                autoComplete="new-password"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                required
              />
            </div>

            <div className="flex items-center justify-between pt-2">
              <span className="text-xs text-slate-400 flex items-center gap-1">
                <Shield size={14} className="text-emerald-400" /> Tiến trình thử nghiệm không lưu credential.
              </span>
              <button
                type="submit"
                disabled={singleDiscovery.isPending}
                className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-6 py-2.5 rounded-xl text-sm transition-all shadow-lg shadow-cyan-950/50 cursor-pointer flex items-center gap-2"
              >
                <MagnifyingGlass size={16} className={singleDiscovery.isPending ? 'animate-spin' : ''} />
                <span>{singleDiscovery.isPending ? 'Đang đọc SNMP...' : 'Đọc thông tin'}</span>
              </button>
            </div>
          </form>

          {/* Single Probe Results Display */}
          {singleDiscovery.data && (
            <div className="mt-8 pt-6 border-t border-slate-800 space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="text-lg font-bold text-slate-100 flex items-center gap-2">
                  <CheckCircle size={20} className="text-emerald-400" /> Kết quả phản hồi SNMP
                </h3>
                <button
                  type="button"
                  onClick={handleRegisterSingle}
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold px-4 py-2 rounded-xl text-xs sm:text-sm transition-all flex items-center gap-1.5 cursor-pointer shadow-lg shadow-emerald-950/50"
                >
                  <span>Chuyển sang Đăng ký máy in này</span>
                  <ArrowRight size={14} />
                </button>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Địa chỉ IP</dt>
                  <dd className="text-sm font-semibold text-cyan-400 font-mono mt-1">{singleDiscovery.data.address}</dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Hãng & Dòng máy nhận diện</dt>
                  <dd className="text-sm font-semibold text-emerald-400 mt-1">
                    {[singleDiscovery.data.manufacturer, singleDiscovery.data.model].filter(Boolean).join(' · ') || '—'}
                  </dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Profile SNMP phù hợp</dt>
                  <dd className="text-xs font-mono font-bold text-cyan-400 mt-1">
                    {singleDiscovery.data.matched_profile_id || 'generic-printer-mib'}
                  </dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Số sê-ri</dt>
                  <dd className="text-sm font-semibold text-slate-200 mt-1">{singleDiscovery.data.serial || '—'}</dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Vị trí (sysLocation)</dt>
                  <dd className="text-sm font-semibold text-cyan-300 mt-1">{singleDiscovery.data.location || '—'}</dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Tên hệ thống (sysName)</dt>
                  <dd className="text-sm font-semibold text-slate-200 mt-1">{singleDiscovery.data.name || '—'}</dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800 sm:col-span-2">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">System Object ID (sysObjectID)</dt>
                  <dd className="text-xs font-mono text-cyan-400 mt-1">{singleDiscovery.data.sys_object_id || '—'}</dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800 sm:col-span-2 lg:col-span-4">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Mô tả thiết bị (sysDescr)</dt>
                  <dd className="text-xs text-slate-300 mt-1 line-clamp-2">
                    {singleDiscovery.data.description || '—'}
                  </dd>
                </div>

                <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800 sm:col-span-2 lg:col-span-4">
                  <dt className="text-[11px] font-bold uppercase text-slate-400">Counter Printer-MIB</dt>
                  {singleDiscovery.data.markers?.length ? (
                    <div className="mt-2 flex flex-wrap gap-3">
                      {singleDiscovery.data.markers.map((marker) => (
                        <span key={marker.instance} className="text-sm font-mono text-cyan-300">
                          instance {marker.instance}: {marker.life_count.toLocaleString('vi-VN')}{' '}
                          {marker.counter_unit_name || `unit ${marker.counter_unit}`}
                        </span>
                      ))}
                    </div>
                  ) : (
                    <dd className="text-sm text-slate-400 mt-1">
                      Không có marker row hợp lệ hoặc thiết bị không hỗ trợ Printer-MIB.
                    </dd>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
