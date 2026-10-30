import { useRef, useState } from 'react'
import { Download, Upload } from 'lucide-react'
import { api, request } from '../api'
import type { PageProps } from '../types'
import { Button, Field, Notice } from '../ui'

export default function SettingsBackup({refresh,notify}:Pick<PageProps,'refresh'|'notify'>){
 const fileInput=useRef<HTMLInputElement>(null); const [passphrase,setPassphrase]=useState('')
 const [busy,setBusy]=useState(false)
 const [error,setError]=useState('')
 async function backup(){
  setBusy(true);setError('')
  try{
   const response=await request('/backup','POST',{passphrase})
   const blob=await response.blob();const link=document.createElement('a')
   link.href=URL.createObjectURL(blob);link.download='prismflow.pfb';link.click()
   setTimeout(()=>URL.revokeObjectURL(link.href),1000);notify('加密备份已下载')
  }catch(e){setError((e as Error).message)}finally{setBusy(false)}
 }
 async function restore(file?:File){
  if(!file||!confirm('恢复将替换入口、用户、证书和网站配置。请先停止内核并备份当前配置。继续？'))return
  setBusy(true);setError('')
  try{
   if(file.size>35*1024*1024)throw new Error('备份文件超过大小限制')
   const bytes=new Uint8Array(await file.arrayBuffer());let archive=''
   for(let i=0;i<bytes.length;i+=8192)archive+=String.fromCharCode(...bytes.subarray(i,i+8192))
   await api('/restore','POST',{passphrase,archive:btoa(archive)})
   await refresh();notify('配置已恢复，可在内核卡片启动服务')
  }catch(e){setError((e as Error).message)}finally{setBusy(false)}
 }
 return <section className="surface settings-backup">
  <div className="section-heading"><h2>加密备份与恢复</h2><span className="subtle">AES-256-GCM</span></div>
  <p className="section-description">备份包含协议凭据、用户、当前证书、内置 ACME 账户和自定义网站。acme.sh 账户、系统证书原文件请另行备份；不包含管理员密码、流量历史和日志。</p>
  {error&&<Notice error>{error}</Notice>}
  <Field label="备份密码" hint="至少 12 字节；恢复时使用创建备份时的密码。"><input type="password" minLength={12} autoComplete="new-password" value={passphrase} onChange={e=>setPassphrase(e.target.value)}/></Field>
  <div className="button-wrap"><Button busy={busy} disabled={passphrase.length<12} onClick={backup}><Download size={16}/>下载备份</Button><Button disabled={busy||passphrase.length<12} onClick={()=>fileInput.current?.click()}><Upload size={16}/>恢复备份</Button><input ref={fileInput} type="file" accept=".pfb" hidden disabled={busy||passphrase.length<12} onChange={e=>{restore(e.target.files?.[0]);e.target.value=''}}/></div>
 </section>
}
