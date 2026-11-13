import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { SESSION_COOKIE } from '$lib/session';
import { env } from '$env/dynamic/private';
import { dev } from '$app/environment';

/**
 * BFF proxy: same-origin /api/v1/* -> Go control plane.
 * - Injects `Authorization: Bearer <cookie token>` server-side.
 * - login/mfa responses: stores the token in an HttpOnly session cookie and
 *   strips it from the JSON returned to the browser.
 * - logout: revokes server-side and clears the cookie.
 * The token is never exposed to client JavaScript. The Go API remains the
 * sole authorization authority; this proxy adds no rules of its own.
 */
const base = () => (env.GO_API_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');

export const POST: RequestHandler = ({ request, cookies, url, params }) => proxy(request, cookies, url, params);
export const GET: RequestHandler = ({ request, cookies, url, params }) => proxy(request, cookies, url, params);
export const PATCH: RequestHandler = ({ request, cookies, url, params }) => proxy(request, cookies, url, params);
export const PUT: RequestHandler = ({ request, cookies, url, params }) => proxy(request, cookies, url, params);
export const DELETE: RequestHandler = ({ request, cookies, url, params }) => proxy(request, cookies, url, params);

async function proxy(
	request: Request,
	cookies: import('@sveltejs/kit').Cookies,
	url: URL,
	params: { path?: string }
): Promise<Response> {
	const path = params.path ?? '';
	const target = `${base()}/api/v1/${path}${url.search}`;
	const token = cookies.get(SESSION_COOKIE);

	const headers = new Headers();
	const ct = request.headers.get('content-type');
	if (ct) headers.set('content-type', ct);
	const idem = request.headers.get('idempotency-key');
	if (idem) headers.set('idempotency-key', idem);
	if (token) headers.set('authorization', `Bearer ${token}`);

	let body: ArrayBuffer | undefined;
	if (request.method !== 'GET' && request.method !== 'HEAD') {
		body = await request.arrayBuffer();
	}

	let upstream: Response;
	try {
		upstream = await fetch(target, {
			method: request.method,
			headers,
			body,
			signal: AbortSignal.timeout(15000)
		});
	} catch {
		throw error(503, 'control plane unavailable');
	}

	// Token-bearing auth responses: capture the token into an HttpOnly cookie
	// and remove it from the payload delivered to the browser.
	const isAuthEndpoint =
		path === 'identity/login' || path === 'identity/mfa' || path === 'identity/logout';
	if (isAuthEndpoint && upstream.ok) {
		const payload = (await upstream.json()) as Record<string, unknown>;
		if (path === 'identity/logout') {
			cookies.delete(SESSION_COOKIE, { path: '/' });
			return json(payload);
		}
		const tokenNew = typeof payload['token'] === 'string' ? payload['token'] : null;
		if (tokenNew) {
			cookies.set(SESSION_COOKIE, tokenNew, {
				path: '/',
				httpOnly: true,
				sameSite: 'strict',
				secure: !dev,
				maxAge: 60 * 60 * 12
			});
			delete payload['token'];
		}
		return json(payload, { status: upstream.status });
	}

	const respHeaders = new Headers();
	for (const h of ['content-type', 'cache-control', 'retry-after']) {
		const v = upstream.headers.get(h);
		if (v) respHeaders.set(h, v);
	}
	const buf = await upstream.arrayBuffer();
	return new Response(buf, { status: upstream.status, headers: respHeaders });
}
