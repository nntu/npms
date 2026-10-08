import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { DailyUsage, Printer, PrinterStatus } from '../../api/types'
import { usePrinters } from './usePrinters'
import { usePrinter } from './usePrinter'
import { usePrinterCounters } from './usePrinterCounters'
import { usePrinterUsage } from './usePrinterUsage'
import { usePollingRuns } from './usePollingRuns'
import { probeDiscovery, registerPrinter, startPrinterPoll } from '../../api/client'
import { usePollJob } from './usePollJob'
import { CounterChart } from './CounterChart'
import { DailyUsageChart } from './DailyUsageChart'
import { useProfiles } from './useProfiles'

const statusLabels: Record<PrinterStatus, string> = {
  online: 'Online',
  offline: 'Offline',
  unknown: 'Unknown',
  unavailable: 'Unavailable',
}

function StatusBadge({ status }: { status: PrinterStatus }) {
  return <span className={`status status-${status}`} aria-label={`Status: ${statusLabels[status]}`}>{statusLabels[status]}</span>
}

type MonthlyUsage = Omit<DailyUsage, 'local_date'> & { month: string }

function aggregateMonthlyUsage(items: DailyUsage[]): MonthlyUsage[] {
  const grouped = new Map<string, MonthlyUsage>()
  for (const item of items) {
    const month = item.local_date.slice(0, 7)
    const key = `${item.definition_key}-${month}`
    const current = grouped.get(key)
    if (current) {
      current.delta += item.delta
      if (current.quality === 'valid' && item.quality !== 'valid') current.quality = item.quality
      continue
    }
    grouped.set(key, { ...item, month })
  }
  return [...grouped.values()].sort((left, right) => left.month.localeCompare(right.month) || left.definition_key.localeCompare(right.definition_key))
}

function PrinterRow({ printer, onSelect }: { printer: Printer; onSelect: (id: string) => void }) {
  return (
    <tr>
      <td><button className="link-button printer-name" onClick={() => onSelect(printer.id)}>{printer.display_name}</button><small>{printer.asset_code ?? 'No asset code'}</small></td>
      <td>{printer.manufacturer || '—'}</td>
      <td>{printer.model || '—'}</td>
      <td><StatusBadge status={printer.status} /></td>
      <td>{printer.last_seen_at ? new Date(printer.last_seen_at).toLocaleString() : 'Never'}</td>
    </tr>
  )
}

