import type {
  CounterReading,
  CreateEndpointInput,
  CreatePrinterInput,
  CreateSNMPCredentialInput,
  DailyUsage,
  DiscoveryProbeInput,
  DiscoveryResult,
  Paginated,
  PollingRun,
  PollJob,
  Printer,
  PrinterProfile,
  RegisterPrinterInput,
  RegisterPrinterResult,
} from './types'

const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1'
const configuredApiToken = import.meta.env.VITE_API_TOKEN as string | undefined
const sessionApiTokenKey = 'npms_api_token'

export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const requestWithToken = (token?: string) =>
    fetch(`${apiBase}${path}`, {
      ...init,
      headers: {
        Accept: 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...init?.headers,
      },
    })

  const sessionApiToken =
    typeof window === 'undefined' ? undefined : (window.sessionStorage.getItem(sessionApiTokenKey) ?? undefined)
  let response = await requestWithToken(configuredApiToken || sessionApiToken)
  if (response.status === 401 && !configuredApiToken && typeof window !== 'undefined') {
    const enteredToken = window.prompt('Nhập API token NPMS để tiếp tục:')?.trim()
    if (enteredToken) {
      window.sessionStorage.setItem(sessionApiTokenKey, enteredToken)
      response = await requestWithToken(enteredToken)
    }
  }
  if (!response.ok) {
    let message = `API request failed (${response.status})`
    try {
      const body = (await response.json()) as { message?: string; error?: { message?: string } }
      if (body.error?.message) message = body.error.message
      else if (body.message) message = body.message
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
): Promise<{ data: import('./types').CartridgeLog[]; total: number }> {
  const params = new URLSearchParams()
  if (cartridgeId) params.set('cartridge_id', cartridgeId)
  if (deviceId) params.set('device_id', deviceId)
  return request<{ data: import('./types').CartridgeLog[]; total: number }>(`/cartridges/logs?${params.toString()}`)
}
