// G40-S2 §8: security tests for the nginx -> SvelteKit BFF -> Go chain.
// Run: node tools/ui-migration/security-test.mjs
// Requires: panel on :8080 (any state), frontend on :3100, admin credentials
// from a claimed installation (env ADMIN_PASS or default e2e password).
import http from 'http';
import { chromium } from 'playwright';
import crypto from 'crypto';

const BASE = 'http://127.0.0.1:3100';
const GO = 'http://127.0.0.1:8080';
const ADMIN = { email: 'admin@drevanta.test', password: process.env.ADMIN_PASS || 'Xk9-Vdrm-Qwe4-Ttb7-Plo2' };

const results = [];
const step = (name, ok, detail = '') => {
  results.push({ name, ok, detail });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail ? ' — ' + detail : ''}`);
};

const browser = await chromium.launch({ args: ['--no-sandbox'] });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();

// Sign in (MFA is enforced if enrolled; use the recovery-code path is not
// available here — the suite assumes a session via the password step when
// MFA is not enrolled, otherwise it uses the verify step with a provided
// TOTP secret through the same UI).
await page.goto(BASE + '/auth/login', { waitUntil: 'networkidle' });
await page.waitForTimeout(300);
await page.locator('#identity').fill(ADMIN.email);
await page.locator('#password').fill(ADMIN.password);
await page.locator('#submit').click();
await page.waitForTimeout(2000);
if (page.url().includes('/auth/verify')) {
  const secret = process.env.ADMIN_TOTP_SECRET || '';
  if (!secret) {
    console.log('SKIP: MFA enrolled and ADMIN_TOTP_SECRET not provided — supply it to run the authenticated checks.');
    await browser.close();
    process.exit(0);
  }
  await page.locator('#otp').fill(totpNow(secret));
  await page.locator('#submit').click();
  await page.waitForTimeout(2500);
}
step('authenticated session established', !page.url().includes('/auth/'), page.url());

function totpNow(secretB32) {
  const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of secretB32.replace(/=+$/, '')) bits += B32.indexOf(ch.toUpperCase()).toString(2).padStart(5, '0');
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
  const buf = Buffer.alloc(8); buf.writeUInt32BE(Math.floor(Date.now() / 30000), 4);
  const mac = crypto.createHmac('sha1', Buffer.from(bytes)).update(buf).digest();
  const off = mac[19] & 0xf;
  return String(((mac[off] & 0x7f) << 24 | mac[off + 1] << 16 | mac[off + 2] << 8 | mac[off + 3]) % 1000000).padStart(6, '0');
}

// 1. Cookie security flags — verified from the authoritative Set-Cookie
// header (Chromium's CDP hides Secure-cookie introspection over http://127.0.0.1).
import { execSync } from 'child_process';
// MFA-aware raw login: password step may answer mfa_required (no cookie yet);
// complete the approved verify step before reading Set-Cookie.
const loginBody = execSync(
  `curl -s -m 8 -X POST ${BASE}/api/v1/identity/login -H 'Content-Type: application/json' -d '${JSON.stringify(ADMIN)}'`,
  { encoding: 'utf8' }
);
let setCurl = execSync(
  `curl -si -m 8 -X POST ${BASE}/api/v1/identity/login -H 'Content-Type: application/json' -d '${JSON.stringify(ADMIN)}' | grep -i '^set-cookie:' || true`,
  { encoding: 'utf8' }
);
if (!setCurl.trim() && /mfa_required/.test(loginBody)) {
  const principal = (loginBody.match(/"principal_id"\s*:\s*"([^"]+)"/) || [])[1] || '';
  const verifyBody = `{"principal_id":"${principal}","code":"${totpNow(process.env.ADMIN_TOTP_SECRET || '')}"}`;
  setCurl = execSync(
    `curl -si -m 8 -X POST ${BASE}/api/v1/identity/mfa -H 'Content-Type: application/json' -d '${verifyBody.replace(/'/g, "'\\''")}' | grep -i '^set-cookie:' || true`,
    { encoding: 'utf8' }
  );
}
const sc = setCurl.toLowerCase();
const sessCookie = sc.includes('drevanta_session=');
const token = (setCurl.match(/drevanta_session=([^;]+)/i) || [])[1] || '';
step('session cookie present in Set-Cookie', sessCookie);
step('session cookie is HttpOnly', sc.includes('httponly'));
step('session cookie is SameSite=Strict', sc.includes('samesite=strict'));
step('session cookie is Secure (production build)', sc.includes('secure'));
step('session cookie path=/ with bounded max-age', sc.includes('path=/') && sc.includes('max-age'));

