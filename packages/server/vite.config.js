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
              format: 'es',
                entryFileNames: 'index.js',
              },
              external: [
                'uuid',
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
      resolve: {
        mainFields: ['module']
      },
      plugins: [
        devServer({
          entry: "./src/app.ts",
        })
      ],
});