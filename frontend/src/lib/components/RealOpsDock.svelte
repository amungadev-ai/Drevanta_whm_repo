<script lang="ts">
	/**
	 * RealOpsDock — reachable live-operations surface on persona screens:
	 * real resources + real intent submission with operation tracking, and
	 * the real scope audit feed. Additive control (launcher) using approved
	 * token classes; recorded in the migration ledger for owner review.
	 */
	import { session } from '$lib/stores/session';
	import ResourcesPanel from '$components/ResourcesPanel.svelte';
	import AuditFeed from '$components/AuditFeed.svelte';

	let open = $state(false);
	let tab = $state<'resources' | 'activity'>('resources');
	const scopeId = $derived($session.active?.workspace_id ?? '');
	// Permission-aware defaults from the governed vocabulary (0002 seeds):
	// server.owner hosts host.* / storage.* actions; website.owner file.apply.
	const flowByKind = $derived(
		$session.active?.kind === 'server'
			? { type: 'host snapshot', action: 'host.inspect', label: 'Inspect host' }
			: $session.active?.kind === 'website'
				? { type: 'filesystem entry', action: 'file.apply', label: 'Publish change' }
				: $session.active?.kind === 'developer_project'
					? { type: 'environment', action: 'runtime.toolchain.configure', label: 'Configure runtime' }
					: { type: 'hosting account', action: '', label: '' } // hosting: real resource.link; account.provision is service-level (no intent verb yet)
	);
</script>

{#if scopeId}
	<button
		class="real-ops-launcher btn primary"
		aria-expanded={open}
		aria-controls="realOpsPanel"
		onclick={() => (open = !open)}
	>
		{open ? 'Close live operations' : 'Live operations'}
	</button>
	{#if open}
		<aside id="realOpsPanel" class="real-ops-panel card" aria-label="Live operations">
			<div class="ops-tabs" role="tablist">
				<button role="tab" aria-selected={tab === 'resources'} class:active={tab === 'resources'} onclick={() => (tab = 'resources')}>Resources</button>
				<button role="tab" aria-selected={tab === 'activity'} class:active={tab === 'activity'} onclick={() => (tab = 'activity')}>Activity</button>
			</div>
			{#if tab === 'resources'}
				<ResourcesPanel scopeId={scopeId} resourceType={flowByKind.type} actionId={flowByKind.action} actionLabel={flowByKind.label} />
			{:else}
				<AuditFeed {scopeId} />
			{/if}
		</aside>
	{/if}
{/if}

<style>
	.real-ops-launcher {
		position: fixed;
		right: 16px;
		bottom: 16px;
		z-index: 55;
	}
	.real-ops-panel {
		position: fixed;
		right: 16px;
		bottom: 64px;
		width: min(480px, calc(100vw - 32px));
		max-height: min(70vh, 560px);
		overflow: auto;
		z-index: 56;
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 16px;
	}
	.ops-tabs {
		display: flex;
		gap: 4px;
	}
	.ops-tabs button {
		border: 1px solid var(--line, #e2e8f0);
		background: transparent;
		padding: 6px 12px;
		border-radius: 8px;
		cursor: pointer;
	}
	.ops-tabs button.active {
		background: var(--accent-soft, rgba(45, 212, 191, 0.12));
	}
</style>
