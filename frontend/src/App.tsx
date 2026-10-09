import { Navigate, Route, Routes } from 'react-router-dom'
import { Navbar } from './components/Navbar'
import { CartridgesPage } from './features/cartridges/CartridgesPage'
import { DevicesPage } from './features/devices/DevicesPage'
import { PrinterDetailPage } from './features/devices/PrinterDetailPage'
import { DiscoveryPage } from './features/discovery/DiscoveryPage'
import { ProfilesPage } from './features/profiles/ProfilesPage'
import { ReportsPage } from './features/reports/ReportsPage'

export function App() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      <Navbar />
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Routes>
          <Route path="/" element={<Navigate to="/printers" replace />} />
          <Route path="/printers" element={<DevicesPage />} />
          <Route path="/printers/:id" element={<PrinterDetailPage />} />
          <Route path="/cartridges" element={<CartridgesPage />} />
          <Route path="/discovery" element={<DiscoveryPage />} />
          <Route path="/reports" element={<ReportsPage />} />
          <Route path="/profiles" element={<ProfilesPage />} />
        </Routes>
      </main>
    </div>
  )
}
