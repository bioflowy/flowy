import { defineConfig } from 'vite'
import devServer from '@hono/vite-dev-server'
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
            format: 'esm',
            external: [...builtinModules],
        },
        
      },
      plugins: [
        devServer({
          entry: "./src/app.ts",
        }),
      ],
    });