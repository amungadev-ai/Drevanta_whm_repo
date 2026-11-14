<script lang="ts">
	/**
	 * PersonaRealData — G40 full-persona wave.
	 *
	 * Takes over the backed sections of the module-ported persona pages:
	 * when the approved module renders a section whose capability has real
	 * API backing, this component replaces the demonstration rows with REAL
	 * control-plane data in the identical approved markup (helpers replicated
	 * in realdata.ts). Nav, layout and non-backed sections stay module-driven.
	 *
	 * Backed sections per persona:
	 *  hosting  → customers, accounts, plans, support (tickets+KB), billing
	 *             (invoices, manual-gateway payments), alerts (notifications),
	 *             activity (audit events)
	 *  server   → monitoring (health observations), audit, jobs (schedules)
	 *  website  → domains/databases/files (real resources + typed intents),
	 *             backups, monitoring
	 *  developer→ builds (+queue), environments (+create/promote),
	 *             deployments (source connections +connect), monitoring
	 *
	 * Every mutation is a real API call; results re-render from the API —
	 * no optimistic fake rows, no simulated success.
	 */
	import { onMount } from 'svelte';
	import { loadSession, session } from '$lib/stores/session';
	import { api } from '$lib/api';
	import {
		esc, head, panel, table, badge, note, btn, field, formPanel, money, fmtWhen
	} from '$lib/realdata';
	import type { ResourceItem, AuditEvent } from '$lib/api';

	let { preferredKind = '' }: { preferredKind?: string } = $props();

	let currentSection = $state('');
	let busy = $state(false);

	const ORG = () => {
		const s = $session;
		return { name: s.active?.name ?? 'Workspace', type: s.active?.role ?? '' };
	};

	// ---------- section registry ----------
	function backed(kind: string, section: string): boolean {
		const map: Record<string, string[]> = {
			hosting: ['customers', 'accounts', 'plans', 'support', 'alerts', 'activity'],
			server: ['monitoring', 'audit', 'jobs'],
			website: ['domains', 'databases', 'files', 'backups', 'monitoring'],
			developer: ['source', 'builds', 'environments', 'monitoring']
		};
		return (map[kind] ?? []).includes(section);
	}

	// ---------- nav interception: record the target section ----------
	function onPageClick(ev: MouseEvent) {
		const t = ev.target as HTMLElement | null;
		if (!t) return;
		const navBtn = t.closest('#nav button, button[data-action], button[data-h-action], button[data-p-action]') as HTMLElement | null;
		if (!navBtn) return;
		const attr = navBtn.getAttribute('data-action') ?? navBtn.getAttribute('data-h-action') ??
			navBtn.getAttribute('data-p-action') ?? '';
		if (attr.startsWith('goto:')) {
			currentSection = attr.slice(5);
			const page = document.getElementById('page');
			if (page) delete page.dataset.realSection;
			// The module re-renders synchronously on its own handler; render
			// after it settles (the observer covers later renders too).
			window.setTimeout(() => { void renderIfBacked(); }, 80);
		}
	}

	// ---------- render orchestration ----------
	async function renderIfBacked(): Promise<void> {
		const s = $session;
		const kind = (s.active?.kind ?? '').replace('hosting_org', 'hosting').replace('_project', '');
		const page = document.getElementById('page');
		if (!page || !s.active || !backed(kind, currentSection)) return;
		if (page.dataset.realSection === currentSection) return;
		busy = true;
		try {
			const scope = s.active.workspace_id;
			const html = await renderSection(kind, currentSection, scope);
			if (html !== null) {
				page.innerHTML = html;
				page.dataset.realSection = currentSection;
				bindActions(kind, currentSection, scope);
			}
		} finally {
			busy = false;
		}
	}

	async function renderSection(kind: string, section: string, scope: string): Promise<string | null> {
		const o = ORG();
		const H = (title: string, sub: string, actions = '') => head(title, sub, o.name, o.type, actions);
		try {
			if (kind === 'hosting') return await hostingSection(section, scope, H);
			if (kind === 'server') return await serverSection(section, scope, H);
			if (kind === 'website') return await websiteSection(section, scope, H);
			if (kind === 'developer') return await developerSection(section, scope, H);
		} catch (e) {
			// Real failure state: say so, never fabricate rows.
			const msg = e instanceof Error && 'status' in e ? `control plane returned ${(e as { status: number }).status}` : 'control plane unavailable';
			return H('Section unavailable', '') + note(`<strong>Real state:</strong> ${esc(msg)}. No demonstration data is shown.`);
		}
		return null;
	}

	// ---------- hosting ----------
	async function hostingSection(section: string, scope: string, H: (t: string, s: string, a?: string) => string): Promise<string> {
		if (section === 'customers') {
			const { customers } = await api.listCustomers(scope);
			return (
				H('Customer Directory', 'Real customer records from the control plane (grant: billing.adjust).', btn('New customer', 'form:customer:create', 'primary')) +
				table(['Email', 'Name', 'State', 'Since'], customers.map((c) => [esc(c.email), esc(c.name), badge(c.state), fmtWhen(c.created_at)])) +
				formPanel('New customer', field('Email', 'rf-email', 'email', 'owner@site.test') + field('Name', 'rf-name', 'text', 'Acme Studio'), 'customer:create', 'Create customer')
			);
		}
		if (section === 'accounts') {
			const [{ accounts }, { plans }, { customers }] = await Promise.all([api.listAccounts(scope), api.listPlans(scope), api.listCustomers(scope)]);
			const cname = (id: string) => customers.find((c) => c.customer_id === id)?.email ?? id.slice(0, 12) + '…';
			return (
				H('Hosting Accounts', 'Provisioned accounts (grant: account.provision).', (plans.length && customers.length ? btn('Provision account', 'form:account:create', 'primary') : '')) +
				table(['Customer', 'Plan', 'State', 'Since'], accounts.map((a) => [esc(cname(a.customer_id)), esc(plans.find((p) => p.plan_id === a.plan_id)?.name ?? '—'), badge(a.state), fmtWhen(a.created_at)])) +
				(plans.length && customers.length
					? formPanel('Provision account',
						`<label class="field"><span>Customer</span><select class="field-input" id="rf-customer">${customers.map((c) => `<option value="${esc(c.customer_id)}">${esc(c.email)}</option>`).join('')}</select></label>
						 <label class="field"><span>Plan</span><select class="field-input" id="rf-plan">${plans.map((p) => `<option value="${esc(p.plan_id)}">${esc(p.name)}</option>`).join('')}</select></label>`,
						'account:create', 'Provision')
					: note('Provisioning needs at least one plan and one customer.'))
			);
		}
		if (section === 'plans') {
			const { plans } = await api.listPlans(scope);
			return (
				H('Service Plans', 'Real plan catalog (grant: billing.adjust).', btn('New plan', 'form:plan:create', 'primary')) +
				table(['Name', 'Price', 'Currency', 'State'], plans.map((p) => [esc(p.name), money(p.price_cents, p.currency), esc(p.currency), badge(p.state)])) +
				formPanel('New plan', field('Name', 'rf-name', 'text', 'starter') + field('Price (cents)', 'rf-price', 'number', '900'), 'plan:create', 'Create plan') +
				note('<strong>Gateway status (real):</strong> payments use the manual gateway; external gateway credentials are an owner input (BLK-001).')
			);
		}
		if (section === 'support') {
			const [{ tickets }, { articles }] = await Promise.all([api.listTickets(scope), api.listKB(scope)]);
			return (
				H('Support Requests', 'Real tickets (grant: support.ticket.create).', btn('New ticket', 'form:ticket:create', 'primary')) +
				table(['Subject', 'State', 'Priority', 'Updated', ''], tickets.map((t) => [
					esc(t.subject), badge(t.state), badge(t.priority), fmtWhen(t.updated_at),
					`<button class="btn secondary sm" data-real-action="ticket:open:${esc(t.ticket_id)}">Open</button>`
				])) +
				formPanel('New ticket', field('Subject', 'rf-subject', 'text', 'Question') + field('Details', 'rf-body', 'text', 'Describe the request'), 'ticket:create', 'Create ticket') +
				`<div id="realTicketDetail"></div>` +
				panel('Knowledge base', table(['Title', 'Updated'], articles.map((a) => [esc(a.title), fmtWhen(a.updated_at)])))
			);
		}
		if (section === 'billing') {
			const { invoices } = await api.listInvoices(scope);
			return (
				H('Billing · Invoices', 'Real invoices; payments record through the manual gateway.', '') +
				table(['Number', 'Total', 'State', 'Issued', ''], invoices.map((i) => [
					esc(i.number), money(i.total_cents, i.currency), badge(i.state), fmtWhen(i.issued_at),
					i.state === 'issued' ? `<button class="btn secondary sm" data-real-action="invoice:pay:${esc(i.invoice_id)}">Record payment</button>` : ''
				])) +
				note('<strong>Real limitation:</strong> external payment-gateway credentials are owner inputs (BLK-001); recording is manual until configured.')
			);
		}
		if (section === 'alerts') {
			const { notifications } = await api.listNotifications(scope);
			return (
				H('Alerts & Tasks', 'Real notifications from the control plane.', '') +
				table(['Kind', 'Body', 'When'], notifications.map((n) => [esc(n.kind), esc(n.body), fmtWhen(n.created_at)]))
			);
		}
		if (section === 'activity') {
			const { events } = await api.scopeAudit(scope);
			return (
				H('Activity Feed', 'Real audit events (grant: audit.read).', '') +
				table(['Action', 'Outcome', 'When'], list.slice(0, 50).map((e) => [esc(e.action_id ?? 'event'), badge(e.outcome ?? ''), fmtWhen(e.created_at ?? null)]))
			);
		}
		return null as unknown as string;
	}

	// ---------- server ----------
	async function serverSection(section: string, scope: string, H: (t: string, s: string, a?: string) => string): Promise<string> {
		if (section === 'monitoring') {
			const { observations } = await api.listObservations(scope);
			const { items } = await api.resources(scope);
			const rname = (id: string) => items.find((r) => r.resource_id === id)?.name ?? id.slice(0, 12) + '…';
			return (
				H('Monitoring & Health', 'Real health observations (grant: monitor.read).', '') +
				table(['Resource', 'Name', 'Value', 'When'], observations.map((ob) => [
					esc(rname(ob.resource_id)), esc(ob.name), esc(ob.value_num !== null ? String(ob.value_num) : ob.value_text ?? ''), fmtWhen(ob.observed_at)
				])) +
				note('<strong>Real state:</strong> observations are recorded by host operations (host.inspect) and system events; an unobserved host shows none.')
			);
		}
		if (section === 'audit') {
			const { events } = await api.scopeAudit(scope);
			const list = (events ?? []) as AuditEvent[];
			return (
				H('Audit & Security Center', 'Real audit events (grant: audit.read).', '') +
				table(['Action', 'Outcome', 'When'], list.slice(0, 80).map((e) => [esc(e.action_id ?? 'event'), badge(e.outcome ?? ''), fmtWhen(e.created_at ?? null)]))
			);
		}
		if (section === 'jobs') {
			const { schedules } = await api.listSchedules(scope);
			return (
				H('Jobs & Automation', 'Real schedules (grant: schedule.configure).', '') +
				table(['Action', 'Every', 'State', 'Next run'], schedules.map((sc) => [esc(sc.action_id), `${sc.every_seconds}s`, badge(sc.state), fmtWhen(sc.next_run_at)]))
			);
		}
		return null as unknown as string;
	}

	// ---------- website ----------
	const WEBSITE_TYPE_SECTION: Record<string, { type: string; action: string; args: string; label: string; createType?: string }> = {
		domains: { type: 'hostname binding', action: 'domain.bind', args: [{ k: 'hostname', ph: 'app.example' }, { k: 'target', ph: 'project-route' }], label: 'Bind hostname', createType: 'hostname binding' },
		databases: { type: 'SQL database', action: 'database.provision', args: [{ k: 'engine', ph: 'sqlite' }], label: 'Provision database', createType: 'SQL database' },
		files: { type: 'filesystem entry', action: 'file.write', args: [{ k: 'path', ph: 'index.html' }, { k: 'content', ph: '<h1>Site</h1>' }], label: 'Write file', createType: 'filesystem entry' }
	};

	async function websiteSection(section: string, scope: string, H: (t: string, s: string, a?: string) => string): Promise<string> {
		if (section in WEBSITE_TYPE_SECTION) {
			const cfg = WEBSITE_TYPE_SECTION[section];
			const { items } = await api.resources(scope, cfg.type);
			const rows = items.map((r) => [
				esc(r.name), badge(String(r.observed_generation)), String(r.desired_generation),
				`<button class="btn secondary sm" data-real-action="res:form:${esc(r.resource_id)}:${esc(section)}">${esc(cfg.label)}</button>`
			]);
			return (
				H(cfg.label + ' targets', `Real ${cfg.type} resources (grant: resource.link).`, cfg.createType ? btn('Register target', `form:resource:create:${section}`, 'primary') : '') +
				table(['Name', 'Observed gen', 'Desired gen', ''], rows) +
				`<div id="realIntentForm"></div>` +
				(cfg.createType ? formPanel('Register ' + cfg.type, field('Name', 'rf-name', 'text', 'primary'), `resource:create:${section}`, 'Register') : '') +
				note('<strong>Real effects:</strong> actions run through the durable operation pipeline (intent → operation → privileged helper) and appear in the operations tracker.')
			);
		}
		if (section === 'backups') {
			const { backups } = await api.listBackups(scope);
			return (
				H('Backups & Restore', 'Real backup manifests (creation is job-driven).', '') +
				table(['Resource', 'Size', 'State', 'Created'], backups.map((b) => [esc(b.resource_id.slice(0, 14)) + '…', `${(b.size_bytes / 1024).toFixed(1)} KB`, badge(b.state), fmtWhen(b.created_at)])) +
				note('<strong>Real state:</strong> restore requires a verified manifest and key gate (grant: backup.restore); an unbacked site shows none.')
			);
		}
		if (section === 'monitoring') {
			const { observations } = await api.listObservations(scope);
			return (
				H('Monitoring & Analytics', 'Real health observations (grant: monitor.read).', '') +
				table(['Name', 'Value', 'When'], observations.map((ob) => [esc(ob.name), esc(ob.value_num !== null ? String(ob.value_num) : ob.value_text ?? ''), fmtWhen(ob.observed_at)]))
			);
		}
		return null as unknown as string;
	}

	// ---------- developer ----------
	async function developerSection(section: string, scope: string, H: (t: string, s: string, a?: string) => string): Promise<string> {
		if (section === 'source') {
			const { connections } = await api.listSourceConnections(scope);
			return (
				H('Source & Repositories', 'Real source connections (grant: source.connect).', btn('Connect source', 'form:source:create', 'primary')) +
				table(['Repository', 'Branch', 'State', 'Since'], connections.map((c) => [esc(c.repo_url), esc(c.default_branch), badge(c.state), fmtWhen(c.created_at)])) +
				formPanel('Connect source', field('Repository URL', 'rf-repo', 'text', 'https://git.example.net/project.git') + field('Default branch', 'rf-branch', 'text', 'main'), 'source:create', 'Connect')
			);
		}
		if (section === 'builds') {
			const [{ builds }, { connections }] = await Promise.all([api.listBuilds(scope), api.listSourceConnections(scope)]);
			const repo = (id: string) => connections.find((c) => c.connection_id === id)?.repo_url ?? id.slice(0, 12) + '…';
			return (
				H('Builds & Artifacts', 'Real builds (grant: source.connect).', connections.length ? btn('Queue build', 'form:build:create', 'primary') : '') +
				table(['Repository', 'Commit', 'State', 'Digest', 'Queued'], builds.map((b) => [
					esc(repo(b.connection_id)), esc(b.commit_ref), badge(b.state), esc(b.artifact_digest ? b.artifact_digest.slice(0, 18) + '…' : '—'), fmtWhen(b.created_at)
				])) +
				(connections.length
					? formPanel('Queue build',
						`<label class="field"><span>Source</span><select class="field-input" id="rf-conn">${connections.map((c) => `<option value="${esc(c.connection_id)}">${esc(c.repo_url)}</option>`).join('')}</select></label>` +
						field('Commit ref', 'rf-commit', 'text', 'main'),
						'build:create', 'Queue build')
					: note('Connect a source first (Deployments & Releases).'))
			);
		}
		if (section === 'environments') {
			const [{ environments }, { builds }] = await Promise.all([api.listEnvironments(scope), api.listBuilds(scope)]);
			const ok = builds.filter((b) => b.state === 'succeeded');
			return (
				H('Environments', 'Real environments and CAS-gated promotions (grant: environment.clone / deployment.promote).', btn('New environment', 'form:env:create', 'primary')) +
				table(['Name', 'Kind', 'Current release', ''], environments.map((e) => [
					esc(e.name), badge(e.kind), esc(e.current_release ? e.current_release.slice(0, 14) + '…' : '—'),
					ok.length ? `<button class="btn secondary sm" data-real-action="env:promote:${esc(e.environment_id)}">Promote</button>` : ''
				])) +
				formPanel('New environment', field('Name', 'rf-name', 'text', 'staging') + field('Kind', 'rf-kind', 'text', 'staging'), 'env:create', 'Create') +
				note(ok.length ? 'Promotion requires a succeeded build with a digest — available builds: ' + ok.length + '.' : 'Promotion unlocks when a build succeeds with a digest.')
			);
		}
		if (section === 'monitoring') {
			const { observations } = await api.listObservations(scope);
			return (
				H('Monitoring & Analytics', 'Real health observations (grant: monitor.read).', '') +
				table(['Name', 'Value', 'When'], observations.map((ob) => [esc(ob.name), esc(ob.value_num !== null ? String(ob.value_num) : ob.value_text ?? ''), fmtWhen(ob.observed_at)]))
			);
		}
		return null as unknown as string;
	}

	// ---------- actions ----------
	async function submitForm(section: string, scope: string): Promise<void> {
		const page = document.getElementById('page');
		if (!page) return;
		const form = page.querySelector('[data-real-form]') as HTMLElement | null;
		const msg = form?.querySelector('.real-form-msg') as HTMLElement | null;
		const val = (id: string) => (document.getElementById(id) as HTMLInputElement | null)?.value?.trim() ?? '';
		const say = (t: string) => { if (msg) msg.textContent = t; };
		const s = $session;
		if (section.startsWith('resource:create:') && !val('rf-name')) {
			say('Name required — nothing was created.');
			return;
		}
		if (['customer:create', 'plan:create', 'ticket:create', 'source:create', 'build:create', 'env:create'].includes(section)) {
			const required = { 'customer:create': 'rf-email', 'plan:create': 'rf-name', 'ticket:create': 'rf-subject', 'source:create': 'rf-repo', 'build:create': 'rf-commit', 'env:create': 'rf-name' }[section];
			if (required && !val(required)) {
				say('Required field is empty — nothing was created.');
				return;
			}
		}
		try {
			switch (section) {
				case 'customer:create':
					await api.createCustomer(scope, { email: val('rf-email'), name: val('rf-name') });
					break;
				case 'account:create':
					await api.provisionAccount(scope, { customer_id: val('rf-customer'), plan_id: val('rf-plan') });
					break;
				case 'plan:create':
					await api.createPlan(scope, { name: val('rf-name'), price_cents: Number(val('rf-price') || '0') });
					break;
				case 'ticket:create':
					await api.createTicket(scope, { subject: val('rf-subject'), body: val('rf-body') });
					break;
				case 'source:create':
					await api.connectSource(scope, { repo_url: val('rf-repo'), default_branch: val('rf-branch') || 'main' });
					break;
				case 'build:create':
					await api.queueBuild(scope, { connection_id: val('rf-conn'), commit_ref: val('rf-commit') });
					break;
				case 'env:create':
					await api.createEnvironment(scope, { name: val('rf-name'), kind: val('rf-kind') || 'staging' });
					break;
				default: {
					if (section.startsWith('ticket:reply:')) {
						await api.answerTicket(scope, section.slice('ticket:reply:'.length), val('rf-reply'));
						const page2 = document.getElementById('page');
						if (page2) page2.dataset.realSection = '';
						await renderIfBacked();
						await runAction('ticket:open:' + section.slice('ticket:reply:'.length));
						return;
					}
					if (section.startsWith('resource:create:')) {
						const sec = section.slice('resource:create:'.length);
						const cfg = WEBSITE_TYPE_SECTION[sec];
						if (cfg?.createType && s.active) {
							await api.createResource(scope, { type: cfg.createType, name: val('rf-name') });
						}
					} else {
						say('Unknown form');
						return;
					}
				}
			}
			page.dataset.realSection = '';
			await renderIfBacked();
		} catch (e) {
			const status = (e as { status?: number }).status;
			say(status === 403 ? 'Denied: your role does not grant this operation.' : status === 404 ? 'Not found in this scope.' : 'Operation failed — real error, nothing simulated.');
		}
	}

	async function runAction(action: string): Promise<void> {
		const s = $session;
		if (!s.active) return;
		const scope = s.active.workspace_id;
		const [kind, ...rest] = action.split(':');
		try {
			if (kind === 'form') {
				const form = document.querySelector('[data-real-form]');
				form?.scrollIntoView({ behavior: 'smooth', block: 'center' });
				return;
			}
			if (kind === 'ticket') {
				const ticketId = rest[1];
				const { ticket, messages } = await api.ticket(scope, ticketId);
				const box = document.getElementById('realTicketDetail');
				if (box) {
					box.innerHTML = panel(
						'Ticket · ' + ticket.subject,
						table(['Author', 'Message', 'When'], messages.map((m) => [esc(m.author_id.slice(0, 10)) + '…', esc(m.body), fmtWhen(m.created_at)])) +
							formPanel('Reply', field('Message', 'rf-reply', 'text', 'Write a reply'), 'ticket:reply:' + ticketId, 'Send') +
							(ticket.state !== 'resolved' ? btn('Resolve ticket', 'ticket:resolve:' + ticketId, 'primary') : badge('resolved'))
					);
				}
			} else if (kind === 'invoice') {
				await api.recordPayment(scope, rest[1], Number(prompt('Amount in cents', '900') ?? '0'));
				await rerender();
			} else if (kind === 'env') {
				const buildId = prompt('Build id to promote');
				if (buildId) await api.promoteRelease(scope, rest[1], buildId.trim());
				await rerender();
			} else if (kind === 'res') {
				// resource intent form: [_, 'form', resourceID, section]
				const [, , resourceId, sec] = action.split(':');
				const cfg = WEBSITE_TYPE_SECTION[sec];
				const box = document.getElementById('realIntentForm');
				if (box && cfg) {
					box.innerHTML = formPanel(
						cfg.label + ' · ' + resourceId.slice(0, 10) + '…',
						cfg.args.map((a) => field(a.k, 'rf-' + a.k, 'text', a.ph)).join(''),
						'res:apply:' + resourceId + ':' + sec,
						cfg.label
					);
				}
			} else if (kind === 'res' && rest[0] === 'apply') {
				const resourceId = rest[1];
				const sec = rest[2];
				const cfg = WEBSITE_TYPE_SECTION[sec];
				const args: Record<string, string> = {};
				for (const a of cfg.args) args[a.k] = (document.getElementById('rf-' + a.k) as HTMLInputElement)?.value ?? '';
				const key = 'iv-' + resourceId + '-' + cfg.action + '-' + Date.now();
				await api.submitIntent(resourceId, { action_id: cfg.action, params: args }, key);
				await rerender();
			}
		} catch (e) {
			const status = (e as { status?: number }).status;
			const detail = document.getElementById('realTicketDetail') ?? document.getElementById('page');
			if (detail) {
				const m = status === 403 ? 'Denied: the role does not grant this operation (real authorization).'
					: status === 404 ? 'Not found in this scope (real tenancy isolation).'
					: status === 409 ? 'State conflict: the operation is not permitted for the current record state.'
					: 'Operation failed — real error, nothing simulated.';
				detail.insertAdjacentHTML('afterbegin', note(`<strong>${esc(m)}</strong>`));
			}
		}
	}

	async function rerender(): Promise<void> {
		const page = document.getElementById('page');
		if (page) page.dataset.realSection = '';
		await renderIfBacked();
	}

	function bindActions(): void {
		const page = document.getElementById('page');
		if (!page) return;
		page.querySelectorAll<HTMLElement>('[data-real-submit]').forEach((b) => {
			b.onclick = (ev) => {
				ev.preventDefault();
				ev.stopPropagation();
				const s = $session;
				if (s.active) void submitForm((b.getAttribute('data-real-submit') ?? ''), s.active.workspace_id);
			};
		});
		page.querySelectorAll<HTMLElement>('[data-real-action]').forEach((b) => {
			b.onclick = (ev) => {
				ev.preventDefault();
				ev.stopPropagation();
				void runAction(b.getAttribute('data-real-action') ?? '');
			};
		});
	}

	onMount(async () => {
		document.addEventListener('click', onPageClick, true);
		await loadSession(preferredKind);
		// Render after module scripts have produced their section.
		const obs = new MutationObserver(() => {
			const page = document.getElementById('page');
			if (page && !page.dataset.realSection && currentSection) void renderIfBacked();
		});
		const page0 = document.getElementById('page');
		if (page0) obs.observe(page0, { childList: true });
		// Initial section from the module's default page.
		setTimeout(() => { if (!currentSection) currentSection = 'dashboard'; }, 600);
	});
</script>

<style>
	.real-form {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 12px 0;
	}
	.real-form .form-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.real-form-msg {
		font-size: 12px;
		opacity: 0.8;
	}
</style>
