import { useEffect, useMemo, useState } from 'react'
import { Activity, ArrowDownLeft, ArrowUpRight } from 'lucide-react'
import { api, bytes } from '../api'
import type { Point, Snapshot } from '../types'
import { Notice } from '../ui'

const ranges = {
  '1h': { label: '最近 1 小时', step: 60, bucket: '1 分钟' },
  '24h': { label: '最近 24 小时', step: 300, bucket: '5 分钟' },
  '7d': { label: '最近 7 天', step: 1800, bucket: '30 分钟' },
  '30d': { label: '最近 30 天', step: 7200, bucket: '2 小时' },
} as const
type Range = keyof typeof ranges
type Totals = Pick<Point, 'upload' | 'download'>
const emptyPoints: Point[] = []
const protocolName = (name: string) => name === 'unknown' ? '未识别协议' : name.toUpperCase()
const amount = (value?: number) => value === undefined ? '—' : bytes(value)
const totalBytes = (counts?: Totals) => counts ? counts.upload + counts.download : 0
const percent = new Intl.NumberFormat('zh-CN', { style: 'percent', maximumFractionDigits: 1 })
const chartDate = new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
const chartTime = new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit' })

function useTrafficHistory(range: Range) {
  const [result, setResult] = useState<{ range: Range; points?: Point[]; error?: string }>()
  useEffect(() => {
    let active = true
    let busy = false
    async function load() {
      if (busy || document.hidden) return
      busy = true
      try {
        const points = await api<Point[]>(`/metrics?range=${range}&bucket=auto`)
        if (active) setResult({ range, points })
      } catch (error) {
        if (active) setResult(previous => ({
          range, points: previous?.range === range ? previous.points : undefined, error: (error as Error).message,
        }))
      } finally { busy = false }
    }
    void load()
    const timer = window.setInterval(load, 30000)
    document.addEventListener('visibilitychange', load)
    return () => {
      active = false
      window.clearInterval(timer)
      document.removeEventListener('visibilitychange', load)
    }
  }, [range])
  return result?.range === range ? result : undefined
}

function summarize(points: Point[]) {
  const totals = new Map<string, Totals>()
  const series = new Map<string, Point[]>()
  for (const point of points) {
    const current = totals.get(point.protocol) ?? { upload: 0, download: 0 }
    current.upload += point.upload
    current.download += point.download
    totals.set(point.protocol, current)
    const values = series.get(point.protocol)
    if (values) values.push(point)
    else series.set(point.protocol, [point])
  }
  return { totals, series }
}

export function useTrafficOverview(snapshot: Snapshot) {
  const [range, setRange] = useState<Range>('24h')
  const [protocol, setProtocol] = useState('total')
  const result = useTrafficHistory(range)
  const points = result?.points ?? emptyPoints
  const { totals, series } = useMemo(() => summarize(points), [points])
  const connections = snapshot.metrics.activeConnections
  const protocols = [...new Set([
    ...snapshot.state.endpoints.map(endpoint => endpoint.protocol),
    ...totals.keys(),
    ...Object.keys(connections ?? {}),
  ])].filter(name => name !== 'total')
  protocols.sort((a, b) => totalBytes(totals.get(b)) - totalBytes(totals.get(a)) || a.localeCompare(b))
  const selection = protocol === 'total' || protocols.includes(protocol) ? protocol : 'total'
  const currentSeries = useMemo(() => {
    if (selection === 'total') return series.get('total') ?? emptyPoints
    const selected = new Map(series.get(selection)?.map(point => [point.time, point]))
    if (!selected.size) return emptyPoints
    return (series.get('total') ?? emptyPoints).map(point =>
      selected.get(point.time) ?? { time: point.time, protocol: selection, upload: 0, download: 0 })
  }, [selection, series])
  return {
    range, setRange, selection, setProtocol, result, totals, protocols, currentSeries, connections, metrics: snapshot.metrics,
  }
}
type TrafficOverviewState = ReturnType<typeof useTrafficOverview>