function PrinterDetail({ id, onBack }: { id: string; onBack: () => void }) {
  const query = usePrinter(id)
    const counters = usePrinterCounters(id)
    const usage = usePrinterUsage(id)
    const monthlyUsage = useMemo(() => aggregateMonthlyUsage(usage.data?.data ?? []), [usage.data?.data])
  const pollingRuns = usePollingRuns(id)
  const queryClient = useQueryClient()
  const [jobID, setJobID] = useState<string | null>(null)
  const job = usePollJob(jobID)
  const poll = useMutation({ mutationFn: () => startPrinterPoll(id), onSuccess: (created) => setJobID(created.id) })
  if (query.isLoading) return <section className="panel state"><div className="spinner" aria-hidden="true" /><p>Loading printer details…</p></section>
  if (query.isError || !query.data) return <section className="panel state error-state"><p className="eyebrow">Printer details</p><h2>Details are unavailable</h2><p>{query.error instanceof Error ? query.error.message : 'The printer was not found.'}</p><button className="button" onClick={onBack}>Back to registry</button></section>

  const printer = query.data
  const refreshAfterPoll = () => { void queryClient.invalidateQueries({ queryKey: ['printer', id] }); void queryClient.invalidateQueries({ queryKey: ['printer-counters', id] }); void queryClient.invalidateQueries({ queryKey: ['printer-polling-runs', id] }) }
  return <section className="panel detail-panel">
    <div className="panel-heading"><div><button className="back-button" onClick={onBack}>← Back to registry</button><p className="eyebrow">Printer details</p><h2>{printer.display_name}</h2></div><div className="detail-actions"><StatusBadge status={printer.status} /><button className="button" disabled={poll.isPending || (job.data?.status === 'queued')} onClick={() => { setJobID(null); poll.mutate() }}>{poll.isPending ? 'Starting…' : 'Poll now'}</button></div></div>
    {poll.isError && <p className="inline-error">{poll.error instanceof Error ? poll.error.message : 'Unable to start polling.'}</p>}
    {job.data && <p className={`job-notice job-${job.data.status}`}>{job.data.status === 'queued' ? 'Polling queued…' : job.data.status === 'success' ? 'Polling completed.' : `Polling failed${job.data.error_code ? `: ${job.data.error_code}` : '.'}`}{job.data.status !== 'queued' && <button className="refresh-link" onClick={refreshAfterPoll}>Refresh data</button>}</p>}
    <dl className="detail-grid">
      <div><dt>Asset code</dt><dd>{printer.asset_code ?? '—'}</dd></div>
      <div><dt>Manufacturer</dt><dd>{printer.manufacturer ?? '—'}</dd></div>
      <div><dt>Model</dt><dd>{printer.model ?? '—'}</dd></div>
      <div><dt>Serial</dt><dd>{printer.serial ?? '—'}</dd></div>
      <div><dt>Last seen</dt><dd>{printer.last_seen_at ? new Date(printer.last_seen_at).toLocaleString() : 'Never'}</dd></div>
      <div><dt>Device ID</dt><dd className="mono">{printer.id}</dd></div>
    </dl>
    <div className="subpanel-heading"><div><p className="eyebrow">Immutable raw readings</p><h3>Counter history</h3></div><span className="count">{counters.data?.total ?? 0} readings</span></div>
    {!counters.isLoading && !counters.isError && (counters.data?.data.length ?? 0) > 0 && <CounterChart readings={counters.data?.data ?? []} />}
    {counters.isLoading ? <div className="state compact-state"><div className="spinner" aria-hidden="true" /><p>Loading counter readings…</p></div> : counters.isError ? <div className="state compact-state error-state"><p>Counter readings are unavailable.</p></div> : (counters.data?.data.length ?? 0) === 0 ? <div className="state compact-state"><p>No counter readings recorded yet.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Counter readings</caption><thead><tr><th scope="col">Counter</th><th scope="col">Raw value</th><th scope="col">Unit</th><th scope="col">Quality</th><th scope="col">Collected</th></tr></thead><tbody>{counters.data?.data.map((reading) => <tr key={reading.id}><td>{reading.definition_key}</td><td>{reading.raw_value.toLocaleString()}</td><td>{reading.unit}</td><td><span className={`quality quality-${reading.quality}`}>{reading.quality}</span></td><td>{new Date(reading.collected_at).toLocaleString()}</td></tr>)}</tbody></table></div>}
    <div className="subpanel-heading"><div><p className="eyebrow">Worker diagnostics</p><h3>Polling runs</h3></div><span className="count">{pollingRuns.data?.total ?? 0} runs</span></div>
    {pollingRuns.isLoading ? <div className="state compact-state"><div className="spinner" aria-hidden="true" /><p>Loading polling runs…</p></div> : pollingRuns.isError ? <div className="state compact-state error-state"><p>Polling diagnostics are unavailable.</p></div> : (pollingRuns.data?.data.length ?? 0) === 0 ? <div className="state compact-state"><p>No polling runs recorded yet.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Polling runs</caption><thead><tr><th scope="col">Job</th><th scope="col">Result</th><th scope="col">Attempts</th><th scope="col">Started</th><th scope="col">Error</th></tr></thead><tbody>{pollingRuns.data?.data.map((run) => <tr key={run.id}><td>{run.job_kind}</td><td><span className={`quality quality-${run.result}`}>{run.result}</span></td><td>{run.attempt_count}</td><td>{new Date(run.started_at).toLocaleString()}</td><td>{run.error_code ?? '—'}</td></tr>)}</tbody></table></div>}
    <div className="subpanel-heading"><div><p className="eyebrow">Derived usage</p><h3>Daily pages</h3></div><span className="count">{usage.data?.total ?? 0} days</span></div>
    {!usage.isLoading && !usage.isError && (usage.data?.data.length ?? 0) > 0 && <DailyUsageChart usage={usage.data?.data ?? []} />}
      {usage.isLoading ? <div className="state compact-state"><div className="spinner" aria-hidden="true" /><p>Calculating daily usage…</p></div> : usage.isError ? <div className="state compact-state error-state"><p>Daily usage is unavailable.</p></div> : (usage.data?.data.length ?? 0) === 0 ? <div className="state compact-state"><p>Not enough trusted readings to calculate daily usage.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Daily printer usage</caption><thead><tr><th scope="col">Date</th><th scope="col">Counter</th><th scope="col">Usage</th><th scope="col">Quality</th></tr></thead><tbody>{usage.data?.data.map((item) => <tr key={`${item.definition_key}-${item.local_date}`}><td>{item.local_date}</td><td>{item.definition_key}</td><td>{item.delta.toLocaleString()} {item.unit}</td><td><span className={`quality quality-${item.quality}`}>{item.quality}</span></td></tr>)}</tbody></table></div>}
      <div className="subpanel-heading"><div><p className="eyebrow">Derived usage</p><h3>Monthly pages</h3></div><span className="count">{monthlyUsage.length} months</span></div>
      {monthlyUsage.length === 0 ? <div className="state compact-state"><p>No monthly usage is available yet.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Monthly printer usage</caption><thead><tr><th scope="col">Month</th><th scope="col">Counter</th><th scope="col">Usage</th><th scope="col">Quality</th></tr></thead><tbody>{monthlyUsage.map((item) => <tr key={`${item.definition_key}-${item.month}`}><td>{item.month}</td><td>{item.definition_key}</td><td>{item.delta.toLocaleString()} {item.unit}</td><td><span className={`quality quality-${item.quality}`}>{item.quality}</span></td></tr>)}</tbody></table></div>}
  </section>
}

