<script lang="ts">
	/**
	 * OperationTracker — submits an intent and follows the real operation
	 * through the control plane (202 -> poll operation status -> terminal
	 * state). No simulated progress: states come from GET /operations/{id}.
	 */
	import { api, ApiFailure, type OperationStatus } from '$lib/api';

	type Phase = 'idle' | 'submitting' | 'tracking' | 'succeeded' | 'failed' | 'rejected' | 'error';

	let { actionId, resourceId, label, onDone = () => {}, disabled = false } = $props<{
		actionId: string;
		resourceId: string;
		label: string;
		onDone?: (op: OperationStatus) => void;
		disabled?: boolean;
	}>();

	let phase = $state<Phase>('idle');
	let operation = $state<OperationStatus | null>(null);
	let errorText = $state('');
	let correlationId = $state('');

	const busy = $derived(phase === 'submitting' || phase === 'tracking');

	function friendlyFailure(status: number, message: string): string {
		if (status === 403) return 'Not permitted for your role in this workspace.';
		if (status === 404) return 'Target not found in this workspace.';
		if (status === 409) return 'Conflicting change already in progress.';
		if (status === 422) return 'The request was rejected by validation.';
		if (status === 429) return 'Too many concurrent operations; retry shortly.';
		return message || 'Operation unavailable. Try again shortly.';
	}

	async function newIdempotencyKey(): Promise<string> {
		const bytes = new Uint8Array(16);
		crypto.getRandomValues(bytes);
		return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
	}

	async function poll(opId: string): Promise<void> {
		for (let attempt = 0; attempt < 60; attempt++) {
			await new Promise((r) => setTimeout(r, attempt < 5 ? 400 : 1200));
			try {
				operation = await api.operation(opId);
			} catch {
				continue; // transient lookup failure: keep tracking
			}
			if (operation.state === 'succeeded') {
				phase = 'succeeded';
				onDone(operation);
				return;
			}
			if (operation.state === 'failed' || operation.state === 'rejected') {
				phase = operation.state as Phase;
				return;
			}
		}
		phase = 'error';
		errorText = 'Operation still running; its result will appear in activity.';
	}

	export async function run(params?: Record<string, unknown>): Promise<void> {
		if (busy || disabled) return;
		errorText = '';
		operation = null;
		correlationId = '';
		phase = 'submitting';
		try {
			const acc = await api.submitIntent(resourceId, { action_id: actionId, params }, await newIdempotencyKey());
			correlationId = acc.correlation_id;
			phase = 'tracking';
			await poll(acc.operation_id);
		} catch (e) {
			phase = 'error';
			errorText =
				e instanceof ApiFailure
					? friendlyFailure(e.status, e.body?.message ?? '')
					: 'Operation unavailable. Try again shortly.';
		}
	}
</script>

<div class="op-tracker" role="status" aria-live="polite">
	{#if phase === 'idle'}
		<button class="btn primary" {disabled} onclick={() => run()}>{label}</button>
	{:else if busy}
		<span class="spinner" aria-hidden="true"></span>
		<span>{phase === 'submitting' ? 'Submitting…' : 'Tracking operation…'}</span>
	{:else}
		<span class="badge {phase === 'succeeded' ? 'success' : 'danger'}">
			{phase === 'succeeded' ? 'Completed' : phase === 'error' ? 'Tracking lost' : phase}
		</span>
		{#if correlationId}<code class="op-corr">{correlationId}</code>{/if}
		{#if errorText}<span class="op-error">{errorText}</span>{/if}
		<button class="btn secondary" onclick={() => (phase = 'idle')}>Run again</button>
	{/if}
</div>
