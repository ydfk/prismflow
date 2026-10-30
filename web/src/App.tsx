import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import { Activity, ChevronDown, Check, Layers3, LogOut, Menu, Moon, Palette, Radio, Settings2, ShieldCheck, Sun, Users, X } from 'lucide-react'
import { api } from './api'
import type { Page, Snapshot } from './types'
import { Notice } from './ui'
import Login from './Login'
import Overview from './pages/Overview'
import Appearance, { restoreMaterial } from './Appearance'
const Endpoints = lazy(() => import('./pages/Endpoints'))
const UserPage = lazy(() => import('./pages/Users'))
const Certificates = lazy(() => import('./pages/Certificates'))

const Settings = lazy(() => import('./pages/Settings'))
const navigation = [
 { id:'overview',name:'总览',icon:Activity,description:'服务器的每一次连接，尽在掌握。' },
 { id:'endpoints',name:'协议入口',icon:Layers3,description:'选好组合，几步完成部署。' },
 { id:'users',name:'用户与订阅',icon:Users,description:'为每位用户，自由选择连接方式。' },
 { id:'certificates',name:'域名与证书',icon:ShieldCheck,description:'系统证书与自动签发，按需选择。' },
 { id:'settings',name:'系统设置',icon:Settings2,description:'管理内核、订阅和服务状态。' },
] as const
export default function App() {
 const [snapshot,setSnapshot]=useState<Snapshot>(); const [auth,setAuth]=useState<'loading'|'login'|'ready'>('loading'); const [setup,setSetup]=useState(false)
 const [page,setPage]=useState<Page>('overview'); const [error,setError]=useState('');const [toast,setToast]=useState(''); const [menu,setMenu]=useState(false);const [appearance,setAppearance]=useState(false)
 const [dark,setDark]=useState(()=>localStorage.getItem('prismflow.theme.v2')!=='light')
 const [mobile,setMobile]=useState(()=>window.matchMedia('(max-width:760px)').matches);const sidebar=useRef<HTMLElement>(null)
 useEffect(()=>restoreMaterial(),[])
 const refresh=useCallback(async()=>{const result=await api<Snapshot>('/state');setSnapshot(result);setAuth('ready');setError('')},[])
 useEffect(()=>{let active=true;api<{setupRequired:boolean}>('/status').then(async status=>{if(!active)return;setSetup(status.setupRequired);if(status.setupRequired)setAuth('login');else try{await refresh()}catch{if(active)setAuth('login')}}).catch(e=>setError(e.message));const expire=()=>setAuth('login');window.addEventListener('session-expired',expire);return()=>{active=false;window.removeEventListener('session-expired',expire)}},[refresh])
 useEffect(()=>{if(auth!=='ready')return;const timer=setInterval(()=>{if(!document.hidden)refresh().catch(e=>setError(e.message))},5000);return()=>clearInterval(timer)},[auth,refresh])
 useEffect(()=>{document.documentElement.dataset.theme=dark?'dark':'light';localStorage.setItem('prismflow.theme.v2',dark?'dark':'light')},[dark])
 useEffect(()=>{if(!toast)return;const timer=setTimeout(()=>setToast(''),4500);return()=>clearTimeout(timer)},[toast])
 useEffect(()=>{const query=window.matchMedia('(max-width:760px)');const change=()=>{setMobile(query.matches);setMenu(false)};query.addEventListener('change',change);return()=>query.removeEventListener('change',change)},[])
 useEffect(()=>{
  if(!mobile||!menu)return;const previous=document.activeElement as HTMLElement|null;const panel=sidebar.current;panel?.querySelector<HTMLButtonElement>('.mobile-close')?.focus()
  const keydown=(event:KeyboardEvent)=>{if(event.key==='Escape'){setMenu(false);return}if(event.key!=='Tab'||!panel)return;const items=[...panel.querySelectorAll<HTMLElement>('button:not(:disabled)')].filter(e=>e.getClientRects().length>0);const first=items[0],last=items.at(-1);if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus()}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus()}}
  document.addEventListener('keydown',keydown);return()=>{document.removeEventListener('keydown',keydown);previous?.focus()}
 },[mobile,menu])
 const navigate=(target:Page)=>{setPage(target);setMenu(false);window.scrollTo({top:0})}
 if(auth==='loading')return <div className="loading-page"><img src="/logo.svg" alt="PrismFlow"/><p>正在连接工作台…</p>{error&&<Notice error>{error}</Notice>}</div>
 if(auth==='login')return <Login setup={setup} onLogin={()=>{setSetup(false);refresh().catch(e=>setError(e.message))}}/>
 if(!snapshot)return null
 const current=navigation.find(item=>item.id===page)!;const props={snapshot,refresh,notify:setToast}
 return <div className="app-shell"><a className="skip-link" href="#main">跳至主要内容</a>
  <header className="masthead"><a className="brand" href="/"><span className="brand-mark"><img src="/logo.svg" alt=""/></span><span>PrismFlow<small>棱流</small></span></a><div className="masthead-actions"><span className="server-pill"><Radio size={14}/><span className={`status-light ${snapshot.core.running?'online':''}`}/>{snapshot.core.running?'服务运行中':'等待启动'}</span><button className="icon-button" aria-label="外观设置" aria-expanded={appearance} onClick={()=>setAppearance(!appearance)}><Palette size={18}/></button><button className="icon-button" aria-label={dark?'切换浅色主题':'切换深色主题'} onClick={()=>setDark(!dark)}>{dark?<Sun size={18}/>:<Moon size={18}/>}</button><button className="icon-button" aria-label="退出登录" onClick={async()=>{try{await api('/session','DELETE');setAuth('login')}catch(e){setError((e as Error).message)}}}><LogOut size={17}/></button></div></header>
  <div className="navigation-bar surface"><button className="icon-button mobile-menu" aria-label="打开导航" aria-expanded={menu} aria-controls="navigation-drawer" onClick={()=>setMenu(true)}><Menu size={20}/></button>
   <nav id="navigation-drawer" ref={sidebar} className={`sidebar ${menu?'open':''}`} inert={mobile&&!menu} aria-label="主要导航" role={mobile?'dialog':undefined} aria-modal={mobile&&menu?true:undefined}><button className="icon-button mobile-close" aria-label="关闭导航" onClick={()=>setMenu(false)}><X size={20}/></button>{navigation.map(item=><button key={item.id} className={`nav-item ${page===item.id?'active':''}`} aria-current={page===item.id?'page':undefined} onClick={()=>navigate(item.id)}><item.icon size={16}/><span>{item.name}</span>{item.id!=='overview'&&<ChevronDown size={12}/>}</button>)}</nav><span className="mobile-current">{current.name}</span></div>
  {menu&&mobile&&<button className="mobile-scrim" tabIndex={-1} aria-label="收起导航" onClick={()=>setMenu(false)}/>}
  <div className="main-shell" inert={mobile&&menu}><main id="main"><div className="page-heading"><div><h1>{current.name}</h1><p>{current.description}</p></div><span className="save-status"><Check size={14}/>{snapshot.core.available?'保存即生效':'配置已保存 · 等待内核'}</span></div>
   {error&&<Notice error>{error}</Notice>}{!snapshot.core.available&&<Notice>尚未安装 Mihomo。前往「系统设置」点击下载内核；安装完成后自动启用已保存的配置。</Notice>}
   <Suspense fallback={<div className="surface empty">正在加载…</div>}>{page==='overview'&&<Overview {...props} navigate={navigate}/>} {page==='endpoints'&&<Endpoints {...props} navigate={navigate}/>} {page==='users'&&<UserPage {...props}/>} {page==='certificates'&&<Certificates {...props}/>} {page==='settings'&&<Settings {...props}/>}</Suspense>
   <footer className="page-footer"><span>PrismFlow · 棱流</span><span>Mihomo {snapshot.core.targetVersion} · 单服务器</span></footer>
  </main></div>{appearance&&<Appearance onClose={()=>setAppearance(false)}/>} {toast&&<div className="toast" role="status"><Check size={17}/>{toast}</div>}
 </div>
}
