import { execFileSync } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve, join } from 'node:path'

// 仅创建、删除本次测试命名的容器与卷，管理端口只映射到回环地址。
const name = `prismflow-test-${randomBytes(5).toString('hex')}`
const volume = `${name}-data`
const fixtureDir=mkdtempSync(join(tmpdir(),`${name}-`))
const fixturePath=join(fixtureDir,'cert.json')
execFileSync('go',['run',resolve(import.meta.dirname,'certificate-fixture.go'),fixturePath],{cwd:resolve(import.meta.dirname,'..'),windowsHide:true})
const docker = (...args) => execFileSync('docker', args, { encoding: 'utf8', windowsHide: true, timeout: 120000 }).trim()
let admin; let publicURL; let cookie = ''; let csrf = ''; let created = false
const password = randomBytes(20).toString('hex')
const pause = ms => new Promise(resolve => setTimeout(resolve, ms))
async function ready() {
  for (let i = 0; i < 120; i++) {
    try { if ((await fetch(`${admin}/api/status`)).ok) return } catch {}
    await pause(250)
  }
  throw new Error('容器未在 30 秒内就绪')
}
async function api(path, method = 'GET', body, expected = 200) {
  const response = await fetch(`${admin}/api${path}`, {
    method, headers: { 'Content-Type': 'application/json', Cookie: cookie, 'X-CSRF-Token': csrf },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  assert.equal(response.status, expected, `${method} ${path}: ${await response.clone().text()}`)
  if (path === '/session' && method === 'POST') cookie = response.headers.get('set-cookie').split(';')[0]
  if (response.headers.get('x-csrf-token')) csrf = response.headers.get('x-csrf-token')
  return response
}
async function login() { csrf = (await (await api('/session', 'POST', { username: 'admin', password })).json()).csrf }
async function state() { return (await api('/state')).json() }
try {
  docker('volume', 'create', volume)
  docker('run', '--detach', '--name', name, '--cap-drop', 'ALL', '--cap-add', 'NET_BIND_SERVICE', '--security-opt', 'no-new-privileges',
    '-p', '127.0.0.1::8080', '-p', '127.0.0.1::8081', '-e', 'PRISMFLOW_LISTEN=0.0.0.0:8080', '-e', 'PRISMFLOW_PUBLIC_LISTEN=0.0.0.0:8081',
    '--mount', `type=volume,source=${volume},target=/data`, process.env.PRISMFLOW_TEST_IMAGE || 'prismflow:local')
  created = true
  admin = `http://${docker('port', name, '8080/tcp')}`
  publicURL = `http://${docker('port', name, '8081/tcp')}`
  await ready()
  assert.equal(docker('exec', name, 'id', '-u'), '10001')
  const token = docker('exec', name, 'cat', '/data/setup.token')
  await api('/setup', 'POST', { token, username: 'admin', password }, 201)
  await login()
  await api('/certificates/import','POST',JSON.parse(readFileSync(fixturePath,'utf8')))
  const certificate=(await state()).state.certificates[0]
  const ports=await (await api('/ports/random','POST',{count:1})).json()
  await api('/endpoints/preset','POST',{variants:['anytls'],host:'127.0.0.1',ports:{anytls:ports[0]},certificateId:certificate.id})
  const endpoint = (await state()).state.endpoints[0]
  await api('/users', 'POST', { name: 'Docker device', enabled: true, endpointIds: [endpoint.id] })
  await api('/settings', 'PUT', { publicUrl: publicURL, retentionDays: 30, traffic: {maxBytes: 1073741824, resetDay: 20, timezone: 'UTC', interface: '', outboundOnly: false} })
  const applied = await state(); assert.equal(applied.core.running, true); assert.equal(applied.appliedRevision, applied.state.revision)
  assert.equal(applied.traffic.available,true,applied.traffic.message);assert.ok(applied.traffic.interface)
  const user = applied.state.users[0]
  const share = await (await api(`/users/${user.id}/subscription`)).json()
  const sub = await fetch(share.url); assert.equal(sub.status, 200); assert.match(await sub.text(), /type: anytls/)
  assert.match(sub.headers.get('subscription-userinfo'), /total=1073741824/);assert.equal(sub.headers.get('x-prismflow-traffic-status'),'current');assert.ok(Number(sub.headers.get('x-prismflow-traffic-reset-at'))>Date.now()/1000)
  assert.equal((await fetch(`${publicURL}/api/state`)).status, 404)
  if (process.env.PRISMFLOW_TEST_DOWNLOAD === '1') {
    await api('/core/install', 'POST', undefined, 202)
    let job
    const deadline = Date.now() + 330000
    do {
      await pause(1000)
      job = await (await api('/core/install')).json()
      if (Date.now() > deadline) throw new Error('内核下载验收超时')
    } while (job.job.status === 'running')
    assert.equal(job.job.status, 'done', job.job.message)
    assert.equal(job.path, '/data/bin/mihomo')
    console.log('Official Mihomo download + SHA-256 + automatic activation: passed')
  }
  const archive = Buffer.from(await (await api('/backup', 'POST', { passphrase: password })).arrayBuffer()).toString('base64')
  await api('/restore', 'POST', { passphrase: password, archive }, 409)
  docker('restart', name); cookie = ''; csrf = ''
  admin = `http://${docker('port', name, '8080/tcp')}`
  const restartedPublic = `http://${docker('port', name, '8081/tcp')}`
  await ready(); await login()
  for (let i = 0; i < 40 && !(await state()).core.running; i++) await pause(250)
  assert.equal((await state()).core.running, true)
  const liveLink = share.url.replace(publicURL, restartedPublic)
  assert.equal((await fetch(liveLink)).status, 200)
  await api(`/users/${user.id}/rotate`, 'POST', { credentials: false })
  assert.equal((await fetch(liveLink)).status, 404)
  await api('/core/stop', 'POST')
  await api(`/users/${user.id}`, 'DELETE')
  await api('/core/stop', 'POST')
  await api('/restore', 'POST', { passphrase: `${password}wrong`, archive }, 400)
  await api('/restore', 'POST', { passphrase: password, archive })
  assert.equal((await state()).state.users[0].id, user.id)
  assert.equal((await state()).core.running, false)
  await api('/core/apply', 'POST')
  assert.equal((await fetch(liveLink)).status, 200)
  const usage = docker('stats', '--no-stream', '--format', '{{.MemUsage}}', name)
  console.log(JSON.stringify({ result: 'passed', image: process.env.PRISMFLOW_TEST_IMAGE || 'prismflow:local', nonRootUID: 10001, listenerPort: endpoint.port,
    checks: ['setup', 'CSRF session', 'automatic apply', 'subscription', 'Linux interface accounting + subscription headers', 'public isolation', 'restart persistence', 'token rotation', 'encrypted backup restore'], idleContainerMemory: usage }, null, 2))
} finally {
  if (created) docker('rm', '--force', name)
  docker('volume', 'rm', volume)
  rmSync(fixtureDir,{recursive:true,force:true})
}
