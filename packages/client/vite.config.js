import { defineConfig } from 'vite'
import { builtinModules } from 'module';

export default defineConfig({
    build: {
        minify: true,
        emptyOutDir: false,
        copyPublicDir: false,
        rollupOptions: {
            input: ['./src/index.ts'],
            output: {
                entryFileNames: 'index.js',
              },
            external: [...builtinModules],
        },
        
      },
    });