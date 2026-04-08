import * as matchers from '@testing-library/jest-dom/matchers'
import { cleanup } from '@testing-library/react'
import { afterEach, expect, vi } from 'vitest'

expect.extend(matchers)

// Polyfill ResizeObserver for jsdom — required by Radix UI components that use
// @radix-ui/react-use-size (Select, Tooltip, etc.).
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

afterEach(() => {
  cleanup()
  sessionStorage.clear()
})
