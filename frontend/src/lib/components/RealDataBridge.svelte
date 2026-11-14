<script lang="ts">
	/**
	 * RealDataBridge — mounts on persona routes, loads the REAL session
	 * (principal + workspaces) and binds it into the approved shell:
	 * workspace identity in the sidebar, crumb, and the real sign-out in
	 * the account surface. Real values replace mock content in the
	 * approved slots only; structure and classes are untouched.
	 */
	import { onMount } from 'svelte';
	import { loadSession, session } from '$lib/stores/session';
	import { accountMenuOpen } from '$lib/stores/ui';
	import { api, routeForWorkspaceKind } from '$lib/api';

	let { preferredKind = '' } = $props<{ preferredKind?: string }>();

	let signingOut = $state(false);
	// Real MFA enrollment state for the signed-in principal (G40-S3 §3).
	let mfaEnrolled = $state<boolean | null>(null);

	const initials = (name: string) =>
		name
			.split(/\s+/)
			.map((p) => p[0] ?? '')
			.join('')
			.slice(0, 2)
			.toUpperCase() || '·';

	function roleLabel(kind: string): string {
		switch (kind) {
			case 'hosting_org':
				return 'Hosting organization';
			case 'server':
				return 'Server administration';
			case 'website':
				return 'Website workspace';
			case 'developer_project':
				return 'Developer workspace';
			default:
				return kind;
		}
	}

	function bindShell(): void {
		const s = $session;
		if (!s.active || !s.me) return;
		// Sidebar workspace identity (approved slot: .workspace strong/small).
		const ws = document.querySelector<HTMLElement>('.sidebar .workspace');
		if (ws) {
			const strong = ws.querySelector('strong');
			const small = ws.querySelector('small');
			if (strong) strong.textContent = s.active.name;
			if (small) small.textContent = roleLabel(s.active.kind);
		}
		// Workspace icon initial.
		const icon = document.querySelector<HTMLElement>('.sidebar .workspace-icon');
		if (icon) icon.textContent = s.active.name.slice(0, 1).toUpperCase();
		// Crumb title follows the active nav section title if present.
		const crumb = document.getElementById('crumbTitle');
		if (crumb && !crumb.textContent?.trim()) crumb.textContent = s.active.name;
		// User card identity. Click ownership lives in ModuleShellControls
		// (capture-phase takeover of the module handlers, shared menu state).
		const card = document.getElementById('userMenu');
		if (card) {
			const strong = card.querySelector('strong');
			const small = card.querySelector('small');
			const avatar = card.querySelector('.avatar');
			if (strong) strong.textContent = s.me.display_name || s.me.email;
			if (small) small.textContent = s.active.role || '';
			if (avatar) avatar.textContent = initials(s.me.display_name || s.me.email);
		}
	}

	async function switchWorkspace(kind: string): Promise<void> {
		location.href = routeForWorkspaceKind(kind);
	}

	async function signOut(): Promise<void> {
		signingOut = true;
		try {
			await api.logout();
		} catch {
			// Cookie is the only client-side session state.
		}
		accountMenuOpen.set(false);
		location.href = '/auth/login?next=%2F';
	}

	onMount(async () => {
		await loadSession(preferredKind);
		bindShell();
		if ($session.active) {
			try {
				mfaEnrolled = (await api.mfaStatus()).enrolled;
			} catch {
				mfaEnrolled = null; // real failure state; never claim an outcome
			}
		}
	});
</script>

{#if $accountMenuOpen && $session.me}
	<!-- Real account menu (approved token classes; security-functional addition). -->
	<div class="menu card real-account-menu" role="menu" aria-label="Account">
		<div class="menu-head">
			<strong>{$session.me.display_name || $session.me.email}</strong>
			<small>{$session.me.email}</small>
		</div>
		{#if $session.workspaces.length > 1}
			<div class="menu-section">Switch workspace</div>
			{#each $session.workspaces.filter((w) => w.workspace_id !== $session.active?.workspace_id) as w (w.workspace_id)}
				<button class="btn secondary" role="menuitem" onclick={() => switchWorkspace(w.kind)}>
					{w.name}
				</button>
			{/each}
		{/if}
		<div class="menu-section" role="menuitem" aria-live="polite">
			Multi-factor authentication:
			{#if mfaEnrolled === null}
				status unavailable
			{:else if mfaEnrolled}
				enrolled
			{:else}
				not enrolled
			{/if}
		</div>
		<button class="btn primary" role="menuitem" disabled={signingOut} onclick={signOut}>
			{signingOut ? 'Signing out…' : 'Sign out'}
		</button>
	</div>
{/if}

<style>
	.real-account-menu {
		position: fixed;
		right: 16px;
		top: 56px;
		z-index: 60;
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-width: 220px;
		padding: 12px;
	}
	.menu-head {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding-bottom: 8px;
		border-bottom: 1px solid var(--line, #e2e8f0);
	}
	.menu-section {
		font-size: 11px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		opacity: 0.7;
	}
</style>
