import {
  ArrowCounterClockwise,
  Buildings,
  CalendarBlank,
  CaretDown,
  CaretRight,
  ChartBar,
  DownloadSimple,
  Drop,
  FileXls,
  Funnel,
  MagnifyingGlass,
  Printer as PrinterIcon,
  Recycle,
  Sparkle,
} from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import ExcelJS from 'exceljs'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { listCartridgeLogs, listCartridges } from '../../api/client'
import type { Cartridge, CartridgeLog, Printer } from '../../api/types'
import { showError, showSuccess } from '../../utils/sweetalert'
import { usePrinters } from '../devices/usePrinters'

type TimeRangeOption = 'all' | '7d' | '30d' | 'this_month' | 'this_quarter' | 'this_year'

interface DepartmentStat {
  department: string
  printers: Printer[]
  logs: CartridgeLog[]
  totalNew: number
  totalRefilled: number
  totalCartridges: number
  refillRate: number
  totalPrintedPages: number
  avgYield: number
  lastReplacedAt?: string
  lastPrinterName?: string
  lastCartridgeName?: string
}

interface PrinterStat {
  printer: Printer
  department: string
  logs: CartridgeLog[]
  totalNew: number
  totalRefilled: number
  totalCartridges: number
  refillRate: number
  totalPrintedPages: number
  avgYield: number
  lastReplacedAt?: string
  lastCartridgeName?: string
  lastSourceType?: string
}

