const { app, BrowserWindow, dialog } = require('electron')
const { spawn, execSync } = require('child_process')
const path = require('path')
const http = require('http')
const fs = require('fs')

const PORT = process.env.PORT || '8787'
const OLLAMA_HOST = '127.0.0.1:11434'
const IS_DEV = process.env.ELECTRON_DEV === '1'

let serverProcess = null
let ollamaProcess = null

// ── helpers ───────────────────────────────────────────────────────────────────

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms))
}

function findOllama() {
  const candidates = [
    '/opt/homebrew/bin/ollama',
    '/usr/local/bin/ollama',
    '/Applications/Ollama.app/Contents/MacOS/Ollama',
  ]
  for (const p of candidates) {
    if (fs.existsSync(p)) return p
  }
  try {
    return execSync('which ollama', { encoding: 'utf8' }).trim()
  } catch {
    return null
  }
}

// ── Ollama ────────────────────────────────────────────────────────────────────

function isOllamaRunning() {
  return new Promise(resolve => {
    const req = http.get(`http://${OLLAMA_HOST}/api/version`, res => {
      res.resume()
      resolve(res.statusCode === 200)
    })
    req.on('error', () => resolve(false))
    req.setTimeout(1000, () => { req.destroy(); resolve(false) })
  })
}

async function ensureOllama() {
  if (await isOllamaRunning()) return

  const bin = findOllama()
  if (!bin) {
    console.log('ollama not found — server will use configured fallback (ANTHROPIC_API_KEY etc.)')
    return
  }

  console.log(`starting ollama serve (${bin})`)
  ollamaProcess = spawn(bin, ['serve'], {
    env: { ...process.env, OLLAMA_HOST },
    stdio: 'ignore',
    detached: false,
  })
  ollamaProcess.on('error', err => console.error('ollama error:', err))

  // Poll up to 3 s for ollama to accept connections
  for (let i = 0; i < 6; i++) {
    await sleep(500)
    if (await isOllamaRunning()) return
  }
  console.warn('ollama did not respond in time — continuing anyway')
}

// ── Go server ─────────────────────────────────────────────────────────────────

async function startGoServer() {
  const dataDir = path.join(app.getPath('userData'), 'data')
  fs.mkdirSync(dataDir, { recursive: true })

  const model = process.env.TUTOR_LOCAL_LLM_MODEL || 'gemma3:4b'
  const env = {
    ...process.env,
    PORT,
    TUTOR_DATA_DIR: dataDir,
    TUTOR_LOCAL_LLM_URL: `http://${OLLAMA_HOST}/v1`,
    TUTOR_LOCAL_LLM_MODEL: model,
  }

  if (IS_DEV) {
    const cwd = path.join(__dirname, '..', 'server')
    serverProcess = spawn('go', ['run', '.'], { cwd, env, stdio: 'inherit' })
  } else {
    const bin = path.join(process.resourcesPath, 'tutor-server')
    serverProcess = spawn(bin, [], { env, stdio: 'inherit' })
  }

  serverProcess.on('error', err =>
    dialog.showErrorBox('Server error', `Failed to start tutor server:\n${err.message}`)
  )
}

function waitForServer(maxMs = 20_000) {
  const start = Date.now()
  return new Promise((resolve, reject) => {
    function attempt() {
      const req = http.get(`http://127.0.0.1:${PORT}/api/health`, res => {
        res.resume()
        if (res.statusCode === 200) return resolve()
        retry()
      })
      req.on('error', retry)
      req.setTimeout(1000, () => { req.destroy(); retry() })
    }
    function retry() {
      if (Date.now() - start > maxMs) {
        return reject(new Error('Tutor server did not start within 20 s'))
      }
      setTimeout(attempt, 400)
    }
    attempt()
  })
}

// ── Window ────────────────────────────────────────────────────────────────────

async function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 860,
    title: 'Tutor',
    webPreferences: { contextIsolation: true },
  })

  if (IS_DEV) {
    await win.loadURL('http://localhost:5173')
    win.webContents.openDevTools({ mode: 'detach' })
  } else {
    const indexHtml = path.join(app.getAppPath(), 'web', 'dist', 'index.html')
    await win.loadFile(indexHtml)
  }
}

// ── App lifecycle ─────────────────────────────────────────────────────────────

app.whenReady().then(async () => {
  try {
    await ensureOllama()
    await startGoServer()
    await waitForServer()
    await createWindow()
  } catch (err) {
    dialog.showErrorBox('Startup failed', String(err))
    app.quit()
  }
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})

app.on('will-quit', () => {
  serverProcess?.kill()
  ollamaProcess?.kill()
})
