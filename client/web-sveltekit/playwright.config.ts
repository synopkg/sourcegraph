import type { PlaywrightTestConfig } from '@playwright/test'
import { devices } from '@playwright/test'

const PORT = process.env.PORT ? Number(process.env.PORT) : 4173

const config: PlaywrightTestConfig = {
    testMatch: 'src/**/*.spec.ts',
    // For local testing
    webServer: process.env.DISABLE_APP_ASSETS_MOCKING
        ? {
              command: 'pnpm build:preview && pnpm preview',
              port: PORT,
              reuseExistingServer: true,
              env: {
                  PLAYWRIGHT: '1',
              },
              timeout: 5 * 60_000,
          }
        : undefined,
    reporter: 'list',
    // note: if you proxy into a locally running vite preview, you may have to raise this to 60 seconds
    timeout: process.env.BAZEL ? 60_000 : 30_000,
    expect: {
        timeout: process.env.BAZEL ? 20_000 : 5_000,
    },
    use: {
        baseURL: `http://localhost:${PORT}`,
    },
    projects: [
        {
            name: 'chromium',
            use: {
                ...devices['Desktop Chrome'],
                launchOptions: {
                    // When in CI, use bazel packaged linux chromium
                    executablePath: process.env.CHROMIUM_BIN,
                },
            },
        },
    ],
}

export default config
