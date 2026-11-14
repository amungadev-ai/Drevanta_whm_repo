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


// ---- Domain capability contracts (G40 full-persona wave) ----

export interface Ticket {
	ticket_id: string;
	scope_id: string;
	requester_id: string;
	subject: string;
	state: string;
	priority: string;
	sla_due_at: string | null;
	created_at: string;
	updated_at: string;
}

export interface TicketMessage {
	message_id: string;
	ticket_id: string;
	author_id: string;
	body: string;
	created_at: string;
}

export interface KBArticle {
	article_id: string;
	slug: string;
	title: string;
	body: string;
	state: string;
	updated_at: string;
}

export interface Plan {
	plan_id: string;
	name: string;
	quota_json: string;
	price_cents: number;
	currency: string;
	state: string;
	created_at: string;
}

export interface Customer {
	customer_id: string;
	email: string;
	name: string;
	state: string;
	created_at: string;
}

export interface HostingAccount {
	account_id: string;
	customer_id: string;
	plan_id: string;
	workspace_id: string | null;
	state: string;
	created_at: string;
}

export interface Invoice {
	invoice_id: string;
	customer_id: string;
	number: string;
	total_cents: number;
	currency: string;
	state: string;
	issued_at: string | null;
	due_at: string | null;
	created_at: string;
}

export interface SourceConnection {
	connection_id: string;
	repo_url: string;
	default_branch: string;
	state: string;
	created_at: string;
}

export interface Build {
	build_id: string;
	connection_id: string;
	commit_ref: string;
	state: string;
	artifact_digest: string;
	artifact_size: number;
	created_at: string;
	finished_at: string | null;
}

export interface Environment {
	environment_id: string;
	name: string;
	kind: string;
	current_release: string | null;
	created_at: string;
}

export interface MailDomain {
	mail_domain_id: string;
	domain: string;
	state: string;
	created_at: string;
}

export interface Mailbox {
	mailbox_id: string;
	mail_domain_id: string;
	local_part: string;
	quota_mb: number;
	state: string;
	created_at: string;
}

export interface MailAlias {
	alias_id: string;
	mail_domain_id: string;
	local_part: string;
	destination: string;
	created_at: string;
}

export interface Observation {
	observation_id: string;
	resource_id: string;
	kind: string;
	name: string;
	value_num: number | null;
	value_text: string | null;
	observed_at: string;
}

export interface Notification {
	notification_id: string;
	principal_id: string | null;
	kind: string;
	body: string;
	read_at: string | null;
	created_at: string;
}

export interface AppTemplate {
	template_id: string;
	slug: string;
	title: string;
	category: string;
	runtime: string;
	artifact_ref: string;
	state: string;
	created_at: string;
}

export interface SiteBackup {
	backup_id: string;
	resource_id: string;
	size_bytes: number;
	state: string;
	created_at: string;
}

export interface Schedule {
	schedule_id: string;
	resource_id: string;
	action_id: string;
	every_seconds: number;
	state: string;
	next_run_at: string;
	last_run_at: string | null;
	created_at: string;
}

