import { redirect, type Handle } from '@sveltejs/kit';
import { SESSION_COOKIE, isPublicPath } from '$lib/session';
import { env } from '$env/dynamic/private';

/**
 * Pending-claim gate. While the installation has no owner yet (state
 * "pending"), every non-public, non-onboarding page redirects to the
 * first-run claim page at /onboarding/administrator. This makes the
 * installer's primary URL (https://<server>/) land on onboarding — not
 * /auth/login — until the first administrator has claimed the install.
 *
 * RFC 7231 §7.1.2: a 302 without a fragment inherits the original
 * request's fragment, so https://<server>/#clm_<token> → 302 → browser
 * navigates to /onboarding/administrator#clm_<token> — the claim token
 * is preserved through the redirect even though the server never sees
 * the hash (fragments are client-side only).
 */
async function isInstallPending(base: string): Promise<boolean> {
        try {
                const st = await fetch(`${base}/api/v1/install/status`, {
                        signal: AbortSignal.timeout(4000)
                });
                if (!st.ok) return false;
                const body = await st.json();
                return body?.state === 'pending';
        } catch {
                // Status check failed — fail closed (treat as NOT pending so the
                // auth gate handles it; this avoids leaking onboarding on a
                // broken control plane).
                return false;
        }
}

/**
 * Protected-route gate. Every non-public navigation must carry a session
 * cookie whose token the Go control plane still accepts (GET /api/v1/me).
 * 401/expiry clears the cookie and bounces to /auth/login. The Go API
 * remains the sole authority — the gate never grants anything the API
 * would deny.
 */
export const handle: Handle = async ({ event, resolve }) => {
        const { pathname, search } = event.url;

        if (isPublicPath(pathname) || pathname.startsWith('/api/v1/')) {
                return resolve(event);
        }

        const base = env.GO_API_URL ?? 'http://127.0.0.1:8080';

        // Pending-claim gate takes precedence over the auth check: if the
        // installation is unclaimed, the operator must complete onboarding
        // before any authenticated UI is reachable.
        if (await isInstallPending(base)) {
                if (pathname !== '/onboarding/administrator' && !pathname.startsWith('/onboarding/administrator/')) {
                        redirect(302, '/onboarding/administrator');
                }
                return resolve(event);
        }

        const token = event.cookies.get(SESSION_COOKIE);
        if (!token) {
                redirect(302, `/auth/login?next=${encodeURIComponent(pathname + search)}`);
        }

        try {
                const me = await fetch(`${base}/api/v1/me`, {
                        headers: { Authorization: `Bearer ${token}` },
                        signal: AbortSignal.timeout(4000)
                });
                if (!me.ok) {
                        event.cookies.delete(SESSION_COOKIE, { path: '/' });
                        redirect(302, `/auth/login?next=${encodeURIComponent(pathname + search)}`);
                }
        } catch {
                // Control plane unreachable: fail closed (no rendering of protected UI).
                redirect(302, `/auth/login?reason=unavailable`);
        }

        return resolve(event);
};
