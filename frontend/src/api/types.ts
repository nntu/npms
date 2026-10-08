export type PrinterStatus = 'online' | 'offline' | 'unknown' | 'unavailable'

export interface Printer {
  id: string
  asset_code?: string | null
  display_name: string
  manufacturer?: string | null
  model?: string | null
  serial?: string | null
  status: PrinterStatus
  last_seen_at?: string | null
}

export interface CreatePrinterInput {
  display_name: string
  asset_code?: string
  manufacturer?: string
  model?: string
  serial?: string
  sys_object_id?: string
}

export interface Paginated<T> {
  data: T[]
  limit: number
  offset: number
  total?: number
}

export type CounterQuality = 'valid' | 'unverified' | 'unsupported' | 'unavailable' | 'suspicious'

export interface CounterReading {
  id: number
  definition_key: string
  unit: string
  scope: string
  raw_value: number
  collected_at: string
  quality: CounterQuality
}

export interface PollingRun {
  id: string
  job_kind: string
  started_at: string
  ended_at?: string | null
  result: 'running' | 'success' | 'failed' | string
  error_code?: string | null
  profile_version?: number
  attempt_count: number
}

export interface PollJob {
  id: string
  device_id: string
  status: 'queued' | 'success' | 'failed'
  error_code?: string | null
}
