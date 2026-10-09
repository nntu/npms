import { Drop, Plus, Recycle, Wrench } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  createCartridge,
  listCartridgeLogs,
  listCartridges,
  refillCartridges,
  updateCartridgeStock,
} from '../../api/client'
import type { Cartridge } from '../../api/types'
import { showError, showSuccess } from '../../utils/sweetalert'

export function CartridgesPage() {
  const queryClient = useQueryClient()
  const cartridgesQuery = useQuery({ queryKey: ['cartridges'], queryFn: () => listCartridges() })
  const logsQuery = useQuery({ queryKey: ['cartridge-logs'], queryFn: () => listCartridgeLogs() })

  const [showAddModal, setShowAddModal] = useState(false)
  const [skuCode, setSkuCode] = useState('')
  const [name, setName] = useState('')
  const [compatibleModels, setCompatibleModels] = useState('')
  const [stockNewInput, setStockNewInput] = useState(0)

  const [showStockModal, setShowStockModal] = useState(false)
  const [selectedCartridge, setSelectedCartridge] = useState<Cartridge | null>(null)
  const [addStockNew, setAddStockNew] = useState(0)
  const [addStockRefilled, setAddStockRefilled] = useState(0)
  const [notes, setNotes] = useState('')

  const [showRefillModal, setShowRefillModal] = useState(false)
  const [refillQuantity, setRefillQuantity] = useState(1)

  const addCartridgeMutation = useMutation({
    mutationFn: () =>
      createCartridge({
        sku_code: skuCode.trim(),
        name: name.trim(),
        compatible_models: compatibleModels.trim(),
        stock_new: stockNewInput,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['cartridges'] })
      void queryClient.invalidateQueries({ queryKey: ['cartridge-logs'] })
      setShowAddModal(false)
      setSkuCode('')
      setName('')
      setCompatibleModels('')
      setStockNewInput(0)
      void showSuccess('Đã thêm loại mực!', 'Loại hộp mực mới đã được lưu vào hệ thống kho.')
    },
    onError: (err) => {
      void showError('Không thể thêm loại mực', err instanceof Error ? err.message : 'Kiểm tra mã SKU.')
    },
  })

  const updateStockMutation = useMutation({
    mutationFn: () => {
      if (!selectedCartridge) throw new Error('Chưa chọn loại mực')
      return updateCartridgeStock({
        cartridge_id: selectedCartridge.id,
        add_stock_new: addStockNew,
        add_stock_refilled: addStockRefilled,
        notes: notes.trim(),
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['cartridges'] })
      void queryClient.invalidateQueries({ queryKey: ['cartridge-logs'] })
      setShowStockModal(false)
      setSelectedCartridge(null)
      setAddStockNew(0)
      setAddStockRefilled(0)
      setNotes('')
      void showSuccess('Đã cập nhật kho!', 'Số lượng bình mực đã được cập nhật thành công.')
    },
    onError: (err) => {
      void showError('Cập nhật thất bại', err instanceof Error ? err.message : 'Lỗi cập nhật kho.')
    },
  })

  const refillMutation = useMutation({
    mutationFn: () => {
      if (!selectedCartridge) throw new Error('Chưa chọn loại mực')
      return refillCartridges({
        cartridge_id: selectedCartridge.id,
        quantity: refillQuantity,
        notes: notes.trim(),
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['cartridges'] })
      void queryClient.invalidateQueries({ queryKey: ['cartridge-logs'] })
      setShowRefillModal(false)
      setSelectedCartridge(null)
      setRefillQuantity(1)
      setNotes('')
      void showSuccess('Đã hoàn tất bơm mực!', 'Các vỏ bình hết đã được chuyển thành bình đã bơm sẵn sàng sử dụng.')
    },
    onError: (err) => {
      void showError('Bơm mực thất bại', err instanceof Error ? err.message : 'Kiểm tra lại số lượng vỏ bình hết.')
    },
  })

  const items = cartridgesQuery.data?.data ?? []
  const logs = logsQuery.data?.data ?? []

  const totalNew = items.reduce((acc, item) => acc + item.stock_new, 0)
  const totalRefilled = items.reduce((acc, item) => acc + item.stock_refilled, 0)
  const totalEmpty = items.reduce((acc, item) => acc + item.stock_empty, 0)

  // Yield calculations
  const replaceLogsWithYield = logs.filter((log) => log.action_type === 'replace' && (log.printed_pages ?? 0) > 0)
  const newYieldLogs = replaceLogsWithYield.filter((log) => log.source_type === 'new')
  const refilledYieldLogs = replaceLogsWithYield.filter((log) => log.source_type === 'refilled')

  const avgYieldNew =
    newYieldLogs.length > 0
      ? Math.round(newYieldLogs.reduce((acc, log) => acc + (log.printed_pages ?? 0), 0) / newYieldLogs.length)
      : 0
  const avgYieldRefilled =
    refilledYieldLogs.length > 0
      ? Math.round(refilledYieldLogs.reduce((acc, log) => acc + (log.printed_pages ?? 0), 0) / refilledYieldLogs.length)
      : 0

  return (
    <div className="space-y-6">
      {/* Page Banner */}
      <div className="glass-panel p-6 sm:p-8 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6">
        <div className="space-y-2 max-w-2xl">
          <p className="text-xs uppercase font-extrabold tracking-widest text-cyan-400">
            Quản lý vật tư & Kho bình mực
          </p>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-100 tracking-tight">
            Quản lý Tồn kho & Thay mực máy in
          </h1>
          <p className="text-slate-400 text-sm leading-relaxed">
            Theo dõi số lượng bình mực mới, bình mực đã bơm sẵn sàng thay và gom các vỏ bình hết để mang đi bơm lại.
          </p>
        </div>
        <div className="flex items-center gap-3 w-full sm:w-auto">
          <button
            type="button"
            onClick={() => setShowAddModal(true)}
            className="bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm shadow-lg shadow-cyan-950/50 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <Plus size={18} weight="bold" />
            <span>Thêm loại mực mới</span>
          </button>
        </div>
      </div>

      {/* KPI Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="glass-card p-5 border-l-4 border-l-emerald-500">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold text-slate-400 uppercase">🟢 Bình mực MỚI 100%</p>
              <h3 className="text-2xl font-black text-emerald-400 mt-1">{totalNew} bình</h3>
            </div>
            <Drop size={32} className="text-emerald-400/60" />
          </div>
          <p className="text-xs text-slate-400 mt-2">Hộp mực chính hãng / mới nguyên đai</p>
        </div>

        <div className="glass-card p-5 border-l-4 border-l-cyan-500">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold text-slate-400 uppercase">🔵 Bình mực ĐÃ BƠM (Sẵn sàng)</p>
              <h3 className="text-2xl font-black text-cyan-400 mt-1">{totalRefilled} bình</h3>
            </div>
            <Recycle size={32} className="text-cyan-400/60" />
          </div>
          <p className="text-xs text-slate-400 mt-2">Hộp mực đã được bơm sẵn sàng thay</p>
        </div>

        <div className="glass-card p-5 border-l-4 border-l-amber-500">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold text-slate-400 uppercase">🔴 Vỏ bình HẾT (Chờ đi bơm)</p>
              <h3 className="text-2xl font-black text-amber-400 mt-1">{totalEmpty} vỏ</h3>
            </div>
            <Wrench size={32} className="text-amber-400/60" />
          </div>
          <p className="text-xs text-slate-400 mt-2">Vỏ bình vừa rút từ máy in ra</p>
        </div>
      </div>

      {/* Yield Performance Evaluation Banner */}
      <div className="glass-panel p-5 bg-slate-900/60 border border-slate-800">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <span className="text-xs font-extrabold uppercase tracking-widest text-cyan-400">
              📊 Đánh giá sản lượng in bình mực (Yield Evaluation)
            </span>
            <p className="text-xs text-slate-400 mt-0.5">
              So sánh trung bình số trang in được thực tế giữa bình mực mới nguyên đai và bình mực bơm lại.
            </p>
          </div>
          <div className="flex items-center gap-6 text-xs sm:text-sm">
            <div className="bg-slate-950/80 px-4 py-2 rounded-xl border border-emerald-900/50">
              <span className="text-slate-400 block text-[11px]">Trung bình 🟢 Bình MỚI:</span>
              <strong className="text-emerald-400 font-bold text-sm">
                {avgYieldNew > 0 ? `${avgYieldNew.toLocaleString('vi-VN')} trang/bình` : 'Chưa có dữ liệu'}
              </strong>
            </div>
            <div className="bg-slate-950/80 px-4 py-2 rounded-xl border border-cyan-900/50">
              <span className="text-slate-400 block text-[11px]">Trung bình 🔵 Bình BƠM:</span>
              <strong className="text-cyan-400 font-bold text-sm">
                {avgYieldRefilled > 0 ? `${avgYieldRefilled.toLocaleString('vi-VN')} trang/bình` : 'Chưa có dữ liệu'}
              </strong>
            </div>
          </div>
        </div>
      </div>

      {/* Main Inventory Table */}
      <div className="glass-panel overflow-hidden">
        <div className="p-6 border-b border-slate-800 flex items-center justify-between">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Danh mục tồn kho</p>
            <h2 className="text-xl font-bold text-slate-100">Các loại bình mực</h2>
          </div>
          <span className="text-xs font-semibold bg-slate-800 text-slate-300 px-3 py-1 rounded-full">
            {items.length} loại mực
          </span>
        </div>

        {items.length === 0 ? (
          <div className="p-12 text-center text-slate-400">Chưa có loại mực nào. Nhấn "Thêm loại mực mới" ở trên.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs sm:text-sm">
              <thead>
                <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                  <th className="p-4 pl-6">Mã mực (SKU) / Tên</th>
                  <th className="p-4">Máy in tương thích</th>
                  <th className="p-4">🟢 Bình MỚI</th>
                  <th className="p-4">🔵 Bình BƠM</th>
                  <th className="p-4">🔴 Vỏ HẾT (Chờ bơm)</th>
                  <th className="p-4 pr-6 text-right">Thao tác kho</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {items.map((cartridge) => (
                  <tr key={cartridge.id} className="hover:bg-slate-800/40 transition-colors">
                    <td className="p-4 pl-6">
                      <span className="font-bold text-slate-100 block">{cartridge.name}</span>
                      <small className="text-cyan-400 font-mono text-xs">{cartridge.sku_code}</small>
                    </td>
                    <td className="p-4 text-slate-300 text-xs">{cartridge.compatible_models || 'Tất cả'}</td>
                    <td className="p-4 font-bold text-emerald-400 text-base">{cartridge.stock_new}</td>
                    <td className="p-4 font-bold text-cyan-400 text-base">{cartridge.stock_refilled}</td>
                    <td className="p-4 font-bold text-amber-400 text-base">{cartridge.stock_empty}</td>
                    <td className="p-4 pr-6 text-right space-x-2">
                      {cartridge.stock_empty > 0 && (
                        <button
                          type="button"
                          onClick={() => {
                            setSelectedCartridge(cartridge)
                            setRefillQuantity(cartridge.stock_empty)
                            setShowRefillModal(true)
                          }}
                          className="bg-amber-600 hover:bg-amber-500 text-white font-semibold px-3 py-1.5 rounded-lg text-xs transition-all cursor-pointer inline-flex items-center gap-1"
                        >
                          <Recycle size={14} /> Bơm {cartridge.stock_empty} vỏ
                        </button>
                      )}
                      <button
                        type="button"
                        onClick={() => {
                          setSelectedCartridge(cartridge)
                          setShowStockModal(true)
                        }}
                        className="bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold px-3 py-1.5 rounded-lg text-xs border border-slate-700 transition-all cursor-pointer"
                      >
                        + Thêm tồn kho
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Cartridge Logs Table */}
      <div className="glass-panel overflow-hidden">
        <div className="p-6 border-b border-slate-800">
          <p className="text-xs font-bold uppercase tracking-wider text-cyan-400">Nhật ký hoạt động</p>
          <h2 className="text-xl font-bold text-slate-100">Lịch sử xuất / nhập / thay / bơm mực & Hiệu suất</h2>
        </div>

        {logs.length === 0 ? (
          <div className="p-8 text-center text-slate-400 text-sm">Chưa có nhật ký hoạt động.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs sm:text-sm">
              <thead>
                <tr className="bg-slate-950/60 text-slate-400 uppercase text-[11px] font-bold tracking-wider border-b border-slate-800">
                  <th className="p-4 pl-6">Thời gian</th>
                  <th className="p-4">Hành động</th>
                  <th className="p-4">Nguồn mực</th>
                  <th className="p-4">Sản lượng in</th>
                  <th className="p-4">Số lượng</th>
                  <th className="p-4 pr-6">Ghi chú</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {logs.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="p-4 pl-6 text-slate-400 text-xs">
                      {new Date(log.performed_at).toLocaleString('vi-VN')}
                    </td>
                    <td className="p-4 font-semibold">
                      {log.action_type === 'replace' ? (
                        <span className="text-cyan-400">🖨️ Thay vào máy in</span>
                      ) : log.action_type === 'refill' ? (
                        <span className="text-amber-400">🔄 Bơm mực xong</span>
                      ) : (
                        <span className="text-emerald-400">📦 Nhập kho thêm</span>
                      )}
                    </td>
                    <td className="p-4 text-xs font-medium text-slate-300">
                      {log.source_type === 'new'
                        ? '🟢 Bình mới 100%'
                        : log.source_type === 'refilled'
                          ? '🔵 Bình đã bơm'
                          : '—'}
                    </td>
                    <td className="p-4 font-bold text-cyan-400 text-xs">
                      {log.action_type === 'replace' ? (
                        log.printed_pages && log.printed_pages > 0 ? (
                          <span>⚡ {log.printed_pages.toLocaleString('vi-VN')} trang</span>
                        ) : log.page_count && log.page_count > 0 ? (
                          <span className="text-slate-400 font-normal">
                            Đang dùng (mốc: {log.page_count.toLocaleString('vi-VN')})
                          </span>
                        ) : (
                          <span className="text-slate-500 font-normal">Đang hoạt động</span>
                        )
                      ) : (
                        <span className="text-slate-600">—</span>
                      )}
                    </td>
                    <td className="p-4 font-bold text-slate-200">{log.quantity}</td>
                    <td className="p-4 pr-6 text-slate-400 text-xs">{log.notes || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal Add Cartridge */}
      {showAddModal && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-cyan-800">
            <h3 className="text-lg font-bold text-slate-100">Thêm chủng loại mực mới</h3>
            <div className="space-y-3">
              <div>
                <label htmlFor="sku-code" className="block text-xs font-bold text-slate-300 mb-1">
                  Mã mực (SKU)
                </label>
                <input
                  id="sku-code"
                  value={skuCode}
                  onChange={(e) => setSkuCode(e.target.value)}
                  placeholder="vd: HP-CF226A, TN-2480"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
              <div>
                <label htmlFor="cartridge-name" className="block text-xs font-bold text-slate-300 mb-1">
                  Tên loại mực
                </label>
                <input
                  id="cartridge-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="vd: Hộp mực HP 26A Black"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
              <div>
                <label htmlFor="compatible-models" className="block text-xs font-bold text-slate-300 mb-1">
                  Dòng máy tương thích
                </label>
                <input
                  id="compatible-models"
                  value={compatibleModels}
                  onChange={(e) => setCompatibleModels(e.target.value)}
                  placeholder="vd: HP LaserJet Pro M404dn, M501dn"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
              <div>
                <label htmlFor="stock-new-input" className="block text-xs font-bold text-slate-300 mb-1">
                  Số bình MỚI nhập ban đầu
                </label>
                <input
                  id="stock-new-input"
                  type="number"
                  min={0}
                  value={stockNewInput}
                  onChange={(e) => setStockNewInput(Number(e.target.value))}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowAddModal(false)}
                className="bg-slate-800 text-slate-300 px-4 py-2 rounded-xl text-sm"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={addCartridgeMutation.isPending || !skuCode.trim() || !name.trim()}
                onClick={() => addCartridgeMutation.mutate()}
                className="bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-4 py-2 rounded-xl text-sm"
              >
                {addCartridgeMutation.isPending ? 'Đang lưu...' : 'Lưu loại mực'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal Update Stock */}
      {showStockModal && selectedCartridge && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-cyan-800">
            <h3 className="text-lg font-bold text-slate-100">Cập nhật tồn kho cho {selectedCartridge.name}</h3>
            <div className="space-y-3">
              <div>
                <label htmlFor="add-stock-new" className="block text-xs font-bold text-slate-300 mb-1">
                  Nhập thêm bình MỚI (+)
                </label>
                <input
                  id="add-stock-new"
                  type="number"
                  min={0}
                  value={addStockNew}
                  onChange={(e) => setAddStockNew(Number(e.target.value))}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
              <div>
                <label htmlFor="add-stock-refilled" className="block text-xs font-bold text-slate-300 mb-1">
                  Nhập thêm bình ĐÃ BƠM (+)
                </label>
                <input
                  id="add-stock-refilled"
                  type="number"
                  min={0}
                  value={addStockRefilled}
                  onChange={(e) => setAddStockRefilled(Number(e.target.value))}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
              <div>
                <label htmlFor="stock-notes" className="block text-xs font-bold text-slate-300 mb-1">
                  Ghi chú
                </label>
                <input
                  id="stock-notes"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="vd: Nhập lô hàng từ nhà cung cấp"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowStockModal(false)}
                className="bg-slate-800 text-slate-300 px-4 py-2 rounded-xl text-sm"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={updateStockMutation.isPending}
                onClick={() => updateStockMutation.mutate()}
                className="bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-4 py-2 rounded-xl text-sm"
              >
                {updateStockMutation.isPending ? 'Đang lưu...' : 'Cập nhật kho'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal Refill All */}
      {showRefillModal && selectedCartridge && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-amber-800">
            <h3 className="text-lg font-bold text-slate-100">Bơm mực cho {selectedCartridge.name}</h3>
            <p className="text-xs text-slate-400">
              Hiện có <strong className="text-amber-400">{selectedCartridge.stock_empty}</strong> vỏ bình hết. Chuyển
              thành bình đã bơm sẵn sàng sử dụng.
            </p>
            <div className="space-y-3">
              <div>
                <label htmlFor="refill-quantity" className="block text-xs font-bold text-slate-300 mb-1">
                  Số lượng vỏ đã bơm xong
                </label>
                <input
                  id="refill-quantity"
                  type="number"
                  min={1}
                  max={selectedCartridge.stock_empty}
                  value={refillQuantity}
                  onChange={(e) => setRefillQuantity(Number(e.target.value))}
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
              <div>
                <label htmlFor="refill-notes" className="block text-xs font-bold text-slate-300 mb-1">
                  Ghi chú
                </label>
                <input
                  id="refill-notes"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="vd: Đã bơm xong tại công ty mực in XYZ"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-sm text-slate-100"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowRefillModal(false)}
                className="bg-slate-800 text-slate-300 px-4 py-2 rounded-xl text-sm"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={refillMutation.isPending}
                onClick={() => refillMutation.mutate()}
                className="bg-amber-600 hover:bg-amber-500 text-white font-semibold px-4 py-2 rounded-xl text-sm"
              >
                {refillMutation.isPending ? 'Đang cập nhật...' : 'Xác nhận đã bơm xong'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
