import { execFileSync } from 'node:child_process'
import { readFileSync, readdirSync, writeFileSync, existsSync } from 'node:fs'
import { resolve, join } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const chunks = ['PrismFlow bundled dependency notices\nGenerated from the locked local dependency versions.\n']
const missing = []
function collect(name, dir) {
  const files = readdirSync(dir, { withFileTypes: true }).filter(f => f.isFile() && /^(licen[cs]e|copying|notice)([._-].*)?$/i.test(f.name))
  if (!files.length) missing.push(name)
  for (const file of files) chunks.push(`\n${'='.repeat(72)}\n${name} — ${file.name}\n${'='.repeat(72)}\n${readFileSync(join(dir, file.name), 'utf8')}`)
}
const list = execFileSync('go', ['list', '-deps', '-f', '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}', './cmd/prismflow'], { cwd: root, encoding: 'utf8', windowsHide: true, env: { ...process.env, GOOS: 'linux', GOARCH: 'amd64', CGO_ENABLED: '0' } })
for (const line of new Set(list.trim().split(/\r?\n/))) {
  const [name, version, dir] = line.split('|')
  if (version && dir && existsSync(dir)) collect(`${name} ${version}`, dir)
}
const goroot = execFileSync('go', ['env', 'GOROOT'], { encoding: 'utf8', windowsHide: true }).trim()
collect('Go standard library', goroot)
const lock = JSON.parse(readFileSync(join(root, 'web/package-lock.json'), 'utf8'))
for (const [path, info] of Object.entries(lock.packages)) {
  if (path && !info.dev) collect(`${path.replace(/^node_modules\//, '')} ${info.version}`, join(root, 'web', path))
}
if (missing.length) throw new Error(`Missing license files: ${missing.join(', ')}`)
writeFileSync(join(root, 'third_party/dependencies.txt'), chunks.join('\n'))
console.log(`Collected ${chunks.length - 1} license/notice files`)
