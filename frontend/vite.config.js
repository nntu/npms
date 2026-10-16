import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
var __dirname = path.dirname(fileURLToPath(import.meta.url));
var versionFilePath = path.resolve(__dirname, '../VERSION');
var version = fs.existsSync(versionFilePath)
    ? fs.readFileSync(versionFilePath, 'utf-8').trim()
    : process.env.VITE_APP_VERSION || '1.0.0';
process.env.VITE_APP_VERSION = version;
if (!process.env.VITE_APP_BUILD_TIME) {
    process.env.VITE_APP_BUILD_TIME = new Date().toISOString();
}
export default defineConfig({
    plugins: [react(), tailwindcss()],
    server: { port: 5173 },
});
