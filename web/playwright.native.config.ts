import {defineConfig,devices} from '@playwright/test';
if(!process.env.LEFORGE_NATIVE_E2E_DATA)throw new Error('Use scripts/test-native-browser.sh for a disposable native profile');
export default defineConfig({
 testDir:'./e2e-native',outputDir:'./test-results-native',fullyParallel:false,workers:1,retries:0,timeout:60000,expect:{timeout:15000},
 reporter:[['list']],use:{baseURL:'http://127.0.0.1:18090',trace:'retain-on-failure'},
 projects:[{name:'native-chromium',use:{...devices['Desktop Chrome']}}],
 webServer:{
  command:'pnpm run build && (cd ../server && go build -o bin/forged-native-e2e ./cmd/forged) && ../server/bin/forged-native-e2e --init-local --data-dir "$LEFORGE_NATIVE_E2E_DATA/state" --web-dir "$PWD/dist" --port 18090 && exec ../server/bin/forged-native-e2e --local --data-dir "$LEFORGE_NATIVE_E2E_DATA/state"',
  url:'http://127.0.0.1:18090/healthz',reuseExistingServer:false,timeout:120000,gracefulShutdown:{signal:'SIGTERM',timeout:15000},
 },
});
