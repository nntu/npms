import { CaretDown, CaretLeft, CaretRight, CaretUp, CaretUpDown, MagnifyingGlass, X } from '@phosphor-icons/react'
import {
  type ColumnDef,
  type ColumnFiltersState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  type SortingState,
  type Table,
  useReactTable,
} from '@tanstack/react-table'
import type React from 'react'
import { useState } from 'react'

export interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[]
  data: TData[]
  searchPlaceholder?: string
  initialPageSize?: number
  onRowClick?: (row: TData) => void
  toolbarActions?: React.ReactNode
  renderCustomFilter?: (table: Table<TData>) => React.ReactNode
  filterSlot?: React.ReactNode
  emptyMessage?: string
  emptySubMessage?: string
}

export function DataTable<TData, TValue>({
  columns,
  data,
  searchPlaceholder = 'Tìm kiếm nhanh...',
  initialPageSize = 10,
  onRowClick,
  toolbarActions,
  renderCustomFilter,
  filterSlot,
  emptyMessage = 'Không tìm thấy dữ liệu phù hợp',
  emptySubMessage = 'Thử điều chỉnh từ khóa tìm kiếm hoặc bộ lọc để hiển thị kết quả.',
}: DataTableProps<TData, TValue>) {
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState('')
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize: initialPageSize,
  })

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      globalFilter,
      columnFilters,
      pagination,
    },
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    onColumnFiltersChange: setColumnFilters,
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  const rows = table.getRowModel().rows
  const totalFiltered = table.getFilteredRowModel().rows.length

  return (
    <div className="space-y-4">
      {/* Table Toolbar */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
        <div className="flex flex-1 flex-wrap items-center gap-3">
          <div className="relative flex-1 min-w-[200px] max-w-md">
            <MagnifyingGlass
              size={16}
              className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none"
            />
            <input
              type="text"
              value={globalFilter ?? ''}
              onChange={(e) => setGlobalFilter(e.target.value)}
              placeholder={searchPlaceholder}
              className="w-full bg-white border border-slate-200 rounded-xl pl-10 pr-9 py-2 text-xs sm:text-sm text-slate-900 placeholder-slate-400 focus:outline-none focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500 transition-colors shadow-2xs"
            />
            {globalFilter && (
              <button
                type="button"
                onClick={() => setGlobalFilter('')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 transition-colors cursor-pointer"
                title="Xóa tìm kiếm"
              >
                <X size={14} />
              </button>
            )}
          </div>
          {filterSlot}
          {renderCustomFilter?.(table)}
        </div>

        {toolbarActions && <div className="flex items-center gap-2 self-end sm:self-auto">{toolbarActions}</div>}
      </div>

      {/* Table Container */}
      <div className="border border-slate-200/90 rounded-2xl bg-white overflow-hidden shadow-xs">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse text-xs sm:text-sm">
            <thead>
              {table.getHeaderGroups().map((headerGroup) => (
                <tr
                  key={headerGroup.id}
                  className="bg-slate-50/90 text-slate-600 uppercase text-[11px] font-bold tracking-wider border-b border-slate-200"
                >
                  {headerGroup.headers.map((header) => {
                    const canSort = header.column.getCanSort()
                    const isSorted = header.column.getIsSorted()
                    return (
                      <th
                        key={header.id}
                        colSpan={header.colSpan}
                        onClick={header.column.getToggleSortingHandler()}
                        className={`p-4 select-none ${
                          canSort ? 'cursor-pointer hover:text-cyan-700 hover:bg-slate-100/60 transition-colors' : ''
                        }`}
                      >
                        <div className="flex items-center gap-1.5">
                          <span>
                            {header.isPlaceholder
                              ? null
                              : flexRender(header.column.columnDef.header, header.getContext())}
                          </span>
                          {canSort && (
                            <span className="text-slate-400 inline-flex items-center">
                              {isSorted === 'asc' ? (
                                <CaretUp size={13} weight="bold" className="text-cyan-600" />
                              ) : isSorted === 'desc' ? (
                                <CaretDown size={13} weight="bold" className="text-cyan-600" />
                              ) : (
                                <CaretUpDown size={13} className="opacity-40 group-hover:opacity-100" />
                              )}
                            </span>
                          )}
                        </div>
                      </th>
                    )
                  })}
                </tr>
              ))}
            </thead>
            <tbody className="divide-y divide-slate-100">
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={columns.length} className="p-12 text-center text-slate-500 space-y-2">
                    <p className="text-sm font-bold text-slate-800">{emptyMessage}</p>
                    <p className="text-xs text-slate-500 max-w-sm mx-auto">{emptySubMessage}</p>
                  </td>
                </tr>
              ) : (
                rows.map((row) => (
                  <tr
                    key={row.id}
                    onClick={() => onRowClick?.(row.original)}
                    className={`border-b border-slate-100 transition-colors ${
                      onRowClick ? 'hover:bg-cyan-50/40 cursor-pointer' : 'hover:bg-slate-50/60'
                    }`}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id} className="p-4 text-slate-700">
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination Footer */}
        <div className="flex flex-wrap items-center justify-between gap-4 p-4 border-t border-slate-200 bg-slate-50/60 text-xs text-slate-500">
          <div className="flex items-center gap-2">
            <span>Hiển thị</span>
            <select
              value={table.getState().pagination.pageSize}
              onChange={(e) => table.setPageSize(Number(e.target.value))}
              className="bg-white border border-slate-200 rounded-lg px-2.5 py-1 text-slate-700 font-semibold focus:outline-none focus:border-cyan-500 cursor-pointer shadow-2xs"
            >
              {[10, 20, 50, 100].map((size) => (
                <option key={size} value={size}>
                  {size} hàng
                </option>
              ))}
            </select>
            <span className="hidden sm:inline">
              / tổng số <strong className="text-slate-800 font-semibold">{totalFiltered}</strong> kết quả
            </span>
          </div>

          <div className="flex items-center gap-3">
            <span>
              Trang{' '}
              <strong className="text-slate-800 font-semibold">
                {table.getPageCount() === 0 ? 0 : table.getState().pagination.pageIndex + 1}
              </strong>{' '}
              / <strong className="text-slate-700 font-semibold">{table.getPageCount() || 1}</strong>
            </span>
            <div className="inline-flex items-center gap-1">
              <button
                type="button"
                onClick={() => table.previousPage()}
                disabled={!table.getCanPreviousPage()}
                className="p-1.5 rounded-lg border border-slate-200 bg-white hover:bg-slate-100 disabled:opacity-30 disabled:cursor-not-allowed text-slate-700 transition-colors cursor-pointer shadow-2xs"
                title="Trang trước"
              >
                <CaretLeft size={16} />
              </button>
              <button
                type="button"
                onClick={() => table.nextPage()}
                disabled={!table.getCanNextPage()}
                className="p-1.5 rounded-lg border border-slate-200 bg-white hover:bg-slate-100 disabled:opacity-30 disabled:cursor-not-allowed text-slate-700 transition-colors cursor-pointer shadow-2xs"
                title="Trang tiếp"
              >
                <CaretRight size={16} />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
