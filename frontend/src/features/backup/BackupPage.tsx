import {
  Archive,
  ArrowClockwise,
  CheckCircle,
  Database,
  DownloadSimple,
  Printer,
  UploadSimple,
  Warning,
  XCircle,
} from '@phosphor-icons/react'
import { useState } from 'react'
import {
  downloadDatabaseBackup,
  downloadPrintersBackup,
  restoreDatabaseBackup,
  restorePrintersBackup,
} from '../../api/client'
import type { PrintersRestoreResponse } from '../../api/types'
import { showConfirm, showError, showSuccess } from '../../utils/sweetalert'

export function BackupPage() {
  // Printers backup / restore state
  const [isExportingPrinters, setIsExportingPrinters] = useState(false)
  const [printersFile, setPrintersFile] = useState<File | null>(null)
  const [overwritePrinters, setOverwritePrinters] = useState(true)
  const [isRestoringPrinters, setIsRestoringPrinters] = useState(false)
  const [printersRestoreResult, setPrintersRestoreResult] = useState<PrintersRestoreResponse | null>(null)

  // Database backup / restore state
  const [isExportingDb, setIsExportingDb] = useState(false)
  const [dbFile, setDbFile] = useState<File | null>(null)
  const [isRestoringDb, setIsRestoringDb] = useState(false)

  // Handle printer export
  const handleExportPrinters = async () => {
    try {
      setIsExportingPrinters(true)
      await downloadPrintersBackup()
      void showSuccess('Tải thành công!', 'File cấu hình máy in (.json) đã được tải về máy của bạn.')
    } catch (err) {
      void showError('Lỗi tải sao lưu', err instanceof Error ? err.message : 'Không thể tải file sao lưu máy in.')
    } finally {
      setIsExportingPrinters(false)
    }
  }

  // Handle printer restore
  const handleRestorePrinters = async () => {
    if (!printersFile) {
      void showError('Chưa chọn file', 'Vui lòng chọn file JSON chứa danh sách máy in cần khôi phục.')
      return
    }

    try {
      setIsRestoringPrinters(true)
      setPrintersRestoreResult(null)
      const res = await restorePrintersBackup(printersFile, overwritePrinters)
      setPrintersRestoreResult(res)
      void showSuccess(
        'Khôi phục máy in hoàn tất!',
        `Tổng: ${res.total} | Thêm mới: ${res.created} | Cập nhật: ${res.updated} | Lỗi: ${res.failed}`,
      )
    } catch (err) {
      void showError('Lỗi khôi phục', err instanceof Error ? err.message : 'Không thể khôi phục danh sách máy in.')
    } finally {
      setIsRestoringPrinters(false)
    }
  }

  // Handle database export
  const handleExportDb = async () => {
    try {
      setIsExportingDb(true)
      await downloadDatabaseBackup()
      void showSuccess('Tải thành công!', 'Bản sao lưu cơ sở dữ liệu (.db) đã được tải về máy của bạn.')
    } catch (err) {
      void showError('Lỗi tải sao lưu', err instanceof Error ? err.message : 'Không thể tải bản sao lưu cơ sở dữ liệu.')
    } finally {
      setIsExportingDb(false)
    }
  }

  // Handle database restore
  const handleRestoreDb = async () => {
    if (!dbFile) {
      void showError('Chưa chọn file', 'Vui lòng chọn file cơ sở dữ liệu SQLite (.db) cần khôi phục.')
      return
    }

    const confirmed = await showConfirm(
      'Xác nhận khôi phục toàn bộ dữ liệu?',
      'CẢNH BÁO: Dữ liệu hiện tại trong hệ thống sẽ được thay thế hoàn toàn bằng dữ liệu từ file sao lưu này. Hãy chắc chắn bạn đã tạo một bản sao lưu trước khi thực hiện!',
      'Ghi đè & Khôi phục',
    )
    if (!confirmed) return

    try {
      setIsRestoringDb(true)
      const res = await restoreDatabaseBackup(dbFile)
      await showSuccess(
        'Khôi phục cơ sở dữ liệu thành công!',
        res.message || 'Hệ thống sẽ tải lại để áp dụng dữ liệu mới.',
      )
      window.location.reload()
    } catch (err) {
      void showError(
        'Lỗi khôi phục database',
        err instanceof Error ? err.message : 'Không thể khôi phục cơ sở dữ liệu.',
      )
    } finally {
      setIsRestoringDb(false)
    }
  }

  return (
    <div className="space-y-8 pb-12">
      {/* Page Header */}
      <div className="glass-panel p-6 sm:p-8 relative overflow-hidden bg-white border border-slate-200 shadow-2xs">
        <div className="max-w-3xl space-y-2 z-10 relative">
          <div className="flex items-center gap-2 text-cyan-700 text-xs uppercase font-extrabold tracking-widest">
            <Archive size={16} weight="bold" />
            <span>Quản trị hệ thống & Dữ liệu</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            Sao lưu & Khôi phục (Backup & Restore)
          </h1>
          <p className="text-slate-600 text-sm leading-relaxed">
            Duy trì tính sẵn sàng và an toàn dữ liệu: Sao lưu độc lập cấu hình máy in (định dạng JSON) để dễ dàng di
            chuyển, hoặc sao lưu toàn bộ cơ sở dữ liệu SQLite (chỉ số trang, bộ đếm, bình mực) để khôi phục nhanh khi
            cần.
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Section 1: Printers Backup & Restore */}
        <div className="glass-panel p-6 sm:p-8 flex flex-col justify-between space-y-6 bg-white border border-slate-200 shadow-2xs">
          <div className="space-y-4">
            <div className="flex items-center gap-3 border-b border-slate-200 pb-4">
              <div className="w-10 h-10 rounded-xl bg-cyan-50 border border-cyan-200 flex items-center justify-center text-cyan-600">
                <Printer size={22} weight="bold" />
              </div>
              <div>
                <h2 className="text-lg font-bold text-slate-900">1. Cấu hình danh sách máy in</h2>
                <p className="text-xs text-slate-500">Định dạng file JSON (.json)</p>
              </div>
            </div>

            <p className="text-slate-600 text-sm leading-relaxed">
              Bao gồm thông tin thiết bị: Tên hiển thị, IP/Hostname, cổng, SNMP community (đã giải mã để import), thông
              tin hãng/model, profile SNMP và phòng ban. Phù hợp để di chuyển máy in sang máy chủ khác hoặc khôi phục
              nhanh.
            </p>

            {/* Export Card */}
            <div className="p-4 rounded-xl bg-slate-50 border border-slate-200 space-y-3">
              <h3 className="text-sm font-semibold text-slate-800 flex items-center gap-2">
                <DownloadSimple size={16} className="text-cyan-600" />
                <span>Sao lưu danh sách máy in (Export)</span>
              </h3>
              <p className="text-xs text-slate-600">
                Tải về toàn bộ danh sách máy in hiện có dưới dạng file JSON chuẩn.
              </p>
              <button
                type="button"
                onClick={() => void handleExportPrinters()}
                disabled={isExportingPrinters}
                className="w-full sm:w-auto bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-xs sm:text-sm flex items-center justify-center gap-2 transition-all cursor-pointer shadow-2xs"
              >
                {isExportingPrinters ? (
                  <ArrowClockwise size={16} className="animate-spin" />
                ) : (
                  <DownloadSimple size={16} weight="bold" />
                )}
                <span>{isExportingPrinters ? 'Đang tạo bản sao lưu...' : 'Tải file sao lưu máy in (.json)'}</span>
              </button>
            </div>

            {/* Restore Card */}
            <div className="p-4 rounded-xl bg-slate-50 border border-slate-200 space-y-4">
              <h3 className="text-sm font-semibold text-slate-800 flex items-center gap-2">
                <UploadSimple size={16} className="text-emerald-600" />
                <span>Khôi phục danh sách máy in (Import)</span>
              </h3>
              <p className="text-xs text-slate-600">Chọn file JSON sao lưu máy in để nạp lại vào hệ thống.</p>

              <div className="space-y-3">
                <div className="flex items-center gap-3">
                  <label className="flex-1 block">
                    <span className="sr-only">Chọn file JSON</span>
                    <input
                      type="file"
                      accept=".json,application/json"
                      onChange={(e) => {
                        const file = e.target.files?.[0] ?? null
                        setPrintersFile(file)
                        setPrintersRestoreResult(null)
                      }}
                      className="block w-full text-xs text-slate-600 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border file:border-slate-200 file:text-xs file:font-semibold file:bg-white file:text-cyan-700 hover:file:bg-slate-100 cursor-pointer shadow-2xs"
                    />
                  </label>
                </div>

                <label className="flex items-center gap-2 text-xs text-slate-700 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={overwritePrinters}
                    onChange={(e) => setOverwritePrinters(e.target.checked)}
                    className="rounded border-slate-300 bg-white text-cyan-600 focus:ring-0"
                  />
                  <span>Cập nhật thông tin nếu máy in đã tồn tại (dựa theo IP hoặc mã tài sản)</span>
                </label>

                <button
                  type="button"
                  onClick={() => void handleRestorePrinters()}
                  disabled={!printersFile || isRestoringPrinters}
                  className="w-full sm:w-auto bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-xs sm:text-sm flex items-center justify-center gap-2 transition-all cursor-pointer shadow-2xs"
                >
                  {isRestoringPrinters ? (
                    <ArrowClockwise size={16} className="animate-spin" />
                  ) : (
                    <UploadSimple size={16} weight="bold" />
                  )}
                  <span>{isRestoringPrinters ? 'Đang khôi phục...' : 'Bắt đầu khôi phục máy in'}</span>
                </button>
              </div>

              {/* Restore Result summary if any */}
              {printersRestoreResult && (
                <div className="mt-4 pt-3 border-t border-slate-200 space-y-3 text-xs">
                  <div className="flex flex-wrap gap-2">
                    <span className="px-2 py-1 rounded bg-slate-100 text-slate-700 font-mono border border-slate-200">
                      Tổng: {printersRestoreResult.total}
                    </span>
                    <span className="px-2 py-1 rounded bg-emerald-50 border border-emerald-200 text-emerald-800 font-mono">
                      Thêm mới: {printersRestoreResult.created}
                    </span>
                    <span className="px-2 py-1 rounded bg-cyan-50 border border-cyan-200 text-cyan-800 font-mono">
                      Cập nhật: {printersRestoreResult.updated}
                    </span>
                    {printersRestoreResult.skipped > 0 && (
                      <span className="px-2 py-1 rounded bg-amber-50 border border-amber-200 text-amber-800 font-mono">
                        Bỏ qua: {printersRestoreResult.skipped}
                      </span>
                    )}
                    {printersRestoreResult.failed > 0 && (
                      <span className="px-2 py-1 rounded bg-rose-50 border border-rose-200 text-rose-800 font-mono">
                        Thất bại: {printersRestoreResult.failed}
                      </span>
                    )}
                  </div>

                  <div className="max-h-40 overflow-y-auto space-y-1.5 pr-1">
                    {printersRestoreResult.results.map((res) => (
                      <div
                        key={`${res.address}-${res.status}`}
                        className="flex items-center justify-between p-2 rounded bg-white border border-slate-200 text-[11px] shadow-2xs"
                      >
                        <div className="flex items-center gap-2 truncate">
                          {res.status === 'created' || res.status === 'updated' ? (
                            <CheckCircle size={14} className="text-emerald-600 shrink-0" />
                          ) : res.status === 'skipped' ? (
                            <Warning size={14} className="text-amber-600 shrink-0" />
                          ) : (
                            <XCircle size={14} className="text-rose-600 shrink-0" />
                          )}
                          <span className="font-semibold text-slate-800 truncate">
                            {res.display_name || res.address}
                          </span>
                          <span className="text-slate-500 font-mono">({res.address})</span>
                        </div>
                        <span
                          className={`font-semibold capitalize px-1.5 py-0.5 rounded text-[10px] border ${
                            res.status === 'created'
                              ? 'text-emerald-800 bg-emerald-50 border-emerald-200'
                              : res.status === 'updated'
                                ? 'text-cyan-800 bg-cyan-50 border-cyan-200'
                                : res.status === 'skipped'
                                  ? 'text-amber-800 bg-amber-50 border-amber-200'
                                  : 'text-rose-800 bg-rose-50 border-rose-200'
                          }`}
                        >
                          {res.status === 'created'
                            ? 'Mới'
                            : res.status === 'updated'
                              ? 'Cập nhật'
                              : res.status === 'skipped'
                                ? 'Đã có'
                                : 'Lỗi'}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Section 2: Full Database Backup & Restore */}
        <div className="glass-panel p-6 sm:p-8 flex flex-col justify-between space-y-6 bg-white border border-slate-200 shadow-2xs">
          <div className="space-y-4">
            <div className="flex items-center gap-3 border-b border-slate-200 pb-4">
              <div className="w-10 h-10 rounded-xl bg-purple-50 border border-purple-200 flex items-center justify-center text-purple-600">
                <Database size={22} weight="bold" />
              </div>
              <div>
                <h2 className="text-lg font-bold text-slate-900">2. Toàn bộ cơ sở dữ liệu hệ thống</h2>
                <p className="text-xs text-slate-500">Định dạng SQLite Database (.db)</p>
              </div>
            </div>

            <p className="text-slate-600 text-sm leading-relaxed">
              Bao gồm toàn bộ dữ liệu: Toàn bộ máy in, lịch sử chỉ số bộ đếm trang theo ngày, nhật ký bình mực (thay
              thế, bơm mực), lịch sử polling và các cấu hình hệ thống.
            </p>

            {/* Warning Alert */}
            <div className="p-3.5 rounded-xl bg-amber-50 border border-amber-200 flex items-start gap-3">
              <Warning size={20} className="text-amber-600 shrink-0 mt-0.5" />
              <div className="text-xs text-amber-900 leading-relaxed">
                <strong className="text-amber-950 block mb-0.5">Khuyến cáo an toàn:</strong>
                Trước khi khôi phục database, hãy tải về một bản sao lưu hiện tại để phòng trường hợp cần hoàn tác.
              </div>
            </div>

            {/* Export DB Card */}
            <div className="p-4 rounded-xl bg-slate-50 border border-slate-200 space-y-3">
              <h3 className="text-sm font-semibold text-slate-800 flex items-center gap-2">
                <DownloadSimple size={16} className="text-purple-600" />
                <span>Sao lưu toàn bộ cơ sở dữ liệu (Export)</span>
              </h3>
              <p className="text-xs text-slate-600">
                Tạo một bản snapshot nguyên tử SQLite (VACUUM INTO) nhất quán và tải về trực tiếp.
              </p>
              <button
                type="button"
                onClick={() => void handleExportDb()}
                disabled={isExportingDb}
                className="w-full sm:w-auto bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-xs sm:text-sm flex items-center justify-center gap-2 transition-all cursor-pointer shadow-2xs"
              >
                {isExportingDb ? (
                  <ArrowClockwise size={16} className="animate-spin" />
                ) : (
                  <DownloadSimple size={16} weight="bold" />
                )}
                <span>{isExportingDb ? 'Đang tạo snapshot database...' : 'Tải bản sao lưu Database (.db)'}</span>
              </button>
            </div>

            {/* Restore DB Card */}
            <div className="p-4 rounded-xl bg-slate-50 border border-slate-200 space-y-4">
              <h3 className="text-sm font-semibold text-slate-800 flex items-center gap-2">
                <UploadSimple size={16} className="text-rose-600" />
                <span>Khôi phục toàn bộ cơ sở dữ liệu (Restore)</span>
              </h3>
              <p className="text-xs text-slate-600">
                Chọn file <code className="text-purple-700 font-semibold">.db</code> hoặc{' '}
                <code className="text-purple-700 font-semibold">.sqlite</code> để khôi phục.
              </p>

              <div className="space-y-3">
                <label className="block">
                  <span className="sr-only">Chọn file Database</span>
                  <input
                    type="file"
                    accept=".db,.sqlite,.sqlite3"
                    onChange={(e) => setDbFile(e.target.files?.[0] ?? null)}
                    className="block w-full text-xs text-slate-600 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border file:border-slate-200 file:text-xs file:font-semibold file:bg-white file:text-purple-700 hover:file:bg-slate-100 cursor-pointer shadow-2xs"
                  />
                </label>

                <button
                  type="button"
                  onClick={() => void handleRestoreDb()}
                  disabled={!dbFile || isRestoringDb}
                  className="w-full sm:w-auto bg-rose-600 hover:bg-rose-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-xs sm:text-sm flex items-center justify-center gap-2 transition-all cursor-pointer shadow-2xs"
                >
                  {isRestoringDb ? (
                    <ArrowClockwise size={16} className="animate-spin" />
                  ) : (
                    <UploadSimple size={16} weight="bold" />
                  )}
                  <span>{isRestoringDb ? 'Đang kiểm tra và khôi phục database...' : 'Khôi phục toàn bộ Database'}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
