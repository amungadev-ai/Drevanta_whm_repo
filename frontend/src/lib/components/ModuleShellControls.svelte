<script lang="ts">
	/**
	 * ModuleShellControls — G40-S3 typed ownership of the approved shell
	 * controls on module-ported persona pages (the 42 PARTIAL matrix rows).
	 *
	 * The approved markup is untouched; this component takes over the
	 * BEHAVIOR of each control with real implementations and preempts the
	 * prototype's demonstration handlers via capture-phase listeners:
	 *
	 *  - Skip to main content: native anchor focus (verified in e2e, G9).
	 *  - Open navigation: drawer with aria-expanded sync, Escape close and
	 *    focus management (G9 keyboard evidence).
	 *  - Palette select / theme toggle: the approved client-side theme
	 *    engine, owned here with localStorage persistence.
	 *  - S1 services / Capability audit links: rewritten from the
	 *    prototype's static .html targets (which do not exist in the
	 *    production build) to the real SvelteKit routes.
	 *  - Search (incl. Ctrl/Cmd+K): real command palette over the session's
	 *    REAL workspaces (GET /workspaces) and REAL resources
	 *    (GET /resources?scope_id=…) plus in-app navigation commands.
	 *    No simulated records.
	 *  - Notifications: real recent audit events for the active workspace
	 *    (GET /scopes/{id}/audit); an empty history renders a real empty
	 *    state, never invented entries.
	 *
	 * The account identity inside #userMenu is bound by RealDataBridge.
	 */
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { loadSession, session } from '$lib/stores/session';
	import { accountMenuOpen } from '$lib/stores/ui';
	import { api } from '$lib/api';
	import type { WorkspaceItem, ResourceItem, AuditEvent } from '$lib/api';

	let { preferredKind = '', paletteMode = 'palette' }: { preferredKind?: string; paletteMode?: 'palette' | 'native' } =
		$props();

	// Real command palette state.
	let paletteOpen = $state(false);
	let paletteQuery = $state('');
	let paletteWorkspaces = $state<WorkspaceItem[]>([]);
	let paletteResources = $state<ResourceItem[]>([]);
	let paletteIndex = $state(0);

	// Real notifications state.
	let notifOpen = $state(false);
	let notifEvents = $state<AuditEvent[]>([]);
	let notifLoaded = $state(false);

	let lastFocus: HTMLElement | null = null;

	const Q = (sel: string): HTMLElement | null => document.querySelector(sel);

	// ---------- capture takeover: module handlers never run ----------
	function captureClick(ev: MouseEvent): void {
		const t = ev.target as HTMLElement | null;
		if (!t) return;
		const hit = (sel: string): HTMLElement | null => (t.closest(sel) as HTMLElement | null) ?? null;

		const userMenu = hit('#userMenu');
		if (userMenu) {
			ev.preventDefault();
			ev.stopImmediatePropagation();
			accountMenuOpen.update((v) => !v);
			return;
		}
		if ($accountMenuOpen && !hit('.real-account-menu')) {
			accountMenuOpen.set(false);
		}
		const menu = hit('#menuButton');
		if (menu) {
			ev.preventDefault();
			ev.stopImmediatePropagation();
			toggleDrawer();
			return;
		}
		const backdrop = hit('#drawerBackdrop');
		if (backdrop && !backdrop.hidden) {
			ev.preventDefault();
			ev.stopImmediatePropagation();
			closeDrawer();
			return;
		}
		const theme = hit('#themeButton');
		if (theme) {
			ev.preventDefault();
			ev.stopImmediatePropagation();
			toggleTheme();
			return;
		}
		if (paletteMode === 'palette') {
			const search = hit('#searchButton');
			if (search) {
				ev.preventDefault();
				ev.stopImmediatePropagation();
				openPalette();
				return;
			}
		}
		const notif = hit('#notificationButton');
		if (notif) {
			ev.preventDefault();
			ev.stopImmediatePropagation();
			toggleNotifications();
			return;
		}
	}

	function captureKeydown(ev: KeyboardEvent): void {
		if (ev.key === 'Escape') {
			if ($accountMenuOpen) {
				ev.preventDefault();
				ev.stopImmediatePropagation();
				accountMenuOpen.set(false);
				return;
			}
			if (paletteOpen) {
				ev.preventDefault();
				ev.stopImmediatePropagation();
				closePalette();
				return;
			}
			if (notifOpen) {
				ev.preventDefault();
				ev.stopImmediatePropagation();
				closeNotifications();
				return;
			}
			const sidebar = Q('#sidebar');
			if (sidebar?.classList.contains('open')) {
				ev.preventDefault();
				ev.stopImmediatePropagation();
				closeDrawer();
				return;
			}
			return;
		}
		if (paletteOpen && (ev.key === 'ArrowDown' || ev.key === 'ArrowUp' || ev.key === 'Enter')) {
			const results = paletteResults();
			if (!results.length) return;
			ev.preventDefault();
			ev.stopImmediatePropagation();
			if (ev.key === 'ArrowDown') paletteIndex = Math.min(paletteIndex + 1, results.length - 1);
			else if (ev.key === 'ArrowUp') paletteIndex = Math.max(paletteIndex - 1, 0);
			else results[Math.min(paletteIndex, results.length - 1)]?.el.click();
			paletteIndex = Math.min(paletteIndex, Math.max(results.length - 1, 0));
			return;
		}
		if (paletteMode === 'palette' && (ev.ctrlKey || ev.metaKey) && ev.key.toLowerCase() === 'k') {
			ev.preventDefault();
			ev.stopImmediatePropagation();
			if (paletteOpen) closePalette();
			else openPalette();
			return;
		}
		if (paletteMode === 'palette' && ev.key === 'Tab' && paletteOpen) trapFocus(ev, '.command[role="dialog"]');
		if ((paletteOpen || notifOpen) && ev.key === 'Tab' && !paletteOpen) {
			trapFocus(ev, '.real-notif-menu');
		}
	}

	function captureChange(ev: Event): void {
		const t = ev.target as HTMLSelectElement | null;
		if (!t || t.id !== 'paletteSelect') return;
		ev.preventDefault();
		ev.stopImmediatePropagation();
		localStorage.setItem('drevanta-palette', t.value);
		applyPalette(t.value);
	}

	function trapFocus(ev: KeyboardEvent, containerSel: string): void {
		const box = document.querySelector(containerSel);
		if (!box) return;
		const focusables = Array.from(
			box.querySelectorAll<HTMLElement>('button:not([disabled]),input,a[href]')
		).filter((x) => x.getClientRects().length);
		if (!focusables.length) return;
		const first = focusables[0];
		const last = focusables[focusables.length - 1];
		if (ev.shiftKey && document.activeElement === first) {
			ev.preventDefault();
			last.focus();
		} else if (!ev.shiftKey && document.activeElement === last) {
			ev.preventDefault();
			first.focus();
		}
	}

	// ---------- drawer ----------
	function toggleDrawer(): void {
		const sidebar = Q('#sidebar');
		if (!sidebar) return;
		if (sidebar.classList.contains('open')) closeDrawer();
		else openDrawer();
	}
	function openDrawer(): void {
		lastFocus = document.activeElement as HTMLElement | null;
		const sidebar = Q('#sidebar');
		const backdrop = Q('#drawerBackdrop');
		const btn = Q('#menuButton');
		sidebar?.classList.add('open');
		if (backdrop) backdrop.hidden = false;
		btn?.setAttribute('aria-expanded', 'true');
		(sidebar?.querySelector<HTMLElement>('button, a') ?? sidebar)?.focus();
	}
	function closeDrawer(): void {
		const sidebar = Q('#sidebar');
		const backdrop = Q('#drawerBackdrop');
		const btn = Q('#menuButton');
		sidebar?.classList.remove('open');
		if (backdrop) backdrop.hidden = true;
		btn?.setAttribute('aria-expanded', 'false');
		if (lastFocus?.isConnected) lastFocus.focus();
		else btn?.focus();
	}

	// ---------- theme (approved client-side engine, owned + persisted) ----------
	function applyPalette(value: string): void {
		if (value === 'default') document.documentElement.removeAttribute('data-palette');
		else document.documentElement.dataset.palette = value;
		const sel = Q('#paletteSelect') as HTMLSelectElement | null;
		if (sel) sel.value = value;
	}
	function toggleTheme(): void {
		const dark = document.documentElement.classList.toggle('dark');
		localStorage.setItem('hc-theme', dark ? 'dark' : 'light');
		const btn = Q('#themeButton');
		if (btn) btn.textContent = dark ? '☀' : '☾';
	}

	// ---------- real command palette ----------
	interface PaletteResult {
		el: HTMLButtonElement;
	}
	function paletteResults(): PaletteResult[] {
		return Array.from(document.querySelectorAll<HTMLButtonElement>('.command .command-result')).map((el) => ({ el }));
	}
	async function openPalette(): Promise<void> {
		lastFocus = document.activeElement as HTMLElement | null;
		paletteQuery = '';
		paletteIndex = 0;
		paletteOpen = true;
		await tick();
		paletteWorkspaces = [];
		paletteResources = [];
		const s = $session;
		if (s.active) {
			// Real data only; failures render real empty states.
			try {
				const ws = await api.workspaces();
				paletteWorkspaces = ws.items;
			} catch {
				paletteWorkspaces = [];
			}
			try {
				const rs = await api.resources(s.active.workspace_id);
				paletteResources = rs.items;
			} catch {
				paletteResources = [];
			}
		}
		await tick();
		document.querySelector<HTMLInputElement>('#realCommandInput')?.focus();
	}
	function closePalette(): void {
		paletteOpen = false;
		if (lastFocus?.isConnected) lastFocus.focus();
		else Q('#searchButton')?.focus();
	}
	function paletteNavCommands(): Array<{ label: string; href: string }> {
		return [
			{ label: 'Open dashboard', href: '/' },
			{ label: 'Open S1 services', href: '/server/services' },
			{ label: 'Open capability audit', href: '/audit' }
		];
	}
	function filteredNav() {
		const q = paletteQuery.toLowerCase();
		return paletteNavCommands().filter((c) => c.label.toLowerCase().includes(q));
	}
	function filteredWorkspaces() {
		const q = paletteQuery.toLowerCase();
		return paletteWorkspaces.filter((w) => `${w.name} ${w.role} ${w.kind}`.toLowerCase().includes(q));
	}
	function filteredResources() {
		const q = paletteQuery.toLowerCase();
		return paletteResources.filter((r) => `${r.name} ${r.type}`.toLowerCase().includes(q));
	}
	function paletteTotal(): number {
		return filteredNav().length + filteredWorkspaces().length + filteredResources().length;
	}
	function routeForKind(kind: string): string {
		switch (kind) {
			case 'server':
				return '/server';
			case 'hosting_org':
				return '/';
			case 'website':
				return '/website';
			case 'developer_project':
				return '/developer';
			default:
				return '/';
		}
	}

	// ---------- real notifications ----------
	async function toggleNotifications(): Promise<void> {
		if (notifOpen) {
			closeNotifications();
			return;
		}
		lastFocus = document.activeElement as HTMLElement | null;
		notifOpen = true;
		notifLoaded = false;
		notifEvents = [];
		await tick();
		const s = $session;
		if (s.active) {
			try {
				const res = await api.scopeAudit(s.active.workspace_id);
				notifEvents = (res.events ?? []).slice(0, 5);
			} catch {
				notifEvents = []; // real failure state, no invented entries
			}
		}
		notifLoaded = true;
	}
	function closeNotifications(): void {
		notifOpen = false;
		if (lastFocus?.isConnected) lastFocus.focus();
		else Q('#notificationButton')?.focus();
	}
	function fmtEvent(e: AuditEvent): string {
		const action = e.action_id ?? 'event';
		const outcome = e.outcome ?? '';
		return outcome ? `${action} · ${outcome}` : action;
	}

	// ---------- real navigation links ----------
	function rewriteLinks(): void {
		const base = document.body.dataset.persona ?? preferredKind;
		document.querySelectorAll<HTMLAnchorElement>('a.audit-top-link').forEach((a) => {
			const href = a.getAttribute('href') ?? '';
			if (href.includes('service-center.html')) {
				const persona = new URL(href, location.origin).searchParams.get('persona') ?? base;
				a.setAttribute('href', `/server/services${persona ? `?persona=${persona}` : ''}`);
			} else if (href.includes('capability-audit.html')) {
				a.setAttribute('href', '/audit');
			}
		});
		// Service-center header links (brand + audit entries).
		document.querySelectorAll<HTMLAnchorElement>('a[href*="index.html"], a[href*="capability-audit.html"]').forEach((a) => {
			const href = a.getAttribute('href') ?? '';
			if (href.includes('index.html')) a.setAttribute('href', '/');
			if (href.includes('capability-audit.html')) a.setAttribute('href', '/audit');
		});
	}

	onMount(async () => {
		document.body.dataset.persona = preferredKind || document.body.dataset.persona || '';
		await loadSession(preferredKind);
		rewriteLinks();
		// Restore the approved theme engine state before the module renders.
		const savedPalette = localStorage.getItem('drevanta-palette') ?? localStorage.getItem('hc-palette') ?? 'default';
		applyPalette(savedPalette);
		document.addEventListener('click', captureClick, true);
		document.addEventListener('keydown', captureKeydown, true);
		document.addEventListener('change', captureChange, true);
	});
