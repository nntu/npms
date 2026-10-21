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
import type { Borders } from 'exceljs'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { listCartridgeLogs, listCartridges } from '../../api/client'
import type { Cartridge, CartridgeLog, Printer } from '../../api/types'
import { showError, showSuccess } from '../../utils/sweetalert'
import { useDepartments } from '../departments/useDepartments'
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

export function CartridgeReportPage() {
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
  const departmentsQuery = useDepartments()
  const departmentsList = useMemo(() => departmentsQuery.data?.data ?? [], [departmentsQuery.data?.data])
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
    for (const d of departmentsList) {
      if (d.name) set.add(d.name)
    }
    for (const p of printers) {
      const dept = (p.department || '').trim() || 'Chưa phân phòng'
      set.add(dept)
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b, 'vi'))
  }, [departmentsList, printers])

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

    return rawLogs.filter((log) => {
      // Only replace actions count as printer consumption
      if (log.action_type !== 'replace') return false

      // Date filter
      if (startDate) {
        const performed = new Date(log.performed_at)
        if (performed < startDate) return false
      }

      // Source filter (mực mới vs mực bơm)
      if (sourceFilter !== 'all' && log.source_type !== sourceFilter) {
        return false
      }

      // Department filter
      const printer = log.device_id ? printerMap.get(log.device_id) : undefined
      const dept = (printer?.department || '').trim() || 'Chưa phân phòng'
      if (departmentFilter !== 'all' && dept !== departmentFilter) {
        return false
      }

      // Search query
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase().trim()
        const cartridge = cartridgeMap.get(log.cartridge_id)
        const matchPrinter =
          printer?.display_name.toLowerCase().includes(q) || printer?.model?.toLowerCase().includes(q)
        const matchCartridge =
          cartridge?.name.toLowerCase().includes(q) || cartridge?.sku_code.toLowerCase().includes(q)
        const matchDept = dept.toLowerCase().includes(q)
        const matchNotes = (log.notes || '').toLowerCase().includes(q)
        if (!matchPrinter && !matchCartridge && !matchDept && !matchNotes) {
          return false
        }
      }

      return true
    })
  }, [rawLogs, timeRange, sourceFilter, departmentFilter, searchQuery, printerMap, cartridgeMap])

  // Thống kê theo phòng ban
  const departmentStats = useMemo(() => {
    const statsMap = new Map<string, DepartmentStat>()

    // Initialize departments with their printers
    for (const p of printers) {
      const dept = (p.department || '').trim() || 'Chưa phân phòng'
      if (!statsMap.has(dept)) {
        statsMap.set(dept, {
          department: dept,
          printers: [],
          logs: [],
          totalNew: 0,
          totalRefilled: 0,
          totalCartridges: 0,
          refillRate: 0,
          totalPrintedPages: 0,
          avgYield: 0,
        })
      }
      statsMap.get(dept)?.printers.push(p)
    }

    // Accumulate logs into departments
    for (const log of filteredLogs) {
      const printer = log.device_id ? printerMap.get(log.device_id) : undefined
      const dept = (printer?.department || '').trim() || 'Chưa phân phòng'

      let stat = statsMap.get(dept)
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
        statsMap.set(dept, stat)
      }

      stat.logs.push(log)
      const qty = log.quantity || 1
      if (log.source_type === 'new') {
        stat.totalNew += qty
      } else if (log.source_type === 'refilled') {
        stat.totalRefilled += qty
      }
      stat.totalCartridges += qty
      stat.totalPrintedPages += log.printed_pages || 0

      // Track most recent replacement
      if (!stat.lastReplacedAt || new Date(log.performed_at) > new Date(stat.lastReplacedAt)) {
        stat.lastReplacedAt = log.performed_at
        stat.lastPrinterName = printer?.display_name
        stat.lastCartridgeName = cartridgeMap.get(log.cartridge_id)?.name
      }
    }

    // Calculate rates and averages
    const result: DepartmentStat[] = []
    for (const stat of statsMap.values()) {
      stat.refillRate = stat.totalCartridges > 0 ? Math.round((stat.totalRefilled / stat.totalCartridges) * 100) : 0
      stat.avgYield = stat.totalCartridges > 0 ? Math.round(stat.totalPrintedPages / stat.totalCartridges) : 0

      // Filter by department if selected
      if (departmentFilter !== 'all' && stat.department !== departmentFilter) {
        continue
      }
      result.push(stat)
    }

    // Sort by total cartridges consumed desc
    return result.sort((a, b) => b.totalCartridges - a.totalCartridges)
  }, [printers, filteredLogs, printerMap, cartridgeMap, departmentFilter])

  // Thống kê theo từng máy in
  const printerStats = useMemo(() => {
    const list: PrinterStat[] = []

    for (const printer of printers) {
      const dept = (printer.department || '').trim() || 'Chưa phân phòng'
      if (departmentFilter !== 'all' && dept !== departmentFilter) {
        continue
      }

      const pLogs = filteredLogs.filter((l) => l.device_id === printer.id)
      let totalNew = 0
      let totalRefilled = 0
      let totalPrintedPages = 0
      let lastReplacedAt: string | undefined
      let lastCartridgeName: string | undefined
      let lastSourceType: string | undefined

      for (const log of pLogs) {
        const qty = log.quantity || 1
        if (log.source_type === 'new') totalNew += qty
        else if (log.source_type === 'refilled') totalRefilled += qty
        totalPrintedPages += log.printed_pages || 0

        if (!lastReplacedAt || new Date(log.performed_at) > new Date(lastReplacedAt)) {
          lastReplacedAt = log.performed_at
          lastCartridgeName = cartridgeMap.get(log.cartridge_id)?.name
          lastSourceType = log.source_type ?? undefined
        }
      }

      const totalCartridges = totalNew + totalRefilled
      const refillRate = totalCartridges > 0 ? Math.round((totalRefilled / totalCartridges) * 100) : 0
      const avgYield = totalCartridges > 0 ? Math.round(totalPrintedPages / totalCartridges) : 0

      // Search filter for printer table
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase().trim()
        const matchName = printer.display_name.toLowerCase().includes(q)
        const matchModel = (printer.model || '').toLowerCase().includes(q)
        const matchIP = (printer.address || '').toLowerCase().includes(q)
        const matchDept = dept.toLowerCase().includes(q)
        if (!matchName && !matchModel && !matchIP && !matchDept) {
          continue
        }
      }

      list.push({
        printer,
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

    return list.sort((a, b) => b.totalCartridges - a.totalCartridges)
  }, [printers, filteredLogs, departmentFilter, cartridgeMap, searchQuery])

  // KPIs tổng hợp
  const kpis = useMemo(() => {
    let grandTotalNew = 0
    let grandTotalRefilled = 0
    let grandTotalPages = 0
    let pagesFromNew = 0
    let countNewWithYield = 0
    let pagesFromRefilled = 0
    let countRefilledWithYield = 0

    for (const log of filteredLogs) {
      const qty = log.quantity || 1
      if (log.source_type === 'new') {
        grandTotalNew += qty
        if (log.printed_pages && log.printed_pages > 0) {
          pagesFromNew += log.printed_pages
          countNewWithYield += qty
        }
      } else if (log.source_type === 'refilled') {
        grandTotalRefilled += qty
        if (log.printed_pages && log.printed_pages > 0) {
          pagesFromRefilled += log.printed_pages
          countRefilledWithYield += qty
        }
      }
      grandTotalPages += log.printed_pages || 0
    }

    const grandTotalCartridges = grandTotalNew + grandTotalRefilled
    const overallRefillRate =
      grandTotalCartridges > 0 ? Math.round((grandTotalRefilled / grandTotalCartridges) * 100) : 0
    const avgYieldNew = countNewWithYield > 0 ? Math.round(pagesFromNew / countNewWithYield) : 0
    const avgYieldRefilled = countRefilledWithYield > 0 ? Math.round(pagesFromRefilled / countRefilledWithYield) : 0

    const topDept = departmentStats.length > 0 && departmentStats[0].totalCartridges > 0 ? departmentStats[0] : null

    return {
      grandTotalNew,
      grandTotalRefilled,
      grandTotalCartridges,
      overallRefillRate,
      grandTotalPages,
      avgYieldNew,
      avgYieldRefilled,
      topDept,
    }
  }, [filteredLogs, departmentStats])

  // Xuất file Excel báo cáo bình mực
  const handleExportExcel = async () => {
    setIsExporting(true)
    try {
      const { default: ExcelJS } = await import('exceljs')
      const workbook = new ExcelJS.Workbook()
      workbook.creator = 'NPMS LAN Printer Platform'
      workbook.created = new Date()

      const headerFont = { bold: true, color: { argb: 'FFFFFFFF' }, size: 10 }
      const thinBorder: Partial<Borders> = {
        top: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        bottom: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        left: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        right: { style: 'thin', color: { argb: 'FFCBD5E1' } },
      }

      // Sheet 1: Báo cáo theo phòng ban
      const deptSheet = workbook.addWorksheet('Theo Phòng Ban', { views: [{ showGridLines: true }] })
      deptSheet.columns = [
        { header: 'STT', key: 'stt', width: 8 },
        { header: 'Phòng ban', key: 'department', width: 28 },
        { header: 'Số máy in', key: 'printer_count', width: 14 },
        { header: 'Bình MỚI', key: 'new', width: 16 },
        { header: 'Bình ĐÃ BƠM', key: 'refilled', width: 16 },
        { header: 'Tổng bình đã dùng', key: 'total', width: 20 },
        { header: 'Tỷ lệ bơm tái sử dụng', key: 'rate', width: 22 },
        { header: 'Tổng trang in đạt được', key: 'pages', width: 24 },
        { header: 'TB trang/bình (Yield)', key: 'yield', width: 22 },
        { header: 'Lần thay gần nhất', key: 'last_date', width: 22 },
        { header: 'Máy thay gần nhất', key: 'last_printer', width: 26 },
      ]

      const deptHeaderRow = deptSheet.getRow(1)
      deptHeaderRow.height = 28
      deptHeaderRow.eachCell((cell) => {
        cell.font = headerFont
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF0284C7' } }
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

      // Sheet 2: Theo từng máy in
      const printerSheet = workbook.addWorksheet('Theo Từng Máy In', { views: [{ showGridLines: true }] })
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
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF059669' } }
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

      // Sheet 3: Nhật ký thay mực
      const logSheet = workbook.addWorksheet('Nhật Ký Thay Mực', { views: [{ showGridLines: true }] })
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
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F46E5' } }
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

      showSuccess('Xuất file Excel báo cáo bình mực thành công!')
    } catch (err) {
      console.error(err)
      showError(`Không thể xuất file Excel: ${err instanceof Error ? err.message : String(err)}`)
    } finally {
      setIsExporting(false)
    }
  }

  const handlePrint = () => {
    window.print()
  }

  const isLoading = printersQuery.isLoading || cartridgesQuery.isLoading || logsQuery.isLoading

  return (
    <div className="space-y-6 pb-12">
      {/* Top Banner */}
      <div className="glass-panel p-6 sm:p-8 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6 relative overflow-hidden bg-white border border-slate-200 shadow-2xs">
        <div className="space-y-2 max-w-2xl z-10">
          <div className="flex items-center gap-2 text-cyan-700 text-xs uppercase font-extrabold tracking-widest">
            <Drop size={16} weight="bold" />
            <span>Quản lý tiêu thụ vật tư in</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            Báo cáo sử dụng & tiêu thụ bình mực
          </h1>
          <p className="text-slate-600 text-sm leading-relaxed">
            Phân tích số lượng bình mực mới và bình đã bơm, tỷ lệ tiết kiệm tái sử dụng, hiệu suất trang in trung bình
            trên mỗi bình mực (Yield) và nhật ký thay thế vật tư toàn diện.
          </p>
        </div>

        {/* Action Buttons & Quick Nav */}
        <div className="flex flex-wrap items-center gap-2.5 z-10 w-full sm:w-auto">
          <Link
            to="/reports/pages"
            className="flex-1 sm:flex-none bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm border border-emerald-200 transition-all flex items-center justify-center gap-2 shadow-2xs"
          >
            <ChartBar size={18} weight="bold" />
            <span>Xem Báo cáo trang in</span>
          </Link>
          <button
            type="button"
            disabled={isExporting}
            onClick={() => void handleExportExcel()}
            className="flex-1 sm:flex-none bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm shadow-2xs transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <FileXls size={18} weight="bold" />
            <span>{isExporting ? 'Đang tạo Excel...' : 'Xuất Excel bình mực'}</span>
          </button>
          <button
            type="button"
            onClick={handlePrint}
            className="flex-1 sm:flex-none bg-white hover:bg-slate-50 text-slate-700 font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm border border-slate-200 transition-all flex items-center justify-center gap-2 cursor-pointer shadow-2xs"
          >
            <DownloadSimple size={18} />
            <span>In báo cáo</span>
          </button>
        </div>
      </div>

      {/* KPI Overview Cards for Cartridges */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 print:grid-cols-4">
        {/* Total Cartridges Used */}
        <div className="glass-card p-5 border-l-4 border-l-cyan-600 bg-white border border-slate-200 shadow-2xs flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-500">Tổng bình mực đã dùng</p>
              <h3 className="text-2xl font-black text-slate-900 mt-1">{kpis.grandTotalCartridges} bình</h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-cyan-50 border border-cyan-200 text-cyan-600 flex items-center justify-center">
              <Drop size={26} />
            </div>
          </div>
          <div className="mt-3 pt-3 border-t border-slate-200 flex items-center justify-between text-xs">
            <span className="text-emerald-700 font-semibold">🟢 Mới: {kpis.grandTotalNew}</span>
            <span className="text-cyan-700 font-semibold">🔵 Bơm: {kpis.grandTotalRefilled}</span>
          </div>
        </div>

        {/* Refill Saving Rate */}
        <div className="glass-card p-5 border-l-4 border-l-emerald-600 bg-white border border-slate-200 shadow-2xs flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-500">Tỷ lệ tái sử dụng (Bơm)</p>
              <h3 className="text-2xl font-black text-emerald-700 mt-1">{kpis.overallRefillRate}%</h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-emerald-50 border border-emerald-200 text-emerald-600 flex items-center justify-center">
              <Recycle size={26} />
            </div>
          </div>
          <p className="text-xs text-slate-600 mt-3 pt-3 border-t border-slate-200">
            {kpis.overallRefillRate > 50
              ? 'Tiết kiệm chi phí vật tư rất tốt nhờ tái sử dụng vỏ bình'
              : 'Nên ưu tiên bơm mực tái sử dụng để tối ưu ngân sách'}
          </p>
        </div>

        {/* Total Pages Printed */}
        <div className="glass-card p-5 border-l-4 border-l-purple-600 bg-white border border-slate-200 shadow-2xs flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-500">Sản lượng in ghi nhận</p>
              <h3 className="text-2xl font-black text-purple-700 mt-1">
                {kpis.grandTotalPages.toLocaleString('vi-VN')} trang
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-purple-50 border border-purple-200 text-purple-600 flex items-center justify-center">
              <Sparkle size={26} />
            </div>
          </div>
          <div className="mt-3 pt-3 border-t border-slate-200 flex items-center justify-between text-xs text-slate-600">
            <span>
              TB Bình Mới:{' '}
              <strong className="text-emerald-700">
                {kpis.avgYieldNew ? `${kpis.avgYieldNew.toLocaleString('vi-VN')} trg` : '—'}
              </strong>
            </span>
            <span>
              TB Bình Bơm:{' '}
              <strong className="text-cyan-700">
                {kpis.avgYieldRefilled ? `${kpis.avgYieldRefilled.toLocaleString('vi-VN')} trg` : '—'}
              </strong>
            </span>
          </div>
        </div>

        {/* Top Department */}
        <div className="glass-card p-5 border-l-4 border-l-amber-500 bg-white border border-slate-200 shadow-2xs flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-500">Phòng dùng nhiều nhất</p>
              <h3
                className="text-lg font-black text-amber-700 mt-1 truncate max-w-[170px]"
                title={kpis.topDept?.department || '—'}
              >
                {kpis.topDept ? kpis.topDept.department : 'Chưa có'}
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-amber-50 border border-amber-200 text-amber-600 flex items-center justify-center">
              <Buildings size={26} />
            </div>
          </div>
          <p className="text-xs text-slate-600 mt-3 pt-3 border-t border-slate-200">
            {kpis.topDept ? (
              <span>
                Đã dùng <strong className="text-slate-900">{kpis.topDept.totalCartridges}</strong> bình mực (
                {kpis.grandTotalCartridges > 0
                  ? Math.round((kpis.topDept.totalCartridges / kpis.grandTotalCartridges) * 100)
                  : 0}
                %)
              </span>
            ) : (
              'Chưa có dữ liệu thay mực'
            )}
          </p>
        </div>
      </div>

      {/* Filter and Control Toolbar */}
      <div className="glass-panel p-5 bg-white border border-slate-200 space-y-4 shadow-2xs print:hidden">
        <div className="flex flex-wrap items-center justify-between gap-4">
          {/* Navigation Tabs */}
          <div className="flex items-center bg-slate-100 p-1 rounded-xl border border-slate-200">
            <button
              type="button"
              onClick={() => setActiveTab('department')}
              className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
                activeTab === 'department' ? 'bg-cyan-600 text-white shadow-2xs' : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              <Buildings size={16} weight="bold" />
              <span>Mực theo phòng ({departmentStats.length})</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('printer')}
              className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
                activeTab === 'printer' ? 'bg-cyan-600 text-white shadow-2xs' : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              <PrinterIcon size={16} weight="bold" />
              <span>Mực theo máy ({printerStats.length})</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('logs')}
              className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
                activeTab === 'logs' ? 'bg-cyan-600 text-white shadow-2xs' : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              <Drop size={16} weight="bold" />
              <span>Nhật ký thay mực ({filteredLogs.length})</span>
            </button>
          </div>

          {/* Quick Search */}
          <div className="relative flex-1 sm:max-w-xs">
            <MagnifyingGlass size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Tìm theo máy, phòng, SKU..."
              className="w-full bg-white border border-slate-200 rounded-xl pl-9 pr-3 py-1.5 text-xs text-slate-800 placeholder:text-slate-400 focus:outline-none focus:border-cyan-500 transition-all shadow-2xs"
            />
          </div>
        </div>

        {/* Filter Controls Row */}
        <div className="flex flex-wrap items-center gap-3 pt-2 border-t border-slate-200 text-xs">
          {/* Time range selector */}
          <div className="flex items-center gap-1.5 bg-slate-50 px-3 py-1.5 rounded-xl border border-slate-200">
            <CalendarBlank size={16} className="text-cyan-600" />
            <span className="text-slate-600 font-medium">Thời gian:</span>
            <select
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value as TimeRangeOption)}
              className="bg-transparent text-slate-800 font-semibold focus:outline-none cursor-pointer"
            >
              <option value="all" className="bg-white text-slate-800">
                Toàn bộ lịch sử
              </option>
              <option value="7d" className="bg-white text-slate-800">
                7 ngày qua
              </option>
              <option value="30d" className="bg-white text-slate-800">
                30 ngày qua
              </option>
              <option value="this_month" className="bg-white text-slate-800">
                Tháng này
              </option>
              <option value="this_quarter" className="bg-white text-slate-800">
                Quý này
              </option>
              <option value="this_year" className="bg-white text-slate-800">
                Năm nay
              </option>
            </select>
          </div>

          {/* Department Filter */}
          <div className="flex items-center gap-1.5 bg-slate-50 px-3 py-1.5 rounded-xl border border-slate-200">
            <Funnel size={16} className="text-cyan-600" />
            <span className="text-slate-600 font-medium">Phòng ban:</span>
            <select
              value={departmentFilter}
              onChange={(e) => setDepartmentFilter(e.target.value)}
              className="bg-transparent text-slate-800 font-semibold focus:outline-none cursor-pointer"
            >
              <option value="all" className="bg-white text-slate-800">
                Tất cả phòng ban
              </option>
              {departmentList.map((d) => (
                <option key={d} value={d} className="bg-white text-slate-800">
                  {d}
                </option>
              ))}
            </select>
          </div>

          {/* Source Type Filter */}
          <div className="flex items-center gap-1.5 bg-slate-50 px-3 py-1.5 rounded-xl border border-slate-200">
            <span className="text-slate-600 font-medium">Nguồn mực:</span>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setSourceFilter('all')}
                className={`px-2 py-0.5 rounded-md font-semibold transition-colors cursor-pointer ${
                  sourceFilter === 'all'
                    ? 'bg-cyan-100 text-cyan-800 border border-cyan-300'
                    : 'text-slate-600 hover:text-slate-900'
                }`}
              >
                Tất cả
              </button>
              <button
                type="button"
                onClick={() => setSourceFilter('new')}
                className={`px-2 py-0.5 rounded-md font-semibold transition-colors cursor-pointer ${
                  sourceFilter === 'new'
                    ? 'bg-emerald-100 text-emerald-800 border border-emerald-300'
                    : 'text-slate-600 hover:text-slate-900'
                }`}
              >
                Mới 100%
              </button>
              <button
                type="button"
                onClick={() => setSourceFilter('refilled')}
                className={`px-2 py-0.5 rounded-md font-semibold transition-colors cursor-pointer ${
                  sourceFilter === 'refilled'
                    ? 'bg-cyan-100 text-cyan-800 border border-cyan-300'
                    : 'text-slate-600 hover:text-slate-900'
                }`}
              >
                Đã bơm
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Main Content Area */}
      {isLoading ? (
        <div className="glass-panel p-16 text-center text-slate-500 flex flex-col items-center justify-center gap-3 bg-white border border-slate-200 shadow-2xs">
          <ArrowCounterClockwise size={32} className="animate-spin text-cyan-600" />
          <p className="text-sm">Đang nạp dữ liệu thống kê sử dụng mực...</p>
        </div>
      ) : activeTab === 'department' ? (
        /* TAB 1: THEO PHÒNG BAN */
        <div className="space-y-6">
          {/* Visual Distribution Chart */}
          {departmentStats.some((d) => d.totalCartridges > 0) && (
            <div className="glass-panel p-5 bg-white border border-slate-200 shadow-2xs">
              <h4 className="text-xs font-bold uppercase tracking-wider text-cyan-700 mb-3">
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
                          <span className="font-semibold text-slate-800 flex items-center gap-1.5">
                            <Buildings size={14} className="text-cyan-600" />
                            {d.department}
                          </span>
                          <span className="text-slate-600">
                            <strong className="text-slate-900">{d.totalCartridges}</strong> bình ({percent}%) · 🟢{' '}
                            {d.totalNew} · 🔵 {d.totalRefilled}
                          </span>
                        </div>
                        <div className="w-full h-2.5 bg-slate-100 rounded-full overflow-hidden flex">
                          <div
                            className="bg-emerald-500 h-full transition-all duration-500"
                            style={{
                              width: `${d.totalCartridges > 0 ? (d.totalNew / d.totalCartridges) * percent : 0}%`,
                            }}
                            title={`Bình mới: ${d.totalNew}`}
                          />
                          <div
                            className="bg-cyan-500 h-full transition-all duration-500"
                            style={{
                              width: `${d.totalCartridges > 0 ? (d.totalRefilled / d.totalCartridges) * percent : 0}%`,
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
          <div className="glass-panel overflow-hidden bg-white border border-slate-200 shadow-2xs">
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="border-b border-slate-200 bg-slate-50/90 text-slate-600 font-bold uppercase tracking-wider text-[11px]">
                    <th className="py-3 px-4">Phòng ban</th>
                    <th className="py-3 px-3 text-center">Số máy</th>
                    <th className="py-3 px-3 text-center">Mới</th>
                    <th className="py-3 px-3 text-center">Đã bơm</th>
                    <th className="py-3 px-3 text-center">Tổng bình</th>
                    <th className="py-3 px-3 text-center">Tỷ lệ bơm</th>
                    <th className="py-3 px-4 text-right">Tổng trang in</th>
                    <th className="py-3 px-4 text-right">TB trang/bình</th>
                    <th className="py-3 px-4 text-right">Lần thay gần nhất</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200/60">
                  {departmentStats.map((stat) => {
                    const isExpanded = expandedDept === stat.department
                    return (
                      <tbody key={stat.department} className="border-b border-slate-200/60">
                        <tr
                          onClick={() => setExpandedDept(isExpanded ? null : stat.department)}
                          className="hover:bg-slate-50/80 transition-colors cursor-pointer"
                        >
                          <td className="py-3.5 px-4 font-semibold text-slate-900 flex items-center gap-2">
                            {isExpanded ? (
                              <CaretDown size={14} className="text-cyan-600" />
                            ) : (
                              <CaretRight size={14} className="text-slate-400" />
                            )}
                            <span>{stat.department}</span>
                          </td>
                          <td className="py-3.5 px-3 text-center text-slate-600">{stat.printers.length} máy</td>
                          <td className="py-3.5 px-3 text-center font-bold text-emerald-700">{stat.totalNew}</td>
                          <td className="py-3.5 px-3 text-center font-bold text-cyan-700">{stat.totalRefilled}</td>
                          <td className="py-3.5 px-3 text-center">
                            <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-700 font-bold border border-slate-200">
                              {stat.totalCartridges}
                            </span>
                          </td>
                          <td className="py-3.5 px-3 text-center">
                            <span
                              className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                                stat.refillRate > 60
                                  ? 'bg-emerald-50 text-emerald-800 border border-emerald-200'
                                  : stat.refillRate > 30
                                    ? 'bg-cyan-50 text-cyan-800 border border-cyan-200'
                                    : 'bg-slate-100 text-slate-600 border border-slate-200'
                              }`}
                            >
                              {stat.refillRate}%
                            </span>
                          </td>
                          <td className="py-3.5 px-4 text-right font-black text-purple-700">
                            {stat.totalPrintedPages.toLocaleString('vi-VN')}
                          </td>
                          <td className="py-3.5 px-4 text-right font-semibold text-slate-700">
                            {stat.avgYield > 0 ? `${stat.avgYield.toLocaleString('vi-VN')} trg` : '—'}
                          </td>
                          <td className="py-3.5 px-4 text-right text-slate-600">
                            {stat.lastReplacedAt ? (
                              <div>
                                <span>{new Date(stat.lastReplacedAt).toLocaleDateString('vi-VN')}</span>
                                {stat.lastPrinterName && (
                                  <span className="block text-[10px] text-slate-400 truncate max-w-[150px] ml-auto">
                                    {stat.lastPrinterName}
                                  </span>
                                )}
                              </div>
                            ) : (
                              '—'
                            )}
                          </td>
                        </tr>

                        {/* Collapsible Drawer: Printers inside this Department */}
                        {isExpanded && (
                          <tr className="bg-slate-50/80">
                            <td colSpan={9} className="p-4 pl-10 border-t border-slate-200">
                              <h5 className="text-[11px] font-bold uppercase tracking-wider text-slate-600 mb-2">
                                Danh sách máy in thuộc phòng: {stat.department} ({stat.printers.length} máy)
                              </h5>
                              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                                {stat.printers.map((p) => {
                                  const pStat = printerStats.find((s) => s.printer.id === p.id)
                                  return (
                                    <div
                                      key={p.id}
                                      className="p-3 rounded-xl bg-white border border-slate-200 space-y-1.5 shadow-2xs"
                                    >
                                      <div className="flex items-center justify-between">
                                        <span className="font-semibold text-slate-800 text-xs truncate max-w-[180px]">
                                          {p.display_name}
                                        </span>
                                        <span className="text-[10px] font-mono text-cyan-700">{p.address}</span>
                                      </div>
                                      <div className="text-[11px] text-slate-600 flex items-center justify-between">
                                        <span>
                                          Đã dùng:{' '}
                                          <strong className="text-slate-900">{pStat?.totalCartridges ?? 0}</strong> bình
                                        </span>
                                        <span className="text-emerald-700">🟢 {pStat?.totalNew ?? 0}</span>
                                        <span className="text-cyan-700">🔵 {pStat?.totalRefilled ?? 0}</span>
                                      </div>
                                    </div>
                                  )
                                })}
                              </div>
                            </td>
                          </tr>
                        )}
                      </tbody>
                    )
                  })}
                  {departmentStats.length === 0 && (
                    <tr>
                      <td colSpan={9} className="py-8 text-center text-slate-500">
                        Không có dữ liệu tiêu thụ mực cho phòng ban nào trong khoảng thời gian đã chọn.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      ) : activeTab === 'printer' ? (
        /* TAB 2: THEO TỪNG MÁY IN */
        <div className="glass-panel overflow-hidden bg-white border border-slate-200 shadow-2xs">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50/90 text-slate-600 font-bold uppercase tracking-wider text-[11px]">
                  <th className="py-3 px-4">Tên máy in / Dòng máy</th>
                  <th className="py-3 px-4">Phòng ban</th>
                  <th className="py-3 px-3 text-center">Địa chỉ IP</th>
                  <th className="py-3 px-3 text-center">Bình mới</th>
                  <th className="py-3 px-3 text-center">Đã bơm</th>
                  <th className="py-3 px-3 text-center">Tổng bình</th>
                  <th className="py-3 px-3 text-center">Tỷ lệ bơm</th>
                  <th className="py-3 px-4 text-right">Tổng trang in</th>
                  <th className="py-3 px-4 text-right">TB trang/bình</th>
                  <th className="py-3 px-4 text-right">Lần thay gần nhất</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200/60">
                {printerStats.map((item) => (
                  <tr key={item.printer.id} className="hover:bg-slate-50/80 transition-colors">
                    <td className="py-3.5 px-4 font-semibold text-slate-900">
                      <div>
                        <span>{item.printer.display_name}</span>
                        {item.printer.model && (
                          <span className="block text-[10px] text-slate-500 font-normal mt-0.5">
                            {item.printer.manufacturer} {item.printer.model}
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="py-3.5 px-4 text-slate-700">{item.department}</td>
                    <td className="py-3.5 px-3 text-center font-mono text-cyan-700">{item.printer.address || '—'}</td>
                    <td className="py-3.5 px-3 text-center font-bold text-emerald-700">{item.totalNew}</td>
                    <td className="py-3.5 px-3 text-center font-bold text-cyan-700">{item.totalRefilled}</td>
                    <td className="py-3.5 px-3 text-center">
                      <span className="px-2 py-0.5 rounded-full bg-slate-100 text-slate-700 font-bold border border-slate-200">
                        {item.totalCartridges}
                      </span>
                    </td>
                    <td className="py-3.5 px-3 text-center">
                      <span
                        className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                          item.refillRate > 60
                            ? 'bg-emerald-50 text-emerald-800 border border-emerald-200'
                            : item.refillRate > 30
                              ? 'bg-cyan-50 text-cyan-800 border border-cyan-200'
                              : 'bg-slate-100 text-slate-600 border border-slate-200'
                        }`}
                      >
                        {item.refillRate}%
                      </span>
                    </td>
                    <td className="py-3.5 px-4 text-right font-black text-purple-700">
                      {item.totalPrintedPages.toLocaleString('vi-VN')}
                    </td>
                    <td className="py-3.5 px-4 text-right font-semibold text-slate-700">
                      {item.avgYield > 0 ? `${item.avgYield.toLocaleString('vi-VN')} trg` : '—'}
                    </td>
                    <td className="py-3.5 px-4 text-right text-slate-600">
                      {item.lastReplacedAt ? (
                        <div>
                          <span>{new Date(item.lastReplacedAt).toLocaleDateString('vi-VN')}</span>
                          {item.lastCartridgeName && (
                            <span className="block text-[10px] text-cyan-700 truncate max-w-[150px] ml-auto">
                              {item.lastCartridgeName} ({item.lastSourceType === 'new' ? 'Mới' : 'Bơm'})
                            </span>
                          )}
                        </div>
                      ) : (
                        '—'
                      )}
                    </td>
                  </tr>
                ))}
                {printerStats.length === 0 && (
                  <tr>
                    <td colSpan={10} className="py-8 text-center text-slate-500">
                      Không tìm thấy máy in nào phù hợp với bộ lọc hiện tại.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        /* TAB 3: NHẬT KÝ THAY MỰC CHI TIẾT */
        <div className="glass-panel overflow-hidden bg-white border border-slate-200 shadow-2xs">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50/90 text-slate-600 font-bold uppercase tracking-wider text-[11px]">
                  <th className="py-3 px-4">Thời gian</th>
                  <th className="py-3 px-4">Máy in</th>
                  <th className="py-3 px-4">Phòng ban</th>
                  <th className="py-3 px-4">Hộp mực</th>
                  <th className="py-3 px-3 text-center">Nguồn mực</th>
                  <th className="py-3 px-3 text-center">Số lượng</th>
                  <th className="py-3 px-4 text-right">Bộ đếm lúc thay</th>
                  <th className="py-3 px-4 text-right">Sản lượng đạt được</th>
                  <th className="py-3 px-4">Ghi chú</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200/60">
                {filteredLogs.map((log) => {
                  const printer = log.device_id ? printerMap.get(log.device_id) : undefined
                  const dept = (printer?.department || '').trim() || 'Chưa phân phòng'
                  const cartridge = cartridgeMap.get(log.cartridge_id)

                  return (
                    <tr key={log.id} className="hover:bg-slate-50/80 transition-colors">
                      <td className="py-3.5 px-4 text-slate-600 font-mono text-[11px]">
                        {new Date(log.performed_at).toLocaleString('vi-VN')}
                      </td>
                      <td className="py-3.5 px-4 font-semibold text-slate-900">
                        <div>
                          <span>{printer?.display_name || 'Máy in đã xóa'}</span>
                          {printer?.address && (
                            <span className="block text-[10px] text-slate-500 font-mono font-normal">
                              {printer.address}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="py-3.5 px-4 text-slate-700">{dept}</td>
                      <td className="py-3.5 px-4 text-slate-900">
                        <span className="font-semibold">{cartridge?.name || 'Vật tư không xác định'}</span>
                        {cartridge?.sku_code && (
                          <span className="block text-[10px] text-slate-500 font-mono">{cartridge.sku_code}</span>
                        )}
                      </td>
                      <td className="py-3.5 px-3 text-center">
                        <span
                          className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${
                            log.source_type === 'new'
                              ? 'bg-emerald-50 text-emerald-800 border border-emerald-200'
                              : 'bg-cyan-50 text-cyan-800 border border-cyan-200'
                          }`}
                        >
                          {log.source_type === 'new' ? 'Bình mới' : 'Bình bơm'}
                        </span>
                      </td>
                      <td className="py-3.5 px-3 text-center font-bold text-slate-900">{log.quantity || 1}</td>
                      <td className="py-3.5 px-4 text-right font-mono text-slate-700">
                        {log.page_count ? log.page_count.toLocaleString('vi-VN') : '—'}
                      </td>
                      <td className="py-3.5 px-4 text-right font-black text-purple-700">
                        {log.printed_pages && log.printed_pages > 0
                          ? `${log.printed_pages.toLocaleString('vi-VN')} trang`
                          : '—'}
                      </td>
                      <td
                        className="py-3.5 px-4 text-slate-600 text-[11px] max-w-[200px] truncate"
                        title={log.notes || ''}
                      >
                        {log.notes || '—'}
                      </td>
                    </tr>
                  )
                })}
                {filteredLogs.length === 0 && (
                  <tr>
                    <td colSpan={9} className="py-8 text-center text-slate-500">
                      Không có bản ghi thay mực nào phù hợp với bộ lọc hiện tại.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
