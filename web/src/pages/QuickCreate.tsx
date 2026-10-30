import { useEffect, useState } from 'react'
import { Check, Shuffle, SlidersHorizontal } from 'lucide-react'
import { api } from '../api'
import type { PageProps } from '../types'
import { AsyncForm, Button, Field, Notice } from '../ui'
import Modal from '../Modal'
import RealityTarget from './RealityTarget'
import UserAssignment from './UserAssignment'
import { newDraft, quickChoices, type EndpointDraft } from './endpointDraft'
export default function QuickCreate({snapshot,refresh,notify,onClose,onCertificates,onCustomize}:PageProps&{onClose:()=>void;onCertificates:()=>void;onCustomize:(draft:EndpointDraft)=>void}){
 const [selected,setSelected]=useState('reality')
 const [draft,setDraft]=useState(()=>newDraft(snapshot))
 const [ports,setPorts]=useState<Record<string,number>>({})
 const [portBusy,setPortBusy]=useState(false);const [error,setError]=useState('')
 const choice=quickChoices.find(p=>p.id===selected)!
 async function randomize(){setPortBusy(true);setError('');try{const values=await api<number[]>('/ports/random','POST',{count:4});setPorts(Object.fromEntries(quickChoices.map((p,i)=>[p.id,values[i]])))}catch(e){setError((e as Error).message)}finally{setPortBusy(false)}}
 useEffect(()=>{randomize()},[])
 function customized():EndpointDraft {return {...draft,protocol:choice.protocol,transport:choice.transport,security:choice.security,port:ports[selected],name:`${choice.name} · ${ports[selected]}`,serverName:choice.security==='reality'?draft.realityTarget.replace(/:\d+$/,''):snapshot.state.certificates.find(c=>c.id===draft.certificateId)?.domain||draft.host}}
 return <Modal title="快速创建" onClose={onClose} wide><div className="creation-mode"><span>选择一个预设</span><Button disabled={portBusy||!ports[selected]} onClick={()=>onCustomize(customized())}><SlidersHorizontal size={14}/>转为普通创建</Button></div><AsyncForm label={draft.userIds.length?'创建并启用':'创建入口'} submit={async()=>{if(portBusy||!ports[selected])throw new Error('请先生成端口');await api('/endpoints/preset','POST',{variants:[selected],host:draft.host,ports,certificateId:draft.certificateId,realityTarget:draft.realityTarget,userIds:draft.userIds});await refresh();notify(draft.userIds.length?'已创建并启用':'已创建，分配用户后启用');onClose()}}><div className="protocol-choices">{quickChoices.map(p=><label key={p.id} className={selected===p.id?'selected':''}><input type="radio" name="quick-protocol" checked={selected===p.id} onChange={()=>setSelected(p.id)}/><strong>{p.name}</strong><span className="mono">{ports[p.id]||'…'}</span>{selected===p.id&&<Check size={14}/>}</label>)}</div><div className="target-tools"><Button type="button" busy={portBusy} onClick={randomize}><Shuffle size={14}/>重新随机端口</Button></div>{error&&<Notice error>{error}</Notice>}<div className="form-grid"><Field label="服务器域名 / 地址"><input required value={draft.host} onChange={e=>setDraft({...draft,host:e.target.value})}/></Field>{choice.security==='tls'&&<Field label="TLS 证书"><select required value={draft.certificateId} onChange={e=>setDraft({...draft,certificateId:e.target.value})}><option value="">选择证书</option>{snapshot.state.certificates.map(c=><option key={c.id} value={c.id}>{c.domain}</option>)}</select>{!snapshot.state.certificates.length&&<button type="button" className="text-button" onClick={onCertificates}>先设置域名与证书</button>}</Field>}</div>{selected==='reality'&&<RealityTarget value={draft.realityTarget} onChange={value=>setDraft({...draft,realityTarget:value})}/>}<UserAssignment users={snapshot.state.users} value={draft.userIds} onChange={userIds=>setDraft({...draft,userIds})}/>{!draft.userIds.length&&<small>未分配用户时暂不监听端口。</small>}</AsyncForm></Modal>
}
