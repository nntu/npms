import { CalendarBlank, ChartBar, Printer } from '@phosphor-icons/react'
import { useMemo, useState } from 'react'
import { Bar, BarChart, CartesianGrid, Cell, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { PageUsageReport } from '../../api/client'

interface PageUsageChartsProps {
  report: PageUsageReport
  departmentFilter: string
}

type ChartTab = 'both' | 'daily' | 'printer'

export function PageUsageCharts({ report, departmentFilter }: PageUsageChartsProps) {
  const [selectedDeviceID, setSelectedDeviceID] = useState<string>('all')
  const [activeTab, setActiveTab] = useState<ChartTab>('both')
  const [printerCategory, setPrinterCategory] = useState<'all' | 'printer' | 'scanner'>('all')

  // Filter printers list for device selector and printer chart
  const filteredPrinters = useMemo(() => {
    return report.printers.filter((p) => {
      if (departmentFilter !== 'all' && p.department !== departmentFilter) return false
      if (printerCategory === 'printer' && p.category !== 'printer') return false
      if (printerCategory === 'scanner' && p.category !== 'scanner') return false
      return true
    })
  }, [report.printers, departmentFilter, printerCategory])

  // Active selected device metadata
  const selectedDevice = useMemo(() => {
    if (selectedDeviceID === 'all') return null
    return report.printers.find((p) => p.device_id === selectedDeviceID) || null
  }, [selectedDeviceID, report.printers])

  // 1. DATA FOR DAILY USAGE CHART
  const dailyChartData = useMemo(() => {
    const rawDaily = report.daily_usage ?? []
    return rawDaily.map((item) => {
      const shortDate = item.date.length >= 10 ? `${item.date.slice(8, 10)}/${item.date.slice(5, 7)}` : item.date

      if (selectedDeviceID === 'all') {
        // If a department filter is applied, only sum usage of printers in that department
        let printPages = 0
        let scanPages = 0
        const activeDevices: Array<{ name: string; pages: number; category: string }> = []

        for (const u of item.device_usage || []) {
          const matchedDev = report.printers.find((p) => p.device_id === u.device_id)
          if (departmentFilter !== 'all' && matchedDev && matchedDev.department !== departmentFilter) {
            continue
          }
          if (u.category === 'scanner') {
            scanPages += u.normalized_pages
          } else {
            printPages += u.normalized_pages
          }
          if (u.normalized_pages > 0) {
            activeDevices.push({
              name: u.display_name,
              pages: u.normalized_pages,
              category: u.category,
            })
          }
        }

        activeDevices.sort((a, b) => b.pages - a.pages)

        return {
          date: item.date,
          shortDate,
          print_pages: departmentFilter === 'all' ? item.total_print_pages : printPages,
          scan_pages: departmentFilter === 'all' ? item.total_scan_pages : scanPages,
          total_pages:
            (departmentFilter === 'all' ? item.total_print_pages : printPages) +
            (departmentFilter === 'all' ? item.total_scan_pages : scanPages),
          topDevices: activeDevices.slice(0, 3),
        }
      }

      // Filtered to a specific device
      const u = (item.device_usage || []).find((du) => du.device_id === selectedDeviceID)
      const pages = u?.normalized_pages ?? 0
      const isScanner = selectedDevice?.category === 'scanner'

      return {
        date: item.date,
        shortDate,
        print_pages: isScanner ? 0 : pages,
        scan_pages: isScanner ? pages : 0,
        total_pages: pages,
        topDevices:
          pages > 0 && selectedDevice
            ? [{ name: selectedDevice.display_name, pages, category: selectedDevice.category }]
            : [],
      }
    })
  }, [report.daily_usage, report.printers, selectedDeviceID, selectedDevice, departmentFilter])

  // Daily chart summary metrics
  const dailyMetrics = useMemo(() => {
    let totalPrint = 0
    let totalScan = 0
    let peakDay: { date: string; pages: number } | null = null
    let daysWithUsage = 0

    for (const d of dailyChartData) {
      totalPrint += d.print_pages
      totalScan += d.scan_pages
      const currentTotal = d.total_pages
      if (currentTotal > 0) daysWithUsage++
      if (!peakDay || currentTotal > peakDay.pages) {
        peakDay = { date: d.shortDate, pages: currentTotal }
      }
    }

    const totalDays = dailyChartData.length || 1
    const avgDaily = Math.round((totalPrint + totalScan) / totalDays)

    return {
      totalPrint,
      totalScan,
      totalAll: totalPrint + totalScan,
      peakDay: peakDay && peakDay.pages > 0 ? peakDay : null,
      avgDaily,
      daysWithUsage,
    }
  }, [dailyChartData])

  // 2. DATA FOR PRINTER COMPARISON CHART
  const printerChartData = useMemo(() => {
    const list = [...filteredPrinters]
      .filter((p) => p.normalized_pages > 0 || filteredPrinters.length <= 15)
      .sort((a, b) => b.normalized_pages - a.normalized_pages)

    return list.map((p) => ({
      device_id: p.device_id,
      name: p.display_name,
      shortName: p.display_name.length > 20 ? `${p.display_name.slice(0, 18)}…` : p.display_name,
      department: p.department || 'Chưa phân phòng',
      model: p.model,
      category: p.category,
      pages: p.normalized_pages,
      initial: p.initial_reading ?? 0,
      current: p.current_reading ?? 0,
    }))
  }, [filteredPrinters])

  const topPrinter = printerChartData[0] || null
  const activePrintersCount = filteredPrinters.filter((p) => p.normalized_pages > 0).length

  return (
    <div className="space-y-6 print:hidden">
      {/* Chart Section Header with View Mode Tabs */}
      <div className="flex flex-wrap items-center justify-between gap-4 p-4 glass-card bg-slate-900 text-white rounded-2xl shadow-sm">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-emerald-500/20 text-emerald-400 flex items-center justify-center border border-emerald-500/30">
            <ChartBar size={22} weight="bold" />
          </div>
          <div>
            <h3 className="text-base sm:text-lg font-black tracking-tight text-white flex items-center gap-2">
              Biểu đồ phân tích sản lượng in ấn
              {report.from && report.to ? (
                <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                  {report.from} → {report.to}
                </span>
              ) : (
                <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-slate-700 text-slate-300">
                  Toàn bộ kỳ
                </span>
              )}
            </h3>
            <p className="text-xs text-slate-400">
              Trực quan hóa số trang in theo từng ngày và so sánh tải giữa các máy in trong tháng.
            </p>
          </div>
        </div>

        {/* View Toggle Tabs */}
        <div className="flex items-center bg-slate-800/80 p-1 rounded-xl border border-slate-700 text-xs">
          <button
            type="button"
            onClick={() => setActiveTab('both')}
            className={`px-3 py-1.5 rounded-lg font-bold transition-all cursor-pointer ${
              activeTab === 'both' ? 'bg-emerald-600 text-white shadow-xs' : 'text-slate-400 hover:text-white'
            }`}
          >
            Cả 2 biểu đồ
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('daily')}
            className={`px-3 py-1.5 rounded-lg font-bold transition-all cursor-pointer ${
              activeTab === 'daily' ? 'bg-emerald-600 text-white shadow-xs' : 'text-slate-400 hover:text-white'
            }`}
          >
            Từng ngày
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('printer')}
            className={`px-3 py-1.5 rounded-lg font-bold transition-all cursor-pointer ${
              activeTab === 'printer' ? 'bg-emerald-600 text-white shadow-xs' : 'text-slate-400 hover:text-white'
            }`}
          >
            Từng máy
          </button>
        </div>
      </div>

      {/* Grid container for Charts */}
      <div className={`grid gap-6 ${activeTab === 'both' ? 'grid-cols-1 xl:grid-cols-2' : 'grid-cols-1'}`}>
        {/* CHART 1: BIỂU ĐỒ SỐ TRANG IN TỪNG NGÀY */}
        {(activeTab === 'both' || activeTab === 'daily') && (
          <div className="glass-card p-5 border border-slate-200 shadow-2xs flex flex-col justify-between">
            {/* Chart 1 Controls & Header */}
            <div>
              <div className="flex flex-wrap items-center justify-between gap-3 pb-3 border-b border-slate-200">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 rounded-lg bg-emerald-50 text-emerald-700 flex items-center justify-center border border-emerald-200">
                    <CalendarBlank size={18} weight="bold" />
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-slate-900">Số trang in từng ngày trong tháng</h4>
                    <p className="text-[11px] text-slate-500">
                      {selectedDevice ? (
                        <span>
                          Thiết bị: <strong className="text-emerald-700">{selectedDevice.display_name}</strong> (
                          {selectedDevice.department})
                        </span>
                      ) : (
                        <span>
                          Phạm vi:{' '}
                          <strong className="text-slate-800">
                            {departmentFilter === 'all' ? 'Tất cả thiết bị' : `Phòng ${departmentFilter}`}
                          </strong>
                        </span>
                      )}
                    </p>
                  </div>
                </div>

                {/* Device Selector Dropdown for Daily Chart */}
                <div className="flex items-center gap-2">
                  <span className="text-xs text-slate-500 font-medium hidden sm:inline">Xem máy:</span>
                  <select
                    value={selectedDeviceID}
                    onChange={(e) => setSelectedDeviceID(e.target.value)}
                    className="bg-slate-50 border border-slate-200 rounded-xl px-2.5 py-1.5 text-xs text-slate-800 font-semibold focus:outline-none focus:border-emerald-500 shadow-2xs cursor-pointer max-w-[200px]"
                  >
                    <option value="all">📊 Tất cả máy in ({filteredPrinters.length} máy)</option>
                    {filteredPrinters.map((p) => (
                      <option key={p.device_id} value={p.device_id}>
                        {p.display_name} ({p.department || '—'})
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Quick Stat Chips */}
              <div className="grid grid-cols-3 gap-2.5 my-4">
                <div className="bg-slate-50 p-2.5 rounded-xl border border-slate-200/80">
                  <span className="text-[11px] font-medium text-slate-500 block">Tổng trang trong kỳ</span>
                  <span className="text-base font-black text-emerald-700 font-mono">
                    {dailyMetrics.totalAll.toLocaleString('vi-VN')}
                  </span>
                  <span className="text-[10px] text-slate-500 ml-1">trang</span>
                </div>
                <div className="bg-slate-50 p-2.5 rounded-xl border border-slate-200/80">
                  <span className="text-[11px] font-medium text-slate-500 block">Ngày in nhiều nhất</span>
                  <span className="text-base font-black text-amber-700 font-mono">
                    {dailyMetrics.peakDay ? dailyMetrics.peakDay.pages.toLocaleString('vi-VN') : '0'}
                  </span>
                  {dailyMetrics.peakDay && (
                    <span className="text-[10px] text-slate-500 ml-1">({dailyMetrics.peakDay.date})</span>
                  )}
                </div>
                <div className="bg-slate-50 p-2.5 rounded-xl border border-slate-200/80">
                  <span className="text-[11px] font-medium text-slate-500 block">Trung bình / ngày</span>
                  <span className="text-base font-black text-cyan-700 font-mono">
                    {dailyMetrics.avgDaily.toLocaleString('vi-VN')}
                  </span>
                  <span className="text-[10px] text-slate-500 ml-1">trang</span>
                </div>
              </div>
            </div>

            {/* Recharts BarChart */}
            <div className="h-72 w-full mt-2">
              {dailyChartData.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-slate-400 text-xs">
                  <CalendarBlank size={32} className="mb-2 opacity-50" />
                  <span>Chưa có bản ghi số trang in theo ngày trong khoảng thời gian này</span>
                </div>
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={dailyChartData} margin={{ top: 10, right: 15, left: -10, bottom: 20 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
                    <XAxis
                      dataKey="shortDate"
                      stroke="#64748b"
                      fontSize={11}
                      tickLine={false}
                      angle={-30}
                      textAnchor="end"
                      height={40}
                    />
                    <YAxis
                      stroke="#64748b"
                      fontSize={11}
                      tickLine={false}
                      tickFormatter={(v: number) => v.toLocaleString('vi-VN')}
                    />
                    <Tooltip
                      content={(props: any) => {
                        if (!props.active || !props.payload?.length) return null
                        const d = props.payload[0].payload
                        return (
                          <div className="bg-white border border-slate-200 p-3.5 rounded-2xl shadow-xl text-xs space-y-2 min-w-[200px]">
                            <div className="flex items-center justify-between border-b border-slate-100 pb-1.5">
                              <span className="font-bold text-slate-900">Ngày {d.date}</span>
                              <span className="text-[10px] text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded-md font-semibold">
                                {d.total_pages.toLocaleString('vi-VN')} trang
                              </span>
                            </div>
                            <div className="space-y-1">
                              <div className="flex justify-between text-slate-600">
                                <span>Trang in (Print):</span>
                                <strong className="text-emerald-700 font-mono">
                                  {d.print_pages.toLocaleString('vi-VN')}
                                </strong>
                              </div>
                              {d.scan_pages > 0 && (
                                <div className="flex justify-between text-slate-600">
                                  <span>Trang quét (Scan):</span>
                                  <strong className="text-cyan-700 font-mono">
                                    {d.scan_pages.toLocaleString('vi-VN')}
                                  </strong>
                                </div>
                              )}
                            </div>
                            {d.topDevices && d.topDevices.length > 0 && (
                              <div className="pt-2 border-t border-slate-100">
                                <span className="text-[10px] text-slate-400 font-bold uppercase block mb-1">
                                  {selectedDeviceID === 'all' ? 'Top thiết bị in trong ngày:' : 'Thiết bị:'}
                                </span>
                                {d.topDevices.map((dev: any, idx: number) => (
                                  <div key={idx} className="flex justify-between text-[11px] text-slate-700">
                                    <span className="truncate max-w-[130px]" title={dev.name}>
                                      {dev.name}
                                    </span>
                                    <span className="font-mono font-bold text-slate-900">
                                      {dev.pages.toLocaleString('vi-VN')}
                                    </span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        )
                      }}
                    />
                    <Legend
                      verticalAlign="top"
                      height={32}
                      formatter={(val) => <span className="text-xs text-slate-600 font-medium">{val}</span>}
                    />
                    <Bar
                      dataKey="print_pages"
                      name="Trang in (Print)"
                      fill="#10b981"
                      radius={[4, 4, 0, 0]}
                      maxBarSize={32}
                    />
                    <Bar
                      dataKey="scan_pages"
                      name="Trang quét (Scan)"
                      fill="#06b6d4"
                      radius={[4, 4, 0, 0]}
                      maxBarSize={32}
                    />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            {selectedDeviceID !== 'all' && (
              <div className="mt-3 pt-2 border-t border-slate-100 flex items-center justify-between text-xs text-slate-500">
                <span>
                  Đang hiển thị riêng cho: <strong className="text-slate-800">{selectedDevice?.display_name}</strong>
                </span>
                <button
                  type="button"
                  onClick={() => setSelectedDeviceID('all')}
                  className="text-emerald-600 hover:text-emerald-700 font-semibold cursor-pointer"
                >
                  ← Xem lại tất cả máy
                </button>
              </div>
            )}
          </div>
        )}

        {/* CHART 2: BIỂU ĐỒ SỐ TRANG IN TỪNG MÁY TRONG THÁNG */}
        {(activeTab === 'both' || activeTab === 'printer') && (
          <div className="glass-card p-5 border border-slate-200 shadow-2xs flex flex-col justify-between">
            {/* Chart 2 Controls & Header */}
            <div>
              <div className="flex flex-wrap items-center justify-between gap-3 pb-3 border-b border-slate-200">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 rounded-lg bg-cyan-50 text-cyan-700 flex items-center justify-center border border-cyan-200">
                    <Printer size={18} weight="bold" />
                  </div>
                  <div>
                    <h4 className="text-sm font-bold text-slate-900">Sản lượng từng máy in trong tháng</h4>
                    <p className="text-[11px] text-slate-500">
                      So sánh tải trang in giữa các thiết bị (xếp hạng cao nhất)
                    </p>
                  </div>
                </div>

                {/* Filter Category Toggle */}
                <div className="flex items-center gap-1 bg-slate-100 p-1 rounded-xl text-xs">
                  <button
                    type="button"
                    onClick={() => setPrinterCategory('all')}
                    className={`px-2.5 py-1 rounded-lg font-semibold transition-all cursor-pointer ${
                      printerCategory === 'all'
                        ? 'bg-white text-slate-900 shadow-2xs font-bold'
                        : 'text-slate-600 hover:text-slate-900'
                    }`}
                  >
                    Tất cả ({report.printers.length})
                  </button>
                  <button
                    type="button"
                    onClick={() => setPrinterCategory('printer')}
                    className={`px-2.5 py-1 rounded-lg font-semibold transition-all cursor-pointer ${
                      printerCategory === 'printer'
                        ? 'bg-white text-slate-900 shadow-2xs font-bold'
                        : 'text-slate-600 hover:text-slate-900'
                    }`}
                  >
                    Máy in
                  </button>
                  <button
                    type="button"
                    onClick={() => setPrinterCategory('scanner')}
                    className={`px-2.5 py-1 rounded-lg font-semibold transition-all cursor-pointer ${
                      printerCategory === 'scanner'
                        ? 'bg-white text-slate-900 shadow-2xs font-bold'
                        : 'text-slate-600 hover:text-slate-900'
                    }`}
                  >
                    Máy quét
                  </button>
                </div>
              </div>

              {/* Quick Stat Chips */}
              <div className="grid grid-cols-2 gap-2.5 my-4">
                <div className="bg-slate-50 p-2.5 rounded-xl border border-slate-200/80">
                  <span className="text-[11px] font-medium text-slate-500 block">Thiết bị in nhiều nhất</span>
                  <span
                    className="text-sm font-black text-emerald-700 truncate block"
                    title={topPrinter ? `${topPrinter.name} (${topPrinter.pages.toLocaleString('vi-VN')} trang)` : '—'}
                  >
                    {topPrinter ? `${topPrinter.name}` : '—'}
                  </span>
                  <span className="text-[11px] text-slate-500">
                    {topPrinter
                      ? `${topPrinter.pages.toLocaleString('vi-VN')} trang (${topPrinter.department})`
                      : 'Chưa có dữ liệu'}
                  </span>
                </div>
                <div className="bg-slate-50 p-2.5 rounded-xl border border-slate-200/80">
                  <span className="text-[11px] font-medium text-slate-500 block">Tỷ lệ máy hoạt động trong kỳ</span>
                  <div className="flex items-center gap-1.5 mt-0.5">
                    <span className="text-base font-black text-cyan-700 font-mono">
                      {activePrintersCount} / {filteredPrinters.length}
                    </span>
                    <span className="text-[11px] text-slate-500">máy có phát sinh in</span>
                  </div>
                  <span className="text-[10px] text-slate-400">
                    {filteredPrinters.length > 0
                      ? Math.round((activePrintersCount / filteredPrinters.length) * 100)
                      : 0}
                    % quy mô
                  </span>
                </div>
              </div>
            </div>

            {/* Recharts BarChart (Horizontal for readability of printer names) */}
            <div className="h-72 w-full mt-2">
              {printerChartData.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-slate-400 text-xs">
                  <Printer size={32} className="mb-2 opacity-50" />
                  <span>Chưa có dữ liệu máy in phù hợp</span>
                </div>
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart
                    data={printerChartData.slice(0, 10)}
                    layout="vertical"
                    margin={{ top: 5, right: 30, left: 10, bottom: 5 }}
                  >
                    <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" horizontal={false} />
                    <XAxis
                      type="number"
                      stroke="#64748b"
                      fontSize={11}
                      tickLine={false}
                      tickFormatter={(v: number) => v.toLocaleString('vi-VN')}
                    />
                    <YAxis
                      dataKey="shortName"
                      type="category"
                      stroke="#64748b"
                      fontSize={11}
                      tickLine={false}
                      width={120}
                    />
                    <Tooltip
                      content={(props: any) => {
                        if (!props.active || !props.payload?.length) return null
                        const p = props.payload[0].payload
                        return (
                          <div className="bg-white border border-slate-200 p-3.5 rounded-2xl shadow-xl text-xs space-y-2 min-w-[220px]">
                            <div className="border-b border-slate-100 pb-1.5">
                              <p className="font-bold text-slate-900">{p.name}</p>
                              <p className="text-[11px] text-slate-500">
                                {p.model} • {p.department}
                              </p>
                            </div>
                            <div className="space-y-1">
                              <div className="flex justify-between">
                                <span className="text-slate-600">Sản lượng kỳ này:</span>
                                <strong className="text-emerald-700 font-mono text-sm">
                                  {p.pages.toLocaleString('vi-VN')} trang
                                </strong>
                              </div>
                              <div className="flex justify-between text-slate-500 text-[11px]">
                                <span>Phân loại:</span>
                                <span className="font-medium text-slate-700">
                                  {p.category === 'scanner' ? 'Máy quét tài liệu' : 'Máy in'}
                                </span>
                              </div>
                              <div className="flex justify-between text-slate-500 text-[11px]">
                                <span>Chỉ số counter hiện tại:</span>
                                <span className="font-mono text-slate-700">{p.current.toLocaleString('vi-VN')}</span>
                              </div>
                            </div>
                            <div className="pt-2 border-t border-slate-100 text-[10px] text-emerald-600 font-medium">
                              💡 Nhấn để xem biểu đồ theo ngày của máy này
                            </div>
                          </div>
                        )
                      }}
                    />
                    <Bar
                      dataKey="pages"
                      name="Trang in quy đổi"
                      radius={[0, 4, 4, 0]}
                      cursor="pointer"
                      onClick={(data: any) => {
                        if (data?.device_id) {
                          setSelectedDeviceID(data.device_id)
                          if (activeTab === 'printer') setActiveTab('both')
                        }
                      }}
                    >
                      {printerChartData.slice(0, 10).map((entry) => (
                        <Cell
                          key={`cell-${entry.device_id}`}
                          fill={
                            entry.device_id === selectedDeviceID
                              ? '#059669'
                              : entry.category === 'scanner'
                                ? '#06b6d4'
                                : '#10b981'
                          }
                        />
                      ))}
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <p className="text-[11px] text-slate-400 mt-2 text-center">
              Hiển thị top 10 máy có sản lượng cao nhất. Nhấp vào thanh máy in để lọc xem biểu đồ theo ngày.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
