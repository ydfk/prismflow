import { useEffect, useId, useState, type KeyboardEvent } from 'react'
import { Activity, DatabaseBackup, Globe, Server } from 'lucide-react'
import { api } from '../api'
import KernelPanel from './KernelPanel'
import UpdatePanel from './UpdatePanel'
import Website from './Website'
import TrafficSettings from './TrafficSettings'
import SettingsBackup from './SettingsBackup'
import type { PageProps } from '../types'
import { AsyncForm, Field, Notice } from '../ui'

const sections=[
 {id:'service',label:'服务管理',icon:Server},
 {id:'traffic',label:'流量与记录',icon:Activity},
 {id:'website',label:'伪装网站',icon:Globe},
 {id:'security',label:'安全与备份',icon:DatabaseBackup},
] as const
type Section=typeof sections[number]['id']

export default function Settings({snapshot,refresh,notify}:PageProps){
 const prefix=useId()
 const [section,setSection]=useState<Section>('service')
 const [retention,setRetention]=useState(snapshot.state.retentionDays)
 const [events,setEvents]=useState<{time:string;message:string}[]>([])
 const [error,setError]=useState('')
 const [current,setCurrent]=useState('')
 const [password,setPassword]=useState('')
 useEffect(()=>{
  let active=true
  api<typeof events>('/events').then(value=>{if(active)setEvents(value)}).catch(e=>{if(active)setError(e.message)})
  return()=>{active=false}
 },[snapshot.appliedRevision,snapshot.core.running])
 function navigateTabs(event:KeyboardEvent<HTMLButtonElement>,index:number){
  let next=index
  if(event.key==='ArrowRight')next=(index+1)%sections.length
  else if(event.key==='ArrowLeft')next=(index+sections.length-1)%sections.length
  else if(event.key==='Home')next=0
  else if(event.key==='End')next=sections.length-1
  else return
  event.preventDefault();setSection(sections[next].id)
  document.getElementById(`${prefix}-tab-${sections[next].id}`)?.focus()
 }
 return <div className="settings-workspace">
  <div className="settings-tabs" role="tablist" aria-label="系统设置分类">{sections.map((item,index)=><button key={item.id} id={`${prefix}-tab-${item.id}`} type="button" role="tab" aria-controls={`${prefix}-panel-${item.id}`} aria-selected={section===item.id} tabIndex={section===item.id?0:-1} onClick={()=>setSection(item.id)} onKeyDown={event=>navigateTabs(event,index)}><item.icon size={15}/>{item.label}</button>)}</div>
  {error&&<Notice error>{error}</Notice>}
  <div className="settings-panel settings-service" id={`${prefix}-panel-service`} role="tabpanel" aria-labelledby={`${prefix}-tab-service`} hidden={section!=='service'}>
   <KernelPanel snapshot={snapshot} refresh={refresh} notify={notify}/>
   <UpdatePanel notify={notify}/>
  </div>
  <div className="settings-panel settings-traffic" id={`${prefix}-panel-traffic`} role="tabpanel" aria-labelledby={`${prefix}-tab-traffic`} hidden={section!=='traffic'}>
   <TrafficSettings snapshot={snapshot} refresh={refresh} notify={notify}/>
   <div className="settings-column"><section className="surface"><div className="section-heading"><h2>历史记录</h2></div><AsyncForm label="保存历史设置" submit={async()=>{await api('/settings','PUT',{retentionDays:retention});await refresh();notify('历史记录设置已保存')}}><Field label="流量历史保留天数"><input type="number" min={1} max={365} value={retention} onChange={e=>setRetention(Number(e.target.value))}/></Field></AsyncForm></section>
    <section className="surface"><div className="section-heading"><h2>运行事件</h2><span className="subtle">最近 100 条</span></div><div className="event-list">{events.length?events.map((event,index)=><div key={`${event.time}-${index}`}><span>{event.message}</span><time>{event.time} UTC</time></div>):<p>暂无事件记录</p>}</div></section>
   </div>
  </div>
  <div className="settings-panel" id={`${prefix}-panel-website`} role="tabpanel" aria-labelledby={`${prefix}-tab-website`} hidden={section!=='website'}><Website snapshot={snapshot} refresh={refresh} notify={notify}/></div>
  <div className="settings-panel settings-security" id={`${prefix}-panel-security`} role="tabpanel" aria-labelledby={`${prefix}-tab-security`} hidden={section!=='security'}>
   <section className="surface"><div className="section-heading"><h2>管理员密码</h2></div><AsyncForm label="更新密码并退出" submit={async()=>{await api('/password','POST',{current,password});window.dispatchEvent(new Event('session-expired'))}}><Field label="当前密码"><input type="password" required autoComplete="current-password" value={current} onChange={e=>setCurrent(e.target.value)}/></Field><Field label="新密码"><input type="password" required minLength={12} maxLength={72} autoComplete="new-password" value={password} onChange={e=>setPassword(e.target.value)}/></Field></AsyncForm></section>
   <SettingsBackup refresh={refresh} notify={notify}/>
  </div>
 </div>
}
