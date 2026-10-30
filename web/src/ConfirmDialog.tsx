import { useState } from 'react'
import Modal from './Modal'
import { Button, Notice } from './ui'
export default function ConfirmDialog({title,message,confirmLabel='确认',onConfirm,onClose}:{title:string;message:string;confirmLabel?:string;onConfirm:()=>Promise<void>;onClose:()=>void}){
 const [busy,setBusy]=useState(false);const [error,setError]=useState('')
 return <Modal title={title} onClose={()=>{if(!busy)onClose()}}><p className="confirm-message">{message}</p>{error&&<Notice error>{error}</Notice>}<div className="form-actions"><Button disabled={busy} onClick={onClose}>取消</Button><Button className="primary" busy={busy} onClick={async()=>{setBusy(true);setError('');try{await onConfirm();onClose()}catch(e){setError((e as Error).message)}finally{setBusy(false)}}}>{confirmLabel}</Button></div></Modal>
}
