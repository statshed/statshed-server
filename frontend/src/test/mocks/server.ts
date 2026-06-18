/**
 * AIDEV-NOTE: MSW server setup for Node.js environment (Vitest)
 */

import { setupServer } from 'msw/node'
import { handlers } from './handlers'

export const server = setupServer(...handlers)
