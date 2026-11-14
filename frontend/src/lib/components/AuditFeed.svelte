<script lang="ts">
	/**
	 * AuditFeed — real scope audit events (GET /scopes/{id}/audit; requires
	 * audit.read on the scope — the Go API enforces it per role).
	 */
	import { onMount } from 'svelte';
	import { api, type AuditEvent } from '$lib/api';

	let { scopeId, limit = 8 } = $props<{ scopeId: string; limit?: number }>();

	let events = $state<AuditEvent[]>([]);
	let loading = $state(true);
	let errorText = $state('');

	onMount(async () => {
		try {
			const res = await api.scopeAudit(scopeId);
			events = res.events.slice(0, limit);
		} catch (e) {
			errorText = e instanceof Error && e.message.includes('403') ? 'Audit read not permitted for your role.' : 'Activity unavailable.';
		} finally {
			loading = false;
		}
	});

	const outcomeBadge = (o: unknown) =>
		o === 'allow' ? 'success' : o === 'deny' || o === 'error' ? 'danger' : 'muted';
</script>

<div class="audit-feed">
	{#if loading}
		<span class="spinner" aria-hidden="true"></span>
	{:else if errorText}
		<span class="op-error">{errorText}</span>
	{:else if events.length === 0}
		<p class="muted">No recorded activity in this workspace yet.</p>
	{:else}
		<ul class="audit-list">
			{#each events as ev (ev.event_id ?? ev.created_at ?? Math.random())}
				<li>
					<span class="badge {outcomeBadge(ev.outcome)}">{ev.outcome ?? 'event'}</span>
					<code>{ev.action_id ?? '—'}</code>
					<small>{ev.safe_reason ?? ''}</small>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.audit-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.audit-list li {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.muted {
		opacity: 0.7;
	}
</style>
