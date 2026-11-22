import { useState } from 'react'
import { api, bytes } from '../api'
import type { PageProps } from '../types'
import { AsyncForm, Field, Notice } from '../ui'

const dateTime = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'short' })
const oldest = (time: number) => time ? `最早 ${dateTime.format(time * 1000)}` : '暂无记录'

export default function HistoryStorageSettings({ snapshot, refresh, notify }: PageProps) {
  const [days, setDays] = useState(String(snapshot.state.retentionDays ?? 30))
  const [limit, setLimit] = useState(String(snapshot.state.historyLimitMiB ?? 256))
  const storage = snapshot.metrics.historyStorage
  return <section className="surface">
    <div className="section-heading"><h2>历史记录</h2></div>
    {snapshot.metrics.historyMaintenanceFailed && <Notice error>流量历史容量检查或清理失败，系统会自动重试。</Notice>}
    {storage?.checkedAt ? <dl className="definition-list">
      <div><dt>总量与协议汇总</dt><dd>{bytes(storage.summaryBytes)} · {oldest(storage.oldestSummaryAt)}</dd></div>
      <div><dt>IP 等明细</dt><dd>{bytes(storage.detailBytes)} · {oldest(storage.oldestDetailAt)}</dd></div>
      <div><dt>数据库与 WAL</dt><dd>{bytes(storage.databaseBytes)}{storage.reusableBytes > 0 && ` · 可复用 ${bytes(storage.reusableBytes)}`}</dd></div>
    </dl> : null}
    <AsyncForm label="保存历史设置" submit={async () => {
      await api('/settings', 'PUT', { retentionDays: Number(days), historyLimitMiB: Number(limit) })
      await refresh()
      notify('历史记录设置已保存')
    }}>
      <Field label="流量历史保留天数">
        <input type="number" required min={0} max={365} value={days} onChange={event => setDays(event.target.value)}/>
      </Field>
      <Field label="流量历史容量（MiB）" help="容量用于流量数据及索引。系统定期检查并清理最旧记录，汇总占四分之一，IP 等明细占四分之三。">
        <input type="number" required min={32} max={4096} value={limit} onChange={event => setLimit(event.target.value)}/>
      </Field>
    </AsyncForm>
  </section>
}
