import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    // Vite's default "localhost" binds IPv6 only on macOS, which IPv4-first
    // browsers cannot reach. Pin the IPv4 loopback instead.
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/v1': 'http://127.0.0.1:8080',
      // Regex, not a prefix: a plain '/s' key also captures '/src/*' and would
      // proxy the app's own source files to the API.
      '^/s/': 'http://127.0.0.1:8080',
      '/healthz': 'http://127.0.0.1:8080',
    },
  },
})
