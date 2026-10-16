import Swal from 'sweetalert2'

const customSwal = Swal.mixin({
  customClass: {
    popup: 'bg-white text-slate-900 border border-slate-200 rounded-2xl shadow-2xl font-sans',
    title: 'text-lg font-bold text-slate-900',
    htmlContainer: 'text-sm text-slate-600',
    confirmButton:
      'bg-cyan-600 hover:bg-cyan-500 text-white font-semibold px-5 py-2.5 rounded-xl text-sm transition-all cursor-pointer shadow-md shadow-cyan-600/20 border-0',
    cancelButton:
      'bg-slate-100 hover:bg-slate-200 text-slate-700 font-semibold px-5 py-2.5 rounded-xl text-sm transition-all cursor-pointer border border-slate-200 ml-3',
  },
  buttonsStyling: false,
  background: '#ffffff',
  color: '#0f172a',
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
    iconColor: '#0284c7',
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
    background: '#ffffff',
    color: '#0f172a',
  })
}
