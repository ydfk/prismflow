import { ArrowDownLeft, ArrowRight, ArrowUpRight, Layers3, ShieldCheck, Users } from 'lucide-react'
import { bytes } from '../api'
import type { Page, PageProps, Snapshot } from '../types'
import { Badge, Button } from '../ui'
import { ProtocolUsage, TrafficTrend, useTrafficOverview } from './TrafficOverview'

export default function Overview({ snapshot, navigate }: PageProps & { navigate: (page: Page) => void }) {
  const traffic = useTrafficOverview(snapshot)
  return <>
    <ResourceSummary snapshot={snapshot} navigate={navigate}/>
    <div className="overview-grid">
      <TrafficTrend traffic={traffic}/>
      <aside className="surface connection-settings">
        <div className="section-heading"><h2>连接设置</h2></div>
        <dl className="definition-list">
          <div><dt>统一域名</dt><dd>{snapshot.state.defaultHost || '未设置'}</dd></div>
          <div><dt>内核</dt><dd>{snapshot.core.running ? '运行中' : '未启动'}</dd></div>
          <div><dt>证书</dt><dd>{snapshot.state.certificates.length} 张</dd></div>
        </dl>
        <div className="button-wrap">
          <Button onClick={() => navigate('endpoints')}>管理协议<ArrowRight size={14}/></Button>
          <Button onClick={() => navigate('certificates')}>设置域名</Button>
        </div>
      </aside>
    </div>
    <ProtocolUsage traffic={traffic}/>
    <section className="surface">
      <div className="section-heading">
        <h2>协议入口</h2>
        <button className="text-button" onClick={() => navigate('endpoints')}>查看全部<ArrowRight size={15}/></button>
      </div>
      {snapshot.state.endpoints.length ? <div className="mini-list">{snapshot.state.endpoints.slice(0, 5).map(endpoint => <div key={endpoint.id}>
        <span className="protocol-monogram">{endpoint.protocol.slice(0, 2).toUpperCase()}</span>
        <div><strong>{endpoint.name}</strong><small>{endpoint.protocol.toUpperCase()} · {endpoint.port}</small></div>
        <Badge active={endpoint.enabled}>{endpoint.enabled ? '已启用' : '已停用'}</Badge>
      </div>)}</div> : <div className="inline-empty">
        <Layers3 size={27}/><p>还没有协议入口</p>
        <button className="text-button" onClick={() => navigate('endpoints')}>添加第一个入口<ArrowRight size={15}/></button>
      </div>}
    </section>
  </>
}

function ResourceSummary({ snapshot, navigate }: { snapshot: Snapshot; navigate: (page: Page) => void }) {
  const { state, traffic } = snapshot
  const items = [
    { label: '协议入口', value: state.endpoints.length, detail: `${state.endpoints.filter(endpoint => endpoint.enabled).length} 个启用`, second: '组合自由搭配', icon: Layers3, page: 'endpoints' },
    { label: '订阅用户', value: state.users.length, detail: `${state.users.filter(user => user.enabled).length} 位可用`, second: '独立订阅链接', icon: Users, page: 'users' },
    { label: '域名证书', value: state.certificates.length, detail: `${state.certificates.filter(cert => cert.method === 'path').length} 个系统来源`, second: '自动检测续期', icon: ShieldCheck, page: 'certificates' },
    { label: '服务器周期流量', value: bytes(traffic.used), detail: traffic.available ? traffic.interface : '网卡监控不可用', second: traffic.total ? `额度 ${bytes(traffic.total)}` : '未设额度', icon: ArrowDownLeft, page: 'settings' },
  ] as const
  return <section className="summary-grid" aria-label="资源概况">{items.map(item => <article className="summary-card surface" key={item.label}>
    <button className="corner-action icon-button" aria-label={`查看${item.label}`} onClick={() => navigate(item.page)}><ArrowUpRight size={18}/></button>
    <div className="summary-label"><span className="round-icon"><item.icon size={19}/></span>{item.label}</div>
    <strong className="summary-value">{item.value}</strong>
    <div className="summary-details"><span>{item.detail}</span><span>{item.second}</span></div>
  </article>)}</section>
}
