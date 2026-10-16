import type {
  BulkRegisterInput,
  BulkRegisterOutput,
  CounterReading,
  CreateEndpointInput,
  CreatePrinterInput,
  CreateSNMPCredentialInput,
  DailyUsage,
  DatabaseRestoreResponse,
  DiscoveryProbeInput,
  DiscoveryResult,
  Paginated,
  PollingRun,
  PollJob,
  Printer,
  PrinterProfile,
  PrintersRestoreResponse,
  RegisterPrinterInput,
  RegisterPrinterResult,
  UpdatePrinterInput,
  VersionInfo,
} from './types'

const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1'
const configuredApiToken = import.meta.env.VITE_API_TOKEN as string | undefined

export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(configuredApiToken ? { Authorization: `Bearer ${configuredApiToken}` } : {}),
      ...init?.headers,
    },
  })
  if (!response.ok) {
    let message = `API request failed (${response.status})`
    try {
      const body = (await response.json()) as {
        detail?: string
        title?: string
        message?: string
        error?: { message?: string }
      }
      if (body.error?.message) message = body.error.message
      else if (body.message) message = body.message
      else if (body.detail) message = body.detail
      else if (body.title) message = body.title
    } catch {
      // Keep the status-based message when the server has no JSON error body.
    }
    throw new ApiError(message, response.status)
  }
  return (await response.json()) as T
}

export async function listPrinters(limit = 50, offset = 0): Promise<Paginated<Printer>> {
  const response = await request<Paginated<Printer> | Printer[]>(`/printers?limit=${limit}&offset=${offset}`)
  if (Array.isArray(response)) return { data: response, limit, offset }
  return response
}

export async function getPrinter(id: string): Promise<Printer> {
  return request<Printer>(`/printers/${encodeURIComponent(id)}`)
}