export function ReportsPage() {
  const printersQuery = usePrinters()
  const cartridgesQuery = useQuery({
    queryKey: ['cartridges'],
    queryFn: () => listCartridges(),
  })
  const logsQuery = useQuery({
    queryKey: ['cartridge-logs-all'],
    queryFn: () => listCartridgeLogs(undefined, undefined, 1000),
  })

  const [activeTab, setActiveTab] = useState<'department' | 'printer' | 'logs'>('department')
  const [timeRange, setTimeRange] = useState<TimeRangeOption>('all')
  const [departmentFilter, setDepartmentFilter] = useState<string>('all')
  const [sourceFilter, setSourceFilter] = useState<'all' | 'new' | 'refilled'>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedDept, setExpandedDept] = useState<string | null>(null)
  const [isExporting, setIsExporting] = useState(false)

  const printers = useMemo(() => printersQuery.data?.data ?? [], [printersQuery.data])
  const cartridges = useMemo(() => cartridgesQuery.data?.data ?? [], [cartridgesQuery.data])
  const rawLogs = useMemo(() => logsQuery.data?.data ?? [], [logsQuery.data])

  // Lookup maps
  const printerMap = useMemo(() => {
    const map = new Map<string, Printer>()
    for (const p of printers) {
      map.set(p.id, p)
    }
    return map
  }, [printers])

  const cartridgeMap = useMemo(() => {
    const map = new Map<string, Cartridge>()
    for (const c of cartridges) {
      map.set(c.id, c)
    }
    return map
  }, [cartridges])

  // Distinct departments
  const departmentList = useMemo(() => {
    const set = new Set<string>()
    for (const p of printers) {
      const dept = (p.department || '').trim() || 'Chưa phân phòng'
      set.add(dept)
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b, 'vi'))
  }, [printers])

  // Filter logs by date, action type, source type, department, and search
  const filteredLogs = useMemo(() => {
    const now = new Date()
    let startDate: Date | null = null

    if (timeRange === '7d') {
      startDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
    } else if (timeRange === '30d') {
      startDate = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
    } else if (timeRange === 'this_month') {
      startDate = new Date(now.getFullYear(), now.getMonth(), 1)
    } else if (timeRange === 'this_quarter') {
      const quarterStartMonth = Math.floor(now.getMonth() / 3) * 3
      startDate = new Date(now.getFullYear(), quarterStartMonth, 1)
    } else if (timeRange === 'this_year') {
      startDate = new Date(now.getFullYear(), 0, 1)
    }

    const q = searchQuery.trim().toLowerCase()

    return rawLogs.filter((log) => {
      // Must be printer replacement or printer refill action
      if (log.action_type !== 'replace' && !(log.action_type === 'refill' && log.device_id)) {
        return false
      }

      // Time range filter
      if (startDate) {
        const logDate = new Date(log.performed_at)
        if (logDate < startDate) return false
      }

      // Source filter
      if (sourceFilter !== 'all' && log.source_type !== sourceFilter) {
        return false
      }

      const printer = log.device_id ? printerMap.get(log.device_id) : undefined
      const printerDept = (printer?.department || '').trim() || 'Chưa phân phòng'

      // Department filter
      if (departmentFilter !== 'all' && printerDept !== departmentFilter) {
        return false
      }

      // Search query
      if (q) {
        const cartridge = cartridgeMap.get(log.cartridge_id)
        const matchText = [
          printerDept,
          printer?.display_name,
          printer?.model,
          printer?.address,
          cartridge?.name,
          cartridge?.sku_code,
          log.notes,
        ]
          .filter(Boolean)
          .join(' ')
          .toLowerCase()

        if (!matchText.includes(q)) return false
      }

      return true
    })
  }, [rawLogs, timeRange, sourceFilter, departmentFilter, searchQuery, printerMap, cartridgeMap])

  // Aggregate by Department
  const departmentStats = useMemo(() => {
    const deptMap = new Map<string, DepartmentStat>()

    // Initialize all departments (or filtered department)
    for (const dept of departmentList) {
      if (departmentFilter !== 'all' && dept !== departmentFilter) continue
      const printersInDept = printers.filter((p) => ((p.department || '').trim() || 'Chưa phân phòng') === dept)
      deptMap.set(dept, {
        department: dept,
        printers: printersInDept,
        logs: [],
        totalNew: 0,
        totalRefilled: 0,
        totalCartridges: 0,
        refillRate: 0,
        totalPrintedPages: 0,
        avgYield: 0,
      })
    }

    // Accumulate logs
    for (const log of filteredLogs) {
      const printer = log.device_id ? printerMap.get(log.device_id) : undefined
      const dept = (printer?.department || '').trim() || 'Chưa phân phòng'
      let stat = deptMap.get(dept)
      if (!stat) {
        stat = {
          department: dept,
          printers: printer ? [printer] : [],
          logs: [],
          totalNew: 0,
          totalRefilled: 0,
          totalCartridges: 0,
          refillRate: 0,
          totalPrintedPages: 0,
          avgYield: 0,
        }
        deptMap.set(dept, stat)
      }

      stat.logs.push(log)
      const qty = log.quantity || 1
      if (log.source_type === 'new') {
        stat.totalNew += qty
      } else {
        stat.totalRefilled += qty
      }
      stat.totalCartridges += qty

      if ((log.printed_pages ?? 0) > 0) {
        stat.totalPrintedPages += log.printed_pages ?? 0
      }

      // Check last replaced
      if (!stat.lastReplacedAt || new Date(log.performed_at) > new Date(stat.lastReplacedAt)) {
        stat.lastReplacedAt = log.performed_at
        stat.lastPrinterName = printer?.display_name || printer?.model || 'Máy in'
        const c = cartridgeMap.get(log.cartridge_id)
        stat.lastCartridgeName = c ? `${c.name} (${c.sku_code})` : undefined
      }
    }

    // Compute yields and rates
    const result: DepartmentStat[] = []
    for (const stat of deptMap.values()) {
      stat.refillRate = stat.totalCartridges > 0 ? Math.round((stat.totalRefilled / stat.totalCartridges) * 100) : 0
      const logsWithYield = stat.logs.filter((l) => (l.printed_pages ?? 0) > 0)
      stat.avgYield =
        logsWithYield.length > 0
          ? Math.round(logsWithYield.reduce((acc, l) => acc + (l.printed_pages ?? 0), 0) / logsWithYield.length)
          : 0
      result.push(stat)
    }

    // Sort: departments with most cartridge usage first
    return result.sort(
      (a, b) => b.totalCartridges - a.totalCartridges || a.department.localeCompare(b.department, 'vi'),
    )
  }, [departmentList, departmentFilter, printers, filteredLogs, printerMap, cartridgeMap])

  // Aggregate by Printer
  const printerStats = useMemo(() => {
    const list: PrinterStat[] = []

    const q = searchQuery.trim().toLowerCase()

    for (const p of printers) {
      const dept = (p.department || '').trim() || 'Chưa phân phòng'
      if (departmentFilter !== 'all' && dept !== departmentFilter) continue

      if (q) {
        const text = [p.display_name, p.model, p.address, p.serial, dept].filter(Boolean).join(' ').toLowerCase()
        if (!text.includes(q)) continue
      }

      const pLogs = filteredLogs.filter((l) => l.device_id === p.id)
      let totalNew = 0
      let totalRefilled = 0
      let totalPrintedPages = 0
      let lastReplacedAt: string | undefined
      let lastCartridgeName: string | undefined
      let lastSourceType: string | undefined

      for (const log of pLogs) {
        const qty = log.quantity || 1
        if (log.source_type === 'new') {
          totalNew += qty
        } else {
          totalRefilled += qty
        }
        if ((log.printed_pages ?? 0) > 0) {
          totalPrintedPages += log.printed_pages ?? 0
        }
        if (!lastReplacedAt || new Date(log.performed_at) > new Date(lastReplacedAt)) {
          lastReplacedAt = log.performed_at
          lastSourceType = log.source_type
          const c = cartridgeMap.get(log.cartridge_id)
          lastCartridgeName = c ? `${c.name} (${c.sku_code})` : undefined
        }
      }

      const totalCartridges = totalNew + totalRefilled
      const refillRate = totalCartridges > 0 ? Math.round((totalRefilled / totalCartridges) * 100) : 0
      const logsWithYield = pLogs.filter((l) => (l.printed_pages ?? 0) > 0)
      const avgYield =
        logsWithYield.length > 0
          ? Math.round(logsWithYield.reduce((acc, l) => acc + (l.printed_pages ?? 0), 0) / logsWithYield.length)
          : 0

      list.push({
        printer: p,
        department: dept,
        logs: pLogs,
        totalNew,
        totalRefilled,
        totalCartridges,
        refillRate,
        totalPrintedPages,
        avgYield,
        lastReplacedAt,
        lastCartridgeName,
        lastSourceType,
      })
    }

    return list.sort(
      (a, b) =>
        b.totalCartridges - a.totalCartridges ||
        a.department.localeCompare(b.department, 'vi') ||
        a.printer.display_name.localeCompare(b.printer.display_name, 'vi'),
    )
  }, [printers, departmentFilter, searchQuery, filteredLogs, cartridgeMap])

  // Overall KPIs
  const kpis = useMemo(() => {
    let grandTotalNew = 0
    let grandTotalRefilled = 0
    let grandTotalPages = 0
    const yieldLogsNew: number[] = []
    const yieldLogsRefilled: number[] = []

    for (const log of filteredLogs) {
      const qty = log.quantity || 1
      if (log.source_type === 'new') {
        grandTotalNew += qty
        if ((log.printed_pages ?? 0) > 0) yieldLogsNew.push(log.printed_pages ?? 0)
      } else {
        grandTotalRefilled += qty
        if ((log.printed_pages ?? 0) > 0) yieldLogsRefilled.push(log.printed_pages ?? 0)
      }
      if ((log.printed_pages ?? 0) > 0) {
        grandTotalPages += log.printed_pages ?? 0
      }
    }

    const grandTotalCartridges = grandTotalNew + grandTotalRefilled
    const overallRefillRate =
      grandTotalCartridges > 0 ? Math.round((grandTotalRefilled / grandTotalCartridges) * 100) : 0

    const avgYieldNew =
      yieldLogsNew.length > 0 ? Math.round(yieldLogsNew.reduce((a, b) => a + b, 0) / yieldLogsNew.length) : 0
    const avgYieldRefilled =
      yieldLogsRefilled.length > 0
        ? Math.round(yieldLogsRefilled.reduce((a, b) => a + b, 0) / yieldLogsRefilled.length)
        : 0

    // Top department and top printer
    const topDept = departmentStats.length > 0 && departmentStats[0].totalCartridges > 0 ? departmentStats[0] : null
    const topPrinter = printerStats.length > 0 && printerStats[0].totalCartridges > 0 ? printerStats[0] : null

    return {
      grandTotalCartridges,
      grandTotalNew,
      grandTotalRefilled,
      overallRefillRate,
      grandTotalPages,
      avgYieldNew,
      avgYieldRefilled,
      topDept,
      topPrinter,
    }
  }, [filteredLogs, departmentStats, printerStats])

  // Export Excel (.xlsx) Handler
  const handleExportExcel = async () => {
    setIsExporting(true)
    try {
      const workbook = new ExcelJS.Workbook()
      workbook.creator = 'NPMS LAN Printer Management'
      workbook.created = new Date()

      const headerFont = { bold: true, color: { argb: 'FFFFFFFF' }, size: 10 }
      const thinBorder: Partial<ExcelJS.Borders> = {
        top: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        bottom: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        left: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        right: { style: 'thin', color: { argb: 'FFCBD5E1' } },
      }

      // ==========================================
      // Sheet 1: Báo cáo theo phòng ban
      // ==========================================
      const deptSheet = workbook.addWorksheet('Theo Phòng Ban', {
        views: [{ showGridLines: true }],
      })
      deptSheet.columns = [
        { header: 'STT', key: 'stt', width: 8 },
        { header: 'Phòng ban', key: 'department', width: 26 },
        { header: 'Số máy in', key: 'printer_count', width: 14 },
        { header: 'Bình MỚI (100%)', key: 'new', width: 18 },
        { header: 'Bình ĐÃ BƠM', key: 'refilled', width: 18 },
        { header: 'Tổng bình đã dùng', key: 'total', width: 20 },
        { header: 'Tỷ lệ bình bơm', key: 'rate', width: 16 },
        { header: 'Tổng số trang in (Trang)', key: 'pages', width: 26 },
        { header: 'TB trang/bình (Yield)', key: 'yield', width: 22 },
        { header: 'Lần thay gần nhất', key: 'last_date', width: 22 },
        { header: 'Máy in thay gần nhất', key: 'last_printer', width: 26 },
      ]

      const deptHeaderRow = deptSheet.getRow(1)
      deptHeaderRow.height = 28
      deptHeaderRow.eachCell((cell) => {
        cell.font = headerFont
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF0E7490' } } // Cyan-700
        cell.alignment = { vertical: 'middle', horizontal: 'center', wrapText: true }
        cell.border = thinBorder
      })

      departmentStats.forEach((stat, idx) => {
        const lastDate = stat.lastReplacedAt ? new Date(stat.lastReplacedAt).toLocaleString('vi-VN') : '—'
        const row = deptSheet.addRow({
          stt: idx + 1,
          department: stat.department,
          printer_count: stat.printers.length,
          new: stat.totalNew,
          refilled: stat.totalRefilled,
          total: stat.totalCartridges,
          rate: stat.totalCartridges > 0 ? `${stat.refillRate}%` : '0%',
          pages: stat.totalPrintedPages,
          yield: stat.avgYield,
          last_date: lastDate,
          last_printer: stat.lastPrinterName || '—',
        })
        row.height = 20
        row.eachCell((cell) => {
          cell.border = thinBorder
          cell.alignment = { vertical: 'middle' }
        })
        row.getCell('stt').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('printer_count').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('new').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('refilled').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('total').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('rate').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('pages').numFmt = '#,##0'
        row.getCell('yield').numFmt = '#,##0'
      })

      // ==========================================
      // Sheet 2: Báo cáo theo từng máy in
      // ==========================================
      const printerSheet = workbook.addWorksheet('Theo Từng Máy In', {
        views: [{ showGridLines: true }],
      })
      printerSheet.columns = [
        { header: 'STT', key: 'stt', width: 8 },
        { header: 'Tên máy in', key: 'name', width: 28 },
        { header: 'Phòng ban', key: 'department', width: 24 },
        { header: 'Địa chỉ IP', key: 'ip', width: 18 },
        { header: 'Hãng & Dòng máy', key: 'model', width: 24 },
        { header: 'Bình MỚI', key: 'new', width: 16 },
        { header: 'Bình ĐÃ BƠM', key: 'refilled', width: 16 },
        { header: 'Tổng bình', key: 'total', width: 16 },
        { header: 'Tỷ lệ bơm', key: 'rate', width: 14 },
        { header: 'Tổng trang in (Yield)', key: 'pages', width: 22 },
        { header: 'TB trang/bình', key: 'yield', width: 20 },
        { header: 'Lần thay gần nhất', key: 'last_date', width: 22 },
        { header: 'Loại mực gần nhất', key: 'last_cartridge', width: 26 },
      ]

      const printerHeaderRow = printerSheet.getRow(1)
      printerHeaderRow.height = 28
      printerHeaderRow.eachCell((cell) => {
        cell.font = headerFont
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF059669' } } // Emerald-600
        cell.alignment = { vertical: 'middle', horizontal: 'center', wrapText: true }
        cell.border = thinBorder
      })

      printerStats.forEach((item, idx) => {
        const lastDate = item.lastReplacedAt ? new Date(item.lastReplacedAt).toLocaleString('vi-VN') : '—'
        const row = printerSheet.addRow({
          stt: idx + 1,
          name: item.printer.display_name,
          department: item.department,
          ip: item.printer.address || '—',
          model: [item.printer.manufacturer, item.printer.model].filter(Boolean).join(' ') || '—',
          new: item.totalNew,
          refilled: item.totalRefilled,
          total: item.totalCartridges,
          rate: item.totalCartridges > 0 ? `${item.refillRate}%` : '0%',
          pages: item.totalPrintedPages,
          yield: item.avgYield,
          last_date: lastDate,
          last_cartridge: item.lastCartridgeName || '—',
        })
        row.height = 20
        row.eachCell((cell) => {
          cell.border = thinBorder
          cell.alignment = { vertical: 'middle' }
        })
        row.getCell('stt').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('new').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('refilled').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('total').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('rate').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('pages').numFmt = '#,##0'
        row.getCell('yield').numFmt = '#,##0'
      })

      // ==========================================
      // Sheet 3: Nhật ký chi tiết thay mực
      // ==========================================
      const logSheet = workbook.addWorksheet('Nhật Ký Thay Mực', {
        views: [{ showGridLines: true }],
      })
      logSheet.columns = [
        { header: 'STT', key: 'stt', width: 8 },
        { header: 'Thời gian thay', key: 'time', width: 22 },
        { header: 'Phòng ban', key: 'department', width: 24 },
        { header: 'Máy in', key: 'printer', width: 26 },
        { header: 'Tên hộp mực', key: 'cartridge', width: 24 },
        { header: 'Mã SKU', key: 'sku', width: 16 },
        { header: 'Nguồn mực', key: 'source', width: 18 },
        { header: 'Số lượng', key: 'qty', width: 12 },
        { header: 'Mốc bộ đếm lúc thay', key: 'counter', width: 22 },
        { header: 'Sản lượng in đạt được (Trang)', key: 'yield', width: 28 },
        { header: 'Ghi chú', key: 'notes', width: 30 },
      ]

      const logHeaderRow = logSheet.getRow(1)
      logHeaderRow.height = 28
      logHeaderRow.eachCell((cell) => {
        cell.font = headerFont
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F46E5' } } // Indigo-600
        cell.alignment = { vertical: 'middle', horizontal: 'center', wrapText: true }
        cell.border = thinBorder
      })

      filteredLogs.forEach((log, idx) => {
        const printer = log.device_id ? printerMap.get(log.device_id) : undefined
        const dept = (printer?.department || '').trim() || 'Chưa phân phòng'
        const cartridge = cartridgeMap.get(log.cartridge_id)
        const row = logSheet.addRow({
          stt: idx + 1,
          time: new Date(log.performed_at).toLocaleString('vi-VN'),
          department: dept,
          printer: printer?.display_name || '—',
          cartridge: cartridge?.name || '—',
          sku: cartridge?.sku_code || '—',
          source: log.source_type === 'new' ? 'Bình mới 100%' : 'Bình đã bơm',
          qty: log.quantity || 1,
          counter: log.page_count || 0,
          yield: log.printed_pages || 0,
          notes: log.notes || '',
        })
        row.height = 20
        row.eachCell((cell) => {
          cell.border = thinBorder
          cell.alignment = { vertical: 'middle' }
        })
        row.getCell('stt').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('qty').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('source').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('counter').numFmt = '#,##0'
        row.getCell('yield').numFmt = '#,##0'
      })

      const dateStr = new Date().toISOString().slice(0, 10)
      const buffer = await workbook.xlsx.writeBuffer()
      const blob = new Blob([buffer], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `Bao_cao_tieu_thu_muc_in_${dateStr}.xlsx`)
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)

      void showSuccess('Xuất Excel thành công!', 'File Excel (.xlsx) đã được tải xuống máy của bạn.')
    } catch (err) {
      void showError('Không thể xuất file Excel', err instanceof Error ? err.message : 'Lỗi tạo workbook.')
    } finally {
      setIsExporting(false)
    }
  }

  const handlePrint = () => {
    window.print()
  }

  const isLoading = printersQuery.isLoading || cartridgesQuery.isLoading || logsQuery.isLoading

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="glass-panel p-6 sm:p-8 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6 print:border-none print:shadow-none print:p-0">
        <div className="space-y-2 max-w-2xl">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-cyan-950/80 border border-cyan-800/60 text-cyan-400 flex items-center justify-center text-xl font-bold">
              <ChartBar size={24} />
            </div>
            <div>
              <span className="text-xs uppercase font-extrabold tracking-widest text-cyan-400">
                Thống kê & Giám sát vật tư
              </span>
              <h1 className="text-2xl sm:text-3xl font-black text-slate-100 tracking-tight">Báo cáo tiêu thụ mực in</h1>
            </div>
          </div>
          <p className="text-slate-400 text-sm leading-relaxed">
            Phân rã số lượng bình mực sử dụng theo từng <strong>Phòng ban</strong> và từng <strong>Máy in</strong> trong
            mạng LAN. Đánh giá hiệu suất số trang in (Yield) giữa bình mới và bình bơm lại.
          </p>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-3 w-full sm:w-auto print:hidden">
          <button
            type="button"
            disabled={isExporting}
            onClick={handleExportExcel}
            className="flex-1 sm:flex-none bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm shadow-lg shadow-emerald-950/50 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <FileXls size={18} weight="bold" />
            <span>{isExporting ? 'Đang tạo Excel...' : 'Xuất file Excel (.xlsx)'}</span>
          </button>
          <button
            type="button"
            onClick={handlePrint}
            className="flex-1 sm:flex-none bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm border border-slate-700 transition-all flex items-center justify-center gap-2 cursor-pointer shadow-sm shadow-slate-950"
          >
            <DownloadSimple size={18} />
            <span>In báo cáo</span>
          </button>
        </div>
      </div>

      {/* KPI Overview Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 print:grid-cols-4">
        {/* Total Cartridges Used */}
        <div className="glass-card p-5 border-l-4 border-l-cyan-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Tổng bình mực đã dùng</p>
              <h3 className="text-2xl font-black text-slate-100 mt-1">{kpis.grandTotalCartridges} bình</h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-cyan-950/80 border border-cyan-800/60 text-cyan-400 flex items-center justify-center">
              <Drop size={26} />
            </div>
          </div>
          <div className="mt-3 pt-3 border-t border-slate-800/80 flex items-center justify-between text-xs">
            <span className="text-emerald-400 font-semibold">🟢 Mới: {kpis.grandTotalNew}</span>
            <span className="text-cyan-400 font-semibold">🔵 Bơm: {kpis.grandTotalRefilled}</span>
          </div>
        </div>

        {/* Refill Saving Rate */}
        <div className="glass-card p-5 border-l-4 border-l-emerald-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Tỷ lệ tái sử dụng (Bơm)</p>
              <h3 className="text-2xl font-black text-emerald-400 mt-1">{kpis.overallRefillRate}%</h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-emerald-950/80 border border-emerald-800/60 text-emerald-400 flex items-center justify-center">
              <Recycle size={26} />
            </div>
          </div>
          <p className="text-xs text-slate-400 mt-3 pt-3 border-t border-slate-800/80">
            {kpis.overallRefillRate > 50
              ? 'Tiết kiệm chi phí vật tư rất tốt nhờ tái sử dụng vỏ bình'
              : 'Nên ưu tiên bơm mực tái sử dụng để tối ưu ngân sách'}
          </p>
        </div>

        {/* Total Pages Printed */}
        <div className="glass-card p-5 border-l-4 border-l-purple-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Sản lượng in ghi nhận</p>
              <h3 className="text-2xl font-black text-purple-400 mt-1">
                {kpis.grandTotalPages.toLocaleString('vi-VN')} trang
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-purple-950/80 border border-purple-800/60 text-purple-400 flex items-center justify-center">
              <Sparkle size={26} />
            </div>
          </div>
          <div className="mt-3 pt-3 border-t border-slate-800/80 flex items-center justify-between text-xs text-slate-400">
            <span>
              TB Bình Mới:{' '}
              <strong className="text-emerald-400">
                {kpis.avgYieldNew ? `${kpis.avgYieldNew.toLocaleString('vi-VN')} trg` : '—'}
              </strong>
            </span>
            <span>
              TB Bình Bơm:{' '}
              <strong className="text-cyan-400">
                {kpis.avgYieldRefilled ? `${kpis.avgYieldRefilled.toLocaleString('vi-VN')} trg` : '—'}
              </strong>
            </span>
          </div>
        </div>

        {/* Top Department */}
        <div className="glass-card p-5 border-l-4 border-l-amber-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Phòng dùng nhiều nhất</p>
              <h3
                className="text-lg font-black text-amber-400 mt-1 truncate max-w-[170px]"
                title={kpis.topDept?.department || '—'}
              >
                {kpis.topDept ? kpis.topDept.department : 'Chưa có'}
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-amber-950/80 border border-amber-800/60 text-amber-400 flex items-center justify-center">
              <Buildings size={26} />
            </div>
          </div>
          <p className="text-xs text-slate-400 mt-3 pt-3 border-t border-slate-800/80">
            {kpis.topDept ? (
              <span>
                Đã tiêu thụ <strong className="text-slate-200">{kpis.topDept.totalCartridges}</strong> bình mực
              </span>
            ) : (
              'Chưa có dữ liệu thay mực'
            )}
          </p>
        </div>
      </div>

      {/* Filter and Control Toolbar */}
      <div className="glass-panel p-5 bg-slate-900/60 border border-slate-800 space-y-4 print:hidden">
        <div className="flex flex-wrap items-center justify-between gap-4">
          {/* Navigation Tabs */}
          <div className="flex items-center bg-slate-950/80 p-1 rounded-xl border border-slate-800">
            <button
              type="button"
              onClick={() => setActiveTab('department')}
              className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
                activeTab === 'department'
                  ? 'bg-cyan-600 text-white shadow-md shadow-cyan-950'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <Buildings size={16} weight="bold" />
              <span>Theo phòng ban ({departmentStats.length})</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('printer')}
              className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
                activeTab === 'printer'
                  ? 'bg-cyan-600 text-white shadow-md shadow-cyan-950'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <PrinterIcon size={16} weight="bold" />
              <span>Theo từng máy in ({printerStats.length})</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('logs')}
              className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
                activeTab === 'logs'
                  ? 'bg-cyan-600 text-white shadow-md shadow-cyan-950'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <Drop size={16} weight="bold" />
              <span>Nhật ký thay mực ({filteredLogs.length})</span>
            </button>
          </div>

          {/* Quick Search */}
          <div className="relative flex-1 sm:max-w-xs">
            <MagnifyingGlass size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Tìm phòng ban, máy in, SKU..."
              className="w-full bg-slate-950/90 border border-slate-800 rounded-xl pl-9 pr-3.5 py-1.5 text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
            />
          </div>
        </div>

        {/* Secondary Filter Options */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-3 border-t border-slate-800/60 text-xs">
          {/* Time range */}
          <div className="flex items-center gap-2">
            <CalendarBlank size={16} className="text-slate-400 shrink-0" />
            <span className="text-slate-400 shrink-0">Thời gian:</span>
            <select
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value as TimeRangeOption)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-cyan-500"
            >
              <option value="all">Toàn bộ thời gian</option>
              <option value="7d">7 ngày gần nhất</option>
              <option value="30d">30 ngày gần nhất</option>
              <option value="this_month">Trong tháng này</option>
              <option value="this_quarter">Trong quý này</option>
              <option value="this_year">Trong năm nay</option>
            </select>
          </div>

          {/* Department selector */}
          <div className="flex items-center gap-2">
            <Buildings size={16} className="text-slate-400 shrink-0" />
            <span className="text-slate-400 shrink-0">Phòng:</span>
            <select
              value={departmentFilter}
              onChange={(e) => setDepartmentFilter(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-cyan-500"
            >
              <option value="all">Tất cả các phòng ban ({departmentList.length})</option>
              {departmentList.map((d) => (
                <option key={d} value={d}>
                  {d}
                </option>
              ))}
            </select>
          </div>

          {/* Source selector */}
          <div className="flex items-center gap-2">
            <Funnel size={16} className="text-slate-400 shrink-0" />
            <span className="text-slate-400 shrink-0">Loại mực:</span>
            <select
              value={sourceFilter}
              onChange={(e) => setSourceFilter(e.target.value as 'all' | 'new' | 'refilled')}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-cyan-500"
            >
              <option value="all">Tất cả nguồn mực</option>
              <option value="new">🟢 Chỉ bình MỚI 100%</option>
              <option value="refilled">🔵 Chỉ bình ĐÃ BƠM</option>
            </select>
          </div>
        </div>
      </div>

      {/* Main Content Area */}
      {isLoading ? (
        <div className="glass-panel p-16 text-center text-slate-400 flex flex-col items-center justify-center gap-3">
          <ArrowCounterClockwise size={32} className="animate-spin text-cyan-400" />
          <p className="text-sm">Đang nạp dữ liệu thống kê sử dụng mực...</p>
        </div>
      ) : activeTab === 'department' ? (
        /* TAB 1: THEO PHÒNG BAN */
        <div className="space-y-6">
          {/* Visual Distribution Chart */}
          {departmentStats.some((d) => d.totalCartridges > 0) && (
            <div className="glass-panel p-5 bg-slate-900/60 border border-slate-800">
              <h4 className="text-xs font-bold uppercase tracking-wider text-cyan-400 mb-3">
                Biểu đồ tỷ lệ tiêu thụ bình mực theo phòng ban
              </h4>
              <div className="space-y-2.5">
                {departmentStats
                  .filter((d) => d.totalCartridges > 0)
                  .map((d) => {
                    const percent =
                      kpis.grandTotalCartridges > 0
                        ? Math.round((d.totalCartridges / kpis.grandTotalCartridges) * 100)
                        : 0
                    return (
                      <div key={d.department} className="space-y-1">
                        <div className="flex justify-between text-xs">
                          <span className="font-semibold text-slate-200 flex items-center gap-1.5">
                            <Buildings size={14} className="text-cyan-400" />
                            {d.department}
                          </span>
                          <span className="text-slate-400">
                            <strong className="text-slate-100">{d.totalCartridges}</strong> bình ({percent}%) · 🟢{' '}
                            {d.totalNew} · 🔵 {d.totalRefilled}
                          </span>
                        </div>
                        <div className="w-full h-2.5 bg-slate-950 rounded-full overflow-hidden flex">
                          <div
                            className="bg-emerald-500 h-full transition-all duration-500"
                            style={{
                              width: `${kpis.grandTotalCartridges > 0 ? (d.totalNew / kpis.grandTotalCartridges) * 100 : 0}%`,
                            }}
                            title={`Bình mới: ${d.totalNew}`}
                          />
                          <div
                            className="bg-cyan-500 h-full transition-all duration-500"
                            style={{
                              width: `${kpis.grandTotalCartridges > 0 ? (d.totalRefilled / kpis.grandTotalCartridges) * 100 : 0}%`,
                            }}
                            title={`Bình bơm: ${d.totalRefilled}`}
                          />
                        </div>
                      </div>
                    )
                  })}
              </div>
            </div>
          )}

          {/* Department Table */}
          <div className="glass-panel overflow-hidden">
            <div className="p-6 border-b border-slate-800 flex items-center justify-between">
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Bảng chi tiết</p>
                <h3 className="text-lg font-bold text-slate-100">Báo cáo tiêu thụ theo phòng ban</h3>
              </div>
              <span className="text-xs text-slate-400 font-semibold bg-slate-800/80 px-3 py-1 rounded-full border border-slate-700">
                {departmentStats.length} phòng ban
              </span>
            </div>

            {departmentStats.length === 0 ? (
              <div className="p-12 text-center text-slate-400 text-sm">
                Không tìm thấy phòng ban nào khớp với bộ lọc.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse text-xs sm:text-sm">
                  <thead>
                    <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                      <th className="p-4 pl-6">Phòng ban</th>
                      <th className="p-4 text-center">Số máy in</th>
                      <th className="p-4 text-center">🟢 Bình MỚI</th>
                      <th className="p-4 text-center">🔵 Bình BƠM</th>
                      <th className="p-4 text-center">Tổng bình đã dùng</th>
                      <th className="p-4 text-center">Tỷ lệ bơm</th>
                      <th className="p-4 text-right">Tổng trang in</th>
                      <th className="p-4 text-right">TB trang/bình</th>
                      <th className="p-4 pr-6">Lần thay gần nhất</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60">
                    {departmentStats.map((stat) => {
                      const isExpanded = expandedDept === stat.department
                      return (
                        <>
                          <tr
                            key={stat.department}
                            onClick={() => setExpandedDept(isExpanded ? null : stat.department)}
                            className="hover:bg-slate-800/40 transition-colors cursor-pointer group"
                          >
                            <td className="p-4 pl-6 font-bold text-slate-100">
                              <div className="flex items-center gap-2">
                                {isExpanded ? (
                                  <CaretDown size={14} className="text-cyan-400" />
                                ) : (
                                  <CaretRight size={14} className="text-slate-500 group-hover:text-cyan-400" />
                                )}
                                <span>{stat.department}</span>
                              </div>
                            </td>
                            <td className="p-4 text-center text-slate-300 font-semibold">{stat.printers.length}</td>
                            <td className="p-4 text-center font-bold text-emerald-400">{stat.totalNew}</td>
                            <td className="p-4 text-center font-bold text-cyan-400">{stat.totalRefilled}</td>
                            <td className="p-4 text-center">
                              <span className="font-black text-slate-100 bg-slate-800/90 px-2.5 py-1 rounded-lg border border-slate-700">
                                {stat.totalCartridges}
                              </span>
                            </td>
                            <td className="p-4 text-center">
                              <span
                                className={`text-xs font-bold px-2 py-0.5 rounded-full ${
                                  stat.refillRate >= 50
                                    ? 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                                    : stat.totalCartridges === 0
                                      ? 'text-slate-500'
                                      : 'bg-amber-950 text-amber-400 border border-amber-800'
                                }`}
                              >
                                {stat.totalCartridges > 0 ? `${stat.refillRate}%` : '—'}
                              </span>
                            </td>
                            <td className="p-4 text-right font-mono text-purple-300">
                              {stat.totalPrintedPages > 0 ? stat.totalPrintedPages.toLocaleString('vi-VN') : '—'}
                            </td>
                            <td className="p-4 text-right font-mono text-cyan-400 font-bold">
                              {stat.avgYield > 0 ? `${stat.avgYield.toLocaleString('vi-VN')} trg` : '—'}
                            </td>
                            <td className="p-4 pr-6 text-slate-400 text-xs">
                              {stat.lastReplacedAt ? (
                                <div>
                                  <span className="font-semibold text-slate-200">
                                    {new Date(stat.lastReplacedAt).toLocaleDateString('vi-VN')}
                                  </span>
                                  {stat.lastPrinterName && (
                                    <span className="block text-[11px] text-slate-500 truncate max-w-[150px]">
                                      {stat.lastPrinterName}
                                    </span>
                                  )}
                                </div>
                              ) : (
                                <span className="text-slate-600">—</span>
                              )}
                            </td>
                          </tr>

                          {/* Accordion detail row */}
                          {isExpanded && (
                            <tr className="bg-slate-950/70 border-b border-slate-800/80">
                              <td colSpan={9} className="p-4 pl-12 pr-6">
                                <div className="space-y-4">
                                  <div className="flex items-center justify-between">
                                    <h5 className="text-xs font-bold uppercase tracking-wider text-cyan-400">
                                      Danh sách máy in thuộc {stat.department} ({stat.printers.length} máy)
                                    </h5>
                                  </div>

                                  {stat.printers.length === 0 ? (
                                    <p className="text-xs text-slate-500 italic">
                                      Không có máy in nào gắn với phòng này.
                                    </p>
                                  ) : (
                                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                                      {stat.printers.map((p) => {
                                        const pLogs = stat.logs.filter((l) => l.device_id === p.id)
                                        const pNew = pLogs.filter((l) => l.source_type === 'new').length
                                        const pRefilled = pLogs.filter((l) => l.source_type === 'refilled').length
                                        return (
                                          <div
                                            key={p.id}
                                            className="bg-slate-900/80 p-3.5 rounded-xl border border-slate-800 flex flex-col justify-between"
                                          >
                                            <div>
                                              <div className="flex items-center justify-between">
                                                <Link
                                                  to={`/printers/${p.id}`}
                                                  className="font-bold text-slate-200 text-xs hover:text-cyan-400 flex items-center gap-1.5"
                                                >
                                                  <PrinterIcon size={14} className="text-cyan-400" />
                                                  {p.display_name}
                                                </Link>
                                                <span
                                                  className={`text-[10px] px-1.5 py-0.5 rounded font-semibold ${
                                                    p.status === 'online'
                                                      ? 'bg-emerald-950 text-emerald-400'
                                                      : 'bg-rose-950 text-rose-400'
                                                  }`}
                                                >
                                                  {p.status}
                                                </span>
                                              </div>
                                              <p className="text-[11px] text-slate-400 mt-1">
                                                {[p.manufacturer, p.model].filter(Boolean).join(' ')} ·{' '}
                                                {p.address || '—'}
                                              </p>
                                            </div>

                                            <div className="mt-2.5 pt-2 border-t border-slate-800/80 flex items-center justify-between text-[11px]">
                                              <span className="text-slate-400">
                                                Đã dùng: <strong className="text-slate-100">{pNew + pRefilled}</strong>{' '}
                                                bình
                                              </span>
                                              <span className="text-emerald-400 font-semibold">🟢 {pNew}</span>
                                              <span className="text-cyan-400 font-semibold">🔵 {pRefilled}</span>
                                            </div>
                                          </div>
                                        )
                                      })}
                                    </div>
                                  )}
                                </div>
                              </td>
                            </tr>
                          )}
                        </>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      ) : activeTab === 'printer' ? (
        /* TAB 2: THEO TỪNG MÁY IN */
        <div className="glass-panel overflow-hidden">
          <div className="p-6 border-b border-slate-800 flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Bảng chi tiết</p>
              <h3 className="text-lg font-bold text-slate-100">Báo cáo tiêu thụ mực theo từng máy in</h3>
            </div>
            <span className="text-xs text-slate-400 font-semibold bg-slate-800/80 px-3 py-1 rounded-full border border-slate-700">
              {printerStats.length} máy in
            </span>
          </div>

          {printerStats.length === 0 ? (
            <div className="p-12 text-center text-slate-400 text-sm">Không tìm thấy máy in nào khớp với bộ lọc.</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse text-xs sm:text-sm">
                <thead>
                  <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                    <th className="p-4 pl-6">Máy in & IP</th>
                    <th className="p-4">Phòng ban</th>
                    <th className="p-4">Dòng máy (Model)</th>
                    <th className="p-4 text-center">🟢 Bình MỚI</th>
                    <th className="p-4 text-center">🔵 Bình BƠM</th>
                    <th className="p-4 text-center">Tổng bình</th>
                    <th className="p-4 text-center">Tỷ lệ bơm</th>
                    <th className="p-4 text-right">Tổng trang in</th>
                    <th className="p-4 text-right">TB trang/bình</th>
                    <th className="p-4 pr-6">Lần thay gần nhất</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60">
                  {printerStats.map((item) => (
                    <tr key={item.printer.id} className="hover:bg-slate-800/40 transition-colors">
                      <td className="p-4 pl-6">
                        <Link
                          to={`/printers/${item.printer.id}`}
                          className="font-bold text-slate-100 hover:text-cyan-400 transition-colors flex items-center gap-1.5"
                        >
                          <PrinterIcon size={16} className="text-cyan-400 shrink-0" />
                          <span>{item.printer.display_name}</span>
                        </Link>
                        <small className="text-slate-500 font-mono text-xs block mt-0.5">
                          {item.printer.address || '—'}
                        </small>
                      </td>
                      <td className="p-4 font-semibold text-slate-300">
                        <span className="inline-flex items-center gap-1 bg-slate-800/60 px-2 py-0.5 rounded text-xs">
                          <Buildings size={12} className="text-cyan-400" />
                          {item.department}
                        </span>
                      </td>
                      <td className="p-4 text-slate-300 text-xs">
                        {[item.printer.manufacturer, item.printer.model].filter(Boolean).join(' ') || '—'}
                      </td>
                      <td className="p-4 text-center font-bold text-emerald-400">{item.totalNew}</td>
                      <td className="p-4 text-center font-bold text-cyan-400">{item.totalRefilled}</td>
                      <td className="p-4 text-center">
                        <span className="font-black text-slate-100 bg-slate-800/90 px-2.5 py-1 rounded-lg border border-slate-700">
                          {item.totalCartridges}
                        </span>
                      </td>
                      <td className="p-4 text-center">
                        <span
                          className={`text-xs font-bold px-2 py-0.5 rounded-full ${
                            item.refillRate >= 50
                              ? 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                              : item.totalCartridges === 0
                                ? 'text-slate-500'
                                : 'bg-amber-950 text-amber-400 border border-amber-800'
                          }`}
                        >
                          {item.totalCartridges > 0 ? `${item.refillRate}%` : '—'}
                        </span>
                      </td>
                      <td className="p-4 text-right font-mono text-purple-300">
                        {item.totalPrintedPages > 0 ? item.totalPrintedPages.toLocaleString('vi-VN') : '—'}
                      </td>
                      <td className="p-4 text-right font-mono text-cyan-400 font-bold">
                        {item.avgYield > 0 ? `${item.avgYield.toLocaleString('vi-VN')} trg` : '—'}
                      </td>
                      <td className="p-4 pr-6 text-slate-400 text-xs">
                        {item.lastReplacedAt ? (
                          <div>
                            <span className="font-semibold text-slate-200">
                              {new Date(item.lastReplacedAt).toLocaleDateString('vi-VN')}
                            </span>
                            <span className="block text-[11px] text-slate-400 truncate max-w-[150px]">
                              {item.lastSourceType === 'new' ? '🟢 Bình mới' : '🔵 Bình bơm'}
                              {item.lastCartridgeName ? ` · ${item.lastCartridgeName}` : ''}
                            </span>
                          </div>
                        ) : (
                          <span className="text-slate-600">—</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : (
        /* TAB 3: NHẬT KÝ CHI TIẾT THAY MỰC */
        <div className="glass-panel overflow-hidden">
          <div className="p-6 border-b border-slate-800 flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Lịch sử sự kiện</p>
              <h3 className="text-lg font-bold text-slate-100">Chi tiết từng lần thay mực máy in</h3>
            </div>
            <span className="text-xs text-slate-400 font-semibold bg-slate-800/80 px-3 py-1 rounded-full border border-slate-700">
              {filteredLogs.length} lần thay
            </span>
          </div>

          {filteredLogs.length === 0 ? (
            <div className="p-12 text-center text-slate-400 text-sm">
              Không có sự kiện thay mực nào phù hợp với bộ lọc đã chọn.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse text-xs sm:text-sm">
                <thead>
                  <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                    <th className="p-4 pl-6">Thời gian</th>
                    <th className="p-4">Phòng ban</th>
                    <th className="p-4">Máy in</th>
                    <th className="p-4">Hộp mực (SKU)</th>
                    <th className="p-4">Nguồn mực</th>
                    <th className="p-4 text-right">Mốc bộ đếm</th>
                    <th className="p-4 text-right">Sản lượng in (Yield)</th>
                    <th className="p-4 pr-6">Ghi chú</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60">
                  {filteredLogs.map((log) => {
                    const printer = log.device_id ? printerMap.get(log.device_id) : undefined
                    const dept = (printer?.department || '').trim() || 'Chưa phân phòng'
                    const cartridge = cartridgeMap.get(log.cartridge_id)
                    return (
                      <tr key={log.id} className="hover:bg-slate-800/40 transition-colors">
                        <td className="p-4 pl-6 text-slate-300 font-mono text-xs whitespace-nowrap">
                          {new Date(log.performed_at).toLocaleString('vi-VN')}
                        </td>
                        <td className="p-4 font-semibold text-slate-300">
                          <span className="inline-flex items-center gap-1 bg-slate-800/60 px-2 py-0.5 rounded text-xs">
                            <Buildings size={12} className="text-cyan-400" />
                            {dept}
                          </span>
                        </td>
                        <td className="p-4">
                          {printer ? (
                            <Link
                              to={`/printers/${printer.id}`}
                              className="font-bold text-slate-200 hover:text-cyan-400 transition-colors"
                            >
                              {printer.display_name}
                            </Link>
                          ) : (
                            <span className="text-slate-500">—</span>
                          )}
                        </td>
                        <td className="p-4">
                          <span className="font-semibold text-slate-200 block">{cartridge?.name || 'Hộp mực'}</span>
                          <small className="text-cyan-400 font-mono text-xs">
                            {cartridge?.sku_code || log.cartridge_id}
                          </small>
                        </td>
                        <td className="p-4 font-semibold text-xs">
                          {log.source_type === 'new' ? (
                            <span className="text-emerald-400 inline-flex items-center gap-1">🟢 Bình mới 100%</span>
                          ) : (
                            <span className="text-cyan-400 inline-flex items-center gap-1">🔵 Bình đã bơm</span>
                          )}
                        </td>
                        <td className="p-4 text-right font-mono text-slate-400 text-xs">
                          {log.page_count && log.page_count > 0 ? log.page_count.toLocaleString('vi-VN') : '—'}
                        </td>
                        <td className="p-4 text-right font-mono text-xs font-bold text-purple-300">
                          {log.printed_pages && log.printed_pages > 0 ? (
                            <span>⚡ {log.printed_pages.toLocaleString('vi-VN')} trg</span>
                          ) : (
                            <span className="text-slate-600 font-normal">Đang dùng</span>
                          )}
                        </td>
                        <td className="p-4 pr-6 text-slate-400 text-xs max-w-xs truncate" title={log.notes || ''}>
                          {log.notes || '—'}
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
    </div>
  )
}
