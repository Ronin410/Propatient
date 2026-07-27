/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      // injectManifest (no generateSW) porque src/sw.ts necesita
      // listeners propios de "push"/"notificationclick" — generateSW
      // solo genera cacheo declarativo de Workbox, no permite lógica
      // custom dentro del service worker.
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'sw.ts',
      injectRegister: 'auto',
      registerType: 'autoUpdate',
      manifest: {
        name: 'ProPatient Clinic',
        short_name: 'PP Clinic',
        description: 'Gestiona tu consultorio digital en un solo lugar.',
        theme_color: '#005073',
        background_color: '#005073',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: '/pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: '/pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: '/maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      // No precachear /uploads (documentos/recetas/avatares del backend)
      // ni las rutas de /api — solo el shell estático de la app.
      injectManifest: {
        globPatterns: ['**/*.{js,css,html,svg,png,ico}'],
      },
      devOptions: {
        // Deshabilitado en dev: con el SW activo en npm run dev, Vite HMR
        // y el service worker compiten por servir los mismos archivos —
        // solo se prueba instalando el build de producción.
        enabled: false,
      },
    }),
  ],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
  },
  server: {
    // Escucha en todas las interfaces de red del contenedor
    host: true, 
    // Asegura que siempre use el puerto 5173
    port: 5173,
    watch: {
      // ESTA ES LA PARTE MÁS IMPORTANTE PARA DOCKER EN WINDOWS:
      // Obliga a Vite a revisar los archivos por intervalos (polling)
      // ya que los eventos del sistema de archivos a veces no cruzan de Windows a Linux.
      usePolling: true,
    },
    // Opcional: si tienes problemas de conexión con el navegador
    strictPort: true,
  },
})
