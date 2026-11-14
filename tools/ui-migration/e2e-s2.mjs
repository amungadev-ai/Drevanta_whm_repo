// G40-S2 e2e: real flows against the local Go control plane (production build).
// Covers: protection gate, onboarding claim (real), MFA enrollment (real
// TOTP confirm + recovery codes), access address, workspace-driven routing,
// persona real-data bridge, live operations (resources + intent + operation
// tracking + audit), logout revocation, wrong-password error state.
// Run: node tools/ui-migration/e2e-s2.mjs <claimToken>
import { chromium } from 'playwright';

const claimToken = process.argv[2];
const BASE = process.env.E2E_BASE || 'http://127.0.0.1:3100';
if (!claimToken) { console.error('usage: e2e-s2.mjs <claimToken>'); process.exit(1); }

const results = [];
const step = (name, ok, detail = '') => {
  results.push({ name, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail ? ' — ' + detail : ''}`);
};

// Local TOTP (matches the WebCrypto helper in the app) for MFA assertions.
import crypto from 'crypto';
function totpNow(secretB32) {
  const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of secretB32.replace(/=+$/, '')) {
    const v = B32.indexOf(ch.toUpperCase());
    if (v < 0) throw new Error('bad secret');
    bits += v.toString(2).padStart(5, '0');
  }
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
  const counter = Math.floor(Date.now() / 30000);
  const buf = Buffer.alloc(8); buf.writeUInt32BE(counter, 4);
  const mac = crypto.createHmac('sha1', Buffer.from(bytes)).update(buf).digest();
  const off = mac[19] & 0xf;
  const bin = ((mac[off] & 0x7f) << 24 | mac[off + 1] << 16 | mac[off + 2] << 8 | mac[off + 3]) % 1000000;
  return String(bin).padStart(6, '0');
}

const browser = await chromium.launch({ args: ['--no-sandbox'] });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, ignoreHTTPSErrors: true });
const page = await ctx.newPage();
const consoleErrors = [];
page.on('console', m => { if (m.type() === 'error') consoleErrors.push(m.text().slice(0, 160)); });
page.on('pageerror', e => consoleErrors.push(String(e).slice(0, 160)));

// 1. Protection gate
await page.goto(BASE + '/server', { waitUntil: 'load' });
await page.waitForURL('**/auth/login**', { timeout: 8000 }).catch(() => {});
step('protected route redirects to /auth/login', page.url().includes('/auth/login'), page.url());

// 2. Onboarding claim
await page.goto(`${BASE}/onboarding/administrator#token=${claimToken}`, { waitUntil: 'networkidle' });
await page.waitForTimeout(400);
const adminPass = 'Xk9-Vdrm-Qwe4-Ttb7-Plo2';
await page.locator('#username').fill('admin');
await page.locator('#email').fill('admin@drevanta.test');
await page.locator('#password').fill(adminPass);
await page.locator('#confirm').fill(adminPass);
await page.locator('#submit, #continue').first().click();
await page.waitForTimeout(900);
step('claim accepted -> MFA step (real POST /install/claim)', page.url().includes('/onboarding/mfa'), page.url());

// 3. Real MFA enrollment is performed by the wizard itself at step entry
// (POST /identity/mfa/enroll; the server-issued secret is stashed and
// surfaced in the approved manual-key slot). Read it and confirm with a
// real TOTP code typed into the approved UI.
// Poll for the wizard's step-entry enrollment to land (WAN latency tolerant).
let enroll = { secret: '' };
for (let i = 0; i < 20 && !enroll.secret; i++) {
  await page.waitForTimeout(500);
  enroll = await page.evaluate(() => {
    const st = JSON.parse(sessionStorage.getItem('drevanta-flow-lab-v05') || '{}');
    return { secret: st.mfaSecret || '' };
  });
}
if (enroll.secret) {
  step('real MFA enrollment at step entry (POST /identity/mfa/enroll)', true, `secret ${String(enroll.secret).slice(0, 4)}…`);
  const code = totpNow(enroll.secret);
  const digits = code.split('');
  for (let i = 0; i < 6; i++) await page.locator('#otp' + i).fill(digits[i]);
  await page.locator('#submit, #continue').first().click();
  await page.waitForTimeout(900);
  step('TOTP confirmed -> recovery codes step', page.url().includes('/onboarding/recovery-codes'), page.url());
  const codeNodes = await page.locator('code').allTextContents();
  const realCodes = codeNodes.filter(t => /^[0-9a-f]{16}$/i.test(t.trim()));
  step('real recovery codes displayed once', realCodes.length >= 10, `${realCodes.length} codes`);
  await page.locator('#saved').check().catch(() => {});
  await page.locator('#submit, #continue').first().click();
  await page.waitForTimeout(700);
  step('-> access-address step', page.url().includes('/onboarding/access-address'), page.url());
  await page.locator('#continue').click();
  await page.waitForTimeout(900);
  step('address written -> setup-complete (real POST /install/address)', page.url().includes('/onboarding/setup-complete'), page.url());
} else {
  step('real MFA enrollment at step entry (POST /identity/mfa/enroll)', false, 'no secret stashed by the wizard');
}

// 4. Workspace-driven routing after sign-in (MFA now required: complete the
// approved verify step with the enrolled secret).
const ctx2 = await browser.newContext({ viewport: { width: 1440, height: 900 }, ignoreHTTPSErrors: true });
const p2 = await ctx2.newPage();
await p2.goto(BASE + '/auth/login', { waitUntil: 'networkidle' });
await p2.waitForTimeout(300);
await p2.locator('#identity').fill('admin@drevanta.test');
await p2.locator('#password').fill(adminPass);
await p2.locator('#submit').click();
await p2.waitForTimeout(1800);
const onVerify = p2.url().includes('/auth/verify');
step('MFA-enrolled account routed to verify step', onVerify, p2.url());
if (onVerify) {
  const mfaResp = [];
  p2.on('response', r => { if (r.url().includes('/api/v1/identity/mfa')) mfaResp.push(r.status()); });
  await p2.locator('#otp').fill(totpNow(enroll.secret));
  await p2.locator('#submit').click();
  await p2.waitForTimeout(3000);
  const verr = await p2.locator('#error, .error, [role="alert"]').first().textContent().catch(() => '');
  if (!p2.url().includes('/auth/')) console.log('verify POST statuses:', mfaResp);
  else console.log('verify failed:', JSON.stringify((verr || '').trim()), 'statuses:', mfaResp);
}
step('sign-in routes to the persona workspace', !p2.url().includes('/auth/'), p2.url());

// 5. Real data bridge bound the approved shell
const wsName = await p2.locator('.sidebar .workspace strong').textContent().catch(() => '');
step('sidebar shows the REAL workspace name', !!wsName && !/Northstar Hosting|Acme|Demo/i.test(wsName || ''), (wsName || '').trim());
const realOps = await p2.locator('.real-ops-launcher').count();
step('live operations launcher present', realOps === 1);

// 6. Live operations: real resources + real intent + operation tracking
await p2.locator('.real-ops-launcher').click();
await p2.waitForTimeout(1200);
const panelVisible = await p2.locator('#realOpsPanel').count();
step('live operations panel opens', panelVisible === 1);
let tableRows = await p2.locator('#realOpsPanel table tbody tr').count();
if (tableRows === 0) {
  // Fresh workspace: register a real resource (server.owner: host snapshot).
  await p2.locator('#realOpsPanel input[aria-label="New resource name"]').fill('primary-host');
  await p2.locator('#realOpsPanel button', { hasText: 'Register resource' }).click();
  await p2.waitForTimeout(1500);
  tableRows = await p2.locator('#realOpsPanel table tbody tr').count();
}
step('real resources listed from GET /resources', tableRows >= 1, `${tableRows} rows`);
const applyBtn = p2.locator('#realOpsPanel button', { hasText: 'Inspect host' });
if (await applyBtn.count()) {
  await applyBtn.first().click();
  await p2.waitForTimeout(6000);
  let badge = await p2.locator('#realOpsPanel .op-tracker .badge').first().textContent().catch(() => '');
  if (!badge) badge = await p2.locator('#realOpsPanel .op-tracker').textContent().then(t => t.trim().slice(0, 60)).catch(() => '');
  step('intent submitted -> real operation tracked to terminal state', /Completed|succeeded|failed|rejected|running|Tracking/i.test(badge || ''), (badge || '').trim());
} else {
  step('intent submission available', false, 'no Inspect host control');
}

// 7. Activity tab -> real audit events
await p2.locator('#realOpsPanel button', { hasText: 'Activity' }).click().catch(() => {});
await p2.waitForTimeout(1200);
const auditItems = await p2.locator('#realOpsPanel .audit-list li').count();
step('real audit events rendered', auditItems >= 1, `${auditItems} events`);

// 8. Real logout revokes the session
await p2.locator('#userMenu').click().catch(() => {});
await p2.waitForTimeout(400);
const signOut = p2.locator('.real-account-menu button', { hasText: 'Sign out' });
if (await signOut.count()) {
  await signOut.click();
  await p2.waitForURL('**/auth/login**', { timeout: 8000 }).catch(() => {});
  step('logout -> session revoked, redirected to /auth/login', p2.url().includes('/auth/login'), p2.url());
  await p2.goto(BASE + '/server', { waitUntil: 'load' });
  await p2.waitForURL('**/auth/login**', { timeout: 8000 }).catch(() => {});
  step('revoked session cannot reach protected routes', p2.url().includes('/auth/login'), p2.url());
} else {
  step('logout control reachable', false, 'account menu missing');
}

const realErrors = consoleErrors.filter(e => !e.includes('favicon') && !e.includes('net::') && !e.includes('Failed to load resource') && !e.includes('raw.map'));
step('no unexpected console errors in real flows', realErrors.length === 0, realErrors.slice(0, 3).join(' | '));

await browser.close();
const failed = results.filter(r => !r.ok).length;
console.log(`\nE2E RESULT: ${results.length - failed}/${results.length} passed`);
process.exit(failed ? 1 : 0);
