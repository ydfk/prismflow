import { useEffect, useRef, useState } from 'react'
import { bytes } from '../api'

type Direction = 'upload' | 'download'
type ChartPoint = Record<Direction, number> & { time: number; end?: number }
type ChartProps = {
  points: ChartPoint[]; step?: number; label: string; emptyLabel: string
  timeWindow?: { from: number; until: number; starts?: number[] }
  formatTick?: (seconds: number) => string
  formatDayTick?: (seconds: number) => string
  selected?: number; onSelect?: (seconds: number) => void
}
const plotWidth = 760
const plotHeight = 190
const grid = [33, 95.5, 158]
const dayWidth = 40
const defaultTick = new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
const fallbackFormat = (seconds: number) => defaultTick.format(seconds * 1000)
const scale = (position: number, length: number) => `${position / length * 100}%`

export function TrafficChart({
  points, step, label, emptyLabel, timeWindow, formatTick = fallbackFormat, formatDayTick, selected, onSelect,
}: ChartProps) {
  const scroller = useRef<HTMLDivElement>(null)
  const lastCalendar = useRef('')
  const [scrollable, setScrollable] = useState(false)
  const dates = formatDayTick ? timeWindow?.starts : undefined
  const calendarPositions = new Map(dates?.map((time, index) => [time, index]))
  const first = timeWindow?.from ?? points[0]?.time ?? 0
  const last = timeWindow?.starts?.at(-1) ?? timeWindow?.until ?? points.at(-1)?.time ?? first
  const max = Math.max(0, ...points.map(point => Math.max(point.upload, point.download)))
  const x = (time: number) => {
    const dateIndex = calendarPositions.get(time)
    if (dateIndex !== undefined && dates) return (dateIndex + 0.5) / dates.length * plotWidth
    return first === last ? plotWidth / 2 : 20 + (time - first) / (last - first) * 720
  }
  const y = (value: number) => 158 - value / (max || 1) * 125
  const selectedIndex = points.findIndex(point => point.time === selected)
  const selectedX = selected === undefined ? undefined : x(selected)
  const latestX = points.length ? x(points[points.length - 1].time) : undefined
  const dateCount = dates?.length ?? 0

  useEffect(() => {
    const node = scroller.current
    if (!node || !dateCount) {
      lastCalendar.current = ''
      return
    }
    function reveal(position: number | undefined) {
      if (!node || position === undefined || node.scrollWidth <= node.clientWidth) return
      const offset = position / plotWidth * node.scrollWidth
      if (offset < node.scrollLeft + dayWidth / 2 || offset > node.scrollLeft + node.clientWidth - dayWidth / 2) {
        node.scrollLeft = offset - node.clientWidth / 2
      }
    }
    const calendar = `${first}:${last}`
    const isNewRange = lastCalendar.current !== calendar
    lastCalendar.current = calendar
    reveal(selectedX ?? (isNewRange ? latestX : undefined))
    let width = node.clientWidth
    setScrollable(node.scrollWidth > width)
    const observer = new ResizeObserver(() => {
      if (node.clientWidth === width) return
      width = node.clientWidth
      reveal(selectedX ?? latestX)
      setScrollable(node.scrollWidth > width)
    })
    observer.observe(node)
    return () => observer.disconnect()
  }, [dateCount, first, last, latestX, selectedX])

  if (!points.length && !dateCount) return <div className="traffic-chart traffic-empty"><div className="chart-placeholder">{emptyLabel}</div></div>

  function path(direction: Direction) {
    return points.map((point, index) => {
      const previous = points[index - 1]
      const joined = previous && (previous.end ?? previous.time + (step ?? 0)) === point.time
      const start = `${x(point.time)},${y(point[direction])}`
      return joined ? `L${start}` : `M${start}h0.1`
    }).join(' ')
  }
  function selectAt(position: number) {
    if (dates?.length) {
      const date = dates[Math.min(dates.length - 1, Math.max(0, Math.floor(position / plotWidth * dates.length)))]
      if (points.some(point => point.time === date)) onSelect?.(date)
      return
    }
    let nearest = points[0]
    for (const point of points) {
      if (Math.abs(x(point.time) - position) < Math.abs(x(nearest.time) - position)) nearest = point
    }
    onSelect?.(nearest.time)
  }
  const plottedDates = new Set(points.map(point => point.time))
  const marks = dates ? points : points.filter((_, index) => index === selectedIndex || (points.length === 1 && onSelect))
  const values = [max ? bytes(max) : '', max > 1 ? bytes(Math.floor(max / 2)) : '', points.length ? '0 B' : '']

  return <div className="traffic-chart">
    <div className="traffic-plot" data-empty={!points.length}>
      <div className="traffic-axis" aria-hidden="true">
        {grid.map((line, index) => <span key={line} style={{ top: scale(line, plotHeight) }}>{values[index]}</span>)}
      </div>
      <div className={dateCount ? 'traffic-date-scroll' : undefined} ref={scroller}>
        <div style={dateCount ? { minWidth: dateCount * dayWidth } : undefined}>
          <div className="traffic-track">
            {points.length ? <svg viewBox={`0 0 ${plotWidth} ${plotHeight}`} role="img"
              aria-label={`每 ${label}上传与下载趋势，峰值 ${bytes(max)}`} preserveAspectRatio="none">
              {grid.map(line => <line key={line} x1="0" x2={plotWidth} y1={line} y2={line} className="grid-line"/>)}
              {selectedIndex >= 0 && <line x1={selectedX} x2={selectedX} y1={18} y2={165} className="traffic-selected-line"/>}
              <path d={path('download')} className="download-line"/><path d={path('upload')} className="upload-line"/>
            </svg> : null}
            {marks.map(point => <div key={point.time} aria-hidden="true" className="traffic-mark" style={{ left: scale(x(point.time), plotWidth) }}>
              <span className="download" data-selected={point.time === selected} style={{ top: scale(y(point.download), plotHeight) }}/>
              <span className="upload" data-selected={point.time === selected} style={{ top: scale(y(point.upload), plotHeight) }}/>
            </div>)}
            {!!points.length && onSelect && <button type="button" className="traffic-hit-area" aria-label="选择图表时段" aria-keyshortcuts="ArrowLeft ArrowRight Home End"
              onClick={event => {
                if (event.detail === 0) {
                  onSelect(points[selectedIndex >= 0 ? selectedIndex : points.length - 1].time)
                  return
                }
                const box = event.currentTarget.getBoundingClientRect()
                selectAt((event.clientX - box.left) / box.width * plotWidth)
              }}
              onKeyDown={event => {
                const lastIndex = points.length - 1
                const positions: Record<string, number | undefined> = {
                  ArrowLeft: selectedIndex < 0 ? lastIndex : Math.max(0, selectedIndex - 1),
                  ArrowRight: Math.min(lastIndex, selectedIndex + 1),
                  Home: 0,
                  End: lastIndex,
                }
                const next = positions[event.key]
                if (next === undefined) return
                event.preventDefault()
                onSelect(points[next].time)
              }}/>}
          </div>
          {dates && formatDayTick && <div className="traffic-date-ticks" role="group" aria-label="逐日日期轴"
            style={{ gridTemplateColumns: `repeat(${dateCount}, minmax(0, 1fr))` }}>
            {dates.map(date => plottedDates.has(date) && onSelect ?
              <button key={date} type="button" tabIndex={-1} aria-pressed={date === selected}
                aria-label={`查看 ${formatTick(date)} 流量明细`} onClick={() => onSelect(date)}>{formatDayTick(date)}</button> :
              <span key={date} title={`${formatTick(date)} · 无记录`}>{formatDayTick(date)}</span>)}
          </div>}
        </div>
      </div>
    </div>
    {!!dateCount && scrollable && <div className="traffic-scroll-tip">横向滚动查看全部日期</div>}
    {!points.length && <div className="traffic-date-empty" role="status">{emptyLabel}</div>}
    {!dateCount && <div className="chart-ticks"><span>{formatTick(first)}</span>{first !== last && <span>{formatTick(last)}</span>}</div>}
  </div>
}
