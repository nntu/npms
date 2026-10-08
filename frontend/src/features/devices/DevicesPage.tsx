import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { Printer, PrinterStatus } from '../../api/types'
import { usePrinters } from './usePrinters'
import { usePrinter } from './usePrinter'
import { usePrinterCounters } from './usePrinterCounters'
import { usePollingRuns } from './usePollingRuns'
import { createPrinter, startPrinterPoll } from '../../api/client'
import { usePollJob } from './usePollJob'

const statusLabels: Record<PrinterStatus, string> = {
  online: 'Online',
  offline: 'Offline',
  unknown: 'Unknown',
  unavailable: 'Unavailable',
}

function StatusBadge({ status }: { status: PrinterStatus }) {
  return <span className={`status status-${status}`} aria-label={`Status: ${statusLabels[status]}`}>{statusLabels[status]}</span>
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
    {counters.isLoading ? <div className="state compact-state"><div className="spinner" aria-hidden="true" /><p>Loading counter readings…</p></div> : counters.isError ? <div className="state compact-state error-state"><p>Counter readings are unavailable.</p></div> : (counters.data?.data.length ?? 0) === 0 ? <div className="state compact-state"><p>No counter readings recorded yet.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Counter readings</caption><thead><tr><th scope="col">Counter</th><th scope="col">Raw value</th><th scope="col">Unit</th><th scope="col">Quality</th><th scope="col">Collected</th></tr></thead><tbody>{counters.data?.data.map((reading) => <tr key={reading.id}><td>{reading.definition_key}</td><td>{reading.raw_value.toLocaleString()}</td><td>{reading.unit}</td><td><span className={`quality quality-${reading.quality}`}>{reading.quality}</span></td><td>{new Date(reading.collected_at).toLocaleString()}</td></tr>)}</tbody></table></div>}
    <div className="subpanel-heading"><div><p className="eyebrow">Worker diagnostics</p><h3>Polling runs</h3></div><span className="count">{pollingRuns.data?.total ?? 0} runs</span></div>
    {pollingRuns.isLoading ? <div className="state compact-state"><div className="spinner" aria-hidden="true" /><p>Loading polling runs…</p></div> : pollingRuns.isError ? <div className="state compact-state error-state"><p>Polling diagnostics are unavailable.</p></div> : (pollingRuns.data?.data.length ?? 0) === 0 ? <div className="state compact-state"><p>No polling runs recorded yet.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Polling runs</caption><thead><tr><th scope="col">Job</th><th scope="col">Result</th><th scope="col">Attempts</th><th scope="col">Started</th><th scope="col">Error</th></tr></thead><tbody>{pollingRuns.data?.data.map((run) => <tr key={run.id}><td>{run.job_kind}</td><td><span className={`quality quality-${run.result}`}>{run.result}</span></td><td>{run.attempt_count}</td><td>{new Date(run.started_at).toLocaleString()}</td><td>{run.error_code ?? '—'}</td></tr>)}</tbody></table></div>}
  </section>
}

export function DevicesPage() {
  const query = usePrinters()
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [displayName, setDisplayName] = useState('')
  const create = useMutation({ mutationFn: () => createPrinter({ display_name: displayName.trim() }), onSuccess: () => { setDisplayName(''); setShowCreate(false); void query.refetch() } })

  if (selectedID) return <PrinterDetail id={selectedID} onBack={() => setSelectedID(null)} />

  if (query.isLoading) return <section className="panel state"><div className="spinner" aria-hidden="true" /><p>Loading printer registry…</p></section>
  if (query.isError) return <section className="panel state error-state"><p className="eyebrow">Connection problem</p><h2>Printer registry is unavailable</h2><p>{query.error instanceof Error ? query.error.message : 'The API did not respond.'}</p><button className="button" onClick={() => void query.refetch()}>Try again</button></section>

  const printers = query.data?.data ?? []
  return (
    <section className="panel">
      <div className="panel-heading"><div><p className="eyebrow">Registry</p><h2>Printers</h2></div><div className="heading-actions"><span className="count">{printers.length} shown</span><button className="button" onClick={() => setShowCreate((value) => !value)}>{showCreate ? 'Cancel' : 'Add printer'}</button></div></div>
      {showCreate && <form className="create-form" onSubmit={(event) => { event.preventDefault(); if (displayName.trim()) create.mutate() }}><label htmlFor="display-name">Display name</label><div className="create-form-row"><input id="display-name" value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="e.g. Front office printer" maxLength={200} required /><button className="button" type="submit" disabled={create.isPending}>{create.isPending ? 'Creating…' : 'Create'}</button></div>{create.isError && <p className="inline-error">{create.error instanceof Error ? create.error.message : 'Unable to create printer.'}</p>}</form>}
      {printers.length === 0 ? <div className="state empty-state"><div className="empty-icon" aria-hidden="true">⌁</div><h3>No printers registered</h3><p>Import a discovery result or add a printer through the management API.</p></div> : <div className="table-wrap"><table><caption className="sr-only">Registered printers</caption><thead><tr><th scope="col">Printer</th><th scope="col">Manufacturer</th><th scope="col">Model</th><th scope="col">Status</th><th scope="col">Last seen</th></tr></thead><tbody>{printers.map((printer) => <PrinterRow key={printer.id} printer={printer} onSelect={setSelectedID} />)}</tbody></table></div>}
    </section>
  )
}
