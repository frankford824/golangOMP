// This startup-process regression uses Node's test runner and is intentionally excluded from Vitest discovery.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const auditScript = path.join(__dirname, 'large-surface-audit.mjs')

test('self-started load audit fails closed when LOAD_PORT is already occupied', async () => {
  const blocker = createServer((_request, response) => {
    response.writeHead(200, { 'content-type': 'text/plain' })
    response.end('unrelated server')
  })
  await listen(blocker, 0)
  const address = blocker.address()
  assert(address && typeof address === 'object')
  const port = address.port

  try {
    const result = await runAudit({ LOAD_PORT: String(port) })
    assert.notEqual(result.code, 0, result.output)
    assert.match(result.output, /Port \d+ is already in use/)
    assert.match(result.output, /exited before ready with code=1/)
    assert.doesNotMatch(result.output, /Large surface audit passed/)
  } finally {
    await close(blocker)
  }

  const probe = createServer()
  await listen(probe, port)
  await close(probe)
})

function runAudit(extraEnv) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [auditScript], {
      cwd: rootDir,
      env: {
        ...process.env,
        ...extraEnv,
        LOAD_BASE_URL: '',
      },
      stdio: ['ignore', 'pipe', 'pipe'],
    })
    let output = ''
    const timeout = setTimeout(() => {
      child.kill('SIGKILL')
      reject(new Error(`load audit startup failure regression timed out:\n${output}`))
    }, 20_000)
    child.stdout.on('data', (chunk) => {
      output += String(chunk)
    })
    child.stderr.on('data', (chunk) => {
      output += String(chunk)
    })
    child.once('error', (error) => {
      clearTimeout(timeout)
      reject(error)
    })
    child.once('exit', (code, signal) => {
      clearTimeout(timeout)
      resolve({ code, signal, output })
    })
  })
}

function listen(server, port) {
  return new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(port, '127.0.0.1', () => {
      server.off('error', reject)
      resolve()
    })
  })
}

function close(server) {
  return new Promise((resolve, reject) => {
    server.close((error) => {
      if (error) reject(error)
      else resolve()
    })
  })
}
