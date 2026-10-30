import { execFileSync, spawnSync } from 'node:child_process'
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
async function waitForCore() {
  const deadline = Date.now() + 360000
  let install
  do {
    install = await (await api('/core/install')).json()
    if (install.job.status === 'failed') throw new Error(install.job.message)
    if (install.job.status !== 'running' && (await state()).core.available) return install
    if (Date.now() > deadline) throw new Error('首次启动自动下载内核超时')
    await pause(1000)
  } while (true)
}
try {
  const image = process.env.PRISMFLOW_TEST_IMAGE || 'prismflow:local'
  docker('run', '--rm', '--network', 'none', '--entrypoint', 'sh', image, '-ec',
    'test ! -e /usr/local/bin/mihomo; test ! -e /data/bin/mihomo; test -f /usr/share/zoneinfo/Asia/Shanghai; curl --version; openssl version')
  docker('volume', 'create', volume)
  // 无网络的首次启动仍可打开面板，失败后可保留数据重新下载。
  docker('run', '--detach', '--network', 'none', '--name', name,
    '--mount', `type=volume,source=${volume},target=/data`, image)
  created = true
  let offlineReady = false
  for (let i = 0; i < 120; i++) {
    const logResult = spawnSync('docker', ['logs', name], { encoding: 'utf8', windowsHide: true, timeout: 10000 })
    assert.equal(logResult.status, 0)
    const logs = logResult.stdout + logResult.stderr
    if (logs.includes('内核安装失败')) {
      assert.match(docker('exec', name, 'curl', '-fsS', 'http://127.0.0.1:8080/api/status'), /setup/)
      assert.equal(docker('exec', name, 'sh', '-c', 'test ! -f /data/bin/mihomo && echo absent'), 'absent')
      offlineReady = true
      break
    }
    await pause(250)
  }
  assert.ok(offlineReady, '无网络启动未在 30 秒内报告下载失败')
  docker('rm', '--force', name)
  created = false
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
  const initialInstall = await waitForCore()
  assert.equal(initialInstall.job.status, 'done', initialInstall.job.message)
  assert.equal(initialInstall.path, '/data/bin/mihomo')
  const coreChecksum = docker('exec', name, 'sha256sum', '/data/bin/mihomo')
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
  assert.equal(docker('exec', name, 'sha256sum', '/data/bin/mihomo'), coreChecksum)
  assert.notEqual((await (await api('/core/install')).json()).job.status, 'running', '重启不应重复下载已有内核')
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
  // 用同一数据卷断网重建，并模拟旧 Compose 的初始内核路径。
  docker('rm', '--force', name)
  created = false
  docker('run', '--detach', '--network', 'none', '--name', name, '-e', 'PRISMFLOW_CORE=/usr/local/bin/mihomo',
    '--mount', `type=volume,source=${volume},target=/data`, image)
  created = true
  let cachedStarted = false
  let cachedSession = ''
  for (let i = 0; i < 120; i++) {
    const result = spawnSync('docker', ['logs', name], { encoding: 'utf8', windowsHide: true, timeout: 10000 })
    assert.equal(result.status, 0)
    const logs = result.stdout + result.stderr
    assert.ok(!logs.includes('开始下载 Mihomo'), '已有内核时断网重建不应触发下载')
    const probe = spawnSync('docker', ['exec', name, 'sh', '-c', 'curl -fsS http://127.0.0.1:8080/api/status'],
      { encoding: 'utf8', windowsHide: true, timeout: 10000 })
    if (probe.status === 0) {
      if (!cachedSession) {
        const loginResponse = docker('exec', name, 'curl', '-fsS', '-D', '-', '-H', 'Content-Type: application/json',
          '--data', JSON.stringify({username: 'admin', password}), 'http://127.0.0.1:8080/api/session')
        cachedSession = loginResponse.match(/set-cookie: ([^;\r\n]+)/i)?.[1]
        assert.ok(cachedSession, '断网重建后登录失败')
      }
      const snapshot = JSON.parse(docker('exec', name, 'curl', '-fsS', '-H', `Cookie: ${cachedSession}`, 'http://127.0.0.1:8080/api/state'))
      if (snapshot.core.running && snapshot.appliedRevision === snapshot.state.revision) {
        assert.equal(docker('exec', name, 'sha256sum', '/data/bin/mihomo'), coreChecksum)
        cachedStarted = true
        break
      }
    }
    await pause(250)
  }
  assert.ok(cachedStarted, '断网重建未能复用已有内核和配置')
  console.log(JSON.stringify({ result: 'passed', image: process.env.PRISMFLOW_TEST_IMAGE || 'prismflow:local', nonRootUID: 10001, listenerPort: endpoint.port,
    checks: ['no bundled core', 'offline startup keeps panel available', 'automatic core download + SHA-256', 'persistent core reused after restart', 'offline recreate with legacy core path', 'setup', 'CSRF session', 'automatic apply', 'subscription', 'Linux interface accounting + subscription headers', 'public isolation', 'restart persistence', 'token rotation', 'encrypted backup restore'], idleContainerMemory: usage }, null, 2))
} finally {
  if (created) docker('rm', '--force', name)
  docker('volume', 'rm', volume)
  rmSync(fixtureDir,{recursive:true,force:true})
}
