/**
 * Load .env from action root and assess dir before any other module that reads process.env.
 * Import this first in run-local-assess so env is set before index.js loads.
 */
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import { config } from 'dotenv';

const __dirname = dirname(fileURLToPath(import.meta.url));
const actionRoot = resolve(__dirname, '../..');
config({ path: resolve(actionRoot, '.env'), override: true });
config({ path: resolve(__dirname, '../.env'), override: true });
