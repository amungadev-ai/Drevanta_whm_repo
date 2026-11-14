// G40-S3 §2/G9: accessibility re-run over the production routes with the
// Stage-3 shell controls, against the same standards as the Stage-2 report
// (WCAG 2.1 AA ruleset via axe-core). Writes docs/ui/a11y-report.json.
// Usage: node tools/ui-migration/a11y-s3.mjs
import fs from 'fs';
import path from 'path';
import { createRequire } from 'module';
import { chromium } from 'playwright';

const require2 = createRequire(import.meta.url);
const ROOT = path.resolve(path.dirname(new URL(import.meta.url).pathname), '../..');
const axeSource = fs.readFileSync(path.join(ROOT, 'frontend/node_modules/axe-core/axe.min.js'), 'utf8');
const BASE = 'http://127.0.0.1:3100';
const ADMIN = { email: 'admin@drevanta.test', password: process.env.ADMIN_PASS || 'Xk9-Vdrm-Qwe4-Ttb7-Plo2' };

const ROUTES = ['/', '/server', '/website', '/developer', '/server/services?persona=server', '/audit'];

const browser = await chromium.launch({ args: ['--no-sandbox'] });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, ignoreHTTPSErrors: true });
const page = await ctx.newPage();

// Real session (fresh install owner or existing admin).
await page.goto(BASE + '/auth/login', { waitUntil: 'networkidle' });
await page.locator('#identity').fill(ADMIN.email);
await page.locator('#password').fill(ADMIN.password);
await page.locator('#submit').click();
await page.waitForTimeout(1500);
if (page.url().includes('/auth/verify')) {
  const b32 = process.env.DREVANTA_TOTP_SECRET;
  if (!b32) {
    console.error('admin requires MFA verify: set DREVANTA_TOTP_SECRET');
    await browser.close();
    process.exit(2);
  }
  const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of b32.replace(/=+$/, '')) bits += B32.indexOf(ch.toUpperCase()).toString(2).padStart(5, '0');
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
  const counter = Math.floor(Date.now() / 30000);
  const { createHmac } = await import('crypto');
  const buf = Buffer.alloc(8); buf.writeUInt32BE(counter, 4);
  const mac = createHmac('sha1', Buffer.from(bytes)).update(buf).digest();
  const off = mac[19] & 0xf;
  const code = String((((mac[off] & 0x7f) << 24 | mac[off + 1] << 16 | mac[off + 2] << 8 | mac[off + 3]) % 1000000)).padStart(6, '0');
  await page.locator('#otp').fill(code);
  await page.locator('#submit').click();
  await page.waitForTimeout(2500);
  if (page.url().includes('/auth/')) {
    console.error('MFA verify failed');
    await browser.close();
    process.exit(2);
  }
}

const report = { generated: new Date().toISOString(), standard: 'WCAG 2.1 AA (axe-core)', routes: [] };

for (const route of ROUTES) {
  await page.goto(BASE + route, { waitUntil: 'networkidle' });
  await page.waitForTimeout(900);
  await page.addScriptTag({ content: axeSource });
  const res = await page.evaluate(() => window.axe.run(document, { resultTypes: ['violations'] }));
  const violations = res.violations.map((v) => ({
    id: v.id,
    impact: v.impact,
    help: v.help,
    nodes: v.nodes.length
  }));
  report.routes.push({ route, viewport: '1440x900', violations });
  console.log(`${route}: ${violations.length} violation kinds`);
}

// Mobile drawer open state (approved mobile navigation) on /developer.
await page.setViewportSize({ width: 375, height: 800 });
await page.goto(BASE + '/developer', { waitUntil: 'networkidle' });
await page.waitForTimeout(700);
await page.locator('#menuButton').click();
await page.waitForTimeout(400);
await page.addScriptTag({ content: axeSource });
const mres = await page.evaluate(() => window.axe.run(document, { resultTypes: ['violations'] }));
report.routes.push({
  route: '/developer (mobile drawer open)',
  viewport: '375x800',
  violations: mres.violations.map((v) => ({ id: v.id, impact: v.impact, help: v.help, nodes: v.nodes.length }))
});
console.log(`/developer (mobile drawer open): ${mres.violations.length} violation kinds`);

const out = path.join(ROOT, 'docs/ui/a11y-report.json');
fs.writeFileSync(out, JSON.stringify(report, null, 2) + '\n');
console.log('written', out);
await browser.close();
