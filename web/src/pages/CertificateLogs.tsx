import Modal from '../Modal'
import type {Job} from '../types'
export default function CertificateLogs({domain,jobs,onClose}:{domain:string;jobs:Job[];onClose:()=>void}){
 return <Modal title={`${domain} · 证书操作记录`} onClose={onClose}><div className="certificate-log-list">{jobs.length?jobs.slice().sort((a,b)=>b.createdAt.localeCompare(a.createdAt)).map(job=><article key={job.id}><strong>{job.operation||'证书申请'} · {job.status==='running'?'进行中':job.status==='done'?'成功':'失败'}</strong><small>{new Date(job.createdAt).toLocaleString('zh-CN')}</small><p>{job.message}</p></article>):<p>暂无操作记录；旧版本产生的记录无法补回。</p>}</div></Modal>
}
