import { useEffect, useId, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
export default function Modal({title,children,onClose,wide=false}:{title:string;children:ReactNode;onClose:()=>void;wide?:boolean}){
 const dialog=useRef<HTMLDialogElement>(null);const label=useId();const close=useRef(onClose);close.current=onClose
 useEffect(()=>{const element=dialog.current!;const previous=document.activeElement as HTMLElement|null;element.showModal();return()=>{element.close();previous?.focus()}},[])
 return createPortal(<dialog ref={dialog} className={`modal surface ${wide?'wide':''}`} aria-labelledby={label} onCancel={e=>{e.preventDefault();close.current()}} onClick={e=>{if(e.target===e.currentTarget){const r=e.currentTarget.getBoundingClientRect();if(e.clientX<r.left||e.clientX>r.right||e.clientY<r.top||e.clientY>r.bottom)close.current()}}}><div className="modal-heading"><h2 id={label}>{title}</h2><button type="button" className="icon-button" aria-label="关闭弹窗" onClick={()=>close.current()}><X size={18}/></button></div><div className="modal-content">{children}</div></dialog>,document.body)
}
