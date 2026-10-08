import type { CounterReading, CreatePrinterInput, Paginated, PollingRun, PollJob, Printer } from './types'

const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1'
const apiToken = import.meta.env.VITE_API_TOKEN as string | undefined

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
      ...(apiToken ? { Authorization: `Bearer ${apiToken}` } : {}),
      ...init?.headers,
    },
  })
  if (!response.ok) {
    let message = `API request failed (${response.status})`
    try {
      const body = (await response.json()) as { message?: string }
      if (body.message) message = body.message
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
  return request<Printer>('/printers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) })
}

export async function listPrinterCounters(id: string, limit = 50, offset = 0): Promise<Paginated<CounterReading>> {
  return request<Paginated<CounterReading>>(`/printers/${encodeURIComponent(id)}/counters?limit=${limit}&offset=${offset}`)
}

export async function listPollingRuns(id: string, limit = 20, offset = 0): Promise<Paginated<PollingRun>> {
  return request<Paginated<PollingRun>>(`/printers/${encodeURIComponent(id)}/polling-runs?limit=${limit}&offset=${offset}`)
}

export async function startPrinterPoll(id: string): Promise<PollJob> {
  const response = await request<{ job_id: string; status: PollJob['status']; status_url: string }>(`/printers/${encodeURIComponent(id)}/poll`, { method: 'POST' })
  return { id: response.job_id, device_id: id, status: response.status }
}

export async function getJob(id: string): Promise<PollJob> {
  return request<PollJob>(`/jobs/${encodeURIComponent(id)}`)
}
