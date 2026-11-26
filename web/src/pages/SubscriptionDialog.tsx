import { useEffect, useState } from 'react'
import { Copy } from 'lucide-react'
import { api, copy } from '../api'
import Modal from '../Modal'
import type { PageProps, User } from '../types'
import { Button, Field, Notice } from '../ui'

type Subscription = { url: string; linksUrl: string; links: string }
type Codes = { clash: string | null; universal: string | null }
const qrOptions = { width: 256, margin: 4, color: { dark: '#143c36', light: '#ffffff' } }

export default function SubscriptionDialog({ user, notify, onClose }: {
  user: User; notify: PageProps['notify']; onClose: () => void
}) {
  const [subscription, setSubscription] = useState<Subscription>()
  const [codes, setCodes] = useState<Codes>()
  const [error, setError] = useState('')

  useEffect(() => {
    const controller = new AbortController()
    async function load() {
      let result: Subscription
      try {
        result = await api<Subscription>(`/users/${user.id}/subscription`, 'GET', undefined, controller.signal)
      } catch (failure) {
        if (!controller.signal.aborted) setError((failure as Error).message)
        return
      }
      if (controller.signal.aborted) return
      setSubscription(result)

      try {
        const { toDataURL } = await import('qrcode')
        const [clash, universal] = await Promise.allSettled([
          toDataURL(result.url, qrOptions),
          toDataURL(result.linksUrl, qrOptions),
        ])
        if (!controller.signal.aborted) setCodes({
          clash: clash.status === 'fulfilled' ? clash.value : null,
          universal: universal.status === 'fulfilled' ? universal.value : null,
        })
      } catch {
        if (!controller.signal.aborted) setCodes({ clash: null, universal: null })
      }
    }
    void load()
    return () => controller.abort()
  }, [user.id])

  async function copyText(text: string) {
    setError('')
    try {
      await copy(text)
      notify('已复制')
    } catch (failure) {
      setError((failure as Error).message)
    }
  }

  return <Modal title={`${user.name} 的订阅`} onClose={onClose} wide>
    <section className="share-panel">
      {error && <Notice error>{error}</Notice>}
      {!subscription && !error && <p className="subtle" role="status">正在读取订阅链接…</p>}
      {subscription && <>
        <div className="share-content">
          <SubscriptionFormat label="Mihomo / Clash Meta 订阅" imageLabel="Clash / Mihomo 订阅二维码" copyLabel="复制 Clash / Mihomo 订阅链接"
            url={subscription.url} qr={codes?.clash} onCopy={copyText}/>
          <SubscriptionFormat label="通用节点链接订阅" imageLabel="通用订阅二维码" copyLabel="复制通用订阅链接"
            url={subscription.linksUrl} qr={codes?.universal} onCopy={copyText}/>
        </div>
        <details>
          <summary>查看节点分享链接</summary>
          <textarea aria-label="节点分享链接" readOnly rows={5} value={subscription.links}/>
          <Button type="button" onClick={() => copyText(subscription.links)}><Copy size={16}/>复制所有节点</Button>
        </details>
        <p className="footnote">订阅链接包含访问凭据，请仅分享给对应用户。显示的是当前已生效的节点配置。</p>
      </>}
    </section>
  </Modal>
}

function SubscriptionFormat({ label, imageLabel, copyLabel, url, qr, onCopy }: {
  label: string; imageLabel: string; copyLabel: string; url: string; qr?: string | null; onCopy: (url: string) => void
}) {
  return <Field label={label}>
    <div className="share-format">
      <div className="share-qr">
        {qr ? <img src={qr} alt={imageLabel} width={160} height={160}/> :
          <span role="status">{qr === null ? '二维码生成失败，可复制下方链接' : '正在生成二维码…'}</span>}
      </div>
      <div className="input-action">
        <input readOnly value={url} onFocus={event => event.target.select()}/>
        <Button type="button" aria-label={copyLabel} onClick={() => onCopy(url)}><Copy size={16}/></Button>
      </div>
    </div>
  </Field>
}
