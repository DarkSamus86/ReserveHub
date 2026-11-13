import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '');
  const gateway = env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:8080';

  return {
    plugins: [react()],
    server: {
      host: '0.0.0.0',
      port: 5173,
      strictPort: true,
      proxy: {
        '/v1': { target: gateway, changeOrigin: true },
        '/health': { target: gateway, changeOrigin: true },
        '/openapi.json': { target: gateway, changeOrigin: true },
        '/swagger': { target: gateway, changeOrigin: true },
      },
    },
  };
});
