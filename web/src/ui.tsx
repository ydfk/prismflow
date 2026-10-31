import { Children, cloneElement, isValidElement, useId, useState, type ReactElement, type ReactNode, type FormEvent } from 'react'
import { LoaderCircle, Check, AlertCircle, Plus } from 'lucide-react'
import Modal from './Modal'
import FieldHelp from './FieldHelp'
import { fieldGuidance } from './fieldGuidance'

export function Button({ children, className = '', busy, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { busy?: boolean }) {
  return <button {...props} className={`button ${className}`} disabled={props.disabled || busy}>{busy && <LoaderCircle size={16} className="spin" />}{children}</button>
}
export function Field({ label, hint, help, children }: { label: string; hint?: string; help?: string; children: ReactNode }) {
  const id = useId()
 const guidance=fieldGuidance[label];const explanation=help||guidance?.help||hint
  const associate = (nodes: ReactNode): ReactNode => Children.map(nodes, node => {
    if (!isValidElement(node)) return node
    const element = node as ReactElement<Record<string, unknown>>
    if (['input', 'select', 'textarea'].includes(String(element.type))) return cloneElement(element, { id, placeholder:element.props.placeholder||guidance?.placeholder||(['text','email','password','url','number',undefined].includes(element.props.type as string|undefined)?`请输入${label}`:undefined), 'aria-describedby': explanation ? `${id}-help` : undefined })
    if (element.type === 'div') return cloneElement(element, {}, associate(element.props.children as ReactNode))
    return element
  })
  return <div className="field"><div className="field-label"><label htmlFor={id}>{label}</label>{explanation&&<FieldHelp label={label} text={explanation} id={`${id}-help`}/>}</div>{associate(children)}</div>
}
export function Notice({ children, error = false }: { children: ReactNode; error?: boolean }) { return <div role={error ? 'alert' : 'status'} className={`notice ${error ? 'error' : ''}`}><AlertCircle size={17} /><span>{children}</span></div> }
export function Empty({ title, description, action }: { title: string; description: string; action?: ReactNode }) { return <div className="empty"><div className="empty-symbol"><Plus size={24} /></div><h3>{title}</h3><p>{description}</p>{action}</div> }
export function Editor({title,children,onClose}:{title:string;children:ReactNode;onClose:()=>void}){return <Modal title={title} onClose={onClose}>{children}</Modal>}
export function AsyncForm({ children, submit, label = '保存配置', onDone }: { children: ReactNode; submit: () => Promise<unknown>; label?: string; onDone?: () => void }) {
  const [busy, setBusy] = useState(false); const [error, setError] = useState('')
  async function handle(event: FormEvent) { event.preventDefault(); setBusy(true); setError(''); try { await submit(); onDone?.() } catch (e) { setError((e as Error).message) } finally { setBusy(false) } }
  return <form onSubmit={handle}>{children}{error && <Notice error>{error}</Notice>}<div className="form-actions"><Button className="primary" type="submit" busy={busy}><Check size={16} />{label}</Button></div></form>
}
export function Badge({ children, active = true }: { children: ReactNode; active?: boolean }) { return <span className={`badge ${active ? '' : 'muted'}`}><i />{children}</span> }
