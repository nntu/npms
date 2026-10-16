import { BookBookmark, CheckCircle, Warning } from '@phosphor-icons/react'
import { useProfiles } from '../devices/useProfiles'

export function ProfilesPage() {
  const profilesQuery = useProfiles()
  const profiles = profilesQuery.data?.data ?? []

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="glass-panel p-6 sm:p-8">
        <div className="flex items-center gap-3 mb-2">
          <BookBookmark size={28} className="text-cyan-600" />
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900">Thư viện Profile nhận diện SNMP</h1>
        </div>
        <p className="text-slate-600 text-sm max-w-2xl">
          Danh sách các profile máy in YAML bên ngoài binary được dùng để đọc chính xác chỉ số bộ đếm trang in theo từng
          dòng máy (HP, Brother, Ricoh, Generic Printer-MIB).
        </p>
      </div>

      {/* Profiles Table */}
      <div className="glass-panel overflow-hidden">
        <div className="p-6 border-b border-slate-200 flex items-center justify-between">
          <div>
            <p className="text-xs font-bold uppercase tracking-wider text-cyan-700">Tệp cấu hình YAML</p>
            <h3 className="text-lg font-bold text-slate-900">Profile máy in đang tải</h3>
          </div>
          <span className="text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200 px-3 py-1 rounded-full">
            {profilesQuery.isLoading ? 'Đang tải...' : `${profiles.length} profiles`}
          </span>
        </div>

        {profilesQuery.isLoading ? (
          <div className="p-10 text-center text-slate-500 text-sm">Đang đọc thư viện profile...</div>
        ) : profilesQuery.isError ? (
          <div className="p-10 text-center text-rose-600 text-sm">Không thể nạp danh sách profile.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs sm:text-sm">
              <thead>
                <tr className="bg-slate-50 text-slate-600 uppercase text-[11px] font-bold tracking-wider border-b border-slate-200">
                  <th className="p-4 pl-6">Mã Profile (ID)</th>
                  <th className="p-4">Hãng sản xuất</th>
                  <th className="p-4">Phiên bản</th>
                  <th className="p-4">Số lượng chỉ số bộ đếm</th>
                  <th className="p-4 pr-6">Trạng thái xác thực</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200">
                {profiles.map((profile) => {
                  const counterCount = profile.counter_keys ? profile.counter_keys.length : 0
                  return (
                    <tr key={profile.id} className="hover:bg-slate-50/70 transition-colors">
                      <td className="p-4 pl-6 font-bold text-slate-900 font-mono text-xs">{profile.id}</td>
                      <td className="p-4 text-slate-700 capitalize">{profile.manufacturer || '—'}</td>
                      <td className="p-4 text-slate-500">v{profile.version}</td>
                      <td className="p-4 font-semibold text-cyan-700 font-mono">{counterCount} bộ đếm</td>
                      <td className="p-4 pr-6">
                        {profile.verification_status === 'verified' ? (
                          <span className="inline-flex items-center gap-1.5 text-xs font-bold text-emerald-800 bg-emerald-50 border border-emerald-200 px-3 py-1 rounded-full">
                            <CheckCircle size={14} /> Đã kiểm thực (Verified)
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1.5 text-xs font-bold text-amber-800 bg-amber-50 border border-amber-200 px-3 py-1 rounded-full">
                            <Warning size={14} /> Thử nghiệm (Experimental)
                          </span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