export async function createPrinter(input: CreatePrinterInput): Promise<Printer> {
  return request<Printer>('/printers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function registerPrinter(input: RegisterPrinterInput): Promise<RegisterPrinterResult> {
  return request<RegisterPrinterResult>('/printers/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function bulkRegisterPrinters(input: BulkRegisterInput): Promise<BulkRegisterOutput> {
  return request<BulkRegisterOutput>('/printers/bulk-register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function updatePrinter(id: string, input: UpdatePrinterInput): Promise<Printer> {
  return request<Printer>(`/printers/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function deletePrinter(id: string): Promise<{ status: string; id: string }> {
  return request<{ status: string; id: string }>(`/printers/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export async function createPrinterCredential(
  id: string,
  input: CreateSNMPCredentialInput,
): Promise<{ id: string; version: string }> {
  return request<{ id: string; version: string }>(`/printers/${encodeURIComponent(id)}/credentials`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function createPrinterEndpoint(id: string, input: CreateEndpointInput): Promise<unknown> {
  return request<unknown>(`/printers/${encodeURIComponent(id)}/endpoints`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function listPrinterCounters(id: string, limit = 50, offset = 0): Promise<Paginated<CounterReading>> {
  return request<Paginated<CounterReading>>(
    `/printers/${encodeURIComponent(id)}/counters?limit=${limit}&offset=${offset}`,
  )
}

export async function listPrinterUsage(
  id: string,
  timezone = Intl.DateTimeFormat().resolvedOptions().timeZone,
): Promise<{ data: DailyUsage[]; total: number; timezone: string }> {
  return request<{ data: DailyUsage[]; total: number; timezone: string }>(
    `/printers/${encodeURIComponent(id)}/usage?timezone=${encodeURIComponent(timezone)}`,
  )
}

export async function listPollingRuns(id: string, limit = 20, offset = 0): Promise<Paginated<PollingRun>> {
  return request<Paginated<PollingRun>>(
    `/printers/${encodeURIComponent(id)}/polling-runs?limit=${limit}&offset=${offset}`,
  )
}

export async function startPrinterPoll(id: string): Promise<PollJob> {
  const response = await request<{ job_id: string; status: PollJob['status']; status_url: string }>(
    `/printers/${encodeURIComponent(id)}/poll`,
    { method: 'POST' },
  )
  return { id: response.job_id, device_id: id, status: response.status }
}

export async function getJob(id: string): Promise<PollJob> {
  return request<PollJob>(`/jobs/${encodeURIComponent(id)}`)
}

export async function listProfiles(): Promise<{ data: PrinterProfile[]; total: number }> {
  return request<{ data: PrinterProfile[]; total: number }>('/snmp/profiles')
}

export async function probeDiscovery(input: DiscoveryProbeInput): Promise<DiscoveryResult> {
  return request<DiscoveryResult>('/discovery/probe', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function scanSubnet(
  input: import('./types').SubnetScanInput,
): Promise<import('./types').SubnetScanResult> {
  return request<import('./types').SubnetScanResult>('/discovery/scan', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function listCartridges(): Promise<{ data: import('./types').Cartridge[]; total: number }> {
  return request<{ data: import('./types').Cartridge[]; total: number }>('/cartridges')
}

export async function createCartridge(
  input: import('./types').CreateCartridgeInput,
): Promise<import('./types').Cartridge> {
  return request<import('./types').Cartridge>('/cartridges', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function updateCartridgeStock(
  input: import('./types').UpdateCartridgeStockInput,
): Promise<{ status: string }> {
  return request<{ status: string }>('/cartridges/stock', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function addRefillBottles(input: import('./types').AddRefillBottlesInput): Promise<{ status: string }> {
  return request<{ status: string }>('/cartridges/stock/refill-bottles', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function replacePrinterCartridge(
  input: import('./types').ReplaceCartridgeInput,
): Promise<{ status: string; message: string; counter: number; counter_quality: string }> {
  return request<{ status: string; message: string; counter: number; counter_quality: string }>('/cartridges/replace', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function refillCartridges(
  input: import('./types').RefillCartridgesInput,
): Promise<{ status: string; message: string }> {
  return request<{ status: string; message: string }>('/cartridges/refill', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function refillPrinterCartridge(
  input: import('./types').RefillPrinterCartridgeInput,
): Promise<{ status: string; message: string; counter: number; counter_quality: string }> {
  return request<{ status: string; message: string; counter: number; counter_quality: string }>(
    '/cartridges/refill-printer',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    },
  )
}

export async function listCartridgeLogs(
  cartridgeId?: string,
  deviceId?: string,
  limit = 1000,
  offset = 0,
): Promise<{ data: import('./types').CartridgeLog[]; total: number }> {
  const params = new URLSearchParams()
  if (cartridgeId) params.set('cartridge_id', cartridgeId)
  if (deviceId) params.set('device_id', deviceId)
  if (limit) params.set('limit', String(limit))
  if (offset) params.set('offset', String(offset))
  return request<{ data: import('./types').CartridgeLog[]; total: number }>(`/cartridges/logs?${params.toString()}`)
}

export async function listJobs(status?: string, limit = 100, offset = 0): Promise<import('./types').ListJobsResponse> {
  const params = new URLSearchParams()
  if (status && status !== 'all') params.set('status', status)
  if (limit) params.set('limit', String(limit))
  if (offset) params.set('offset', String(offset))
  return request<import('./types').ListJobsResponse>(`/jobs?${params.toString()}`)
}

export interface PageUsageReport {
  from?: string
  to?: string
  total_departments: number
  total_printers: number
  total_print_pages: number
  total_scan_pages: number
  total_initial_life: number
  total_current_life: number
  departments: DepartmentPageUsage[]
  printers: PrinterPageUsage[]
}

export interface DepartmentPageUsage {
  department: string
  printer_count: number
  total_print_pages: number
  total_scan_pages: number
  total_initial_life: number
  total_current_life: number
  printers: PrinterPageUsage[]
}

export interface PrinterPageUsage {
  device_id: string
  display_name: string
  model: string
  manufacturer: string
  address: string
  serial: string
  department: string
  category: 'printer' | 'scanner'
  status: string
  counter_key: string
  raw_unit: string
  initial_reading?: number
  current_reading?: number
  period_usage: number
  conversion_factor: number
  normalized_pages: number
  conversion_note: string
}

export async function getPageUsageReport(params?: {
  from?: string
  to?: string
  department?: string
}): Promise<PageUsageReport> {
  const query = new URLSearchParams()
  if (params?.from) query.set('from', params.from)
  if (params?.to) query.set('to', params.to)
  if (params?.department && params.department !== 'all') query.set('department', params.department)
  const qs = query.toString()
  return request<PageUsageReport>(`/reports/pages${qs ? `?${qs}` : ''}`)
}

export async function downloadPrintersBackup(): Promise<void> {
  const response = await fetch(`${apiBase}/backup/printers`, {
    headers: {
      ...(configuredApiToken ? { Authorization: `Bearer ${configuredApiToken}` } : {}),
    },
  })
  if (!response.ok) {
    throw new ApiError(`Không thể tải bản sao lưu máy in (${response.status})`, response.status)
  }
  const blob = await response.blob()
  const contentDisposition = response.headers.get('content-disposition')
  let filename = 'npms-printers-backup.json'
  if (contentDisposition) {
    const match = /filename="?([^"]+)"?/.exec(contentDisposition)
    if (match?.[1]) filename = match[1]
  }
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export async function restorePrintersBackup(file: File, overwrite = true): Promise<PrintersRestoreResponse> {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('overwrite', String(overwrite))

  const response = await fetch(`${apiBase}/backup/printers/restore`, {
    method: 'POST',
    body: formData,
    headers: {
      Accept: 'application/json',
      ...(configuredApiToken ? { Authorization: `Bearer ${configuredApiToken}` } : {}),
    },
  })
  if (!response.ok) {
    let message = `Khôi phục máy in thất bại (${response.status})`
    try {
      const err = (await response.json()) as { message?: string; error?: { message?: string } }
      if (err.message) message = err.message
      else if (err.error?.message) message = err.error.message
    } catch {
      // ignore
    }
    throw new ApiError(message, response.status)
  }
  return response.json()
}

export async function downloadDatabaseBackup(): Promise<void> {
  const response = await fetch(`${apiBase}/backup/database`, {
    headers: {
      ...(configuredApiToken ? { Authorization: `Bearer ${configuredApiToken}` } : {}),
    },
  })
  if (!response.ok) {
    throw new ApiError(`Không thể tải bản sao lưu cơ sở dữ liệu (${response.status})`, response.status)
  }
  const blob = await response.blob()
  const contentDisposition = response.headers.get('content-disposition')
  let filename = 'npms-database-backup.db'
  if (contentDisposition) {
    const match = /filename="?([^"]+)"?/.exec(contentDisposition)
    if (match?.[1]) filename = match[1]
  }
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export async function restoreDatabaseBackup(file: File): Promise<DatabaseRestoreResponse> {
  const formData = new FormData()
  formData.append('file', file)

  const response = await fetch(`${apiBase}/backup/database/restore`, {
    method: 'POST',
    body: formData,
    headers: {
      Accept: 'application/json',
      ...(configuredApiToken ? { Authorization: `Bearer ${configuredApiToken}` } : {}),
    },
  })
  if (!response.ok) {
    let message = `Khôi phục cơ sở dữ liệu thất bại (${response.status})`
    try {
      const err = (await response.json()) as { message?: string; error?: { message?: string } }
      if (err.message) message = err.message
      else if (err.error?.message) message = err.error.message
    } catch {
      // ignore
    }
    throw new ApiError(message, response.status)
  }
  return response.json()
}

export async function relocatePrinterIP(
  id: string,
  newAddress: string,
): Promise<{ success: boolean; message: string; device_id: string; old_address: string; new_address: string }> {
  return request<{ success: boolean; message: string; device_id: string; old_address: string; new_address: string }>(
    `/printers/${id}/relocate-ip`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ address: newAddress }),
    },
  )
}

export async function getVersion(): Promise<VersionInfo> {
  return request<VersionInfo>('/version')
}
