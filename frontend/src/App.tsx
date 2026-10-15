import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { Navbar } from './components/Navbar'

const CartridgesPage = lazy(() =>
  import('./features/cartridges/CartridgesPage').then((module) => ({ default: module.CartridgesPage })),
)
const DevicesPage = lazy(() =>
  import('./features/devices/DevicesPage').then((module) => ({ default: module.DevicesPage })),
)
const PrinterDetailPage = lazy(() =>
  import('./features/devices/PrinterDetailPage').then((module) => ({ default: module.PrinterDetailPage })),
)
const DiscoveryPage = lazy(() =>
  import('./features/discovery/DiscoveryPage').then((module) => ({ default: module.DiscoveryPage })),
)
const ProfilesPage = lazy(() =>
  import('./features/profiles/ProfilesPage').then((module) => ({ default: module.ProfilesPage })),
)
const PageUsageReportPage = lazy(() =>
  import('./features/reports/PageUsageReportPage').then((module) => ({ default: module.PageUsageReportPage })),
)
const CartridgeReportPage = lazy(() =>
  import('./features/reports/CartridgeReportPage').then((module) => ({ default: module.CartridgeReportPage })),
)
const JobsPage = lazy(() => import('./features/jobs/JobsPage').then((module) => ({ default: module.JobsPage })))
const BackupPage = lazy(() => import('./features/backup/BackupPage').then((module) => ({ default: module.BackupPage })))

function RouteLoading() {
  return (
    <div className="flex min-h-64 items-center justify-center text-sm text-slate-400" role="status">
      Đang tải dữ liệu…
    </div>
  )
}

export function App() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      <Navbar />
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Suspense fallback={<RouteLoading />}>
          <Routes>
            <Route path="/" element={<Navigate to="/printers" replace />} />
            <Route path="/printers" element={<DevicesPage />} />
            <Route path="/printers/:id" element={<PrinterDetailPage />} />
            <Route path="/cartridges" element={<CartridgesPage />} />
            <Route path="/discovery" element={<DiscoveryPage />} />
            <Route path="/jobs" element={<JobsPage />} />
            <Route path="/reports" element={<Navigate to="/reports/pages" replace />} />
            <Route path="/reports/pages" element={<PageUsageReportPage />} />
            <Route path="/reports/cartridges" element={<CartridgeReportPage />} />
            <Route path="/profiles" element={<ProfilesPage />} />
            <Route path="/backup" element={<BackupPage />} />
          </Routes>
        </Suspense>
      </main>
    </div>
  )
}
