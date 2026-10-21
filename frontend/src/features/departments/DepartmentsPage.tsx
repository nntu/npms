import { Buildings, MagnifyingGlass, PencilSimple, Plus, Printer, Trash, X } from '@phosphor-icons/react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import Swal from 'sweetalert2'
import type { Department } from '../../api/types'
import { useCreateDepartment, useDeleteDepartment, useDepartments, useUpdateDepartment } from './useDepartments'

export function DepartmentsPage() {
  const { data, isLoading, error } = useDepartments()
  const createMutation = useCreateDepartment()
  const updateMutation = useUpdateDepartment()
  const deleteMutation = useDeleteDepartment()

  const [searchQuery, setSearchQuery] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editingDept, setEditingDept] = useState<Department | null>(null)

  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const departments = useMemo(() => data?.data ?? [], [data?.data])

  const filteredDepartments = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q) return departments
    return departments.filter(
      (d) =>
        d.code.toLowerCase().includes(q) ||
        d.name.toLowerCase().includes(q) ||
        (d.description?.toLowerCase().includes(q) ?? false),
    )
  }, [departments, searchQuery])

  const totalDevicesAssigned = useMemo(() => {
    return departments.reduce((acc, curr) => acc + (curr.device_count || 0), 0)
  }, [departments])

  const openCreateModal = () => {
    setEditingDept(null)
    setCode('')
    setName('')
    setDescription('')
    setFormError(null)
    setModalOpen(true)
  }

  const openEditModal = (dept: Department) => {
    setEditingDept(dept)
    setCode(dept.code)
    setName(dept.name)
    setDescription(dept.description || '')
    setFormError(null)
    setModalOpen(true)
  }

  const closeModal = () => {
    setModalOpen(false)
    setEditingDept(null)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)

    const cleanCode = code.trim().toUpperCase()
    const cleanName = name.trim()
    const cleanDesc = description.trim()

    if (!cleanCode) {
      setFormError('Vui lòng nhập mã phòng ban (ví dụ: KT, IT, HCNS)')
      return
    }
    if (!cleanName) {
      setFormError('Vui lòng nhập tên phòng ban (ví dụ: Phòng Kế toán)')
      return
    }

    try {
      if (editingDept) {
        await updateMutation.mutateAsync({
          id: editingDept.id,
          input: {
            code: cleanCode,
            name: cleanName,
            description: cleanDesc || undefined,
          },
        })
        Swal.fire({
          icon: 'success',
          title: 'Đã cập nhật',
          text: `Đã cập nhật phòng ban "${cleanName}" thành công!`,
          timer: 1800,
          showConfirmButton: false,
        })
      } else {
        await createMutation.mutateAsync({
          code: cleanCode,
          name: cleanName,
          description: cleanDesc || undefined,
        })
        Swal.fire({
          icon: 'success',
          title: 'Đã tạo mới',
          text: `Đã thêm phòng ban "${cleanName}" thành công!`,
          timer: 1800,
          showConfirmButton: false,
        })
      }
      closeModal()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Có lỗi xảy ra'
      setFormError(msg)
    }
  }

  const handleDelete = (dept: Department) => {
    const warningText =
      dept.device_count > 0
        ? `Phòng ban này hiện có ${dept.device_count} máy in liên kết. Khi xóa, các máy in sẽ chuyển về trạng thái Chưa phân phòng.`
        : 'Hành động này sẽ xóa vĩnh viễn phòng ban khỏi hệ thống.'

    Swal.fire({
      title: `Xác nhận xóa "${dept.name}"?`,
      text: warningText,
      icon: 'warning',
      showCancelButton: true,
      confirmButtonColor: '#e11d48',
      cancelButtonColor: '#64748b',
      confirmButtonText: 'Đồng ý xóa',
      cancelButtonText: 'Hủy bỏ',
    }).then(async (result) => {
      if (result.isConfirmed) {
        try {
          await deleteMutation.mutateAsync(dept.id)
          Swal.fire({
            icon: 'success',
            title: 'Đã xóa!',
            text: `Đã xóa phòng ban "${dept.name}".`,
            timer: 1500,
            showConfirmButton: false,
          })
        } catch (err: unknown) {
          const msg = err instanceof Error ? err.message : 'Không thể xóa phòng ban'
          Swal.fire({
            icon: 'error',
            title: 'Lỗi',
            text: msg,
          })
        }
      }
    })
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2.5">
            <div className="w-10 h-10 rounded-2xl bg-cyan-100 text-cyan-700 flex items-center justify-center shadow-xs">
              <Buildings size={24} weight="bold" />
            </div>
            <div>
              <h1 className="text-xl sm:text-2xl font-black text-slate-900 tracking-tight">Danh mục Phòng ban</h1>
              <p className="text-xs sm:text-sm text-slate-500 mt-0.5">
                Chuẩn hóa danh mục phòng ban và phân bổ thiết bị để thống kê sản lượng in ấn chính xác
              </p>
            </div>
          </div>
        </div>

        <button
          type="button"
          onClick={openCreateModal}
          className="inline-flex items-center gap-2 bg-cyan-600 hover:bg-cyan-500 text-white px-4 py-2.5 rounded-xl font-semibold text-sm shadow-xs transition-all cursor-pointer self-start sm:self-auto"
        >
          <Plus size={18} weight="bold" />
          <span>Thêm phòng ban</span>
        </button>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="glass-panel p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-bold text-slate-500 uppercase tracking-wider">Tổng số phòng ban</div>
            <div className="text-2xl font-black text-slate-900 mt-1 font-mono">{departments.length}</div>
          </div>
          <div className="w-10 h-10 rounded-xl bg-cyan-50 text-cyan-600 flex items-center justify-center">
            <Buildings size={22} weight="duotone" />
          </div>
        </div>

        <div className="glass-panel p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-bold text-slate-500 uppercase tracking-wider">Máy in đã phân phòng</div>
            <div className="text-2xl font-black text-emerald-600 mt-1 font-mono">{totalDevicesAssigned}</div>
          </div>
          <div className="w-10 h-10 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
            <Printer size={22} weight="duotone" />
          </div>
        </div>

        <div className="glass-panel p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-bold text-slate-500 uppercase tracking-wider">Báo cáo theo phòng</div>
            <Link
              to="/reports/pages"
              className="text-xs font-bold text-cyan-600 hover:text-cyan-700 hover:underline mt-1 inline-block"
            >
              Xem thống kê sản lượng →
            </Link>
          </div>
          <div className="w-10 h-10 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
            <Printer size={22} weight="duotone" />
          </div>
        </div>
      </div>

      {/* Search and Table Container */}
      <div className="glass-panel p-4 sm:p-5 space-y-4">
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="relative flex-1 max-w-md">
            <MagnifyingGlass size={18} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              placeholder="Tìm kiếm mã phòng, tên phòng ban..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-white border border-slate-200 rounded-xl pl-10 pr-4 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:outline-hidden focus:border-cyan-500 shadow-2xs"
            />
          </div>

          <span className="text-xs font-medium text-slate-500 self-center">
            Hiển thị {filteredDepartments.length} / {departments.length} phòng ban
          </span>
        </div>

        {/* Table */}
        <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white">
          <table className="w-full text-left text-sm border-collapse">
            <thead>
              <tr className="border-b border-slate-200 bg-slate-50/80 text-[11px] font-bold text-slate-600 uppercase tracking-wider">
                <th className="py-3 px-4 w-28">Mã phòng</th>
                <th className="py-3 px-4">Tên phòng ban</th>
                <th className="py-3 px-4">Mô tả / Vị trí</th>
                <th className="py-3 px-4 text-center w-36">Số máy in</th>
                <th className="py-3 px-4 text-right w-28">Thao tác</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {isLoading ? (
                <tr>
                  <td colSpan={5} className="py-12 text-center text-slate-400">
                    Đang tải danh sách phòng ban...
                  </td>
                </tr>
              ) : error ? (
                <tr>
                  <td colSpan={5} className="py-12 text-center text-rose-500">
                    Không thể tải dữ liệu phòng ban: {error.message}
                  </td>
                </tr>
              ) : filteredDepartments.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-12 text-center text-slate-400">
                    {searchQuery
                      ? 'Không tìm thấy phòng ban nào khớp với từ khóa tìm kiếm.'
                      : 'Chưa có phòng ban nào. Nhấn "+ Thêm phòng ban" để tạo mới.'}
                  </td>
                </tr>
              ) : (
                filteredDepartments.map((dept) => (
                  <tr key={dept.id} className="hover:bg-slate-50/70 transition-colors">
                    <td className="py-3.5 px-4 font-mono font-bold text-xs text-slate-800">
                      <span className="bg-slate-100 text-slate-800 border border-slate-200 px-2 py-0.5 rounded-md">
                        {dept.code}
                      </span>
                    </td>
                    <td className="py-3.5 px-4 font-semibold text-slate-900">
                      <div className="flex items-center gap-2">
                        <Buildings size={16} className="text-cyan-600 shrink-0" />
                        <span>{dept.name}</span>
                      </div>
                    </td>
                    <td className="py-3.5 px-4 text-slate-600 text-xs max-w-xs truncate">{dept.description || '—'}</td>
                    <td className="py-3.5 px-4 text-center">
                      {dept.device_count > 0 ? (
                        <Link
                          to={`/printers?department=${encodeURIComponent(dept.name)}`}
                          className="inline-flex items-center gap-1.5 bg-cyan-50 hover:bg-cyan-100 text-cyan-800 border border-cyan-200 font-mono text-xs font-bold px-2.5 py-1 rounded-full transition-colors"
                        >
                          <Printer size={13} weight="bold" />
                          <span>{dept.device_count} máy in</span>
                        </Link>
                      ) : (
                        <span className="text-xs text-slate-400">0 máy in</span>
                      )}
                    </td>
                    <td className="py-3.5 px-4 text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        <button
                          type="button"
                          onClick={() => openEditModal(dept)}
                          title="Sửa phòng ban"
                          className="p-1.5 rounded-lg text-slate-600 hover:text-cyan-700 hover:bg-cyan-50 transition-colors cursor-pointer"
                        >
                          <PencilSimple size={16} weight="bold" />
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDelete(dept)}
                          title="Xóa phòng ban"
                          className="p-1.5 rounded-lg text-slate-400 hover:text-rose-600 hover:bg-rose-50 transition-colors cursor-pointer"
                        >
                          <Trash size={16} weight="bold" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Modal Thêm / Sửa Phòng ban */}
      {modalOpen && (
        <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs z-50 flex items-center justify-center p-4">
          <div className="glass-panel p-6 w-full max-w-md space-y-4 shadow-xl border-cyan-200">
            <div className="flex items-center justify-between border-b border-slate-200 pb-3">
              <h3 className="text-base font-bold text-slate-900 flex items-center gap-2">
                <Buildings size={20} className="text-cyan-600" />
                <span>{editingDept ? 'Sửa thông tin phòng ban' : 'Thêm phòng ban mới'}</span>
              </h3>
              <button
                type="button"
                onClick={closeModal}
                className="text-slate-400 hover:text-slate-600 p-1 rounded-lg cursor-pointer"
              >
                <X size={18} />
              </button>
            </div>

            {formError && (
              <div className="bg-rose-50 border border-rose-200 text-rose-700 text-xs px-3.5 py-2.5 rounded-xl">
                {formError}
              </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-3.5">
              <div>
                <label htmlFor="dept-code" className="block text-xs font-bold text-slate-700 mb-1">
                  Mã phòng ban <span className="text-rose-500">*</span>
                </label>
                <input
                  id="dept-code"
                  type="text"
                  placeholder="Ví dụ: KT, IT, HCNS, KD"
                  value={code}
                  onChange={(e) => setCode(e.target.value.toUpperCase())}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 font-mono focus:border-cyan-500 shadow-2xs uppercase"
                  required
                />
                <p className="text-[11px] text-slate-400 mt-1">
                  Mã định danh duy nhất của phòng ban (viết hoa, không dấu).
                </p>
              </div>

              <div>
                <label htmlFor="dept-name" className="block text-xs font-bold text-slate-700 mb-1">
                  Tên phòng ban <span className="text-rose-500">*</span>
                </label>
                <input
                  id="dept-name"
                  type="text"
                  placeholder="Ví dụ: Phòng Kế toán"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 focus:border-cyan-500 shadow-2xs"
                  required
                />
              </div>

              <div>
                <label htmlFor="dept-desc" className="block text-xs font-bold text-slate-700 mb-1">
                  Mô tả / Vị trí đặt máy
                </label>
                <input
                  id="dept-desc"
                  type="text"
                  placeholder="Ví dụ: Tầng 2, Tòa nhà Điều hành"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="w-full bg-white border border-slate-200 rounded-xl px-3.5 py-2 text-sm text-slate-900 focus:border-cyan-500 shadow-2xs"
                />
              </div>

              <div className="flex justify-end gap-2 pt-3 border-t border-slate-200">
                <button
                  type="button"
                  onClick={closeModal}
                  className="bg-slate-100 hover:bg-slate-200 text-slate-700 font-semibold px-4 py-2 rounded-xl text-sm border border-slate-200 cursor-pointer"
                >
                  Hủy
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending || updateMutation.isPending}
                  className="bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-semibold px-5 py-2 rounded-xl text-sm cursor-pointer shadow-xs"
                >
                  {createMutation.isPending || updateMutation.isPending
                    ? 'Đang lưu...'
                    : editingDept
                      ? 'Lưu thay đổi'
                      : 'Tạo phòng ban'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
