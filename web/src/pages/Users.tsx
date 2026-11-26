import { useState } from 'react'
import { KeyRound, Link2, Pencil, Plus, QrCode, Trash2 } from 'lucide-react'
import { api, date } from '../api'
import ConfirmDialog from '../ConfirmDialog'
import type { PageProps, User } from '../types'
import { AsyncForm, Badge, Button, Editor, Empty, Field } from '../ui'
import SubscriptionDialog from './SubscriptionDialog'

type EditUser = Pick<User, 'name' | 'enabled' | 'endpointIds'> & { id?: string }
const confirmations = {
  credentials: { title: '重置用户凭据', message: '旧订阅链接和节点凭据立即失效，需重新导入。Shadowsocks 共享入口密码保持不变。', label: '确认重置' },
  subscription: { title: '重置订阅链接', message: '旧订阅地址立即失效。已导入的节点凭据仍可使用，请提供新订阅地址。', label: '确认重置' },
  delete: { title: '删除用户', message: '旧订阅链接和个人节点凭据立即失效。Shadowsocks 共享入口需要另外轮换密钥。', label: '确认删除' },
}
type UserAction = keyof typeof confirmations

export default function Users({ snapshot, refresh, notify }: PageProps) {
  const [confirming,setConfirming]=useState<{user:User;action:UserAction}>();const [editing, setEditing] = useState<EditUser>(); const [sharing, setSharing] = useState<User>(); const [search, setSearch] = useState('')
  async function confirmAction(user: User, action: UserAction) {
    const path = `/users/${user.id}`
    if (action === 'delete') await api(path, 'DELETE')
    else await api(`${path}/rotate`, 'POST', { credentials: action === 'credentials' })
    setSharing(undefined)
    await refresh()
    notify(action === 'delete' ? '用户已删除' : '重置成功')
  }
  const users = snapshot.state.users.filter(u => u.name.toLowerCase().includes(search.toLowerCase()))
  return <>{confirming&&<ConfirmDialog title={confirmations[confirming.action].title} message={`用户「${confirming.user.name}」。${confirmations[confirming.action].message}`} confirmLabel={confirmations[confirming.action].label} onClose={() => setConfirming(undefined)} onConfirm={() => confirmAction(confirming.user, confirming.action)}/>}<div className="toolbar"><input className="search" aria-label="搜索用户" placeholder="搜索用户…" value={search} onChange={e => setSearch(e.target.value)}/><Button className="primary" onClick={() => setEditing({ name: '', enabled: true, endpointIds: snapshot.state.endpoints.filter(e => e.enabled && !e.native).map(e => e.id) })}><Plus size={17}/>添加用户</Button></div>
    {editing && <Editor title={editing.id ? '编辑用户' : '添加用户'} onClose={() => setEditing(undefined)}><AsyncForm submit={async () => { const { name, enabled, endpointIds } = editing; await api(`/users${editing.id ? `/${editing.id}` : ''}`, editing.id ? 'PUT' : 'POST', { name, enabled, endpointIds }); await refresh(); notify('用户与所选协议已自动生效'); setEditing(undefined) }}><Field label="用户名称"><input required maxLength={100} value={editing.name} onChange={e => setEditing({ ...editing, name: e.target.value })} placeholder="例如：我的手机"/></Field><fieldset className="assignment"><legend>自由选择可用协议</legend><div className="selection-tools"><button type="button" className="text-button" onClick={()=>setEditing({...editing,endpointIds:snapshot.state.endpoints.filter(e=>!e.native).map(e=>e.id)})}>全选</button><button type="button" className="text-button" onClick={()=>setEditing({...editing,endpointIds:[]})}>清空</button></div>{snapshot.state.endpoints.some(e=>!e.native) ? snapshot.state.endpoints.filter(e=>!e.native).map(endpoint => <label key={endpoint.id}><input type="checkbox" checked={editing.endpointIds.includes(endpoint.id)} onChange={e => setEditing({ ...editing, endpointIds: e.target.checked ? [...editing.endpointIds, endpoint.id] : editing.endpointIds.filter(id => id !== endpoint.id) })}/><span>{endpoint.name}<small>{endpoint.protocol.toUpperCase()} · {endpoint.port}</small></span></label>) : <p>先创建一个协议入口，再分配给用户。</p>}</fieldset><div className="checks"><label><input type="checkbox" checked={editing.enabled} onChange={e => setEditing({ ...editing, enabled: e.target.checked })}/>启用用户</label></div></AsyncForm></Editor>}
    {sharing && <SubscriptionDialog key={sharing.id} user={sharing} notify={notify} onClose={() => setSharing(undefined)}/>}
    <section className="surface table-surface">{users.length ? <div className="table-scroll"><table><thead><tr><th>用户</th><th>可用入口</th><th>创建日期</th><th>状态</th><th>操作</th></tr></thead><tbody>{users.map(u => <tr key={u.id}><td><strong>{u.name}</strong></td><td>{u.endpointIds.length} 个入口</td><td>{date(u.createdAt)}</td><td><Badge active={u.enabled}>{u.enabled ? '可用' : '停用'}</Badge></td><td><div className="row-actions" role="group" aria-label={`${u.name} 的操作`}>
      <button type="button" className="icon-button" aria-label="订阅" title="查看订阅链接和二维码" onClick={() => setSharing(u)}><QrCode size={16}/></button>
      <button type="button" className="icon-button" aria-label={`编辑 ${u.name}`} title="编辑用户资料和协议分配" onClick={() => setEditing({ id: u.id, name: u.name, enabled: u.enabled, endpointIds: u.endpointIds })}><Pencil size={16}/></button>
      <button type="button" className="icon-button" aria-label="重置订阅" title="重置订阅链接，保留节点凭据" onClick={() => setConfirming({ user: u, action: 'subscription' })}><Link2 size={16}/></button>
      <button type="button" className="icon-button" aria-label={`重置 ${u.name} 凭据`} title="重置订阅链接和用户节点凭据" onClick={() => setConfirming({ user: u, action: 'credentials' })}><KeyRound size={16}/></button>
      <button type="button" className="icon-button danger" aria-label={`删除用户 ${u.name}`} title="删除用户并撤销订阅" onClick={() => setConfirming({ user: u, action: 'delete' })}><Trash2 size={16}/></button>
    </div></td></tr>)}</tbody></table></div> : <Empty title={search ? '没有找到用户' : '把连接分配给需要的人'} description={search ? '尝试其他关键词。' : '每位用户拥有独立订阅链接，可选择能访问的协议入口。'}/> }</section></>
}
