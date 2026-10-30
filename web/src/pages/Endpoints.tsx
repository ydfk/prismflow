import { useState } from 'react'
import { Plus, Pencil, Trash2, Zap, Stethoscope } from 'lucide-react'
import { api } from '../api'
import type { Endpoint, Page, PageProps } from '../types'
import { Badge, Button, Empty, Notice } from '../ui'
import QuickCreate from './QuickCreate'
import EndpointCheck from './EndpointCheck'
import EndpointEditor from './EndpointEditor'
import { newDraft, type EndpointDraft } from './endpointDraft'
function label(e:Endpoint){if(e.native)return `${e.protocol.toUpperCase()} · 原生`;return e.protocol==='vless'?e.transport==='xhttp'?'VLESS · XHTTP · TLS':e.security==='reality'?'VLESS · REALITY · Vision':'VLESS · TLS':e.protocol.toUpperCase()}
export default function Endpoints({snapshot,refresh,notify,navigate}:PageProps&{navigate:(page:Page)=>void}){
 const [checking,setChecking]=useState<Endpoint>();const [quick,setQuick]=useState(false);const [editing,setEditing]=useState<EndpointDraft>();const [error,setError]=useState('');const [loading,setLoading]=useState(false)
 async function remove(e:Endpoint){if(!confirm(`删除 ${e.name}？`))return;try{await api(`/endpoints/${e.id}`,'DELETE');await refresh();notify('入口已删除')}catch(e){setError((e as Error).message)}}
 async function edit(e:Endpoint){setError('');setLoading(true);try{const details=await api<Endpoint>(`/endpoints/${e.id}`);setEditing({...details,userIds:snapshot.state.users.filter(u=>u.endpointIds.includes(e.id)).map(u=>u.id)})}catch(e){setError((e as Error).message)}finally{setLoading(false)}}
 const actions=<div className="creation-actions"><Button onClick={()=>setEditing(newDraft(snapshot))}><Plus size={15}/>普通创建</Button><Button className="primary" onClick={()=>setQuick(true)}><Zap size={15}/>快速创建</Button></div>
 return <>{checking&&<EndpointCheck endpoint={checking} snapshot={snapshot} onClose={()=>setChecking(undefined)}/>}{error&&<Notice error>{error}</Notice>}<div className="toolbar"><span>{snapshot.state.endpoints.length} 个入口{snapshot.state.defaultHost?` · ${snapshot.state.defaultHost}`:''}</span>{actions}</div>
 {quick&&<QuickCreate snapshot={snapshot} refresh={refresh} notify={notify} onClose={()=>setQuick(false)} onCustomize={draft=>{setQuick(false);setEditing(draft)}} onCertificates={()=>{setQuick(false);navigate('certificates')}}/>}
 {editing&&<EndpointEditor key={editing.id||'new'} initial={editing} snapshot={snapshot} refresh={refresh} notify={notify} onClose={()=>setEditing(undefined)}/>}
 <section className="surface table-surface">{snapshot.state.endpoints.length?<div className="table-scroll"><table><thead><tr><th>名称</th><th>协议</th><th>地址 / 端口</th><th>用户</th><th>状态</th><th>操作</th></tr></thead><tbody>{snapshot.state.endpoints.map(e=><tr key={e.id}><td><strong>{e.name}</strong></td><td>{label(e)}</td><td><span className="mono">{e.protocol==='tun'?'虚拟网卡':`${e.host}:${e.tlsMode==='external'?(e.publicPort||443):e.port}`}</span>{e.tlsMode==='external'&&<small>反代 TLS → 127.0.0.1:{e.port}</small>}</td><td>{e.native?'自行管理':snapshot.state.users.filter(u=>u.endpointIds.includes(e.id)).length}</td><td><Badge active={e.enabled}>{e.enabled?'启用':'停用'}</Badge></td><td><div className="row-actions"><button className="icon-button" aria-label={`检测 ${e.name}`} onClick={()=>setChecking(e)}><Stethoscope size={15}/></button><button disabled={loading} className="icon-button" aria-label={`编辑 ${e.name}`} onClick={()=>edit(e)}><Pencil size={15}/></button><button className="icon-button danger" aria-label={`删除 ${e.name}`} onClick={()=>remove(e)}><Trash2 size={15}/></button></div></td></tr>)}</tbody></table></div>:<Empty title="还没有协议" description="使用常用预设，或自行配置 Mihomo 监听器。"/>}</section></>
}
