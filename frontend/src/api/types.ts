import type { components } from './generated'

export type PrinterStatus = 'online' | 'offline' | 'unknown' | 'unavailable'

export type Printer = components['schemas']['Printer']

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

export type CounterReading = components['schemas']['Counter']

export type DailyUsage = components['schemas']['DailyUsage']

export type PollingRun = components['schemas']['PollingRun']

export type PollJob = components['schemas']['JobOutputBody']

export type PrinterProfile = components['schemas']['Profile']

export interface DiscoveryProbeInput {
  address: string
  version: '2c' | '3'
  community?: string
  port?: number
  timeout?: number
}

export type DiscoveryResult = components['schemas']['DiscoveryOutputBody']

export type Cartridge = components['schemas']['Cartridge']

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

export type CartridgeLog = components['schemas']['CartridgeLog']
