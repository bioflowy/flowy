import { builtinModules } from "module";
import { defineConfig } from 'vite'
import devServer from '@hono/vite-dev-server'

export default defineConfig({
    build: {
        target: "esnext",
        minify: false,
        emptyOutDir: false,
        copyPublicDir: false,
        rollupOptions: {
            input: ['./src/index.ts'],
            output: {
                entryFileNames: 'index.js',
              },
              external: [
                'node:path',
                "node:stream",
                "node:fs/promises",
                'node:fs',
                'node:os',
                'node:crypto',
                'node:child_process',
                'node:url',
                ...builtinModules],
        },
        
      },
      plugins: [
        devServer({
          entry: "./src/app.ts",
        })
      ],
});