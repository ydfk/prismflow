import { useEffect, useId, useRef, useState } from 'react'
import { CircleHelp } from 'lucide-react'

export default function FieldHelp({label,text,id}:{label:string;text:string;id?:string}) {
 const fallback=useId();const target=id||fallback;const popup=useRef<HTMLDivElement>(null);const [open,setOpen]=useState(false)
 useEffect(()=>{
  const dismiss=()=>{if(popup.current?.matches(':popover-open'))popup.current.hidePopover()}
  const scroll=(event:Event)=>{if(event.target!==popup.current)dismiss()}
  window.addEventListener('resize',dismiss);window.addEventListener('scroll',scroll,true)
  return()=>{window.removeEventListener('resize',dismiss);window.removeEventListener('scroll',scroll,true)}
 },[])
 function toggle(button:HTMLButtonElement) {
  const element=popup.current!
  if(element.matches(':popover-open')){element.hidePopover();return}
  const rect=button.getBoundingClientRect();const width=Math.min(320,window.innerWidth-24)
  element.style.width=`${width}px`;element.showPopover()
  const height=element.getBoundingClientRect().height
  element.style.left=`${Math.max(12,Math.min(rect.left,window.innerWidth-width-12))}px`
  element.style.top=`${rect.bottom+8+height<=window.innerHeight-12?rect.bottom+8:Math.max(12,rect.top-height-8)}px`
 }
 return <><button type="button" className="field-help-toggle" aria-label={`解释 ${label}`} aria-expanded={open} aria-controls={target} onClick={e=>toggle(e.currentTarget)}><CircleHelp size={14}/></button><div ref={popup} id={target} popover="auto" className="field-help-note" role="note" onToggle={()=>setOpen(!!popup.current?.matches(':popover-open'))}>{text}</div></>
}
