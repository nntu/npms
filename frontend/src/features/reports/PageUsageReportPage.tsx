import { CalendarBlank, ChartBar, DownloadSimple, Drop, FileXls, Funnel, MagnifyingGlass } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import type { Borders } from 'exceljs'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { getPageUsageReport } from '../../api/client'
import { showError, showSuccess } from '../../utils/sweetalert'
import { useDepartments } from '../departments/useDepartments'
import { usePrinters } from '../devices/usePrinters'
import { PageUsageReportView } from './PageUsageReportView'

type TimeRangeOption = 'all' | '7d' | '30d' | 'this_month' | 'this_quarter' | 'this_year'

export function PageUsageReportPage() {
  const printersQuery = usePrinters()
  const printers = useMemo(() => printersQuery.data?.data ?? [], [printersQuery.data])
  const departmentsQuery = useDepartments()
  const departmentsList = useMemo(() => departmentsQuery.data?.data ?? [], [departmentsQuery.data?.data])

  const [timeRange, setTimeRange] = useState<TimeRangeOption>('this_month')
  const [departmentFilter, setDepartmentFilter] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [isExporting, setIsExporting] = useState(false)

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

  const pageDateParams = useMemo(() => {
    const now = new Date()
    const to = now.toISOString().slice(0, 10)
    let from: string | undefined
    if (timeRange === '7d') {
      from = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    } else if (timeRange === '30d') {
      from = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    } else if (timeRange === 'this_month') {
      from = new Date(now.getFullYear(), now.getMonth(), 1).toISOString().slice(0, 10)
    } else if (timeRange === 'this_quarter') {
      const quarterMonth = Math.floor(now.getMonth() / 3) * 3
      from = new Date(now.getFullYear(), quarterMonth, 1).toISOString().slice(0, 10)
    } else if (timeRange === 'this_year') {
      from = new Date(now.getFullYear(), 0, 1).toISOString().slice(0, 10)
    }
    return { from, to, department: departmentFilter !== 'all' ? departmentFilter : undefined }
  }, [timeRange, departmentFilter])

  const pageUsageQuery = useQuery({
    queryKey: ['page-usage-report', pageDateParams],
    queryFn: () => getPageUsageReport(pageDateParams),
  })

  // Export Page Usage Excel
  const handleExportPageUsageExcel = async () => {
    const reportData = pageUsageQuery.data
    if (!reportData) return
    setIsExporting(true)
    try {
      const { default: ExcelJS } = await import('exceljs')
      const workbook = new ExcelJS.Workbook()
      workbook.creator = 'NPMS LAN Printer Management'
      workbook.created = new Date()

      const headerFont = { bold: true, color: { argb: 'FFFFFFFF' }, size: 10 }
      const thinBorder: Partial<Borders> = {
        top: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        bottom: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        left: { style: 'thin', color: { argb: 'FFCBD5E1' } },
        right: { style: 'thin', color: { argb: 'FFCBD5E1' } },
      }

      // Sheet 1: Báo cáo theo phòng ban
      const deptSheet = workbook.addWorksheet('Trang In Theo Phòng Ban', {
        views: [{ showGridLines: true }],
      })
      deptSheet.columns = [
        { header: 'STT', key: 'stt', width: 8 },
        { header: 'Phòng ban', key: 'department', width: 28 },
        { header: 'Số thiết bị', key: 'printer_count', width: 14 },
        { header: 'Số trang in quy đổi (Trang)', key: 'print_pages', width: 26 },
        { header: 'Số trang quét (Trang scan)', key: 'scan_pages', width: 26 },
        { header: 'Mốc counter ban đầu', key: 'initial_life', width: 24 },
        { header: 'Tổng counter hiện tại', key: 'current_life', width: 24 },
        { header: 'Ghi chú quy đổi', key: 'note', width: 35 },
      ]

      const deptHeaderRow = deptSheet.getRow(1)
      deptHeaderRow.height = 28
      deptHeaderRow.eachCell((cell) => {
        cell.font = headerFont
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF059669' } }
        cell.alignment = { vertical: 'middle', horizontal: 'center', wrapText: true }
        cell.border = thinBorder
      })

      reportData.departments.forEach((dept, idx) => {
        const row = deptSheet.addRow({
          stt: idx + 1,
          department: dept.department,
          printer_count: dept.printer_count,
          print_pages: dept.total_print_pages,
          scan_pages: dept.total_scan_pages,
          initial_life: dept.total_initial_life,
          current_life: dept.total_current_life,
          note: dept.total_scan_pages > 0 ? 'Có máy quét tài liệu tách riêng' : 'Chuẩn trang in A4',
        })
        row.height = 20
        row.eachCell((cell) => {
          cell.border = thinBorder
          cell.alignment = { vertical: 'middle' }
        })
        row.getCell('stt').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('printer_count').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('print_pages').numFmt = '#,##0'
        row.getCell('scan_pages').numFmt = '#,##0'
        row.getCell('initial_life').numFmt = '#,##0'
        row.getCell('current_life').numFmt = '#,##0'
      })

      // Sheet 2: Chi tiết từng máy in
      const printerSheet = workbook.addWorksheet('Chi Tiết Từng Máy In', {
        views: [{ showGridLines: true }],
      })
      printerSheet.columns = [
        { header: 'STT', key: 'stt', width: 8 },
        { header: 'Tên máy in', key: 'name', width: 28 },
        { header: 'Phòng ban', key: 'department', width: 24 },
        { header: 'Địa chỉ IP', key: 'ip', width: 18 },
        { header: 'Hãng & Dòng máy', key: 'model', width: 24 },
        { header: 'Phân loại', key: 'cat', width: 16 },
        { header: 'Mốc ban đầu', key: 'initial', width: 20 },
        { header: 'Bộ đếm hiện tại', key: 'reading', width: 20 },
        { header: 'Sản lượng trong kỳ', key: 'usage', width: 22 },
        { header: 'Hệ số quy đổi', key: 'factor', width: 16 },
        { header: 'Trang in quy đổi', key: 'normalized', width: 22 },
        { header: 'Ghi chú', key: 'note', width: 30 },
      ]

      const printerHeaderRow = printerSheet.getRow(1)
      printerHeaderRow.height = 28
      printerHeaderRow.eachCell((cell) => {
        cell.font = headerFont
        cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF0D9488' } }
        cell.alignment = { vertical: 'middle', horizontal: 'center', wrapText: true }
        cell.border = thinBorder
      })

      reportData.printers.forEach((p, idx) => {
        const row = printerSheet.addRow({
          stt: idx + 1,
          name: p.display_name,
          department: p.department || '—',
          ip: p.address,
          model: `${p.manufacturer} ${p.model}`.trim() || '—',
          cat: p.category === 'scanner' ? 'Máy quét (Scan)' : 'Máy in (Print)',
          initial: p.initial_reading ?? 0,
          reading: p.current_reading ?? 0,
          usage: p.period_usage,
          factor: p.conversion_factor,
          normalized: p.normalized_pages,
          note: p.conversion_note,
        })
        row.height = 20
        row.eachCell((cell) => {
          cell.border = thinBorder
          cell.alignment = { vertical: 'middle' }
        })
        row.getCell('stt').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('cat').alignment = { vertical: 'middle', horizontal: 'center' }
        row.getCell('initial').numFmt = '#,##0'
        row.getCell('reading').numFmt = '#,##0'
        row.getCell('usage').numFmt = '#,##0'
        row.getCell('normalized').numFmt = '#,##0'
      })

      const dateStr = new Date().toISOString().slice(0, 10)
      const buffer = await workbook.xlsx.writeBuffer()
      const blob = new Blob([buffer], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `Bao_cao_san_luong_trang_in_${dateStr}.xlsx`)
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)

      showSuccess('Xuất báo cáo trang in Excel thành công!')
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

  return (
    <div className="space-y-6 pb-12">
      {/* Top Banner */}
      <div className="glass-panel p-6 sm:p-8 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6 relative overflow-hidden">
        <div className="space-y-2 max-w-2xl z-10">
          <div className="flex items-center gap-2 text-emerald-700 text-xs uppercase font-extrabold tracking-widest">
            <ChartBar size={16} weight="bold" />
            <span>Phân tích lưu lượng in ấn</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            Báo cáo số trang in của máy in
          </h1>
          <p className="text-slate-600 text-sm leading-relaxed">
            Thống kê sản lượng in ấn quy đổi và số trang quét (scanner) theo từng phòng ban và từng thiết bị từ chỉ số
            bộ đếm SNMP thời gian thực.
          </p>
        </div>

        {/* Action Buttons & Quick Nav */}
        <div className="flex flex-wrap items-center gap-2.5 z-10 w-full sm:w-auto">
          <Link
            to="/reports/cartridges"
            className="flex-1 sm:flex-none bg-white hover:bg-slate-50 text-cyan-800 font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm border border-cyan-200 transition-all flex items-center justify-center gap-2 shadow-2xs"
          >
            <Drop size={18} weight="bold" />
            <span>Xem Báo cáo bình mực</span>
          </Link>
          <button
            type="button"
            disabled={isExporting || pageUsageQuery.isLoading}
            onClick={() => void handleExportPageUsageExcel()}
            className="flex-1 sm:flex-none bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm shadow-md shadow-emerald-600/20 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <FileXls size={18} weight="bold" />
            <span>{isExporting ? 'Đang tạo Excel...' : 'Xuất Excel trang in'}</span>
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

      {/* Filter and Control Toolbar */}
      <div className="glass-panel p-5 bg-white border border-slate-200 space-y-4 print:hidden shadow-2xs">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-2 text-sm font-bold text-slate-900">
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse" />
            <span>Bộ lọc dữ liệu trang in</span>
          </div>

          {/* Quick Search */}
          <div className="relative flex-1 sm:max-w-xs">
            <MagnifyingGlass size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Tìm theo tên máy, IP, model..."
              className="w-full bg-white border border-slate-200 rounded-xl pl-9 pr-3 py-1.5 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:border-emerald-500 shadow-2xs transition-all"
            />
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3 pt-2 border-t border-slate-200 text-xs">
          {/* Time range selector */}
          <div className="flex items-center gap-1.5 bg-slate-50 px-3 py-1.5 rounded-xl border border-slate-200 shadow-2xs">
            <CalendarBlank size={16} className="text-emerald-600" />
            <span className="text-slate-600 font-medium">Khoảng thời gian:</span>
            <select
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value as TimeRangeOption)}
              className="bg-transparent text-slate-800 font-semibold focus:outline-none cursor-pointer"
            >
              <option value="all">Toàn bộ thời gian (Tổng counter)</option>
              <option value="7d">7 ngày qua</option>
              <option value="30d">30 ngày qua</option>
              <option value="this_month">Tháng này</option>
              <option value="this_quarter">Quý này</option>
              <option value="this_year">Năm nay</option>
            </select>
          </div>

          {/* Department Filter */}
          <div className="flex items-center gap-1.5 bg-slate-50 px-3 py-1.5 rounded-xl border border-slate-200 shadow-2xs">
            <Funnel size={16} className="text-cyan-700" />
            <span className="text-slate-600 font-medium">Phòng ban:</span>
            <select
              value={departmentFilter}
              onChange={(e) => setDepartmentFilter(e.target.value)}
              className="bg-transparent text-slate-800 font-semibold focus:outline-none cursor-pointer"
            >
              <option value="all">Tất cả phòng ban</option>
              {departmentList.map((d) => (
                <option key={d} value={d}>
                  {d}
                </option>
              ))}
            </select>
          </div>

          <div className="ml-auto text-xs text-slate-500">
            Số thiết bị ghi nhận:{' '}
            <strong className="text-emerald-700 font-mono font-bold">
              {pageUsageQuery.data?.total_printers ?? printers.length}
            </strong>{' '}
            máy
          </div>
        </div>
      </div>

      {/* Render Page Usage Report View Component */}
      <PageUsageReportView
        data={pageUsageQuery.data}
        isLoading={pageUsageQuery.isLoading}
        searchQuery={searchQuery}
        departmentFilter={departmentFilter}
        onExportExcel={() => void handleExportPageUsageExcel()}
        isExporting={isExporting}
      />
    </div>
  )
}
