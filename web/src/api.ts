let csrf = ''
export async function request(path: string, method = 'GET', body?: unknown): Promise<Response> {
  const headers: Record<string, string> = {}
  if (method !== 'GET') headers['X-CSRF-Token'] = csrf
  if (body !== undefined && !(body instanceof Blob)) headers['Content-Type'] = 'application/json'
  const response = await fetch(new URL(`api${path}`, document.baseURI), { method, headers, credentials: 'same-origin', body: body instanceof Blob ? body : body === undefined ? undefined : JSON.stringify(body) })
  csrf = response.headers.get('X-CSRF-Token') || csrf
  if (!response.ok) {
    const failure = await response.json().catch(() => ({ error: `请求失败（${response.status}）` }))
    if (response.status === 401 && path !== '/session') window.dispatchEvent(new Event('session-expired'))
    throw new Error(failure.error || '操作失败')
  }
  return response
}
export async function api<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await request(path, method, body)
  const result = await response.json()
  if (result.csrf) csrf = result.csrf
  return result as T
}
export function bytes(value: number) { if (!value) return '0 B'; const i = Math.min(4, Math.floor(Math.log(Math.max(1, value)) / Math.log(1024))); return `${(value / 1024 ** i).toFixed(i ? 1 : 0)} ${['B', 'KiB', 'MiB', 'GiB', 'TiB'][i]}` }
export function date(value: string) { return new Date(value).toLocaleDateString('zh-CN') }
export async function copy(value: string) { if (!navigator.clipboard) throw new Error('当前连接不支持自动复制，请选择文本手动复制，或使用 HTTPS'); await navigator.clipboard.writeText(value) }
