import {useEffect,useState} from 'react'
import {Check,Shuffle,SlidersHorizontal} from 'lucide-react'
import {api} from '../api'
import type {PageProps} from '../types'
import {AsyncForm,Button,Field,Notice} from '../ui'
import Modal from '../Modal'
import TLSMode from './TLSMode'
import RealityTarget from './RealityTarget'
import UserAssignment from './UserAssignment'
import {certificateName,newDraft,quickChoices,type EndpointDraft} from './endpointDraft'
export default function QuickCreate({snapshot,refresh,notify,onClose,onCertificates,onCustomize}:PageProps&{onClose:()=>void;onCertificates:()=>void;onCustomize:(draft:EndpointDraft)=>void}){
 const [selected,setSelected]=useState<string[]>(['reality'])
 const [draft,setDraft]=useState(()=>newDraft(snapshot));const [ports,setPorts]=useState<Record<string,number>>({})
 const [portBusy,setPortBusy]=useState(false);const [error,setError]=useState('')
 const external=selected.includes('xhttp')&&draft.tlsMode==='external'
 const needsCertificate=quickChoices.some(p=>selected.includes(p.id)&&p.security==='tls'&&(p.id!=='xhttp'||!external))
 const ready=selected.length>0&&!portBusy&&selected.every(id=>ports[id])
 async function randomize(){setPortBusy(true);setError('');try{const values=await api<number[]>('/ports/random','POST',{count:4});setPorts(Object.fromEntries(quickChoices.map((p,i)=>[p.id,values[i]])))}catch(e){setError((e as Error).message)}finally{setPortBusy(false)}}
 useEffect(()=>{randomize()},[])
 function toggle(id:string){setSelected(current=>current.includes(id)?current.filter(item=>item!==id):quickChoices.filter(p=>current.includes(p.id)||p.id===id).map(p=>p.id))}
 function customize(){
  if(selected.length!==1)return
  const choice=quickChoices.find(p=>p.id===selected[0])!
  onCustomize({...draft,protocol:choice.protocol,transport:choice.transport,security:choice.security,port:ports[choice.id],name:`${choice.name} · ${ports[choice.id]}`,serverName:choice.security==='reality'?draft.realityTarget.replace(/:\d+$/,''):certificateName(snapshot,draft.certificateId,draft.host)})
 }
 return <Modal title="快速创建" onClose={onClose} wide><div className="creation-mode"><span>已选 {selected.length} 个协议</span><Button disabled={!ready||selected.length!==1} title={selected.length!==1?'仅选择一个协议时可转为普通创建':'保留当前参数，转为普通创建'} onClick={customize}><SlidersHorizontal size={14}/>转为普通创建</Button></div><AsyncForm label={selected.length>1?`创建 ${selected.length} 个入口`:draft.userIds.length?'创建并启用':'创建入口'} submit={async()=>{
 if(!selected.length)throw new Error('请至少选择一个协议')
 if(!ready)throw new Error('请先生成端口')
 await api('/endpoints/preset','POST',{variants:selected,host:draft.host,ports,tlsMode:external?'external':'core',publicPort:external?(draft.publicPort||443):0,certificateId:needsCertificate?draft.certificateId:'',realityTarget:draft.realityTarget,userIds:draft.userIds})
 await refresh();notify(`已创建 ${selected.length} 个入口${draft.userIds.length?'并启用':'，分配用户后启用'}`);onClose()
 }}><div className="protocol-choices">{quickChoices.map(p=><label key={p.id} className={selected.includes(p.id)?'selected':''}><input type="checkbox" checked={selected.includes(p.id)} onChange={()=>toggle(p.id)}/><strong>{p.name}</strong><span className="mono">{ports[p.id]||'…'}</span>{selected.includes(p.id)&&<Check size={14}/>}</label>)}</div><div className="target-tools"><Button type="button" onClick={()=>setSelected(quickChoices.map(p=>p.id))}>全选协议</Button><Button type="button" onClick={()=>setSelected([])}>清空协议</Button><Button type="button" busy={portBusy} onClick={randomize}><Shuffle size={14}/>重新随机端口</Button></div>{error&&<Notice error>{error}</Notice>}<div className="form-grid"><Field label="节点连接地址"><input required value={draft.host} onChange={e=>setDraft({...draft,host:e.target.value})}/></Field>{needsCertificate&&<Field label="TLS 证书"><select required value={draft.certificateId} onChange={e=>setDraft({...draft,certificateId:e.target.value})}><option value="">选择证书</option>{snapshot.state.certificates.map(c=><option key={c.id} value={c.id}>{c.domain}</option>)}</select>{!snapshot.state.certificates.length&&<button type="button" className="text-button" onClick={onCertificates}>先设置域名与证书</button>}</Field>}</div>{selected.includes('xhttp')&&<TLSMode labelPrefix={selected.length>1?'XHTTP ':''} value={draft.tlsMode||'core'} onChange={tlsMode=>setDraft({...draft,tlsMode})} port={draft.publicPort||443} onPort={publicPort=>setDraft({...draft,publicPort})}/>}{selected.includes('reality')&&<RealityTarget value={draft.realityTarget} onChange={value=>setDraft({...draft,realityTarget:value})}/>}<UserAssignment users={snapshot.state.users} value={draft.userIds} onChange={userIds=>setDraft({...draft,userIds})}/>{!draft.userIds.length&&<small>未分配用户时暂不监听端口。</small>}</AsyncForm></Modal>
}
