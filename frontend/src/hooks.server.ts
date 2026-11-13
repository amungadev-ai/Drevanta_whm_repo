import { redirect, type Handle } from '@sveltejs/kit';
import { SESSION_COOKIE, isPublicPath } from '$lib/session';
import { env } from '$env/dynamic/private';

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

	const token = event.cookies.get(SESSION_COOKIE);
	if (!token) {
		redirect(302, `/auth/login?next=${encodeURIComponent(pathname + search)}`);
	}

	const base = env.GO_API_URL ?? 'http://127.0.0.1:8080';
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