export interface InvoiceLineInput {
	description: string;
	quantity: number;
	unit_cents: number;
	tax_cents?: number;
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
	// ---- Domain capabilities (scope-scoped; grants enforced server-side) ----
	listTickets: (scopeId: string) =>
		request<{ tickets: Ticket[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/tickets`),
	createTicket: (scopeId: string, body: { subject: string; priority?: string; body?: string }) =>
		request<{ ticket_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/tickets`, body),
	ticket: (scopeId: string, ticketId: string) =>
		request<{ ticket: Ticket; messages: TicketMessage[] }>(
			'GET',
			`scopes/${encodeURIComponent(scopeId)}/tickets/${encodeURIComponent(ticketId)}`
		),
	answerTicket: (scopeId: string, ticketId: string, body: string) =>
		request<{ ok: boolean }>(
			'POST',
			`scopes/${encodeURIComponent(scopeId)}/tickets/${encodeURIComponent(ticketId)}/messages`,
			{ body }
		),
	resolveTicket: (scopeId: string, ticketId: string) =>
		request<{ ok: boolean }>(
			'POST',
			`scopes/${encodeURIComponent(scopeId)}/tickets/${encodeURIComponent(ticketId)}/resolve`
		),
	listKB: (scopeId: string) =>
		request<{ articles: KBArticle[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/kb`),
	listPlans: (scopeId: string) =>
		request<{ plans: Plan[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/plans`),
	createPlan: (scopeId: string, body: { name: string; quota_json?: string; price_cents: number; currency?: string }) =>
		request<{ plan_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/plans`, body),
	listCustomers: (scopeId: string) =>
		request<{ customers: Customer[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/customers`),
	createCustomer: (scopeId: string, body: { email: string; name?: string }) =>
		request<{ customer_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/customers`, body),
	listAccounts: (scopeId: string) =>
		request<{ accounts: HostingAccount[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/accounts`),
	provisionAccount: (scopeId: string, body: { customer_id: string; plan_id: string }) =>
		request<{ account_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/accounts`, body),
	listInvoices: (scopeId: string) =>
		request<{ invoices: Invoice[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/invoices`),
	createInvoice: (scopeId: string, body: { customer_id: string; due_in_days?: number; lines: InvoiceLineInput[] }) =>
		request<{ invoice_id: string; total_cents: number }>('POST', `scopes/${encodeURIComponent(scopeId)}/invoices`, body),
	recordPayment: (scopeId: string, invoiceId: string, amount_cents: number) =>
		request<{ payment_id: string }>(
			'POST',
			`scopes/${encodeURIComponent(scopeId)}/invoices/${encodeURIComponent(invoiceId)}/payments`,
			{ amount_cents }
		),
	listSourceConnections: (scopeId: string) =>
		request<{ connections: SourceConnection[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/source-connections`),
	connectSource: (scopeId: string, body: { repo_url: string; default_branch?: string }) =>
		request<{ connection_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/source-connections`, body),
	listBuilds: (scopeId: string) =>
		request<{ builds: Build[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/builds`),
	queueBuild: (scopeId: string, body: { connection_id: string; commit_ref: string }) =>
		request<{ build_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/builds`, body),
	listEnvironments: (scopeId: string) =>
		request<{ environments: Environment[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/environments`),
	createEnvironment: (scopeId: string, body: { name: string; kind?: string }) =>
		request<{ environment_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/environments`, body),
	promoteRelease: (scopeId: string, envId: string, build_id: string) =>
		request<{ release_id: string }>(
			'POST',
			`scopes/${encodeURIComponent(scopeId)}/environments/${encodeURIComponent(envId)}/promote`,
			{ build_id }
		),
	listMailDomains: (scopeId: string) =>
		request<{ mail_domains: MailDomain[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/mail-domains`),
	addMailDomain: (scopeId: string, domain: string) =>
		request<{ mail_domain_id: string; state: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/mail-domains`, { domain }),
	verifyMailDomain: (scopeId: string, mailDomainId: string) =>
		request<{ ok: boolean }>(
			'POST',
			`scopes/${encodeURIComponent(scopeId)}/mail-domains/${encodeURIComponent(mailDomainId)}/verify`
		),
	listMailboxes: (scopeId: string) =>
		request<{ mailboxes: Mailbox[]; aliases: MailAlias[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/mailboxes`),
	createMailbox: (scopeId: string, body: { mail_domain_id: string; local_part: string; quota_mb?: number }) =>
		request<{ mailbox_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/mailboxes`, body),
	createAlias: (scopeId: string, body: { mail_domain_id: string; local_part: string; destination: string }) =>
		request<{ alias_id: string }>('POST', `scopes/${encodeURIComponent(scopeId)}/mail-aliases`, body),
	listObservations: (scopeId: string, resourceId?: string) =>
		request<{ observations: Observation[] }>(
			'GET',
			`scopes/${encodeURIComponent(scopeId)}/observations${resourceId ? `?resource_id=${encodeURIComponent(resourceId)}` : ''}`
		),
	listNotifications: (scopeId: string) =>
		request<{ notifications: Notification[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/notifications`),
	listTemplates: (scopeId: string) =>
		request<{ templates: AppTemplate[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/templates`),
	installTemplate: (scopeId: string, templateId: string, resource_id: string) =>
		request<{ install_id: string }>(
			'POST',
			`scopes/${encodeURIComponent(scopeId)}/templates/${encodeURIComponent(templateId)}/install`,
			{ resource_id }
		),
	listBackups: (scopeId: string) =>
		request<{ backups: SiteBackup[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/backups`),
	listSchedules: (scopeId: string) =>
		request<{ schedules: Schedule[] }>('GET', `scopes/${encodeURIComponent(scopeId)}/schedules`),
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
