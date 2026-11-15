import { spawn } from 'node:child_process'
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
const index = Number(process.argv[2] ?? 0)
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const dir = mkdtempSync(path.join(os.tmpdir(), 'local-sender-e2e-'))
const config = path.join(dir, 'settings.json')
writeFileSync(config, JSON.stringify({ tcpAddress: `127.0.0.1:${19460 + index}`, udpAddress: `127.0.0.1:${19470 + index}`, broadcastAddress: `127.0.0.1:${19471 - index}`, searchTimeout: 1 }))
const child = spawn(path.join(root, 'bin', process.platform === 'win32' ? 'local-sender-test-server.exe' : 'local-sender-test-server'), [], {
 cwd: root, stdio: 'inherit', env: { ...process.env, LOCAL_SENDER_CONFIG: config, WAILS_SERVER_HOST: '127.0.0.1', WAILS_SERVER_PORT: String(19450 + index) },
})
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal))
child.on('exit', code => { rmSync(dir, { recursive: true, force: true }); process.exit(code ?? 0) })
