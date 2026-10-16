import { Drop, Plus, Recycle, Wrench } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import {
  createCartridge,
  listCartridgeLogs,
  listCartridges,
  refillCartridges,
  updateCartridgeStock,
} from '../../api/client'
import type { Cartridge, CartridgeLog } from '../../api/types'
import { DataTable } from '../../components/DataTable'
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

  const cartridgeColumns = useMemo<ColumnDef<Cartridge>[]>(
    () => [
      {
        id: 'name',
        accessorFn: (row) => `${row.name} ${row.sku_code}`,
        header: 'Mã mực (SKU) / Tên',
        cell: ({ row }) => (
          <div>
            <span className="font-bold text-slate-900 block">{row.original.name}</span>
            <small className="text-cyan-700 font-mono text-xs">{row.original.sku_code}</small>
          </div>
        ),
      },
      {
        id: 'compatible_models',
        accessorKey: 'compatible_models',
        header: 'Máy in tương thích',
        cell: ({ row }) => <span className="text-slate-600 text-xs">{row.original.compatible_models || 'Tất cả'}</span>,
      },
      {
        id: 'stock_new',
        accessorKey: 'stock_new',
        header: '🟢 Bình MỚI',
        cell: ({ row }) => (
          <span className="font-bold text-emerald-700 font-mono text-base">{row.original.stock_new}</span>
        ),
      },
      {
        id: 'stock_refilled',
        accessorKey: 'stock_refilled',
        header: '🔵 Bình BƠM',
        cell: ({ row }) => (
          <span className="font-bold text-cyan-700 font-mono text-base">{row.original.stock_refilled}</span>
        ),
      },
      {
        id: 'stock_empty',
        accessorKey: 'stock_empty',
        header: '🔴 Vỏ HẾT (Chờ bơm)',
        cell: ({ row }) => (
          <span className="font-bold text-amber-700 font-mono text-base">{row.original.stock_empty}</span>
        ),
      },
      {
        id: 'actions',
        header: () => <span className="block text-right">Thao tác kho</span>,
        enableSorting: false,
        cell: ({ row }) => {
          const cartridge = row.original
          return (
            <div className="flex items-center justify-end space-x-2">
              {cartridge.stock_empty > 0 && (
                <button
                  type="button"
                  onClick={() => {
                    setSelectedCartridge(cartridge)
                    setRefillQuantity(cartridge.stock_empty)
                    setShowRefillModal(true)
                  }}
                  className="bg-amber-600 hover:bg-amber-500 text-white font-semibold px-3 py-1.5 rounded-lg text-xs transition-all cursor-pointer inline-flex items-center gap-1 shadow-xs"
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
                className="bg-white hover:bg-slate-50 text-slate-700 font-semibold px-3 py-1.5 rounded-lg text-xs border border-slate-200 transition-all cursor-pointer shadow-2xs"
              >
                + Thêm tồn kho
              </button>
            </div>
          )
        },
      },
    ],
    [],
  )

  const cartridgeLogColumns = useMemo<ColumnDef<CartridgeLog>[]>(
    () => [
      {
        id: 'performed_at',
        accessorKey: 'performed_at',
        header: 'Thời gian',
        cell: ({ row }) => (
          <span className="text-slate-500 text-xs">{new Date(row.original.performed_at).toLocaleString('vi-VN')}</span>
        ),
      },
      {
        id: 'action_type',
        accessorKey: 'action_type',
        header: 'Hành động',
        cell: ({ row }) => {
          const act = row.original.action_type
          return (
            <span className="font-semibold text-xs">
              {act === 'replace' ? (
                <span className="text-cyan-700">🖨️ Thay vào máy in</span>
              ) : act === 'refill' ? (
                <span className="text-amber-700">🔄 Bơm mực xong</span>
              ) : (
                <span className="text-emerald-700">📦 Nhập kho thêm</span>
              )}
            </span>
          )
        },
      },
      {
        id: 'source_type',
        accessorKey: 'source_type',
        header: 'Nguồn mực',
        cell: ({ row }) => (
          <span className="text-xs font-medium text-slate-700">
            {row.original.source_type === 'new'
              ? '🟢 Bình mới 100%'
              : row.original.source_type === 'refilled'
                ? '🔵 Bình đã bơm'
                : '—'}
          </span>
        ),
      },
      {
        id: 'printed_pages',
        header: 'Sản lượng in',
        accessorFn: (row) => row.printed_pages ?? row.page_count ?? 0,
        cell: ({ row }) => {
          const log = row.original
          if (log.action_type !== 'replace') return <span className="text-slate-400">—</span>
          if (log.printed_pages && log.printed_pages > 0) {
            return (
              <span className="text-cyan-700 font-bold font-mono text-xs">
                ⚡ {log.printed_pages.toLocaleString('vi-VN')} trang
              </span>
            )
          }
          if (log.page_count && log.page_count > 0) {
            return (
              <span className="text-slate-600 font-normal text-xs">
                Đang dùng ({log.page_count.toLocaleString('vi-VN')})
              </span>
            )
          }
          return <span className="text-slate-500 font-normal text-xs">Đang hoạt động</span>
        },
      },
      {
        id: 'quantity',
        accessorKey: 'quantity',
        header: 'Số lượng',
        cell: ({ row }) => <span className="font-bold text-slate-800">{row.original.quantity}</span>,
      },
      {
        id: 'notes',
        accessorKey: 'notes',
        header: 'Ghi chú',
        cell: ({ row }) => <span className="text-slate-500 text-xs">{row.original.notes || '—'}</span>,
      },
    ],
    [],
  )

  return (
    <div className="space-y-6">
      {/* Page Banner */}
      <div className="glass-panel p-6 sm:p-8 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6">
        <div className="space-y-2 max-w-2xl">
          <p className="text-xs uppercase font-extrabold tracking-widest text-cyan-700">
            Quản lý vật tư & Kho bình mực
          </p>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            Quản lý Tồn kho & Thay mực máy in
          </h1>
          <p className="text-slate-600 text-sm leading-relaxed">
            Theo dõi số lượng bình mực mới, bình mực đã bơm sẵn sàng thay và gom các vỏ bình hết để mang đi bơm lại.
          </p>
        </div>
        <div className="flex items-center gap-3 w-full sm:w-auto">
          <button
            type="button"
            onClick={() => setShowAddModal(true)}
            className="bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-4 py-2.5 rounded-xl text-xs sm:text-sm shadow-md shadow-cyan-600/20 transition-all flex items-center justify-center gap-2 cursor-pointer"
          >
            <Plus size={18} weight="bold" />
            <span>Thêm loại mực mới</span>
          </button>
        </div>
      </div>

      {/* KPI Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="glass-card p-5 border-l-4 border-l-emerald-500 shadow-2xs">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold text-slate-500 uppercase">🟢 Bình mực MỚI 100%</p>
              <h3 className="text-2xl font-black text-emerald-700 font-mono mt-1">{totalNew} bình</h3>
            </div>
            <Drop size={32} className="text-emerald-600/60" />
          </div>
          <p className="text-xs text-slate-500 mt-2">Hộp mực chính hãng / mới nguyên đai</p>
        </div>

        <div className="glass-card p-5 border-l-4 border-l-cyan-500 shadow-2xs">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold text-slate-500 uppercase">🔵 Bình mực ĐÃ BƠM (Sẵn sàng)</p>
              <h3 className="text-2xl font-black text-cyan-700 font-mono mt-1">{totalRefilled} bình</h3>
            </div>
            <Recycle size={32} className="text-cyan-600/60" />
          </div>
          <p className="text-xs text-slate-500 mt-2">Hộp mực đã được bơm sẵn sàng thay</p>
        </div>

        <div className="glass-card p-5 border-l-4 border-l-amber-500 shadow-2xs">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-bold text-slate-500 uppercase">🔴 Vỏ bình HẾT (Chờ đi bơm)</p>
              <h3 className="text-2xl font-black text-amber-700 font-mono mt-1">{totalEmpty} vỏ</h3>
            </div>
            <Wrench size={32} className="text-amber-600/60" />
          </div>
          <p className="text-xs text-slate-500 mt-2">Vỏ bình vừa rút từ máy in ra</p>
        </div>
      </div>

      {/* Yield Performance Evaluation Banner */}
      <div className="glass-panel p-5 bg-white border border-slate-200 shadow-2xs">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <span className="text-xs font-extrabold uppercase tracking-widest text-cyan-700">
              📊 Đánh giá sản lượng in bình mực (Yield Evaluation)
            </span>
            <p className="text-xs text-slate-500 mt-0.5">
              So sánh trung bình số trang in được thực tế giữa bình mực mới nguyên đai và bình mực bơm lại.
            </p>
          </div>
          <div className="flex items-center gap-6 text-xs sm:text-sm">
            <div className="bg-emerald-50/70 px-4 py-2 rounded-xl border border-emerald-200">
              <span className="text-slate-600 block text-[11px]">Trung bình 🟢 Bình MỚI:</span>
              <strong className="text-emerald-800 font-bold text-sm font-mono">
                {avgYieldNew > 0 ? `${avgYieldNew.toLocaleString('vi-VN')} trang/bình` : 'Chưa có dữ liệu'}
              </strong>
            </div>
            <div className="bg-cyan-50/70 px-4 py-2 rounded-xl border border-cyan-200">
              <span className="text-slate-600 block text-[11px]">Trung bình 🔵 Bình BƠM:</span>
              <strong className="text-cyan-800 font-bold text-sm font-mono">
                {avgYieldRefilled > 0 ? `${avgYieldRefilled.toLocaleString('vi-VN')} trang/bình` : 'Chưa có dữ liệu'}
              </strong>
            </div>
          </div>
        </div>
      </div>

      {/* Main Inventory Table */}
      <div className="glass-panel p-6 sm:p-8 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-1">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-700">Danh mục tồn kho</p>
            <h2 className="text-xl font-bold text-slate-900">Các loại bình mực</h2>
          </div>
          <span className="text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200 px-3 py-1 rounded-full">
            {items.length} loại mực
          </span>
        </div>

        <DataTable
          columns={cartridgeColumns}
          data={items}
          searchPlaceholder="Tìm kiếm loại mực theo tên, SKU, máy in tương thích..."
          initialPageSize={10}
          emptyMessage="Chưa có loại mực nào phù hợp"
          emptySubMessage="Nhấn 'Thêm loại mực mới' ở trên để khai báo vào hệ thống."
        />
      </div>

      {/* Cartridge Logs Table */}
      <div className="glass-panel p-6 sm:p-8 space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-1">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-700">Nhật ký hoạt động</p>
            <h2 className="text-xl font-bold text-slate-900">Lịch sử xuất / nhập / thay / bơm mực & Hiệu suất</h2>
          </div>
          <span className="text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200 px-3 py-1 rounded-full">
            {logs.length} bản ghi
          </span>
        </div>

        <DataTable
          columns={cartridgeLogColumns}
          data={logs}
          searchPlaceholder="Tìm kiếm nhật ký theo hành động, nguồn mực, ghi chú..."
          initialPageSize={10}
          emptyMessage="Chưa có nhật ký hoạt động nào"
          emptySubMessage="Nhật ký sẽ tự động ghi lại mỗi khi thay mực vào máy in, bơm mực hoặc nhập kho."
        />
      </div>

      {/* Modal Add Cartridge */}
      {showAddModal && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-slate-200 shadow-xl">
            <h3 className="text-lg font-bold text-slate-900">Thêm chủng loại mực mới</h3>
            <div className="space-y-3">
              <div>
                <label htmlFor="sku-code" className="block text-xs font-bold text-slate-700 mb-1">
                  Mã mực (SKU)
                </label>
                <input
                  id="sku-code"
                  value={skuCode}
                  onChange={(e) => setSkuCode(e.target.value)}
                  placeholder="vd: HP-CF226A, TN-2480"
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs placeholder-slate-400"
                />
              </div>
              <div>
                <label htmlFor="cartridge-name" className="block text-xs font-bold text-slate-700 mb-1">
                  Tên loại mực
                </label>
                <input
                  id="cartridge-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="vd: Hộp mực HP 26A Black"
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs placeholder-slate-400"
                />
              </div>
              <div>
                <label htmlFor="compatible-models" className="block text-xs font-bold text-slate-700 mb-1">
                  Dòng máy tương thích
                </label>
                <input
                  id="compatible-models"
                  value={compatibleModels}
                  onChange={(e) => setCompatibleModels(e.target.value)}
                  placeholder="vd: HP LaserJet Pro M404dn, M501dn"
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs placeholder-slate-400"
                />
              </div>
              <div>
                <label htmlFor="stock-new-input" className="block text-xs font-bold text-slate-700 mb-1">
                  Số bình MỚI nhập ban đầu
                </label>
                <input
                  id="stock-new-input"
                  type="number"
                  min={0}
                  value={stockNewInput}
                  onChange={(e) => setStockNewInput(Number(e.target.value))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowAddModal(false)}
                className="bg-slate-100 hover:bg-slate-200 text-slate-700 px-4 py-2 rounded-xl text-sm border border-slate-200 cursor-pointer"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={addCartridgeMutation.isPending || !skuCode.trim() || !name.trim()}
                onClick={() => addCartridgeMutation.mutate()}
                className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-sm cursor-pointer shadow-xs"
              >
                {addCartridgeMutation.isPending ? 'Đang lưu...' : 'Lưu loại mực'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal Update Stock */}
      {showStockModal && selectedCartridge && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-slate-200 shadow-xl">
            <h3 className="text-lg font-bold text-slate-900">Cập nhật tồn kho cho {selectedCartridge.name}</h3>
            <div className="space-y-3">
              <div>
                <label htmlFor="add-stock-new" className="block text-xs font-bold text-slate-700 mb-1">
                  Nhập thêm bình MỚI (+)
                </label>
                <input
                  id="add-stock-new"
                  type="number"
                  min={0}
                  value={addStockNew}
                  onChange={(e) => setAddStockNew(Number(e.target.value))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                />
              </div>
              <div>
                <label htmlFor="add-stock-refilled" className="block text-xs font-bold text-slate-700 mb-1">
                  Nhập thêm bình ĐÃ BƠM (+)
                </label>
                <input
                  id="add-stock-refilled"
                  type="number"
                  min={0}
                  value={addStockRefilled}
                  onChange={(e) => setAddStockRefilled(Number(e.target.value))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                />
              </div>
              <div>
                <label htmlFor="stock-notes" className="block text-xs font-bold text-slate-700 mb-1">
                  Ghi chú
                </label>
                <input
                  id="stock-notes"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="vd: Nhập lô hàng từ nhà cung cấp"
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs placeholder-slate-400"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowStockModal(false)}
                className="bg-slate-100 hover:bg-slate-200 text-slate-700 px-4 py-2 rounded-xl text-sm border border-slate-200 cursor-pointer"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={updateStockMutation.isPending}
                onClick={() => updateStockMutation.mutate()}
                className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-sm cursor-pointer shadow-xs"
              >
                {updateStockMutation.isPending ? 'Đang lưu...' : 'Cập nhật kho'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal Refill All */}
      {showRefillModal && selectedCartridge && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 border-amber-200 shadow-xl">
            <h3 className="text-lg font-bold text-slate-900">Bơm mực cho {selectedCartridge.name}</h3>
            <p className="text-xs text-slate-600">
              Hiện có <strong className="text-amber-700 font-mono">{selectedCartridge.stock_empty}</strong> vỏ bình hết.
              Chuyển thành bình đã bơm sẵn sàng sử dụng.
            </p>
            <div className="space-y-3">
              <div>
                <label htmlFor="refill-quantity" className="block text-xs font-bold text-slate-700 mb-1">
                  Số lượng vỏ đã bơm xong
                </label>
                <input
                  id="refill-quantity"
                  type="number"
                  min={1}
                  max={selectedCartridge.stock_empty}
                  value={refillQuantity}
                  onChange={(e) => setRefillQuantity(Number(e.target.value))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs"
                />
              </div>
              <div>
                <label htmlFor="refill-notes" className="block text-xs font-bold text-slate-700 mb-1">
                  Ghi chú
                </label>
                <input
                  id="refill-notes"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="vd: Đã bơm xong tại công ty mực in XYZ"
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 shadow-2xs placeholder-slate-400"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowRefillModal(false)}
                className="bg-slate-100 hover:bg-slate-200 text-slate-700 px-4 py-2 rounded-xl text-sm border border-slate-200 cursor-pointer"
              >
                Hủy
              </button>
              <button
                type="button"
                disabled={refillMutation.isPending}
                onClick={() => refillMutation.mutate()}
                className="bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-xl text-sm cursor-pointer shadow-xs"
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
