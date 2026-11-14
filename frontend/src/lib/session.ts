/**
 * Session cookie contract (BFF pattern — the bearer token never reaches
 * client JS; see docs/ui/FRONTEND-ARCHITECTURE.md).
 */
export const SESSION_COOKIE = 'drevanta_session';

/** Paths reachable without an authenticated session. */
export function isPublicPath(pathname: string): boolean {
	if (pathname.startsWith('/login')) return true;
	if (pathname.startsWith('/auth/')) return true;
	if (pathname.startsWith('/onboarding')) return true;
	if (pathname.startsWith('/css/') || pathname.startsWith('/js/') || pathname.startsWith('/_app/')) return true;
	if (pathname === '/favicon.ico' || pathname === '/favicon.svg') return true;
	return false;
}

/** Proxy endpoints usable pre-auth (Go enforces install-state rules itself). */
export function isPublicApi(path: string): boolean {
	return (
		path === '/api/v1/identity/login' ||
		path === '/api/v1/identity/mfa' ||
		path === '/api/v1/identity/recover-request' ||
		path === '/api/v1/identity/recover-confirm' ||
		path.startsWith('/api/v1/install/')
	);
}
