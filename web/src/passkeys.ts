import { api } from './api'

export type Passkey = { id: string; name: string; rpId: string; createdAt: string; lastUsedAt?: string }

export function passkeySupported() {
  const { hostname, protocol } = window.location
  const usableAddress = hostname === 'localhost' || (protocol === 'https:' && !/^[\d.]+$|^\[/.test(hostname))
  return usableAddress && window.isSecureContext &&
    typeof PublicKeyCredential !== 'undefined' &&
    typeof PublicKeyCredential.parseCreationOptionsFromJSON === 'function' &&
    typeof PublicKeyCredential.parseRequestOptionsFromJSON === 'function' &&
    typeof PublicKeyCredential.prototype.toJSON === 'function'
}

type Ceremony<T> = { flowId: string; options: { publicKey: T } }

async function readCredential(operation: Promise<Credential | null>) {
  try {
    const credential = await operation
    if (credential instanceof PublicKeyCredential) return credential.toJSON()
  } catch (error) {
    if (!(error instanceof DOMException)) throw error
    if (error.name === 'InvalidStateError') throw new Error('此 Passkey 已登记，请选择其他凭据')
    if (error.name !== 'NotAllowedError' && error.name !== 'AbortError') {
      throw new Error('浏览器无法使用 Passkey，请检查面板地址和认证设备')
    }
  }
  throw new Error('Passkey 操作已取消、超时或没有匹配的凭据')
}

export async function loginWithPasskey() {
  const flow = await api<Ceremony<PublicKeyCredentialRequestOptionsJSON>>('/session/passkey/options', 'POST')
  const publicKey = PublicKeyCredential.parseRequestOptionsFromJSON(flow.options.publicKey)
  const credential = await readCredential(navigator.credentials.get({ publicKey }))
  await api('/session/passkey', 'POST', { flowId: flow.flowId, credential })
}

export async function registerPasskey(name: string, password: string) {
  const flow = await api<Ceremony<PublicKeyCredentialCreationOptionsJSON>>('/passkeys/options', 'POST', { name, password })
  const publicKey = PublicKeyCredential.parseCreationOptionsFromJSON(flow.options.publicKey)
  const credential = await readCredential(navigator.credentials.create({ publicKey }))
  return api<Passkey>('/passkeys', 'POST', { flowId: flow.flowId, credential })
}
