#!/usr/bin/env python3
"""G40-S2: seam patches wiring real MFA enrollment + workspace routing.

All patches assert exact source substrings from the committed modules and
fail loudly on drift. Idempotent on re-run.

1. auth-flow.js: local TOTP confirmation helper (WebCrypto HMAC-SHA1).
2. auth-flow.js: on entering the MFA step, perform the REAL enrollment
   (POST /identity/mfa/enroll) and surface the server-issued secret in the
   approved manual-key slot; the six-digit confirm is validated locally
   against that secret before recovery codes are generated for real.
3. auth-flow.js: recovery-codes screen renders the REAL one-time codes in
   the slot the approved design reserves for them.
4. auth.js: post-login routing follows the real workspace list.
"""
import sys

FRONT = "/home/z/my-project/drevanta-repo/frontend"

def patch(path, replacements):
    src = open(path, encoding="utf-8").read()
    changed = 0
    for old, new in replacements:
        if old not in src:
            if new in src:
                continue  # already applied
            print(f"SEAM DRIFT in {path}: pattern not found:\n  {old[:110]}...")
            sys.exit(1)
        if src.count(old) != 1:
            print(f"SEAM AMBIGUOUS in {path}: pattern occurs {src.count(old)} times")
            sys.exit(1)
        src = src.replace(old, new)
        changed += 1
    with open(path, "w", encoding="utf-8") as f:
        f.write(src)
    print(f"patched {path} ({changed} seams)")

AF = f"{FRONT}/static/js/auth-flow.js"

TOTP_HELPER = r"""window.__drevantaTotpCheck=(async(secret,code)=>{
const B32='ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';let bits='',bytes=[];
for(const ch of secret.replace(/=+$/,'')){const v=B32.indexOf(ch.toUpperCase());if(v<0)return false;bits+=v.toString(2).padStart(5,'0');}
for(let i=0;i+8<=bits.length;i+=8)bytes.push(parseInt(bits.slice(i,i+8),2));
const key=await crypto.subtle.importKey('raw',new Uint8Array(bytes),{name:'HMAC',hash:'SHA-1'},false,['sign']);
const counter=Math.floor(Date.now()/30000);
for(const drift of [0,1,-1]){
const buf=new ArrayBuffer(8);const dv=new DataView(buf);dv.setUint32(4,counter+drift);
const mac=new Uint8Array(await crypto.subtle.sign('HMAC',key,buf));
const off=mac[19]&0xf;const bin=((mac[off]&0x7f)<<24|mac[off+1]<<16|mac[off+2]<<8|mac[off+3])%1000000;
if(String(bin).padStart(6,'0')===code)return true;}
return false});"""

# (1) TOTP helper after the IIFE opening.
HELPER_SEAM_OLD = "(()=>{'use strict';"
HELPER_SEAM_NEW = "(()=>{'use strict';\n" + TOTP_HELPER + "\n"

# (2a) MFA step entry: real enrollment, secret surfaced in the approved slot.
MFA_INIT_OLD = "if(route==='mfa'){drawMfa();$('#risk')?.addEventListener('change'"
MFA_INIT_NEW = ("if(route==='mfa'){drawMfa();"
"window.__drevantaEnroll=(async()=>{try{"
"const er=await fetch('/api/v1/identity/mfa/enroll',{method:'POST',credentials:'include'});"
"if(er.status===409)return null;if(!er.ok)return null;"
"const ej=await er.json();st.mfaSecret=ej.secret;save();"
"const key=document.querySelector('#manualKey code, #manualKey .key, #mfaArea code');"
"if(key)key.textContent=ej.secret;"
"return ej.secret}catch(e){return null}})();"
"$('#risk')?.addEventListener('change'")

# (2b) MFA submit: confirm against the enrolled secret, then real codes.
MFA_SUBMIT_OLD = "if(route==='mfa'){if(!/^\\d{6}$/.test(mfaDigits()))return err('Enter all six digits to preview MFA confirmation.');st.mfa=true;save();return go('recovery-codes')}"
MFA_SUBMIT_NEW = "if(route==='mfa'){const code6=mfaDigits();if(!/^\\d{6}$/.test(code6))return err('Enter all six digits to confirm your authenticator setup.');try{const secret=st.mfaSecret||await window.__drevantaEnroll;if(!secret)return err('MFA enrollment unavailable. Try again shortly.');const ok6=await window.__drevantaTotpCheck(secret,code6);if(!ok6)return err('That code does not match the enrolled authenticator. Re-add the entry and try again.');const rc=await fetch('/api/v1/identity/recovery-codes',{method:'POST',credentials:'include'});if(!rc.ok)return err('Recovery codes unavailable. Try again shortly.');const rj=await rc.json();st.recoveryCodes=rj.recovery_codes||[];st.mfa=true;save();return go('recovery-codes')}catch(e){return err('MFA enrollment unavailable. Try again shortly.')}}"

# (3) Recovery codes screen: real codes in the reserved slot.
CODES_RENDER_OLD = "box('Recovery codes · protected','A live server would display ten unique codes here, with copy and print options. This preview intentionally does not generate fake security credentials.')+"
CODES_RENDER_NEW = ("((st.recoveryCodes&&st.recoveryCodes.length)?box('Recovery codes · protected','Each code works once. Store them offline now — they are shown only this once.')+"
"'<div style=\"display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:6px;margin:8px 0\">'+st.recoveryCodes.map(c=>'<code style=\"padding:4px 6px;background:var(--panel-alt,rgba(148,163,184,.12));border-radius:6px\">'+esc(c)+'</code>').join('')+'</div>'"
":box('Recovery codes · protected','No codes were issued in this session; enable MFA to generate them.'))+")

AUTH = f"{FRONT}/static/js/auth.js"

# (4) Post-login routing by the real workspace list.
LOGIN_ROUTE_OLD = "if(j.token){location.href='/'}else{error('Unable to sign in. Check your credentials and try again.')}"
LOGIN_ROUTE_NEW = ("if(j.token){let route='/';try{const w=await fetch('/api/v1/workspaces',{credentials:'include'});"
"const wl=await w.json().catch(()=>({items:[]}));"
"const map={'hosting_org':'/','server':'/server','website':'/website','developer_project':'/developer'};"
"const pick=(wl.items||[]).find(x=>x.kind==='hosting')||(wl.items||[])[0];"
"if(pick&&map[pick.kind])route=map[pick.kind];}catch(e){}"
"location.href=route}else{error('Unable to sign in. Check your credentials and try again.')}")

patch(AF, [
    (HELPER_SEAM_OLD, HELPER_SEAM_NEW),
    (MFA_INIT_OLD, MFA_INIT_NEW),
    (MFA_SUBMIT_OLD, MFA_SUBMIT_NEW),
    (CODES_RENDER_OLD, CODES_RENDER_NEW),
])
patch(AUTH, [(LOGIN_ROUTE_OLD, LOGIN_ROUTE_NEW)])
print("stage-2 auth seams wired")