export function DevicesPage() {
  const query = usePrinters()
  const queryClient = useQueryClient()
  const profiles = useProfiles()
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [displayName, setDisplayName] = useState('')
  const [address, setAddress] = useState('')
  const [community, setCommunity] = useState('')
  const [showDiscovery, setShowDiscovery] = useState(false)
  const [probeAddress, setProbeAddress] = useState('')
  const [probeCommunity, setProbeCommunity] = useState('')
  const [registrationJobID, setRegistrationJobID] = useState<string | null>(null)
  const registrationJob = usePollJob(registrationJobID)
  const discovery = useMutation({ mutationFn: () => probeDiscovery({ address: probeAddress.trim(), version: '2c', community: probeCommunity }), onSuccess: () => setShowDiscovery(true) })
  const registerDiscovered = () => {
    if (!discovery.data) return
    setDisplayName(discovery.data.name || discovery.data.description || discovery.data.address)
    setAddress(discovery.data.address)
    setCommunity(probeCommunity)
    setShowDiscovery(false)
    setShowCreate(true)
  }
  const create = useMutation({ mutationFn: () => registerPrinter({ display_name: displayName.trim(), address: address.trim(), version: '2c', community }), onSuccess: (result) => { setRegistrationJobID(result.poll_job_id ?? null); setDisplayName(''); setAddress(''); setCommunity(''); setShowCreate(false); void query.refetch() } })
  useEffect(() => {
    if (registrationJob.data?.status === 'success' || registrationJob.data?.status === 'failed') void queryClient.invalidateQueries({ queryKey: ['printers'] })
  }, [registrationJob.data?.status, queryClient])

  if (selectedID) return <PrinterDetail id={selectedID} onBack={() => setSelectedID(null)} />

  if (query.isLoading) return <section className="panel state"><div className="spinner" aria-hidden="true" /><p>Loading printer registry…</p></section>
  if (query.isError) return <section className="panel state error-state"><p className="eyebrow">Connection problem</p><h2>Printer registry is unavailable</h2><p>{query.error instanceof Error ? query.error.message : 'The API did not respond.'}</p><button className="button" onClick={() => void query.refetch()}>Try again</button></section>

  const printers = query.data?.data ?? []
  return (
    <section className="panel">
      <div className="panel-heading"><div><p className="eyebrow">Registry</p><h2>Printers</h2></div><div className="heading-actions"><span className="count">{printers.length} shown</span><button className="button" onClick={() => setShowDiscovery((value) => !value)}>{showDiscovery ? 'Close probe' : 'Probe SNMP'}</button><button className="button" onClick={() => setShowCreate((value) => !value)}>{showCreate ? 'Cancel' : 'Add printer'}</button></div></div>
      {showDiscovery && <form className="create-form" onSubmit={(event) => { event.preventDefault(); if (probeAddress.trim() && probeCommunity) discovery.mutate() }}><p className="eyebrow">Single-IP discovery</p><p className="form-hint">Only the supplied IP is queried. The probe does not save a device or credential.</p><label htmlFor="probe-address">IP address</label><div className="create-form-row"><input id="probe-address" value={probeAddress} onChange={(event) => setProbeAddress(event.target.value)} placeholder="e.g. 192.168.1.20" required /></div><label htmlFor="probe-community">SNMP v2c community</label><div className="create-form-row"><input id="probe-community" type="password" value={probeCommunity} onChange={(event) => setProbeCommunity(event.target.value)} autoComplete="new-password" required /><button className="button" type="submit" disabled={discovery.isPending}>{discovery.isPending ? 'Reading…' : 'Read identity'}</button></div>{discovery.isError && <p className="inline-error">{discovery.error instanceof Error ? discovery.error.message : 'SNMP probe failed.'}</p>}{discovery.data && <><dl className="probe-result"><div><dt>Address</dt><dd>{discovery.data.address}</dd></div><div><dt>Name</dt><dd>{discovery.data.name || '—'}</dd></div><div><dt>Description</dt><dd>{discovery.data.description || '—'}</dd></div><div><dt>Object ID</dt><dd className="mono">{discovery.data.sys_object_id || '—'}</dd></div><div><dt>Serial</dt><dd>{discovery.data.serial || '—'}</dd></div></dl><button className="button register-button" type="button" onClick={registerDiscovered}>Register this printer</button></>}</form>}
      <div className="profile-summary" aria-live="polite"><div><p className="eyebrow">Profile library</p><strong>{profiles.isLoading ? 'Loading profiles…' : `${profiles.data?.total ?? 0} profiles loaded`}</strong></div>{profiles.isError ? <span className="summary-warning">Unavailable</span> : <span className="summary-meta">{profiles.data?.data.filter((profile) => profile.verification_status === 'verified').length ?? 0} verified</span>}</div>
      {create.isSuccess && <p className="job-notice job-running" aria-live="polite">Printer registered. {registrationJobID ? registrationJob.data?.status === 'success' ? 'Initial status poll completed.' : registrationJob.data?.status === 'failed' ? 'Initial status poll failed; inspect the printer details.' : 'Initial status poll is running…' : 'Initial polling is not configured.'}</p>}
      {showCreate && <form className="create-form" onSubmit={(event) => { event.preventDefault(); if (displayName.trim() && address.trim() && community) create.mutate() }}><label htmlFor="display-name">Display name</label><div className="create-form-row"><input id="display-name" value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="e.g. Front office printer" maxLength={200} required /></div><label htmlFor="printer-address">IP address or hostname</label><div className="create-form-row"><input id="printer-address" value={address} onChange={(event) => setAddress(event.target.value)} placeholder="e.g. 192.168.1.20" maxLength={253} required /></div><label htmlFor="snmp-community">SNMP v2c community</label><div className="create-form-row"><input id="snmp-community" type="password" value={community} onChange={(event) => setCommunity(event.target.value)} placeholder="Read-only community" autoComplete="new-password" required /><button className="button" type="submit" disabled={create.isPending}>{create.isPending ? 'Creating…' : 'Create and connect'}</button></div><p className="form-hint">The community string is encrypted before it is stored and is never shown again.</p>{create.isError && <p className="inline-error">{create.error instanceof Error ? create.error.message : 'Unable to create printer connection.'}</p>}</form>}
      {printers.length === 0 ? <div className="state empty-state"><div className="empty-icon" aria-hidden="true">⌁</div><h3>No printers registered</h3><p>Import a discovery result or add a printer through the management API.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Registered printers</caption><thead><tr><th scope="col">Printer</th><th scope="col">Manufacturer</th><th scope="col">Model</th><th scope="col">Status</th><th scope="col">Last seen</th></tr></thead><tbody>{printers.map((printer) => <PrinterRow key={printer.id} printer={printer} onSelect={setSelectedID} />)}</tbody></table></div>}
    </section>
  )
}
