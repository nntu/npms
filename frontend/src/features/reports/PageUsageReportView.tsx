import {
  Buildings,
  CaretDown,
  CaretRight,
  CheckCircle,
  FileXls,
  Info,
  Printer,
  Scan,
  Sparkle,
} from '@phosphor-icons/react'
import { useMemo, useState } from 'react'
import type { PageUsageReport } from '../../api/client'

interface PageUsageReportViewProps {
  data?: PageUsageReport
  isLoading: boolean
  searchQuery: string
  departmentFilter: string
  onExportExcel: () => void
  isExporting: boolean
}

export function PageUsageReportView({
  data,
  isLoading,
  searchQuery,
  departmentFilter,
  onExportExcel,
  isExporting,
}: PageUsageReportViewProps) {
  const [viewMode, setPageViewMode] = useState<'department' | 'printer'>('department')
  const [expandedDept, setExpandedDept] = useState<string | null>(null)

  // Filtered printers
  const filteredPrinters = useMemo(() => {
    if (!data?.printers) return []
    const q = searchQuery.trim().toLowerCase()
    return data.printers.filter((p) => {
      if (departmentFilter !== 'all' && p.department !== departmentFilter) return false
      if (q) {
        const text = [p.display_name, p.model, p.address, p.serial, p.department]
          .filter(Boolean)
          .join(' ')
          .toLowerCase()
        if (!text.includes(q)) return false
      }
      return true
    })
  }, [data?.printers, searchQuery, departmentFilter])

  // Filtered departments
  const filteredDepartments = useMemo(() => {
    if (!data?.departments) return []
    const q = searchQuery.trim().toLowerCase()
    return data.departments
      .filter((dept) => {
        if (departmentFilter !== 'all' && dept.department !== departmentFilter) return false
        if (q) {
          const matchDept = dept.department.toLowerCase().includes(q)
          const matchPrinter = dept.printers.some((p) =>
            [p.display_name, p.model, p.address, p.serial].filter(Boolean).join(' ').toLowerCase().includes(q),
          )
          if (!matchDept && !matchPrinter) return false
        }
        return true
      })
      .map((dept) => {
        if (!q) return dept
        return {
          ...dept,
          printers: dept.printers.filter((p) =>
            [p.display_name, p.model, p.address, p.serial].filter(Boolean).join(' ').toLowerCase().includes(q),
          ),
        }
      })
  }, [data?.departments, searchQuery, departmentFilter])

  // Top printing department
  // Top printing department
  const topDept = useMemo(() => {
    if (!data?.departments || data.departments.length === 0) return null
    return [...data.departments].sort((a, b) => b.total_print_pages - a.total_print_pages)[0]
  }, [data?.departments])

  if (isLoading) {
    return (
      <div className="glass-panel p-16 text-center text-slate-400 flex flex-col items-center justify-center gap-3">
        <div className="w-8 h-8 border-4 border-emerald-500 border-t-transparent rounded-full animate-spin" />
        <p className="text-sm">Đang tổng hợp báo cáo số trang in theo phòng và máy...</p>
      </div>
    )
  }

  if (!data || data.total_printers === 0) {
    return (
      <div className="glass-panel p-16 text-center text-slate-400 flex flex-col items-center justify-center gap-3">
        <Printer size={48} className="text-slate-600" />
        <p className="text-base font-semibold text-slate-300">Chưa có dữ liệu số trang in</p>
        <p className="text-xs text-slate-500 max-w-md">
          Chưa tìm thấy bản ghi số trang in cho các thiết bị trong khoảng thời gian đã chọn.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* KPI Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Total Print Pages */}
        <div className="glass-card p-5 border-l-4 border-l-emerald-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">
                Sản lượng in trong kỳ (Quy đổi)
              </p>
              <h3 className="text-2xl font-black text-emerald-400 mt-1">
                {data.total_print_pages.toLocaleString('vi-VN')} <span className="text-sm font-semibold">trang</span>
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-emerald-950/80 border border-emerald-800/60 text-emerald-400 flex items-center justify-center">
              <Printer size={26} weight="bold" />
            </div>
          </div>
          <p className="text-xs text-slate-400 mt-3 pt-3 border-t border-slate-800/80 flex items-center justify-between">
            <span>Tổng counter hiện tại:</span>
            <strong className="text-slate-200">{data.total_current_life.toLocaleString('vi-VN')} trang</strong>
          </p>
        </div>

        {/* Total Scan Pages */}
        <div className="glass-card p-5 border-l-4 border-l-cyan-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Trang quét tài liệu (Scan)</p>
              <h3 className="text-2xl font-black text-cyan-400 mt-1">
                {data.total_scan_pages.toLocaleString('vi-VN')} <span className="text-sm font-semibold">trang</span>
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-cyan-950/80 border border-cyan-800/60 text-cyan-400 flex items-center justify-center">
              <Scan size={26} weight="bold" />
            </div>
          </div>
          <p className="text-xs text-slate-400 mt-3 pt-3 border-t border-slate-800/80">
            Tách riêng từ dòng máy quét Brother ADS
          </p>
        </div>

        {/* Top Printing Department */}
        <div className="glass-card p-5 border-l-4 border-l-amber-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Phòng ban in nhiều nhất</p>
              <h3
                className="text-lg font-black text-amber-400 mt-1 truncate max-w-[170px]"
                title={topDept?.department || '—'}
              >
                {topDept ? topDept.department : '—'}
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-amber-950/80 border border-amber-800/60 text-amber-400 flex items-center justify-center">
              <Buildings size={26} weight="bold" />
            </div>
          </div>
          <p className="text-xs text-slate-400 mt-3 pt-3 border-t border-slate-800/80 flex items-center justify-between">
            <span>Sản lượng:</span>
            <strong className="text-amber-300">
              {topDept ? topDept.total_print_pages.toLocaleString('vi-VN') : 0} trang
            </strong>
          </p>
        </div>

        {/* Total Printers & Devices */}
        <div className="glass-card p-5 border-l-4 border-l-purple-500 flex flex-col justify-between">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Quy mô quản lý thiết bị</p>
              <h3 className="text-2xl font-black text-purple-400 mt-1">
                {data.total_printers} <span className="text-sm font-semibold">máy</span>
              </h3>
            </div>
            <div className="w-12 h-12 rounded-2xl bg-purple-950/80 border border-purple-800/60 text-purple-400 flex items-center justify-center">
              <Sparkle size={26} weight="bold" />
            </div>
          </div>
          <p className="text-xs text-slate-400 mt-3 pt-3 border-t border-slate-800/80 flex items-center justify-between">
            <span>Phân bổ qua:</span>
            <strong className="text-slate-200">{data.total_departments} phòng ban</strong>
          </p>
        </div>
      </div>

      {/* Unit Conversion Explanation Banner */}
      <div className="glass-panel p-4 bg-emerald-950/20 border border-emerald-800/40 rounded-xl flex items-start gap-3">
        <Info size={20} className="text-emerald-400 shrink-0 mt-0.5" weight="bold" />
        <div className="text-xs text-slate-300 space-y-1">
          <p className="font-bold text-emerald-300">Nguyên tắc quy đổi chuẩn hóa số trang in tại NPMS:</p>
          <ul className="list-disc list-inside space-y-0.5 text-slate-400">
            <li>
              <strong className="text-slate-200">Đơn vị Impressions (HP, Lexmark):</strong> 1 impression = 1 trang in
              (tính theo số mặt in thực tế của bộ ghi hình).
            </li>
            <li>
              <strong className="text-slate-200">Đơn vị Sheets (Brother, Ricoh):</strong> 1 tờ giấy (sheet) quy đổi
              tương đương 1 trang in (chuẩn định mức văn phòng).
            </li>
            <li>
              <strong className="text-slate-200">Máy quét tài liệu (Brother ADS Series):</strong> Bộ đếm số trang quét
              được thống kê tách riêng vào cột <span className="text-cyan-400 font-semibold">Trang quét</span>, không
              cộng dồn vào trang in để đảm bảo tính chính xác của chi phí in ấn.
            </li>
          </ul>
        </div>
      </div>

      {/* View Mode Toggle and Export Button Bar */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center bg-slate-950/80 p-1 rounded-xl border border-slate-800">
          <button
            type="button"
            onClick={() => setPageViewMode('department')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
              viewMode === 'department'
                ? 'bg-emerald-600 text-white shadow-md shadow-emerald-950'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Buildings size={16} weight="bold" />
            <span>Tổng hợp theo phòng ban ({filteredDepartments.length})</span>
          </button>
          <button
            type="button"
            onClick={() => setPageViewMode('printer')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-bold transition-all cursor-pointer ${
              viewMode === 'printer'
                ? 'bg-emerald-600 text-white shadow-md shadow-emerald-950'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Printer size={16} weight="bold" />
            <span>Chi tiết từng máy in ({filteredPrinters.length})</span>
          </button>
        </div>

        <button
          type="button"
          onClick={onExportExcel}
          disabled={isExporting}
          className="flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-xl text-xs font-bold shadow-lg shadow-emerald-950/40 transition-all cursor-pointer"
        >
          <FileXls size={16} weight="bold" />
          <span>{isExporting ? 'Đang xuất Excel...' : 'Xuất Excel Báo Cáo Trang In'}</span>
        </button>
      </div>

      {/* VIEW 1: GROUPED BY DEPARTMENT */}
      {viewMode === 'department' ? (
        <div className="glass-panel overflow-hidden border border-slate-800">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-950/90 text-slate-400 font-bold uppercase tracking-wider text-[11px]">
                  <th className="py-3 px-4 w-12 text-center">STT</th>
                  <th className="py-3 px-4">Phòng ban</th>
                  <th className="py-3 px-4 text-center">Số thiết bị</th>
                  <th className="py-3 px-4 text-right">Số trang in quy đổi</th>
                  <th className="py-3 px-4 text-right">Số trang quét (Scan)</th>
                  <th className="py-3 px-4 text-right">Tổng counter tích lũy</th>
                  <th className="py-3 px-4 text-center w-28">Chi tiết máy</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {filteredDepartments.map((dept, idx) => {
                  const isExpanded = expandedDept === dept.department
                  const deptPrint = dept.total_print_pages
                  return (
                    <tr key={dept.department} className="hover:bg-slate-800/30 transition-colors">
                      <td className="py-3.5 px-4 text-center font-semibold text-slate-500">{idx + 1}</td>
                      <td className="py-3.5 px-4">
                        <div className="flex items-center gap-2">
                          <Buildings size={16} className="text-emerald-400 shrink-0" weight="bold" />
                          <span className="font-bold text-slate-100 text-sm">{dept.department}</span>
                        </div>
                      </td>
                      <td className="py-3.5 px-4 text-center">
                        <span className="px-2.5 py-1 rounded-full bg-slate-800 text-slate-300 font-semibold border border-slate-700">
                          {dept.printer_count} máy
                        </span>
                      </td>
                      <td className="py-3.5 px-4 text-right">
                        <span className="font-black text-emerald-400 text-sm">{deptPrint.toLocaleString('vi-VN')}</span>{' '}
                        <span className="text-slate-500 font-normal">trang</span>
                      </td>
                      <td className="py-3.5 px-4 text-right">
                        {dept.total_scan_pages > 0 ? (
                          <span className="font-bold text-cyan-400">
                            {dept.total_scan_pages.toLocaleString('vi-VN')}{' '}
                            <span className="text-slate-500 font-normal">trang</span>
                          </span>
                        ) : (
                          <span className="text-slate-600">—</span>
                        )}
                      </td>
                      <td className="py-3.5 px-4 text-right text-slate-300 font-semibold">
                        {dept.total_current_life.toLocaleString('vi-VN')}
                      </td>
                      <td className="py-3.5 px-4 text-center">
                        <button
                          type="button"
                          onClick={() => setExpandedDept(isExpanded ? null : dept.department)}
                          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition-all cursor-pointer"
                        >
                          <span>{isExpanded ? 'Đóng' : 'Xem'}</span>
                          {isExpanded ? <CaretDown size={14} /> : <CaretRight size={14} />}
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          {/* Collapsible detail panel for expanded department */}
          {expandedDept && (
            <div className="p-4 bg-slate-950/90 border-t border-slate-800 space-y-3">
              <div className="flex items-center justify-between">
                <h4 className="text-xs font-bold uppercase tracking-wider text-emerald-400 flex items-center gap-1.5">
                  <Printer size={16} weight="bold" />
                  Danh sách máy in & máy scan phòng {expandedDept}
                </h4>
                <button
                  type="button"
                  onClick={() => setExpandedDept(null)}
                  className="text-xs text-slate-400 hover:text-slate-200 cursor-pointer"
                >
                  ✕ Đóng chi tiết
                </button>
              </div>

              <div className="overflow-x-auto rounded-lg border border-slate-800 bg-slate-900/50">
                <table className="w-full text-left text-xs">
                  <thead>
                    <tr className="border-b border-slate-800 bg-slate-950/70 text-slate-400 font-bold uppercase text-[10px]">
                      <th className="py-2.5 px-3">Tên thiết bị</th>
                      <th className="py-2.5 px-3">IP & Model</th>
                      <th className="py-2.5 px-3 text-center">Phân loại</th>
                      <th className="py-2.5 px-3 text-center">Đơn vị gốc</th>
                      <th className="py-2.5 px-3 text-right">Mốc ban đầu</th>
                      <th className="py-2.5 px-3 text-right">Chỉ số hiện tại</th>
                      <th className="py-2.5 px-3 text-right">Sản lượng kỳ</th>
                      <th className="py-2.5 px-3 text-right">Trang quy đổi</th>
                      <th className="py-2.5 px-3">Công thức quy đổi</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/40">
                    {filteredDepartments
                      .find((d) => d.department === expandedDept)
                      ?.printers.map((p) => {
                        return (
                          <tr key={p.device_id} className="hover:bg-slate-800/20">
                            <td className="py-2 px-3 font-semibold text-slate-200">{p.display_name}</td>
                            <td className="py-2 px-3 text-slate-400">
                              <span className="font-mono text-cyan-400">{p.address}</span> · {p.model}
                            </td>
                            <td className="py-2 px-3 text-center">
                              <span
                                className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase ${
                                  p.category === 'scanner'
                                    ? 'bg-cyan-950 text-cyan-400 border border-cyan-800'
                                    : 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                                }`}
                              >
                                {p.category === 'scanner' ? 'Máy quét' : 'Máy in'}
                              </span>
                            </td>
                            <td className="py-2 px-3 text-center">
                              <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono text-[10px]">
                                {p.raw_unit}
                              </span>
                            </td>
                            <td className="py-2 px-3 text-right font-mono text-slate-400">
                              {p.initial_reading ? p.initial_reading.toLocaleString('vi-VN') : '—'}
                            </td>
                            <td className="py-2 px-3 text-right font-mono text-slate-300">
                              {p.current_reading ? p.current_reading.toLocaleString('vi-VN') : '—'}
                            </td>
                            <td className="py-2 px-3 text-right font-mono text-amber-300 font-semibold">
                              {p.period_usage.toLocaleString('vi-VN')}
                            </td>
                            <td className="py-2 px-3 text-right font-black text-emerald-400">
                              {p.normalized_pages.toLocaleString('vi-VN')} trang
                            </td>
                            <td className="py-2 px-3 text-slate-400 text-[11px] italic">{p.conversion_note}</td>
                          </tr>
                        )
                      })}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      ) : (
        /* VIEW 2: FLAT TABLE OF ALL PRINTERS */
        <div className="glass-panel overflow-hidden border border-slate-800">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-950/90 text-slate-400 font-bold uppercase tracking-wider text-[11px]">
                  <th className="py-3 px-4 w-12 text-center">STT</th>
                  <th className="py-3 px-4">Thiết bị</th>
                  <th className="py-3 px-4">Model & Hãng</th>
                  <th className="py-3 px-4">Phòng ban</th>
                  <th className="py-3 px-4 text-center">Phân loại</th>
                  <th className="py-3 px-4 text-center">Đơn vị gốc</th>
                  <th className="py-3 px-4 text-right">Mốc ban đầu</th>
                  <th className="py-3 px-4 text-right">Chỉ số hiện tại</th>
                  <th className="py-3 px-4 text-right">Sản lượng kỳ</th>
                  <th className="py-3 px-4 text-right">Số trang quy đổi</th>
                  <th className="py-3 px-4">Ghi chú quy đổi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {filteredPrinters.map((p, idx) => {
                  return (
                    <tr key={p.device_id} className="hover:bg-slate-800/30 transition-colors">
                      <td className="py-3 px-4 text-center font-semibold text-slate-500">{idx + 1}</td>
                      <td className="py-3 px-4">
                        <div className="flex flex-col">
                          <span className="font-bold text-slate-200">{p.display_name}</span>
                          <span className="font-mono text-cyan-400 text-[11px]">{p.address}</span>
                        </div>
                      </td>
                      <td className="py-3 px-4 text-slate-300">
                        <span>
                          {p.manufacturer} {p.model}
                        </span>
                        {p.serial && <span className="block text-[10px] text-slate-500 font-mono">SN: {p.serial}</span>}
                      </td>
                      <td className="py-3 px-4">
                        <span className="font-semibold text-slate-200 flex items-center gap-1.5">
                          <Buildings size={14} className="text-cyan-400" />
                          {p.department}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-center">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase ${
                            p.category === 'scanner'
                              ? 'bg-cyan-950 text-cyan-400 border border-cyan-800'
                              : 'bg-emerald-950 text-emerald-400 border border-emerald-800'
                          }`}
                        >
                          {p.category === 'scanner' ? 'Máy quét' : 'Máy in'}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-center">
                        <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono text-[10px]">
                          {p.raw_unit}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-right font-mono text-slate-400">
                        {p.initial_reading ? p.initial_reading.toLocaleString('vi-VN') : '—'}
                      </td>
                      <td className="py-3 px-4 text-right font-mono text-slate-300">
                        {p.current_reading ? p.current_reading.toLocaleString('vi-VN') : '—'}
                      </td>
                      <td className="py-3 px-4 text-right font-mono text-amber-300 font-semibold">
                        {p.period_usage.toLocaleString('vi-VN')}
                      </td>
                      <td className="py-3 px-4 text-right">
                        <span className="font-black text-emerald-400 text-sm">
                          {p.normalized_pages.toLocaleString('vi-VN')}
                        </span>{' '}
                        <span className="text-slate-500 font-normal">trang</span>
                      </td>
                      <td className="py-3 px-4 text-slate-400 text-[11px]">
                        <span className="flex items-center gap-1">
                          <CheckCircle size={13} className="text-emerald-400 shrink-0" />
                          {p.conversion_note}
                        </span>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
