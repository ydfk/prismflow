import { useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { api, bytes } from '../api'
import type { Snapshot } from '../types'
import { Button, Field, Notice } from '../ui'
import { protocolName, TrafficChart } from './TrafficOverview'
import { BreakdownTable, TimelineTable, type Breakdown, type HistoryBucket } from './TrafficHistoryTables'

const granularities = { minute: '分钟', hour: '小时', day: '天' } as const
const groups = { ip: '来源 IP', protocol: '协议', inbound: '入口', user: '用户', network: '传输网络' } as const
type Granularity = keyof typeof granularities
type Group = keyof typeof groups
type Filters = Partial<Record<Group, string>>
type Range = { from: string; to: string; granularity: Granularity }
type History = { from: number; until: number; buckets: HistoryBucket[] }
const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const days = new Intl.DateTimeFormat('zh-CN', { timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit' })
const times = new Intl.DateTimeFormat('zh-CN', { timeZone: timezone, month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZoneName: 'shortOffset' })
const emptyBuckets: HistoryBucket[] = []
const presets = [{ label: '今天', days: 1 }, { label: '最近 7 天', days: 7 }, { label: '最近 30 天', days: 30 }]

function calendarDate(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function recentRange(length: number): Range {
  const now = new Date()
  const first = new Date(now.getFullYear(), now.getMonth(), now.getDate() - length + 1)
  return { from: calendarDate(first), to: calendarDate(now), granularity: length < 30 ? 'hour' : 'day' }
}

function useTrafficQuery<T>(path: string, revision: number) {
  const [result, setResult] = useState<{ path: string; revision: number; value?: T; error?: string }>()
  useEffect(() => {
    const controller = new AbortController()
    api<T>(path, 'GET', undefined, controller.signal)
      .then(value => { if (!controller.signal.aborted) setResult({ path, revision, value }) })
      .catch((error: Error) => { if (!controller.signal.aborted) setResult({ path, revision, error: error.message }) })
    return () => controller.abort()
  }, [path, revision])
  return result?.path === path && result.revision === revision ? result : undefined
}

export default function TrafficHistory({ snapshot }: { snapshot: Snapshot }) {
  const [range, setRange] = useState(() => recentRange(1))
  const [draft, setDraft] = useState(range)
  const [ip, setIP] = useState('')
  const [filters, setFilters] = useState<Filters>({})
  const [group, setGroup] = useState<Group>('ip')
  const [page, setPage] = useState(1)
  const [slot, setSlot] = useState<number>()
  const [revision, setRevision] = useState(0)
  const rankingHeading = useRef<HTMLHeadingElement>(null)
  const search = new URLSearchParams({ ...range, ...filters, timezone })
  const history = useTrafficQuery<History>(`/metrics/history?${search}`, revision)
  const rankQuery = new URLSearchParams(search)
  rankQuery.set('group', group)
  rankQuery.set('page', String(page))
  if (slot !== undefined) rankQuery.set('slot', String(slot))
  const breakdown = useTrafficQuery<Breakdown>(`/metrics/breakdown?${rankQuery}`, revision)
  const buckets = history?.value?.buckets ?? emptyBuckets
  const totals = buckets.reduce((sum, bucket) => ({ upload: sum.upload + bucket.upload, download: sum.download + bucket.download }), { upload: 0, download: 0 })
  const today = calendarDate(new Date())
  const retention = snapshot.state.retentionDays
  const formatTime = (value: number) => (range.granularity === 'day' ? days : times).format(value * 1000)
  const knownUsers = snapshot.state.users
  const knownEntries = snapshot.state.endpoints
  function name(key: Group, value: string) {
    if (key === 'inbound') return knownEntries.find(entry => entry.id === value)?.name ?? value
    if (key === 'user') return knownUsers.find(user => user.id === value)?.name ?? value
    if (key === 'protocol') return protocolName(value)
    if (key === 'network') return value.toUpperCase()
    return value
  }
  function resetPosition() {
    setSlot(undefined)
    setPage(1)
  }
  function applyRange(next: Range) {
    setRange(next)
    resetPosition()
    setRevision(current => current + 1)
  }
  function changeDates(next: Range) {
    const duration = (Date.parse(next.to) - Date.parse(next.from)) / 86400000 + 1
    const granularity = duration > 60 ? 'day' : duration > 1 && next.granularity === 'minute' ? 'hour' : next.granularity
    setDraft({ ...next, granularity })
  }
  function changeFilter(key: Group, value: string) {
    const next = { ...filters }
    if (value) next[key] = value
    else delete next[key]
    setFilters(next)
    if (key === 'ip') setIP(value)
    setPage(1)
  }
  const draftDays = (Date.parse(draft.to) - Date.parse(draft.from)) / 86400000 + 1
  const activeFilters = Object.keys(filters) as Group[]
  const storage = snapshot.metrics.historyStorage
  const firstRecorded = activeFilters.some(key => key !== 'protocol') ? storage?.oldestDetailAt : storage?.oldestSummaryAt
  const emptyLabel = history?.error ? '暂时无法读取流量记录' : history ? '所选时段没有匹配的流量记录' : '正在读取流量…'

  return <div className="history-layout">
    <section className="surface history-summary">
      <div className="section-heading history-heading">
        <h2>流量趋势</h2>
        <span className="subtle">{timezone}{retention !== undefined && ` · ${retention === 0 ? '不按天清理' : `最多保留 ${retention} 天`}`}</span>
      </div>
      <div className="history-presets" aria-label="快捷日期" role="group">
        {presets.map(preset => <Button key={preset.days} onClick={() => {
          const next = recentRange(preset.days)
          setDraft(next)
          applyRange(next)
        }}>{preset.label}</Button>)}
      </div>
      <form className="history-filters" onSubmit={event => {
        event.preventDefault()
        changeFilter('ip', ip.trim())
        applyRange(draft)
      }}>
        <Field label="开始日期"><input type="date" required value={draft.from} max={draft.to || today} onChange={event => changeDates({ ...draft, from: event.target.value })}/></Field>
        <Field label="结束日期"><input type="date" required value={draft.to} min={draft.from} max={today} onChange={event => changeDates({ ...draft, to: event.target.value })}/></Field>
        <Field label="统计粒度"><select value={draft.granularity} onChange={event => setDraft({ ...draft, granularity: event.target.value as Granularity })}>
          <option value="minute" disabled={draftDays > 1}>按分钟</option>
          <option value="hour" disabled={draftDays > 60}>按小时</option>
          <option value="day">按天</option>
        </select></Field>
        <Field label="来源 IP"><input type="text" value={ip} maxLength={64} placeholder="IPv4 或 IPv6" onChange={event => setIP(event.target.value)}/></Field>
        <Button className="primary" type="submit">查询</Button>
      </form>
      {activeFilters.length > 0 && <div className="history-applied-filters" role="group" aria-label="已应用筛选">
        {activeFilters.map(key => <button key={key} type="button" className="text-button" aria-label={`移除${groups[key]}筛选`} onClick={() => changeFilter(key, '')}>
          {groups[key]} · {name(key, filters[key]!)}<X size={14}/>
        </button>)}
      </div>}
      {snapshot.metrics.historyWriteFailed && <Notice error>最近一次历史写入失败，采集器会在下一次采样时重试。</Notice>}
      {history?.error && <Notice error>{history.error}</Notice>}
      <dl className="history-totals">
        <div><dt>上传</dt><dd>{buckets.length ? bytes(totals.upload) : '—'}</dd></div>
        <div><dt>下载</dt><dd>{buckets.length ? bytes(totals.download) : '—'}</dd></div>
        <div><dt>合计</dt><dd>{buckets.length ? bytes(totals.upload + totals.download) : '—'}</dd></div>
      </dl>
      <TrafficChart points={buckets} label={`1 ${granularities[range.granularity]}`} emptyLabel={emptyLabel} window={history?.value} formatTick={formatTime}/>
      <div className="chart-foot">
        <span><i className="legend up"/>上传<i className="legend down"/>下载 · 按{granularities[range.granularity]}汇总</span>
        {firstRecorded && history?.value && firstRecorded > history.value.from ? <span>最早记录 {formatTime(firstRecorded)}</span> : null}
      </div>
    </section>

    <section className="surface">
      <div className="section-heading history-heading">
        <div><h2 tabIndex={-1} ref={rankingHeading}>用量排行</h2>
          <p className="subtle">{slot === undefined ? `${range.from} 至 ${range.to}` : `${formatTime(slot)} · 该${granularities[range.granularity]}用量`}</p>
        </div>
        <div className="traffic-controls">
          {slot !== undefined && <Button onClick={resetPosition}>查看完整时段</Button>}
          <select className="compact" aria-label="用量分组" value={group} onChange={event => { setGroup(event.target.value as Group); setPage(1) }}>
            {(Object.keys(groups) as Group[]).map(key => <option value={key} key={key}>按{groups[key]}</option>)}
          </select>
        </div>
      </div>
      {breakdown?.error && breakdown.error !== history?.error && <Notice error>{breakdown.error}</Notice>}
      <BreakdownTable result={breakdown?.value} loading={!breakdown} label={groups[group]} page={page} setPage={setPage}
        getName={value => name(group, value)} selected={filters[group]} onSelect={value => changeFilter(group, filters[group] === value ? '' : value)}/>
      <p className="usage-note">明细来自 Mihomo 活动连接采样，短连接和结束前的增量可能漏计。来源 IP 以 Mihomo 实际看到的地址为准，用户用量仅包含已识别的订阅用户。</p>
    </section>

    <TimelineTable key={`${search}:${revision}`} buckets={buckets} emptyLabel={emptyLabel} filename={`prismflow-traffic-${range.from}-${range.to}.csv`}
      formatTime={formatTime} slot={slot} onSelect={time => {
        setSlot(slot === time ? undefined : time)
        setPage(1)
        rankingHeading.current?.focus()
      }}/>
  </div>
}
