import { DevicesPage } from './features/devices/DevicesPage'

export function App() {
  return (
    <div className="app-shell">
      <aside className="sidebar"><div className="brand"><span className="brand-mark">N</span><span>NPMS</span></div><nav aria-label="Primary navigation"><a className="nav-link active" href="#printers"><span>▦</span>Printers</a><a className="nav-link muted" href="#reports"><span>◷</span>Reports</a><a className="nav-link muted" href="#discovery"><span>⌁</span>Discovery</a></nav><div className="sidebar-footer"><span className="signal-dot" />LAN collector<br /><small>Standalone mode</small></div></aside>
      <main className="main-content"><header className="topbar"><div><p className="eyebrow">Network Printer Management</p><h1>Operations overview</h1></div><div className="header-meta"><span className="environment">Local environment</span><span className="avatar" aria-label="Current user">OP</span></div></header><div className="content-grid"><div className="intro"><div><p className="eyebrow">Fleet workspace</p><h2>Keep every device observable.</h2><p className="intro-copy">SNMP-first monitoring for a small, reliable LAN printer fleet.</p></div><div className="intro-line" /></div><DevicesPage /></div></main>
    </div>
  )
}