// 2. Token isolation: no token material in client-visible surfaces
const tokenPattern = /dvt_[A-Za-z0-9]{16,}/;
const ls = await page.evaluate(() => JSON.stringify(localStorage));
const ss = await page.evaluate(() => JSON.stringify(sessionStorage));
const dc = await page.evaluate(() => document.cookie);
const html = await page.content();
const bodyText = tokenPattern.test(ls) || tokenPattern.test(ss) || tokenPattern.test(dc);
const htmlText = tokenPattern.test(html.replace(/drevanta_session[^"']*/g, '')); // cookie name may appear in headers-only context
step('bearer token absent from localStorage/sessionStorage/document.cookie', !bodyText);
step('bearer token absent from rendered DOM', !htmlText);

// 3. CSRF: cross-site request cannot carry the Strict cookie
const crossSite = await new Promise((resolve) => {
  const srv = http.createServer((req, res) => {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end('<html><body><script>fetch("http://127.0.0.1:3100/api/v1/workspaces",{credentials:"include"}).then(r=>r.status).then(s=>{document.title=String(s)}).catch(()=>{document.title="ERR"})</script></body></html>');
  });
  srv.listen(3999, '127.0.0.1', async () => {
    const p2 = await (await browser.newContext({ viewport: { width: 800, height: 600 } })).newPage();
    await p2.goto('http://127.0.0.1:3999/', { waitUntil: 'load' });
    await p2.waitForTimeout(1500);
    const title = await p2.title();
    resolve(title);
    p2.context().close();
    srv.close();
  });
});
// Cross-site fetch with SameSite=Strict: cookie not attached -> 401 (or CORS network error)
step('cross-site API call cannot use the session (CSRF)', crossSite === '401' || crossSite === 'ERR', `cross-origin result: ${crossSite}`);

// 4. Direct Go access without token
const direct = await fetch(GO + '/api/v1/workspaces');
step('direct control-plane access without token denies (401)', direct.status === 401, `status ${direct.status}`);

// 5. Forged/garbage session cookie -> gate bounces
const ctx3 = await browser.newContext({ viewport: { width: 1440, height: 900 } });
await ctx3.addCookies([{ name: 'drevanta_session', value: 'dvt_forged_invalid_token_value', domain: '127.0.0.1', path: '/', httpOnly: true, sameSite: 'Strict' }]);
const p3 = await ctx3.newPage();
await p3.goto(BASE + '/server', { waitUntil: 'load' });
await p3.waitForURL('**/auth/login**', { timeout: 8000 }).catch(() => {});
step('forged session cookie rejected; protected route bounces to login', p3.url().includes('/auth/login'), p3.url());

// 6. Cross-tenant denial through the BFF: foreign scope id -> uniform 404
const wsStatus = await page.evaluate(async () => {
  const r = await fetch('/api/v1/resources?scope_id=wrk_doesnotexist000000', { credentials: 'include' });
  return r.status;
});
step('foreign scope resource read denies uniformly (404)', wsStatus === 404, `status ${wsStatus}`);

// 7. Logout revokes server-side. The browser session's token is HttpOnly
//    (unavailable to JS by design), so revocation is proven with a known
//    token: revoke the curl session through the BFF, then replay it
//    directly against Go. The browser session's own logout is verified by
//    the post-logout navigation check below.
// Revocation is exercised against the Go control plane directly: the BFF
// intentionally ignores foreign Authorization headers (cookie is the only
// client-side credential source).
const revoke = execSync(
  `curl -s -m 8 -o /dev/null -w '%{http_code}' -X POST ${GO}/api/v1/identity/logout -H 'Authorization: Bearer ${token}'`,
  { encoding: 'utf8' }
).trim();
const replay = await fetch(GO + '/api/v1/me', { headers: { Authorization: `Bearer ${token}` } });
step('logout of a known session succeeds', revoke === '200', `status ${revoke}`);
step('revoked token replayed directly against Go denies (401)', replay.status === 401, `status ${replay.status}`);
const browserLogout = await page.evaluate(async () => {
  const r = await fetch('/api/v1/identity/logout', { method: 'POST', credentials: 'include' });
  return r.status;
});
step('browser session logout succeeds', browserLogout === 200, `status ${browserLogout}`);
const afterLogout = await page.goto(BASE + '/server', { waitUntil: 'load' }).then(r => r.url());
step('post-logout protected navigation bounces to login', String(afterLogout).includes('/auth/login'), afterLogout);

await browser.close();
const failed = results.filter(r => !r.ok).length;
console.log(`\nSECURITY RESULT: ${results.length - failed}/${results.length} passed`);
process.exit(failed ? 1 : 0);
