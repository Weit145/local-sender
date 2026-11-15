import { spawnSync } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
mkdirSync(path.join(root, 'bin'), { recursive: true })
const binary = path.join(root, 'bin', process.platform === 'win32' ? 'local-sender-test-server.exe' : 'local-sender-test-server')
const result = spawnSync('go', ['build', '-tags', 'server', '-o', binary, './cmd/desktop'], { cwd: root, stdio: 'inherit', env: { ...process.env, CGO_ENABLED: '0' } })
process.exit(result.status ?? 1)
