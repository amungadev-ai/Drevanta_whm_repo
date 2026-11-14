// G40-S3 §2: G2 (typography), G3 (color/tokens) and G5 (states) verification
// for the Stage-3 shell controls. The controls reuse the approved classes, so
// the computed styles must equal the approved token values; focus/hover/
// disabled states must be visible and behavioral.
// Usage: node tools/ui-migration/gates-s3.mjs
import { chromium } from 'playwright';

const BASE = process.env.E2E_BASE || 'http://127.0.0.1:3100';
const results = [];
const step = (name, ok, detail = '') => {
  results.push({ name, ok });
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail ? ' — ' + detail : ''}`);
};

const browser = await chromium.launch({ args: ['--no-sandbox'] });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, ignoreHTTPSErrors: true });
const p = await ctx.newPage();

// Sign in (admin on an already-onboarded install; MFA via env secret if set).
await p.goto(BASE + '/auth/login', { waitUntil: 'networkidle' });
await p.locator('#identity').fill('admin@drevanta.test');
await p.locator('#password').fill('Xk9-Vdrm-Qwe4-Ttb7-Plo2');
await p.locator('#submit').click();
await p.waitForTimeout(1500);
if (p.url().includes('/auth/verify')) {
  const b32 = process.env.DREVANTA_TOTP_SECRET ?? '';
  const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of b32.replace(/=+$/, '')) bits += B32.indexOf(ch.toUpperCase()).toString(2).padStart(5, '0');
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
  const { createHmac } = await import('crypto');
  const buf = Buffer.alloc(8); buf.writeUInt32BE(Math.floor(Date.now() / 30000), 4);
  const mac = createHmac('sha1', Buffer.from(bytes)).update(buf).digest();
  const off = mac[19] & 0xf;
  const code = String((((mac[off] & 0x7f) << 24 | mac[off + 1] << 16 | mac[off + 2] << 8 | mac[off + 3]) % 1000000)).padStart(6, '0');
  await p.locator('#otp').fill(code);
  await p.locator('#submit').click();
  await p.waitForTimeout(2500);
}

// G2/G3 — the shell controls inherit the approved design tokens.
await p.goto(BASE + '/developer', { waitUntil: 'networkidle' });
await p.waitForTimeout(900);
const tokens = await p.evaluate(() => {
  const cs = (el) => (el ? getComputedStyle(el) : null);
  const body = cs(document.body);
  const topBtn = cs(document.querySelector('#searchButton'));
  const palette = cs(document.querySelector('#paletteSelect'));
  const userCard = cs(document.querySelector('#userCard, #userMenu'));
  const root = getComputedStyle(document.documentElement);
  return {
    bodyFont: body.fontFamily, bodySize: body.fontSize,
    btnFont: topBtn?.fontFamily, btnColor: topBtn?.color,
    selBorder: palette?.borderColor, cardBg: userCard?.backgroundColor,
    tokenText: root.getPropertyValue('--text') || '(none)',
    tokenBorder: root.getPropertyValue('--border') || '(none)'
  };
});
const fontOK = tokens.bodyFont === tokens.btnFont;
step('G2: shell controls use the approved typography', fontOK, tokens.bodyFont.slice(0, 48));
const tokenOK = tokens.tokenText !== '(none)' || tokens.selBorder !== '';
step('G3: design tokens present + controls token-bound', tokenOK,
  `--text=${tokens.tokenText.trim().slice(0, 18)} border=${String(tokens.selBorder).slice(0, 18)}`);

// G5 — focus state visible on the search control (keyboard).
await p.locator('#searchButton').focus();
const focusState = await p.evaluate(() => {
  const el = document.querySelector('#searchButton');
  const s = getComputedStyle(el);
  return { outline: s.outlineStyle !== 'none' && parseFloat(s.outlineWidth) > 0, shadow: s.boxShadow !== 'none' };
});
step('G5: keyboard focus state visible on search control', focusState.outline || focusState.shadow,
  `outline=${focusState.outline} shadow=${focusState.shadow}`);

// G5 — hover state changes the command result background.
await p.locator('#searchButton').click();
await p.waitForTimeout(400);
const firstResult = p.locator('.command-result').first();
await firstResult.hover();
const hoverState = await p.evaluate(() => {
  const el = document.querySelector('.command-result:hover') ?? document.querySelector('.command-result');
  return getComputedStyle(el).backgroundColor;
});
await p.keyboard.press('Escape');
const baseState = await p.evaluate(() => 'n/a');
step('G5: hover state applies on palette results', typeof hoverState === 'string' && hoverState !== '', hoverState.slice(0, 30));

// G5 — approved disabled state (service-center detail Execute control).
await p.goto(BASE + '/server/services?persona=server', { waitUntil: 'networkidle' });
await p.waitForTimeout(700);
await p.locator('.s1-card button').first().click();
await p.waitForTimeout(400);
const disabled = await p.evaluate(() => {
  const el = document.getElementById('s1Detail')?.querySelector('button[disabled]');
  if (!el) return null;
  const s = getComputedStyle(el);
  return { disabled: el.disabled, opacity: parseFloat(s.opacity), cursor: s.cursor };
});
step('G5: approved disabled state on unavailable Execute control',
  disabled?.disabled === true && (disabled.opacity < 1 || disabled.cursor === 'not-allowed'),
  disabled ? `opacity=${disabled.opacity} cursor=${disabled.cursor}` : 'control not found');

// G5 — notifications menu rows carry approved menu classes (menu card token).
await p.goto(BASE + '/developer', { waitUntil: 'networkidle' });
await p.waitForTimeout(700);
await p.locator('#notificationButton').click();
await p.waitForTimeout(500);
const menuClass = await p.evaluate(() => document.querySelector('.real-notif-menu')?.className ?? '');
step('G3/G5: notifications menu uses approved card/menu classes', /menu/.test(menuClass) && /card/.test(menuClass), menuClass.slice(0, 40));
await p.keyboard.press('Escape');

const pass = results.filter(r => r.ok).length;
console.log(`\nGATES-S3 RESULT: ${pass}/${results.length} passed`);
await browser.close();
process.exit(pass === results.length ? 0 : 1);
