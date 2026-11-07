import { useEffect, useState } from 'react'
import { KeyRound, Plus, Trash2 } from 'lucide-react'
import { api, date } from '../api'
import ConfirmDialog from '../ConfirmDialog'
import { passkeySupported, registerPasskey, type Passkey } from '../passkeys'
import { AsyncForm, Button, Editor, Field, Notice } from '../ui'

export default function PasskeySettings({ notify }: { notify: (message: string) => void }) {
  const [keys, setKeys] = useState<Passkey[]>()
  const [error, setError] = useState('')
  const [adding, setAdding] = useState(false)
  const [revoking, setRevoking] = useState<Passkey>()
  const supported = passkeySupported()

  useEffect(() => {
    let active = true
    api<Passkey[]>('/passkeys')
      .then(value => { if (active) setKeys(value) })
      .catch(error => { if (active) setError(error.message) })
    return () => { active = false }
  }, [])

  return <section className="surface">
    <div className="section-heading">
      <h2>Passkey</h2>
      <Button disabled={!supported || !keys} onClick={() => setAdding(true)}><Plus size={16}/>添加 Passkey</Button>
    </div>
    <p className="section-description">用设备解锁或安全密钥登录管理员面板。Passkey 绑定登记时的域名。</p>
    {!supported && <Notice>添加 Passkey 需要支持此功能的浏览器，以及 HTTPS 域名或 localhost。</Notice>}
    {error ? <Notice error>{error}</Notice> : !keys ? <p className="subtle" role="status">正在读取 Passkey…</p> :
      keys.length === 0 ? <p className="subtle">尚未登记 Passkey</p> :
        <div className="mini-list">{keys.map(key => <div key={key.id}>
          <KeyRound size={17} aria-hidden="true"/>
          <div className="passkey-details">
            <strong>{key.name}</strong>
            <small>{key.rpId} · 登记于 {date(key.createdAt)}</small>
            <small>{key.lastUsedAt ? `最近使用于 ${date(key.lastUsedAt)}` : '尚未使用'}</small>
          </div>
          <button type="button" className="icon-button danger" aria-label={`撤销 Passkey ${key.name}`} title="撤销此 Passkey 的登录权限" onClick={() => setRevoking(key)}><Trash2 size={16}/></button>
        </div>)}</div>}
    {adding && <PasskeyEditor onClose={() => setAdding(false)} onAdded={key => {
      setKeys(previous => [...(previous ?? []), key])
      setAdding(false)
      notify('Passkey 已添加')
    }}/>}
    {revoking && <ConfirmDialog title="撤销 Passkey" message={`撤销「${revoking.name}」后，它将无法再用于面板登录。`} confirmLabel="确认撤销" onClose={() => setRevoking(undefined)} onConfirm={async () => {
      await api(`/passkeys/${revoking.id}`, 'DELETE')
      setKeys(previous => previous?.filter(key => key.id !== revoking.id))
      notify('Passkey 已撤销')
    }}/>}
  </section>
}

function PasskeyEditor({ onClose, onAdded }: { onClose: () => void; onAdded: (key: Passkey) => void }) {
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  return <Editor title="添加 Passkey" onClose={() => { if (!busy) onClose() }}>
    <AsyncForm label="登记 Passkey" submit={async () => {
      setBusy(true)
      try { onAdded(await registerPasskey(name, password)) }
      finally { setBusy(false) }
    }}>
      <Field label="Passkey 名称"><input required maxLength={64} value={name} onChange={event => setName(event.target.value)} placeholder="例如：我的笔记本"/></Field>
      <Field label="当前密码"><input type="password" required autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)}/></Field>
    </AsyncForm>
  </Editor>
}
