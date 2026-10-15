import { Navigate } from 'react-router-dom'

export function ReportsPage() {
  return <Navigate to="/reports/pages" replace />
}

export { CartridgeReportPage } from './CartridgeReportPage'
export { PageUsageReportPage } from './PageUsageReportPage'
