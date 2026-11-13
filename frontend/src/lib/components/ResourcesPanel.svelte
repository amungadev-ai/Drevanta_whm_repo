<script lang="ts">
	/**
	 * ResourcesPanel — real resources of a scope (GET /resources?scope_id)
	 * with a real change flow: file.apply intent through OperationTracker.
	 */
	import { onMount } from 'svelte';
	import { api, type ResourceItem } from '$lib/api';
	import OperationTracker from '$components/OperationTracker.svelte';

	let { scopeId, actionId = 'file.apply', actionLabel = 'Apply change', resourceType = 'filesystem entry' } = $props<{
		scopeId: string;
		actionId?: string;
		actionLabel?: string;
		resourceType?: string;
	}>();

	let items = $state<ResourceItem[]>([]);
	let loading = $state(true);
	let errorText = $state('');
	let selected = $state<ResourceItem | null>(null);
	let newName = $state('');
	let newType = $state(resourceType);
	let creating = $state(false);
	let createError = $state('');

	const RESOURCE_TYPES = [
		'filesystem entry', 'vhost', 'hostname binding', 'SQL database', 'application',
		'environment', 'host snapshot', 'backup artifact', 'volume', 'schedule'
	];

	async function registerResource(): Promise<void> {
		if (!newName.trim() || creating) return;
		creating = true;
		createError = '';
		try {
			await api.createResource(scopeId, { name: newName.trim(), type: newType });
			newName = '';
			await refresh();
		} catch (e) {
			createError = e instanceof Error && e.message.includes('403')
				? 'Resource registration not permitted for your role.'
				: 'Registration unavailable. Try again shortly.';
		} finally {
			creating = false;
		}
	}

	async function refresh(): Promise<void> {
		loading = true;
		errorText = '';
		try {
			items = (await api.resources(scopeId)).items;
			if (!selected && items.length > 0) selected = items[0];
		} catch {
			errorText = 'Resources unavailable.';
		} finally {
			loading = false;
		}
	}

	const drift = (r: ResourceItem) => r.desired_generation - r.observed_generation;

	onMount(refresh);
</script>

<div class="resources-panel">
	<div class="register-row">
		<input
			class="field-input"
			placeholder="New resource name"
			aria-label="New resource name"
			bind:value={newName}
			disabled={creating}
		/>
		<select class="palette-select" aria-label="Resource type" bind:value={newType} disabled={creating}>
			{#each RESOURCE_TYPES as t}<option value={t}>{t}</option>{/each}
		</select>
		<button class="btn secondary" disabled={creating || !newName.trim()} onclick={registerResource}>
			{creating ? 'Registering…' : 'Register resource'}
		</button>
	</div>
	{#if createError}<span class="op-error">{createError}</span>{/if}
	{#if loading}
		<span class="spinner" aria-hidden="true"></span>
	{:else if errorText}
		<span class="op-error">{errorText}</span>
	{:else if items.length === 0}
		<p class="muted">No resources in this workspace yet.</p>
	{:else}
		<table class="data-table">
			<thead>
				<tr><th>Resource</th><th>Type</th><th>Desired</th><th>Observed</th><th>State</th></tr>
			</thead>
			<tbody>
				{#each items as r (r.resource_id)}
					<tr class:selected={selected?.resource_id === r.resource_id}>
						<td><code>{r.name}</code></td>
						<td>{r.type}</td>
						<td>{r.desired_generation}</td>
						<td>{r.observed_generation}</td>
						<td>
							{#if drift(r) > 0}
								<span class="badge warn">converging (+{drift(r)})</span>
							{:else}
								<span class="badge success">in sync</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		<div class="resource-actions">
			<select
				class="palette-select"
				aria-label="Choose resource"
				onchange={(e) => (selected = items.find((i) => i.resource_id === e.currentTarget.value) ?? null)}
			>
				<option value="" disabled selected={selected === null}>Choose a resource…</option>
				{#each items as r (r.resource_id)}
					<option value={r.resource_id}>{r.name} ({r.type})</option>
				{/each}
			</select>
			{#if selected}
				<OperationTracker
					actionId={actionId}
					resourceId={selected.resource_id}
					label={actionLabel}
					onDone={refresh}
				/>
			{/if}
		</div>
	{/if}
</div>

<style>
	.resources-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.resource-actions {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.register-row {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}
	.register-row .field-input {
		flex: 1 1 140px;
	}
	tr.selected :global(td) {
		background: var(--accent-soft, rgba(45, 212, 191, 0.08));
	}
</style>
