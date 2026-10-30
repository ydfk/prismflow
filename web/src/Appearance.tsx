import { useEffect, useRef, useState } from 'react'
import { RotateCcw, X } from 'lucide-react'
import { Button, Field } from './ui'
const defaults={blur:20,opacity:72,saturation:125}
export default function Appearance({onClose}:{onClose:()=>void}){
 const panel=useRef<HTMLElement>(null)
 useEffect(()=>{const previous=document.activeElement as HTMLElement|null;panel.current?.querySelector<HTMLButtonElement>('button')?.focus();return()=>previous?.focus()},[])
 const [values,setValues]=useState(()=>{try{return {...defaults,...JSON.parse(localStorage.getItem('prismflow.material')||'{}')}}catch{return defaults}})
 useEffect(()=>{for(const [key,value] of Object.entries(values))document.documentElement.style.setProperty(`--glass-${key}`,key==='blur'?`${value}px`:`${value}%`);localStorage.setItem('prismflow.material',JSON.stringify(values))},[values])
 useEffect(()=>{const close=(e:KeyboardEvent)=>{if(e.key==='Escape')onClose()};window.addEventListener('keydown',close);return()=>window.removeEventListener('keydown',close)},[onClose])
 return <aside ref={panel} className="appearance surface" aria-label="外观设置"><div className="section-heading"><h2>玻璃材质</h2><button className="icon-button" aria-label="关闭外观设置" onClick={onClose}><X size={18}/></button></div><p className="subtle">调整工作台的透明度与折光感，设置仅保存在这台浏览器。</p>{([{key:'blur',label:'背景模糊',min:0,max:40,unit:'px'},{key:'opacity',label:'面板不透明度',min:45,max:100,unit:'%'},{key:'saturation',label:'色彩饱和度',min:50,max:180,unit:'%'}] as const).map(item=><Field key={item.key} label={`${item.label} · ${values[item.key]}${item.unit}`}><input type="range" min={item.min} max={item.max} value={values[item.key]} onChange={e=>setValues({...values,[item.key]:Number(e.target.value)})}/></Field>)}<Button onClick={()=>setValues(defaults)}><RotateCcw size={15}/>恢复默认</Button></aside>
}
export function restoreMaterial(){try{const v={...defaults,...JSON.parse(localStorage.getItem('prismflow.material')||'{}')};for(const [k,n]of Object.entries(v))document.documentElement.style.setProperty(`--glass-${k}`,k==='blur'?`${n}px`:`${n}%`)}catch{/* 损坏的浏览器偏好沿用默认样式。 */}}
