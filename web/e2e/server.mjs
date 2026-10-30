import { spawn, execFileSync } from 'node:child_process'
import { mkdir, mkdtemp, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const root = resolve('..')
await mkdir(resolve(root, '.cache'), { recursive: true })
const data = await mkdtemp(resolve(root, '.cache/e2e-'))
await writeFile(resolve(root, '.cache/e2e-path.txt'), data)
execFileSync('go', ['run', resolve(root, 'scripts/certificate-fixture.go'), resolve(data, 'certificate-fixture.json')], { cwd: root, windowsHide: true })
const suffix = process.platform === 'win32' ? '.exe' : ''
const child = spawn(resolve(root, `.cache/prismflow${suffix}`), [
  '--data', data, '--listen', '127.0.0.1:18880', '--public-listen', '127.0.0.1:18881', '--controller', '127.0.0.1:19097', '--core', resolve(root, `.cache/mihomo${suffix}`),
], { stdio: 'inherit', windowsHide: true })
child.on('exit', code => process.exit(code ?? 0))
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal))
