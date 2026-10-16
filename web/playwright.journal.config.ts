import { defineConfig } from '@playwright/test';
import base from './playwright.config';

if (!process.env.LEFORGE_WORKSPACE_JOURNAL) throw new Error('Run make test-workspace-journal to capture a real kernel journal first.');
export default defineConfig({ ...base, testDir: './e2e-journal' });
