import { chromium } from 'playwright';
const BASE = 'http://127.0.0.1:3100';
const browser = await chromium.launch({ args: ['--no-sandbox'] });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, ignoreHTTPSErrors: true });
const p = await ctx.newPage();
await p.goto(BASE + '/auth/login', { waitUntil: 'networkidle' });
await p.locator('#identity').fill('admin@drevanta.test');
await p.locator('#password').fill('Xk9-Vdrm-Qwe4-Ttb7-Plo2');
await p.locator('#submit').click();
await p.waitForTimeout(1800);
if (p.url().includes('/auth/verify')) {
  const sec = process.env.TOTP || '';
  const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of sec.replace(/=+$/, '')) bits += B32.indexOf(ch.toUpperCase()).toString(2).padStart(5, '0');
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2));
  const crypto = await import('crypto');
  const buf = Buffer.alloc(8); buf.writeUInt32BE(Math.floor(Date.now() / 30000), 4);
  const mac = crypto.createHmac('sha1', Buffer.from(bytes)).update(buf).digest();
  const off = mac[19] & 0xf;
  const code = String((((mac[off] & 0x7f) << 24 | mac[off + 1] << 16 | mac[off + 2] << 8 | mac[off + 3]) % 1000000)).padStart(6, '0');
  await p.locator('#otp').fill(code);
  await p.locator('#submit').click();
  await p.waitForTimeout(2200);
}
for (const route of ['/', '/server', '/website', '/developer']) {
  await p.goto(BASE + route, { waitUntil: 'networkidle' });
  await p.waitForTimeout(800);
  const nav = await p.evaluate(() => {
    const el = document.getElementById('nav');
    if (!el) return 'NO #nav';
    const btns = Array.from(el.querySelectorAll('button')).slice(0, 4);
    return { count: el.querySelectorAll('button').length, all: Array.from(el.querySelectorAll('button')).map(b => (b.getAttribute('data-action') || b.getAttribute('data-h-action') || b.getAttribute('data-p-action') || '').replace('goto:','')).filter(Boolean),
    first: btns.map(b => ({ txt: b.textContent.trim().slice(0, 24), attrs: b.getAttributeNames().filter(a => a.startsWith('data')).map(a => a + '=' + (b.getAttribute(a) || '')).join(',') })) };
  });
  let sect = null;
  if (route === '/server') {
    await p.locator('#nav button', { hasText: 'Monitoring' }).first().click();
    await p.waitForTimeout(1700);
    await p.locator('#nav button', { hasText: 'Audit' }).first().click();
    await p.waitForTimeout(1700);
    sect = await p.evaluate(() => ({ h1: document.querySelector('#page h1')?.textContent ?? '', marker: document.getElementById('page')?.getAttribute('data-real-section') ?? '', t: document.getElementById('page')?.textContent?.slice(0, 150) ?? '' }));
  } else { sect = { nav: (nav.all || []).slice(0, 4) }; }
  if (route === '/server') {
    await p.locator('.real-ops-launcher').click().catch(() => {});
    await p.waitForTimeout(1500);
    const ops = await p.evaluate(() => ({
      active: document.querySelector('.sidebar .workspace strong')?.textContent ?? '',
      rows: document.querySelectorAll('#realOpsPanel table tbody tr').length,
      err: document.querySelector('#realOpsPanel')?.textContent?.match(/not permitted|unavailable|Registration[^.]*/)?.[0] ?? '',
      head: document.querySelector('#realOpsPanel')?.textContent?.slice(0, 120) ?? ''
    }));
    console.log('OPS:', JSON.stringify(ops));
    // Register a resource through the panel exactly as the suite does.
    await p.locator('#realOpsPanel input[aria-label="New resource name"]').fill('probe-host-3');
    // EXACTLY the suite path: fill only, rely on the default type.
    const selInfo = await p.evaluate(() => {
      const sel = document.querySelector('#realOpsPanel select[aria-label="Resource type"]');
      return sel ? { value: sel.value, options: Array.from(sel.options).length } : { missing: true };
    });
    console.log('SEL:', JSON.stringify(selInfo));
    await p.locator('#realOpsPanel button', { hasText: 'Register resource' }).click();
    await p.waitForTimeout(1800);
    const after = await p.evaluate(() => ({
      rows: document.querySelectorAll('#realOpsPanel table tbody tr').length,
      err: document.querySelector('#realOpsPanel')?.textContent?.match(/not permitted|unavailable|Registration unavailable[^.]*/)?.[0] ?? ''
    }));
    console.log('OPS-after:', JSON.stringify(after));
  }
  console.log(route, JSON.stringify(sect));
}
await browser.close();
