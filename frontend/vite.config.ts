import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const versionFilePath = path.resolve(__dirname, '../VERSION')
const version = fs.existsSync(versionFilePath)
  ? fs.readFileSync(versionFilePath, 'utf-8').trim()
  : process.env.VITE_APP_VERSION || '1.0.0'

process.env.VITE_APP_VERSION = version
if (!process.env.VITE_APP_BUILD_TIME) {
  process.env.VITE_APP_BUILD_TIME = new Date().toISOString()
}

export default defineConfig({
  plugins: [react()],
  server: { port: 5173 },
})
