import Swal from 'sweetalert2'

const customSwal = Swal.mixin({
  customClass: {
    popup: 'bg-slate-900 text-slate-100 border border-slate-800 rounded-2xl shadow-2xl font-sans',
    title: 'text-lg font-bold text-slate-100',
    htmlContainer: 'text-sm text-slate-300',
    confirmButton:
      'bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all cursor-pointer shadow-lg shadow-cyan-900/30 border-0',
    cancelButton:
      'bg-slate-800 hover:bg-slate-700 text-slate-300 font-semibold px-5 py-2.5 rounded-xl text-sm transition-all cursor-pointer border-0 ml-3',
  },
  buttonsStyling: false,
  background: '#0f172a',
  color: '#f8fafc',
})

export function showSuccess(title: string, text?: string) {
  return customSwal.fire({
    icon: 'success',
    title,
    text,
    timer: 2500,
    showConfirmButton: false,
    iconColor: '#10b981',
  })
}

export function showError(title: string, text?: string) {
  return customSwal.fire({
    icon: 'error',
    title,
    text,
    confirmButtonText: 'Đóng',
    iconColor: '#f43f5e',
  })
}

export function showConfirm(title: string, text: string, confirmButtonText = 'Xác nhận') {
  return customSwal.fire({
    icon: 'question',
    title,
    text,
    showCancelButton: true,
    confirmButtonText,
    cancelButtonText: 'Hủy bỏ',
    iconColor: '#38bdf8',
  })
}

export function showToast(title: string, icon: 'success' | 'error' | 'info' | 'warning' = 'success') {
  return Swal.fire({
    toast: true,
    position: 'top-end',
    icon,
    title,
    showConfirmButton: false,
    timer: 3000,
    timerProgressBar: true,
    background: '#1e293b',
    color: '#f8fafc',
  })
}
