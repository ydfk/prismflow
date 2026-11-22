import { useState } from 'react'
import { ChevronLeft, ChevronRight, Download } from 'lucide-react'
import { bytes } from '../api'
import { Button } from '../ui'

type Counts = { upload: number; download: number }
export type HistoryBucket = Counts & { time: number; end: number }
export type Breakdown = { rows: (Counts & { value: string })[]; hasMore: boolean }
const timelinePageSize = 24
const number = new Intl.NumberFormat('zh-CN')

function ByteCells({ upload, download }: Counts) {
  return <>{[upload, download, upload + download].map((value, index) =>
    <td key={index} title={`${number.format(value)} B`}>{bytes(value)}</td>,
  )}</>
}

function PageControls({ page, hasMore, setPage }: { page: number; hasMore?: boolean; setPage: (page: number) => void }) {
  if (page <= 1 && !hasMore) return null
  return <div className="history-pagination">
    <Button type="button" disabled={page <= 1} onClick={() => setPage(page - 1)}><ChevronLeft size={15}/>上一页</Button>
    <span>第 {page} 页</span>
    <Button type="button" disabled={!hasMore} onClick={() => setPage(page + 1)}>下一页<ChevronRight size={15}/></Button>
  </div>
}

type BreakdownProps = {
  result?: Breakdown; loading: boolean; label: string; page: number; setPage: (page: number) => void
  getName: (value: string) => string; selected?: string; onSelect: (value: string) => void
}

export function BreakdownTable({ result, loading, label, page, setPage, getName, selected, onSelect }: BreakdownProps) {
  return <>
    {result?.rows.length ? <div className="table-scroll" role="region" aria-label={`${label}用量排行`} tabIndex={0}>
      <table className="usage-table history-table">
        <thead><tr><th scope="col">{label}</th><th scope="col">上传</th><th scope="col">下载</th><th scope="col">合计</th></tr></thead>
        <tbody>{result.rows.map(row => <tr key={row.value}>
          <th scope="row"><button type="button" className="text-button" title={`按${label}筛选 · ${getName(row.value)}`}
            aria-pressed={selected === row.value} onClick={() => onSelect(row.value)}>{getName(row.value)}</button></th>
          <ByteCells upload={row.upload} download={row.download}/>
        </tr>)}</tbody>
      </table>
    </div> : <div className="history-empty">{loading ? '正在读取用量排行…' : result ? '所选时段没有匹配的用量明细' : '暂时无法读取用量排行'}</div>}
    <PageControls page={page} hasMore={result?.hasMore} setPage={setPage}/>
  </>
}

function exportTimeline(buckets: HistoryBucket[], filename: string) {
  const rows = buckets.map(({ time, end, upload, download }) => [
    new Date(time * 1000).toISOString(), new Date(end * 1000).toISOString(), upload, download, upload + download,
  ].join(','))
  const csv = ['区间开始（UTC）,区间结束（UTC）,上传（字节）,下载（字节）,合计（字节）', ...rows].join('\r\n')
  const url = URL.createObjectURL(new Blob(['\ufeff', csv], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

type TimelineProps = {
  buckets: HistoryBucket[]; emptyLabel: string; filename: string
  formatTime: (value: number) => string; slot?: number; onSelect: (value: number) => void
}

export function TimelineTable({ buckets, emptyLabel, filename, formatTime, slot, onSelect }: TimelineProps) {
  const [page, setPage] = useState(1)
  const last = buckets.length - (page - 1) * timelinePageSize
  const first = Math.max(0, last - timelinePageSize)
  const rows = buckets.slice(first, last).reverse()
  return <section className="surface">
    <div className="section-heading history-heading">
      <h2>时间明细</h2>
      <Button type="button" disabled={!buckets.length} onClick={() => exportTimeline(buckets, filename)}><Download size={15}/>导出 CSV</Button>
    </div>
    {rows.length ? <div className="table-scroll" role="region" aria-label="时间用量明细" tabIndex={0}>
      <table className="usage-table history-table">
        <thead><tr><th scope="col">区间开始</th><th scope="col">上传</th><th scope="col">下载</th><th scope="col">合计</th></tr></thead>
        <tbody>{rows.map(row => <tr key={row.time}>
          <th scope="row"><button type="button" className="text-button" title="查看这段时间的用量排行"
            aria-pressed={slot === row.time} onClick={() => onSelect(row.time)}>{formatTime(row.time)}</button></th>
          <ByteCells upload={row.upload} download={row.download}/>
        </tr>)}</tbody>
      </table>
    </div> : <div className="history-empty">{emptyLabel}</div>}
    <PageControls page={page} hasMore={first > 0} setPage={setPage}/>
  </section>
}