</script>

{#if paletteOpen}
	<!-- Approved command palette structure/styles (app.js pattern), real data. -->
	<div class="command-backdrop" id="realCommandBackdrop">
		<section class="command" role="dialog" aria-modal="true" aria-label="Command palette">
			<input
				id="realCommandInput"
				class="input"
				placeholder="Search workspaces, resources and pages..."
				aria-label="Search commands"
				bind:value={paletteQuery}
				oninput={() => (paletteIndex = 0)}
			/>
			<div id="realCommandResults">
				{#each filteredNav() as c (c.href)}
					<button class="command-result" onclick={() => { closePalette(); goto(c.href); }}>
						{c.label}<small>Open page ↵</small>
					</button>
				{/each}
				{#each filteredWorkspaces() as w (w.workspace_id)}
					<button
						class="command-result"
						onclick={() => { closePalette(); goto(routeForKind(w.kind)); }}
					>
						{w.name} · {w.role}<small>Open workspace ↵</small>
					</button>
				{/each}
				{#each filteredResources() as r (r.resource_id)}
					<button
						class="command-result"
						onclick={() => { closePalette(); goto(routeForKind($session.active?.kind ?? '')); }}
					>
						{r.name} · {r.type}<small>Open resource ↵</small>
					</button>
				{/each}
				{#if paletteTotal() === 0}
					<div class="empty">No matching workspaces, resources or pages</div>
				{/if}
			</div>
			<div class="command-hint">↑ ↓ to navigate · Enter to open · Esc to close</div>
		</section>
	</div>
{/if}

{#if notifOpen}
	<div class="menu card real-notif-menu" role="menu" aria-label="Notifications">
		<div class="menu-section">Recent activity · live audit</div>
		{#if !notifLoaded}
			<div class="menu-empty">Loading real audit events…</div>
		{:else if notifEvents.length === 0}
			<div class="menu-empty">No recent activity for this workspace</div>
		{:else}
			{#each notifEvents as e (e.event_id ?? e.created_at ?? fmtEvent(e))}
				<div class="notif-row">
					<strong>{fmtEvent(e)}</strong>
					<small>{e.created_at ?? ''}</small>
				</div>
			{/each}
		{/if}
		<button class="btn secondary" role="menuitem" onclick={() => { closeNotifications(); goto('/audit'); }}>
			Open audit center
		</button>
	</div>
{/if}

<style>
	.real-notif-menu {
		position: fixed;
		right: 16px;
		top: 56px;
		z-index: 60;
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-width: 260px;
		max-width: 340px;
		padding: 12px;
	}
	.menu-section {
		font-size: 11px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		opacity: 0.7;
	}
	.menu-empty {
		font-size: 13px;
		opacity: 0.75;
		padding: 4px 0;
	}
	.notif-row {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 6px 0;
		border-bottom: 1px solid var(--line, #e2e8f0);
	}
	.notif-row small {
		opacity: 0.6;
		font-size: 11px;
	}
</style>
