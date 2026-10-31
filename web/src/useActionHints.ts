import {useEffect} from 'react'

// 统一补齐图标按钮提示，保留业务组件写明的详细用途。
export default function useActionHints(){
 useEffect(()=>{
  const hint=(event:Event)=>{
   const button=(event.target as Element)?.closest?.('button')
   if(!button||button.title&&!button.dataset.autoHint)return
   const text=button.getAttribute('aria-label')||button.textContent?.trim()
   if(text){button.title=text;button.dataset.autoHint='true'}
  }
  document.addEventListener('pointerover',hint,true);document.addEventListener('focusin',hint,true)
  return()=>{document.removeEventListener('pointerover',hint,true);document.removeEventListener('focusin',hint,true)}
 },[])
}
