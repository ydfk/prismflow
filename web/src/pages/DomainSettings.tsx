import { useState } from 'react'
import { api } from '../api'
import type { PageProps } from '../types'
import { AsyncForm, Field } from '../ui'
export default function DomainSettings({snapshot,refresh,notify}:PageProps){
 const [host,setHost]=useState(snapshot.state.defaultHost||'');const [certificate,setCertificate]=useState(snapshot.state.defaultCertificateId||'');const [sync,setSync]=useState(true)
 return <section className="surface domain-settings"><div className="section-heading"><h2>统一域名</h2></div><AsyncForm label="保存域名" submit={async()=>{await api('/settings','PUT',{publicUrl:snapshot.state.publicUrl,retentionDays:snapshot.state.retentionDays,defaultHost:host,defaultCertificateId:certificate,applyHost:sync});await refresh();notify('统一域名已更新')}}><div className="form-grid"><Field label="默认服务器域名 / 地址"><input value={host} onChange={e=>setHost(e.target.value)} placeholder="node.example.com"/></Field><Field label="默认证书"><select value={certificate} onChange={e=>setCertificate(e.target.value)}><option value="">不指定</option>{snapshot.state.certificates.map(c=><option key={c.id} value={c.id}>{c.domain}</option>)}</select></Field></div><div className="checks"><label><input type="checkbox" checked={sync} onChange={e=>setSync(e.target.checked)}/>同步现有入口域名与证书</label></div></AsyncForm></section>
}