export function TrafficTrend({ traffic }: { traffic: TrafficOverviewState }) {
  const { range, setRange, selection, setProtocol, result, totals, protocols, currentSeries, connections, metrics } = traffic
  const rates = metrics.rates.find(point => point.protocol === selection)
  const sampledRate = metrics.rates.length > 0
  const selected = totals.get(selection)
  const activeCount = connections ? connections[selection] ?? 0 : undefined
  const lastSample = metrics.online ? chartTime.format(new Date(metrics.sampledAt)) : undefined
  return <section className="traffic-panel surface">
    <div className="section-heading traffic-heading">
      <h2>流量动态</h2>
      <div className="traffic-controls">
        <select className="compact" aria-label="流量协议" value={selection} onChange={event => setProtocol(event.target.value)}>
          <option value="total">全部协议</option>
          {protocols.map(name => <option value={name} key={name}>{protocolName(name)}</option>)}
        </select>
        <select className="compact" aria-label="流量时间范围" value={range} onChange={event => setRange(event.target.value as Range)}>
          {Object.entries(ranges).map(([value, option]) => <option value={value} key={value}>{option.label}</option>)}
        </select>
      </div>
    </div>
    <div className="traffic-readings">
      <div><span><ArrowUpRight size={15}/>实时上传</span><strong>{sampledRate ? bytes(rates?.upload ?? 0) : '—'}{sampledRate && <small>/s</small>}</strong></div>
      <div><span><ArrowDownLeft size={15}/>实时下载</span><strong>{sampledRate ? bytes(rates?.download ?? 0) : '—'}{sampledRate && <small>/s</small>}</strong></div>
      <div><span><Activity size={15}/>活动连接</span><strong>{activeCount ?? '—'}</strong></div>
      <div><span>所选时段累计</span><strong>{selected ? bytes(totalBytes(selected)) : '—'}</strong></div>
    </div>
    {result?.error && <Notice error>{result.error}{result.points ? '，下方保留上次结果。' : ''}</Notice>}
    <TrafficChart points={currentSeries} step={ranges[range].step} label={ranges[range].bucket}
      emptyLabel={result?.error ? '暂时无法读取流量记录' : !result ? '正在读取流量…' : '所选时段没有流量记录'}/>
    <div className="chart-foot">
      <span><i className="legend up"/>上传<i className="legend down"/>下载 · 每 {ranges[range].bucket}汇总</span>
      <span>{lastSample ? `最近采样 ${lastSample}` : '等待内核连接'}</span>
    </div>
  </section>
}

export function ProtocolUsage({ traffic }: { traffic: TrafficOverviewState }) {
  const { range, selection, setProtocol, result, totals, protocols, connections } = traffic
  const sampledBytes = protocols.reduce((sum, name) => sum + totalBytes(totals.get(name)), 0)
  return <section className="surface protocol-usage-panel">
    <div className="section-heading"><h2>协议用量</h2><span className="subtle">{ranges[range].label}</span></div>
    {protocols.length ? <div className="table-scroll" role="region" aria-label="协议用量明细" tabIndex={0}>
      <table className="usage-table" aria-label="各协议流量和活动连接">
        <thead><tr><th scope="col">协议</th><th scope="col">上传</th><th scope="col">下载</th><th scope="col">合计</th><th scope="col">采样占比</th><th scope="col">活动连接</th></tr></thead>
        <tbody>{protocols.map(name => {
          const counts = totals.get(name)
          return <tr key={name}>
            <th scope="row"><button type="button" className="text-button" aria-pressed={selection === name} title={`在趋势图中查看 ${protocolName(name)}`} onClick={() => setProtocol(selection === name ? 'total' : name)}>{protocolName(name)}</button></th>
            <td>{amount(counts?.upload)}</td><td>{amount(counts?.download)}</td>
            <td>{counts ? bytes(totalBytes(counts)) : '—'}</td>
            <td>{counts && sampledBytes > 0 ? percent.format(totalBytes(counts) / sampledBytes) : '—'}</td>
            <td>{connections ? connections[name] ?? 0 : '—'}</td>
          </tr>
        })}</tbody>
      </table>
    </div> : <div className="inline-empty"><Activity size={24}/><p>{result ? '暂无协议用量' : '正在读取流量…'}</p></div>}
    <p className="usage-note">协议用量按活动连接采样估算，短连接可能漏计。占比基于表中已采样的流量；活动连接为实时快照。</p>
  </section>
}

function TrafficChart({ points, step, label, emptyLabel }: { points: Point[]; step: number; label: string; emptyLabel: string }) {
  if (!points.length) return <div className="traffic-chart traffic-empty"><div className="chart-placeholder">{emptyLabel}</div></div>
  const first = points[0].time
  const last = points[points.length - 1].time
  const max = Math.max(...points.map(point => Math.max(point.upload, point.download)))
  const x = (time: number) => first === last ? 380 : 20 + (time - first) / (last - first) * 720
  const y = (value: number) => 158 - value / (max || 1) * 125
  function path(direction: keyof Totals) {
    return points.map((point, index) => {
      const joined = index > 0 && point.time - points[index - 1].time === step
      const start = `${x(point.time)},${y(point[direction])}`
      return joined ? `L${start}` : `M${start}h0.1`
    }).join(' ')
  }
  return <div className="traffic-chart">
    <div className="traffic-plot">
      <div className="traffic-axis" aria-hidden="true"><span>{max ? bytes(max) : ''}</span><span>{max > 1 ? bytes(Math.floor(max / 2)) : ''}</span><span>0 B</span></div>
      <svg viewBox="0 0 760 190" role="img" aria-label={`每 ${label}上传与下载趋势，峰值 ${bytes(max)}`} preserveAspectRatio="none">
        {[33, 95.5, 158].map(line => <line key={line} x1="20" x2="740" y1={line} y2={line} className="grid-line"/>)}
        <path d={path('download')} className="download-line"/><path d={path('upload')} className="upload-line"/>
      </svg>
    </div>
    <div className="chart-ticks"><span>{chartDate.format(first * 1000)}</span>{first !== last && <span>{chartDate.format(last * 1000)}</span>}</div>
  </div>
}
