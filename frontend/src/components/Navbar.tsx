import {
  Archive,
  BookBookmark,
  ChartBar,
  Cpu,
  Drop,
  MagnifyingGlass,
  Printer,
  Pulse,
  Recycle,
} from '@phosphor-icons/react'
import { NavLink } from 'react-router-dom'
import { VersionBadge } from './VersionBadge'

export function Navbar() {
  const navItems = [
    { path: '/printers', label: 'Danh sách máy in', icon: Printer },
    { path: '/cartridges', label: 'Quản lý bình mực', icon: Drop },
    { path: '/reports/pages', label: 'Báo cáo trang in', icon: ChartBar },
    { path: '/reports/cartridges', label: 'Báo cáo bình mực', icon: Recycle },
    { path: '/discovery', label: 'Dò tìm SNMP', icon: MagnifyingGlass },
    { path: '/jobs', label: 'Tiến trình', icon: Cpu },
    { path: '/profiles', label: 'Thư viện Profile', icon: BookBookmark },
    { path: '/backup', label: 'Sao lưu & Dữ liệu', icon: Archive },
  ]

  return (
    <header className="bg-white/95 border-b border-slate-200/90 sticky top-0 z-50 backdrop-blur-md shadow-xs">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
        {/* Brand Logo */}
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-cyan-600 to-emerald-500 flex items-center justify-center shadow-md shadow-cyan-600/20 text-white font-black text-lg">
            N
          </div>
          <div>
            <span className="font-extrabold text-slate-900 tracking-wider text-base">NPMS</span>
            <span className="hidden sm:inline-block ml-2 text-[10px] uppercase font-bold tracking-widest text-cyan-800 bg-cyan-50 border border-cyan-200 px-2 py-0.5 rounded-full">
              LAN Printer Platform
            </span>
          </div>
        </div>

        {/* Navigation Tabs */}
        <nav className="flex items-center gap-1 sm:gap-2">
          {navItems.map((item) => {
            const Icon = item.icon
            return (
              <NavLink
                key={item.path}
                to={item.path}
                className={({ isActive }: { isActive: boolean }) =>
                  `flex items-center gap-2 px-3 sm:px-4 py-2 rounded-xl text-xs sm:text-sm font-semibold transition-all ${
                    isActive
                      ? 'bg-cyan-50/90 text-cyan-800 shadow-xs border border-cyan-200'
                      : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/70'
                  }`
                }
              >
                <Icon size={18} weight="bold" />
                <span>{item.label}</span>
              </NavLink>
            )
          })}
        </nav>

        {/* Right Environment Status & Version */}
        <div className="flex items-center gap-2 sm:gap-3">
          <VersionBadge />
          <div className="hidden lg:flex items-center gap-2 text-xs font-semibold text-slate-700 bg-slate-100/80 border border-slate-200 px-3 py-1.5 rounded-full">
            <Pulse size={14} className="text-emerald-500 animate-pulse" />
            <span>Mạng LAN độc lập</span>
          </div>
        </div>
      </div>
    </header>
  )
}
