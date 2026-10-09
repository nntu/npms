export type PrinterStatus = 'online' | 'offline' | 'unknown' | 'unavailable'

export interface Printer {
  id: string
  asset_code?: string | null
  display_name: string
  manufacturer?: string | null
  model?: string | null
  serial?: string | null
  department?: string | null
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
  department?: string
}

export interface CreateSNMPCredentialInput {
  version: '2c' | '3'
  community?: string
  username?: string
  auth_protocol?: string
  auth_passphrase?: string
  priv_protocol?: string
  priv_passphrase?: string
}

export interface CreateEndpointInput {
  address: string
  protocol?: 'snmp'
  port?: number
  credential_id: string
  is_primary?: boolean
}

export interface RegisterPrinterInput extends CreatePrinterInput {
	profile_id?: string
  address: string
  port?: number
  version: '2c' | '3'
  community?: string
  username?: string
  auth_protocol?: string
  auth_passphrase?: string
  priv_protocol?: string
  priv_passphrase?: string
}

export interface RegisterPrinterResult {
  printer: Printer
  credential_id: string
  endpoint_id: string
  poll_job_id?: string | null
  poll_status: 'queued' | 'not_started'
	profile_id?: string | null
	counter_definition_count?: number
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

export interface DailyUsage {
  definition_key: string
  unit: string
  scope: string
  local_date: string
  delta: number
  quality: 'valid' | 'unverified' | string
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

export interface PrinterProfile {
  id: string
  version: number
  manufacturer: string
  verification_status: 'verified' | 'unverified' | 'experimental' | string
  counter_keys: string[]
}

export interface DiscoveryProbeInput {
  address: string
  version: '2c' | '3'
  community?: string
  port?: number
  timeout?: number
}

export interface DiscoveryResult {
  address: string
  name: string
  description: string
  sys_object_id: string
  serial: string
}

export interface Cartridge {
  id: string
  sku_code: string
  name: string
  compatible_models?: string | null
  stock_new: number
  stock_refilled: number
  stock_empty: number
  stock_refill_bottles: number
  created_at: string
  updated_at: string
}

export interface CreateCartridgeInput {
  sku_code: string
  name: string
  compatible_models?: string
  stock_new?: number
  stock_refilled?: number
  stock_empty?: number
  stock_refill_bottles?: number
}

export interface UpdateCartridgeStockInput {
  cartridge_id: string
  add_stock_new?: number
  add_stock_refilled?: number
  add_stock_empty?: number
  notes?: string
}

export interface AddRefillBottlesInput {
  cartridge_id: string
  quantity: number
  notes?: string
}

export interface ReplaceCartridgeInput {
  cartridge_id: string
  device_id: string
  source_type: 'new' | 'refilled'
  page_count?: number
  notes?: string
}

export interface RefillCartridgesInput {
  cartridge_id: string
  quantity: number
  notes?: string
}

export interface RefillPrinterCartridgeInput {
  cartridge_id: string
  device_id: string
  quantity: number
  page_count?: number
  notes?: string
}

export interface CartridgeLog {
  id: string
  cartridge_id: string
  device_id?: string | null
  action_type: 'import' | 'replace' | 'refill' | 'discard'
  source_type?: 'new' | 'refilled' | null
  quantity: number
  page_count?: number
  printed_pages?: number
  counter_quality?: 'valid' | 'unverified' | 'unsupported' | 'unavailable' | 'suspicious'
  counter_collected_at?: string | null
  notes?: string | null
  performed_at: string
}
