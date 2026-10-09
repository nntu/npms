import { BookBookmark, ChartBar, Drop, MagnifyingGlass, Printer, Pulse } from '@phosphor-icons/react'
import { NavLink } from 'react-router-dom'

export function Navbar() {
  const navItems = [
    { path: '/printers', label: 'Danh sách máy in', icon: Printer },
    { path: '/cartridges', label: 'Quản lý bình mực', icon: Drop },
    { path: '/discovery', label: 'Dò tìm SNMP', icon: MagnifyingGlass },
    { path: '/reports', label: 'Báo cáo sử dụng', icon: ChartBar },
    { path: '/profiles', label: 'Thư viện Profile', icon: BookBookmark },
  ]

  return (
    <header className="bg-slate-900/90 border-b border-slate-800/80 sticky top-0 z-50 backdrop-blur-md">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
        {/* Brand Logo */}
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-cyan-600 to-emerald-400 flex items-center justify-center shadow-lg shadow-cyan-900/30 text-slate-950 font-black text-lg">
            N
          </div>
          <div>
            <span className="font-extrabold text-slate-100 tracking-wider text-base">NPMS</span>
            <span className="hidden sm:inline-block ml-2 text-[10px] uppercase font-bold tracking-widest text-cyan-400 bg-cyan-950/80 border border-cyan-800/60 px-2 py-0.5 rounded-full">
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
                      ? 'bg-slate-800 text-cyan-400 shadow-sm shadow-slate-900 border border-slate-700/80'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
                  }`
                }
              >
                <Icon size={18} weight="bold" />
                <span>{item.label}</span>
              </NavLink>
            )
          })}
        </nav>

        {/* Right Environment Status */}
        <div className="hidden lg:flex items-center gap-3">
          <div className="flex items-center gap-2 text-xs font-semibold text-slate-300 bg-slate-800/80 border border-slate-700/60 px-3 py-1.5 rounded-full">
            <Pulse size={14} className="text-emerald-400 animate-pulse" />
            <span>Mạng LAN độc lập</span>
          </div>
        </div>
      </div>
    </header>
  )
}
