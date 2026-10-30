import { useState } from 'react'
import { ArrowUp, FileKey, Folder, Search } from 'lucide-react'
import { api } from '../api'
import { Button, Notice } from '../ui'
type Listing={path:string;parent:string;entries:{name:string;path:string;directory:boolean}[]}
export default function CertificateBrowser({onSelect}:{onSelect:(path:string)=>void}){
 const [path,setPath]=useState('/etc/letsencrypt/live');const [listing,setListing]=useState<Listing>();const [error,setError]=useState('');const [busy,setBusy]=useState(false)
 async function browse(target=path){setBusy(true);setError('');try{const result=await api<Listing>(`/certificates/files?path=${encodeURIComponent(target)}`);setListing(result);setPath(result.path)}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
 return <div className="file-browser"><div className="input-action"><input aria-label="服务器证书目录" value={path} onChange={e=>setPath(e.target.value)} placeholder="/etc/letsencrypt/live"/><Button type="button" busy={busy} onClick={()=>browse()}><Search size={16}/>浏览</Button></div>{error&&<Notice error>{error}</Notice>}{listing&&<div className="file-browser-list"><button type="button" onClick={()=>browse(listing.parent)}><ArrowUp size={15}/>上级目录</button>{listing.entries.map(e=><button type="button" key={e.path} onClick={()=>e.directory?browse(e.path):onSelect(e.path)}>{e.directory?<Folder size={15}/>:<FileKey size={15}/>} {e.name}</button>)}</div>}<small>浏览的是服务器文件系统；Docker 内只能看到容器中已挂载的目录。</small></div>
}
