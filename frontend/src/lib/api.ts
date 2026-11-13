/**
 * Typed client for the Drevanta control plane (via the same-origin BFF
 * proxy). Contracts mirror internal/api/handlers.go exactly. The bearer
 * token never appears here — the BFF attaches it from the HttpOnly cookie.
 */

export interface Me {
	principal_id: string;
	email: string;
	display_name: string;
	memberships: string[];
}

export interface WorkspaceItem {
	workspace_id: string;
	kind: string; // 'server' | 'hosting_org' | 'website' | 'developer_project'
	name: string;
	role: string;
}

export interface ResourceItem {
	resource_id: string;
	type: string;
	name: string;
	desired_generation: number;
	observed_generation: number;
	owner_revision: number;
}

export interface OperationAccepted {
	operation_id: string;
	correlation_id: string;
	state: string;
	operation_status_url: string;
	replayed: boolean;
}

export interface OperationStatus {
	operation_id: string;
	state: string; // pending | dispatching | running | succeeded | failed | rejected
	action_id: string;
	scope_id: string;
	correlation_id: string;
	created_at: string;
	updated_at: string;
	intent?: unknown;
	result?: unknown;
}

export interface AuditEvent {
	event_id?: string;
	actor_id?: string;
	action_id?: string;
	outcome?: string;
	safe_reason?: string;
	created_at?: string;
	[key: string]: unknown;
}

export interface ApiError {
	error: string;
	code: string;
	message: string;
}

export class ApiFailure extends Error {
	constructor(
		public status: number,
		public body: ApiError | null
	) {
		super(body?.message ?? `request failed (${status})`);
	}
}

async function request<T>(method: string, path: string, body?: unknown, idempotencyKey?: string): Promise<T> {
	const res = await fetch(`/api/v1/${path}`, {
		method,
		credentials: 'include',
		headers: {
			...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
			...(idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : {})
		},
		body: body !== undefined ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		let parsed: ApiError | null = null;
		try {
			parsed = (await res.json()) as ApiError;
		} catch {
			parsed = null;
		}
		throw new ApiFailure(res.status, parsed);
	}
	return (await res.json()) as T;
}

export const api = {
	me: () => request<Me>('GET', 'me'),
	workspaces: () => request<{ items: WorkspaceItem[]; next_cursor: string | null }>('GET', 'workspaces'),
	resources: (scopeId: string, type?: string) =>
		request<{ items: ResourceItem[] }>(
			'GET',
			`resources?scope_id=${encodeURIComponent(scopeId)}${type ? `&type=${encodeURIComponent(type)}` : ''}`
		),
	submitIntent: (resourceId: string, intent: { action_id: string; params?: unknown }, idempotencyKey: string) =>
		request<OperationAccepted>('POST', `resources/${encodeURIComponent(resourceId)}/intents`, intent, idempotencyKey),
	operation: (operationId: string) =>
		request<OperationStatus>('GET', `operations/${encodeURIComponent(operationId)}`),
	scopeAudit: (scopeId: string) =>
		request<{ events: AuditEvent[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/audit`),
	mfaStatus: () => request<{ enrolled: boolean }>('GET', 'identity/mfa/status'),
	mfaEnroll: () =>
		request<{ secret: string; otpauth_uri: string }>('POST', 'identity/mfa/enroll', {}),
	recoveryCodes: () =>
		request<{ recovery_codes: string[] }>('POST', 'identity/recovery-codes', {}),
	logout: () => request<{ status: string }>('POST', 'identity/logout', {}),
	listApiKeys: (scopeId: string) =>
		request<{ items: Array<{ key_id: string; created_at?: string; state?: string; prefix?: string }> }>(
			'GET',
			`scopes/${encodeURIComponent(scopeId)}/api-keys`
		),
	issueApiKey: (scopeId: string) =>
		request<{ key_id: string; token: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/api-keys`, {}),
	createResource: (scopeId: string, body: { name: string; type: string; host_id?: string }) =>
		request<{ resource_id: string; name: string; type: string }>(
			'POST',
			`scopes/${encodeURIComponent(scopeId)}/resources`,
			body
		),
	revokeApiKey: (scopeId: string, keyId: string) =>
		request<{ status: string }>('DELETE', `scopes/${encodeURIComponent(scopeId)}/api-keys/${encodeURIComponent(keyId)}`),
	listMemberships: (workspaceId: string) =>
		request<{ items: Array<{ principal_id: string; role_id: string; state: string; expires_at: string | null }> }>(
			'GET',
			`workspaces/${encodeURIComponent(workspaceId)}/memberships`
		)
};

/** Persona workspace kinds -> production route (approved navigation). */
export function routeForWorkspaceKind(kind: string): string {
	switch (kind) {
		case 'hosting_org':
			return '/';
		case 'server':
			return '/server';
		case 'website':
			return '/website';
		case 'developer_project':
			return '/developer';
		default:
			return '/';
	}
}
