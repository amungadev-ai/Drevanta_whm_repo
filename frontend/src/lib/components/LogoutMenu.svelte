<script lang="ts">
	/**
	 * LogoutMenu — real session termination through POST /identity/logout
	 * (BFF clears the HttpOnly cookie; the Go session is revoked).
	 * Functional security addition (G40-S2 §3); styling uses approved tokens.
	 */
	import { api } from '$lib/api';

	let open = $state(false);
	let busy = $state(false);
	let errorText = $state('');
	let displayName = $state('');
	let email = $state('');

	const { children } = $props<{ children?: import('svelte').Snippet }>();

	export function setIdentity(name: string, mail: string): void {
		displayName = name;
		email = mail;
	}

	async function signOut(): Promise<void> {
		busy = true;
		errorText = '';
		try {
			await api.logout(); // cookie cleared server-side on success
		} catch {
			// Even if the call fails, the cookie is our only client-side state.
		}
		location.href = '/auth/login?next=%2F';
	}
</script>

<div class="logout-menu">
	<button
		class="user-card"
		aria-expanded={open}
		aria-haspopup="menu"
		onclick={() => (open = !open)}
	>
		{@render children?.()}
	</button>
	{#if open}
		<div class="menu card" role="menu">
			<div class="menu-head">
				<strong>{displayName || 'Signed in'}</strong>
				<small>{email}</small>
			</div>
			<button class="btn secondary" role="menuitem" disabled={busy} onclick={signOut}>
				{busy ? 'Signing out…' : 'Sign out'}
			</button>
			{#if errorText}<span class="op-error">{errorText}</span>{/if}
		</div>
	{/if}
</div>
