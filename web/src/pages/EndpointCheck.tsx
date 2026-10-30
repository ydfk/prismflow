import { useEffect, useState } from 'react'
import { api } from '../api'
import type { Endpoint, Snapshot } from '../types'
import Modal from '../Modal'
import { Badge, Button, Notice } from '../ui'
type Result={state:string;checkedAt:string;delayMs:number;message:string;steps:{name:string;passed:boolean;message:string}[]}
export default function EndpointCheck({endpoint,snapshot,onClose}:{endpoint:Endpoint;snapshot:Snapshot;onClose:()=>void}){
 const [result,setResult]=useState<Result>();const [busy,setBusy]=useState(false);const [error,setError]=useState('')
 useEffect(()=>setResult(undefined),[snapshot.appliedRevision,snapshot.core.running])
 async function check(){setBusy(true);setError('');setResult(undefined);try{setResult(await api<Result>(`/endpoints/${endpoint.id}/check`,'POST'))}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
 return <Modal title={`检测 ${endpoint.name}`} onClose={onClose}><p className="endpoint-check-summary">使用实际订阅参数运行临时客户端；不会重启在线服务。检测成功说明服务器侧链路可用，外部客户端仍需验证公网可达性。</p>{result&&<><Badge active={result.state==='available'}>{result.state==='available'?'可用':result.state==='unavailable'?'未通过':'尚不能判断'}</Badge><p>{result.message}</p><div className="check-steps">{result.steps.map((s,i)=><div key={i}><strong>{s.passed?'✓':'×'} {s.name}</strong><small>{s.message}</small></div>)}</div><small>{new Date(result.checkedAt).toLocaleString()}{result.state==='available'?` · ${result.delayMs} ms`:''}</small></>}{error&&<Notice error>{error}</Notice>}<div className="form-actions"><Button className="primary" busy={busy} onClick={check}>{result?'重新检测':'开始检测'}</Button></div></Modal>
}
