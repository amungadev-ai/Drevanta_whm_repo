<script lang="ts">
	/**
	 * ServiceCenterScope — G40-S3 §1/§3: binds the S1 service-center
	 * persona scope view to the session's REAL workspace memberships
	 * (GET /workspaces). The approved catalog, filters and interactions
	 * are untouched; this adds the real authorization context so the
	 * "My operational scope" view can never imply a membership the
	 * control plane does not hold.
	 */
	import { onMount } from 'svelte';
	import { loadSession, session } from '$lib/stores/session';
	import { api } from '$lib/api';

	let { preferredKind = '' }: { preferredKind?: string } = $props();

	const personaOfKind: Record<string, string> = {
		server: 'server',
		hosting_org: 'hosting',
		website: 'website',
		developer_project: 'developer'
	};
	const personaLabel: Record<string, string> = {
		server: 'Server Administrator',
		hosting: 'Hosting Provider',
		website: 'Website Owner',
		developer: 'Developer'
	};

	let membershipPersonas = $state<string[]>([]);
	let currentPersona = $state('');
	let noticeEl = $state<HTMLDivElement | null>(null);

	onMount(async () => {
		await loadSession(preferredKind);
		currentPersona = new URLSearchParams(location.search).get('persona') ?? 'hosting';
		const s = $session;
		if (s.workspaces.length) {
			membershipPersonas = [
				...new Set(s.workspaces.map((w) => personaOfKind[w.kind]).filter(Boolean))
			];
		}
		// Keep the notice inside the page's main landmark (region rule).
		if (noticeEl) {
			const main = document.querySelector('main');
			if (main && noticeEl.parentElement !== main) main.prepend(noticeEl);
		}
	});
</script>

<div bind:this={noticeEl} class="s1-notice real-scope-notice" data-testid="real-scope-notice">
	<strong>Active operational scope (live control plane):</strong>
	{#if membershipPersonas.length === 0}
		no active workspace memberships — the persona views below describe role
		contracts only and grant nothing.
	{:else}
		{membershipPersonas.map((p) => personaLabel[p] ?? p).join(', ')}. Other
		persona views describe contracts for roles this identity does not hold.
	{/if}
</div>

<style>
	.real-scope-notice {
		margin: 8px 0 16px;
	}
</style>
