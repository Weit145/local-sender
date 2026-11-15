import { spawnSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const platform = process.env.GOOS || ({ win32: 'windows', darwin: 'darwin', linux: 'linux' }[process.platform])
const arch = process.env.GOARCH || ({ x64: 'amd64', arm64: 'arm64' }[process.arch])
if (!['linux', 'windows', 'darwin'].includes(platform) || !arch) throw new Error('Unsupported platform')
const bin = path.join(root, 'bin')
mkdirSync(bin, { recursive: true })
let output = path.join(bin, platform === 'windows' ? 'local-sender.exe' : 'local-sender')
if (platform === 'darwin') {
 const contents = path.join(bin, 'Local Sender.app', 'Contents')
 mkdirSync(path.join(contents, 'MacOS'), { recursive: true })
 mkdirSync(path.join(contents, 'Resources'), { recursive: true })
 writeFileSync(path.join(contents, 'Info.plist'), `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>Local Sender</string>
<key>CFBundleDisplayName</key><string>Local Sender</string>
<key>CFBundleExecutable</key><string>local-sender</string>
<key>CFBundleIdentifier</key><string>io.localsender.desktop</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>0.1.0</string>
<key>CFBundleVersion</key><string>1</string>
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>NSHighResolutionCapable</key><true/>
<key>NSLocalNetworkUsageDescription</key><string>Local Sender discovers nearby devices and exchanges messages on your local network.</string>
</dict></plist>`)
 output = path.join(contents, 'MacOS', 'local-sender')
}
const flags = platform === 'windows' ? '-s -w -H windowsgui' : '-s -w'
const result = spawnSync('go', ['build', '-trimpath', '-tags', 'production', '-ldflags', flags, '-o', output, './cmd/desktop'], { cwd: root, stdio: 'inherit' })
if (result.status !== 0) process.exit(result.status ?? 1)
if (platform === 'darwin' && process.platform === 'darwin') {
 const signed = spawnSync('codesign', ['--force', '--deep', '--sign', '-', path.join(bin, 'Local Sender.app')], { stdio: 'inherit' })
 if (signed.status !== 0) process.exit(signed.status ?? 1)
}
console.log(`Built ${platform}/${arch}: ${output}`)
