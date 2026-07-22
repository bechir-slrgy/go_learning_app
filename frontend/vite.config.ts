import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // The Go API has no CORS middleware, so a browser on :5173 calling :8090
    // directly would be blocked. This proxy makes the browser think the API is
    // same-origin: the request goes to /api/... on :5173, and Vite forwards it
    // to :8090 server-side, where the same-origin policy does not apply.
    //
    // The alternative is adding CORS middleware to the Go router. The proxy is
    // preferred in development because the backend stays untouched.
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        changeOrigin: true,
      },
    },
  },
})
