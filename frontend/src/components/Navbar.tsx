import {
  Archive,
  BookBookmark,
  CaretDown,
  ChartBar,
  Cpu,
  Drop,
  List,
  MagnifyingGlass,
  Printer,
  Pulse,
  Recycle,
  Sliders,
  X,
} from '@phosphor-icons/react'
import { useEffect, useRef, useState } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { VersionBadge } from './VersionBadge'

export function Navbar() {
  const location = useLocation()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [reportsDropdownOpen, setReportsDropdownOpen] = useState(false)
  const [systemDropdownOpen, setSystemDropdownOpen] = useState(false)

  const reportsDropdownRef = useRef<HTMLDivElement>(null)
  const systemDropdownRef = useRef<HTMLDivElement>(null)

  const isReportsActive = location.pathname.startsWith('/reports')
  const isSystemActive = location.pathname === '/profiles' || location.pathname === '/backup'

  // Close dropdowns on route change
  useEffect(() => {
    setReportsDropdownOpen(false)
    setSystemDropdownOpen(false)
    setMobileMenuOpen(false)
  }, [location.pathname])

  // Handle click outside to close dropdowns
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (reportsDropdownRef.current && !reportsDropdownRef.current.contains(event.target as Node)) {
        setReportsDropdownOpen(false)
      }
      if (systemDropdownRef.current && !systemDropdownRef.current.contains(event.target as Node)) {
        setSystemDropdownOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  return (
    <header className="bg-white/95 border-b border-slate-200/90 sticky top-0 z-50 backdrop-blur-md shadow-2xs">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-2 lg:gap-4">
        {/* Brand Logo */}
        <Link to="/printers" className="flex items-center gap-2.5 shrink-0 select-none group">
          <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-cyan-600 to-emerald-500 flex items-center justify-center shadow-xs text-white font-black text-lg transition-transform group-hover:scale-105">
            N
          </div>
          <div className="flex items-center gap-2">
            <span className="font-black text-slate-900 tracking-wider text-base">NPMS</span>
            <span className="hidden sm:inline-block text-[10px] uppercase font-bold tracking-widest text-cyan-800 bg-cyan-50 border border-cyan-200 px-2 py-0.5 rounded-full whitespace-nowrap">
              LAN Printer
            </span>
          </div>
        </Link>

        {/* Desktop Navigation Tabs (Structured & Compact) */}
        <nav className="hidden lg:flex items-center gap-1 xl:gap-1.5 shrink-0">
          {/* Máy in */}
          <NavLink
            to="/printers"
            className={({ isActive }: { isActive: boolean }) =>
              `flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs xl:text-sm font-semibold whitespace-nowrap transition-all ${
                isActive
                  ? 'bg-cyan-50 text-cyan-800 border border-cyan-200 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/80'
              }`
            }
          >
            <Printer size={17} weight="bold" />
            <span>Máy in</span>
          </NavLink>

          {/* Bình mực */}
          <NavLink
            to="/cartridges"
            className={({ isActive }: { isActive: boolean }) =>
              `flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs xl:text-sm font-semibold whitespace-nowrap transition-all ${
                isActive
                  ? 'bg-cyan-50 text-cyan-800 border border-cyan-200 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/80'
              }`
            }
          >
            <Drop size={17} weight="bold" />
            <span>Bình mực</span>
          </NavLink>

          {/* Báo cáo (Dropdown) */}
          <div className="relative" ref={reportsDropdownRef}>
            <button
              type="button"
              onClick={() => {
                setReportsDropdownOpen((prev) => !prev)
                setSystemDropdownOpen(false)
              }}
              className={`flex items-center gap-1 px-3 py-1.5 rounded-xl text-xs xl:text-sm font-semibold whitespace-nowrap transition-all cursor-pointer ${
                isReportsActive
                  ? 'bg-cyan-50 text-cyan-800 border border-cyan-200 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/80'
              }`}
            >
              <ChartBar size={17} weight="bold" />
              <span>Báo cáo</span>
              <CaretDown
                size={14}
                className={`transition-transform duration-200 ${reportsDropdownOpen ? 'rotate-180 text-cyan-700' : 'text-slate-400'}`}
              />
            </button>

            {reportsDropdownOpen && (
              <div className="absolute left-0 mt-2 w-60 rounded-2xl bg-white border border-slate-200 p-2 shadow-xl shadow-slate-900/5 animate-in fade-in slide-in-from-top-2 duration-150 z-50">
                <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 px-3 py-1.5">
                  Thống kê & Phân tích
                </div>
                <NavLink
                  to="/reports/pages"
                  onClick={() => setReportsDropdownOpen(false)}
                  className={({ isActive }: { isActive: boolean }) =>
                    `flex items-start gap-2.5 p-2.5 rounded-xl transition-all ${
                      isActive
                        ? 'bg-cyan-50/90 text-cyan-900 font-semibold'
                        : 'text-slate-700 hover:bg-slate-50 hover:text-slate-900'
                    }`
                  }
                >
                  <ChartBar size={18} weight="bold" className="text-cyan-600 mt-0.5 shrink-0" />
                  <div>
                    <div className="text-xs font-bold leading-none">Báo cáo trang in</div>
                    <div className="text-[11px] text-slate-500 mt-1">Sản lượng in, copy, scan theo phòng & máy</div>
                  </div>
                </NavLink>

                <NavLink
                  to="/reports/cartridges"
                  onClick={() => setReportsDropdownOpen(false)}
                  className={({ isActive }: { isActive: boolean }) =>
                    `flex items-start gap-2.5 p-2.5 rounded-xl transition-all ${
                      isActive
                        ? 'bg-cyan-50/90 text-cyan-900 font-semibold'
                        : 'text-slate-700 hover:bg-slate-50 hover:text-slate-900'
                    }`
                  }
                >
                  <Recycle size={18} weight="bold" className="text-emerald-600 mt-0.5 shrink-0" />
                  <div>
                    <div className="text-xs font-bold leading-none">Báo cáo bình mực</div>
                    <div className="text-[11px] text-slate-500 mt-1">Mực mới vs mực bơm, tỷ lệ tái sử dụng & Yield</div>
                  </div>
                </NavLink>
              </div>
            )}
          </div>

          {/* Dò tìm SNMP */}
          <NavLink
            to="/discovery"
            className={({ isActive }: { isActive: boolean }) =>
              `flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs xl:text-sm font-semibold whitespace-nowrap transition-all ${
                isActive
                  ? 'bg-cyan-50 text-cyan-800 border border-cyan-200 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/80'
              }`
            }
          >
            <MagnifyingGlass size={17} weight="bold" />
            <span>Dò tìm SNMP</span>
          </NavLink>

          {/* Tiến trình */}
          <NavLink
            to="/jobs"
            className={({ isActive }: { isActive: boolean }) =>
              `flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs xl:text-sm font-semibold whitespace-nowrap transition-all ${
                isActive
                  ? 'bg-cyan-50 text-cyan-800 border border-cyan-200 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/80'
              }`
            }
          >
            <Cpu size={17} weight="bold" />
            <span>Tiến trình</span>
          </NavLink>

          {/* Quản trị & Cấu hình (Dropdown) */}
          <div className="relative" ref={systemDropdownRef}>
            <button
              type="button"
              onClick={() => {
                setSystemDropdownOpen((prev) => !prev)
                setReportsDropdownOpen(false)
              }}
              className={`flex items-center gap-1 px-3 py-1.5 rounded-xl text-xs xl:text-sm font-semibold whitespace-nowrap transition-all cursor-pointer ${
                isSystemActive
                  ? 'bg-cyan-50 text-cyan-800 border border-cyan-200 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100/80'
              }`}
            >
              <Sliders size={17} weight="bold" />
              <span>Hệ thống</span>
              <CaretDown
                size={14}
                className={`transition-transform duration-200 ${systemDropdownOpen ? 'rotate-180 text-cyan-700' : 'text-slate-400'}`}
              />
            </button>

            {systemDropdownOpen && (
              <div className="absolute right-0 mt-2 w-64 rounded-2xl bg-white border border-slate-200 p-2 shadow-xl shadow-slate-900/5 animate-in fade-in slide-in-from-top-2 duration-150 z-50">
                <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 px-3 py-1.5">
                  Cấu hình & Quản trị
                </div>
                <NavLink
                  to="/profiles"
                  onClick={() => setSystemDropdownOpen(false)}
                  className={({ isActive }: { isActive: boolean }) =>
                    `flex items-start gap-2.5 p-2.5 rounded-xl transition-all ${
                      isActive
                        ? 'bg-cyan-50/90 text-cyan-900 font-semibold'
                        : 'text-slate-700 hover:bg-slate-50 hover:text-slate-900'
                    }`
                  }
                >
                  <BookBookmark size={18} weight="bold" className="text-cyan-600 mt-0.5 shrink-0" />
                  <div>
                    <div className="text-xs font-bold leading-none">Thư viện Profile</div>
                    <div className="text-[11px] text-slate-500 mt-1">Mẫu nhận diện OID & MIB chuẩn các dòng máy</div>
                  </div>
                </NavLink>

                <NavLink
                  to="/backup"
                  onClick={() => setSystemDropdownOpen(false)}
                  className={({ isActive }: { isActive: boolean }) =>
                    `flex items-start gap-2.5 p-2.5 rounded-xl transition-all ${
                      isActive
                        ? 'bg-cyan-50/90 text-cyan-900 font-semibold'
                        : 'text-slate-700 hover:bg-slate-50 hover:text-slate-900'
                    }`
                  }
                >
                  <Archive size={18} weight="bold" className="text-purple-600 mt-0.5 shrink-0" />
                  <div>
                    <div className="text-xs font-bold leading-none">Sao lưu & Khôi phục</div>
                    <div className="text-[11px] text-slate-500 mt-1">Sao lưu JSON máy in & cơ sở dữ liệu SQLite</div>
                  </div>
                </NavLink>
              </div>
            )}
          </div>
        </nav>

        {/* Right Status Badges & Mobile Menu Toggle */}
        <div className="flex items-center gap-2 sm:gap-3 shrink-0">
          <VersionBadge />

          <div className="hidden xl:flex items-center gap-1.5 text-xs font-semibold text-slate-700 bg-slate-100/90 border border-slate-200 px-2.5 py-1 rounded-full whitespace-nowrap">
            <Pulse size={13} className="text-emerald-500 animate-pulse shrink-0" />
            <span>Mạng LAN độc lập</span>
          </div>

          {/* Mobile Menu Hamburger Button */}
          <button
            type="button"
            onClick={() => setMobileMenuOpen((prev) => !prev)}
            className="lg:hidden p-2 rounded-xl text-slate-600 hover:text-slate-900 hover:bg-slate-100 transition-colors cursor-pointer border border-slate-200"
            aria-label="Toggle navigation menu"
          >
            {mobileMenuOpen ? <X size={20} weight="bold" /> : <List size={20} weight="bold" />}
          </button>
        </div>
      </div>

      {/* Mobile Navigation Drawer / Dropdown */}
      {mobileMenuOpen && (
        <div className="lg:hidden border-t border-slate-200 bg-white px-4 py-4 space-y-2 animate-in fade-in duration-150 shadow-lg">
          <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 px-2">Điều hướng chính</div>
          <div className="grid grid-cols-2 gap-1.5">
            <NavLink
              to="/printers"
              onClick={() => setMobileMenuOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                  isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                }`
              }
            >
              <Printer size={16} weight="bold" className="text-cyan-600" />
              <span>Máy in</span>
            </NavLink>

            <NavLink
              to="/cartridges"
              onClick={() => setMobileMenuOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                  isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                }`
              }
            >
              <Drop size={16} weight="bold" className="text-cyan-600" />
              <span>Bình mực</span>
            </NavLink>

            <NavLink
              to="/discovery"
              onClick={() => setMobileMenuOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                  isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                }`
              }
            >
              <MagnifyingGlass size={16} weight="bold" className="text-cyan-600" />
              <span>Dò tìm SNMP</span>
            </NavLink>

            <NavLink
              to="/jobs"
              onClick={() => setMobileMenuOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                  isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                }`
              }
            >
              <Cpu size={16} weight="bold" className="text-cyan-600" />
              <span>Tiến trình</span>
            </NavLink>
          </div>

          <div className="pt-2 border-t border-slate-100">
            <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 px-2 mb-1.5">
              Báo cáo & Phân tích
            </div>
            <div className="space-y-1">
              <NavLink
                to="/reports/pages"
                onClick={() => setMobileMenuOpen(false)}
                className={({ isActive }) =>
                  `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                    isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                  }`
                }
              >
                <ChartBar size={16} weight="bold" className="text-cyan-600" />
                <span>Báo cáo trang in (Sản lượng)</span>
              </NavLink>
              <NavLink
                to="/reports/cartridges"
                onClick={() => setMobileMenuOpen(false)}
                className={({ isActive }) =>
                  `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                    isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                  }`
                }
              >
                <Recycle size={16} weight="bold" className="text-emerald-600" />
                <span>Báo cáo bình mực (Tiêu thụ & Yield)</span>
              </NavLink>
            </div>
          </div>

          <div className="pt-2 border-t border-slate-100">
            <div className="text-[11px] font-bold uppercase tracking-wider text-slate-400 px-2 mb-1.5">
              Hệ thống & Dữ liệu
            </div>
            <div className="grid grid-cols-2 gap-1.5">
              <NavLink
                to="/profiles"
                onClick={() => setMobileMenuOpen(false)}
                className={({ isActive }) =>
                  `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                    isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                  }`
                }
              >
                <BookBookmark size={16} weight="bold" className="text-cyan-600" />
                <span>Thư viện Profile</span>
              </NavLink>
              <NavLink
                to="/backup"
                onClick={() => setMobileMenuOpen(false)}
                className={({ isActive }) =>
                  `flex items-center gap-2 p-2.5 rounded-xl text-xs font-semibold ${
                    isActive ? 'bg-cyan-50 text-cyan-800 border border-cyan-200' : 'text-slate-700 hover:bg-slate-100'
                  }`
                }
              >
                <Archive size={16} weight="bold" className="text-purple-600" />
                <span>Sao lưu & Dữ liệu</span>
              </NavLink>
            </div>
          </div>
        </div>
      )}
    </header>
  )
}
