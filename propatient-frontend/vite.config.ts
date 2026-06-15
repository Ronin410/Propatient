import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
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
