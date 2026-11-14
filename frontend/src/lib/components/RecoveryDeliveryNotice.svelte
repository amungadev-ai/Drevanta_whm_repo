<script lang="ts">
	/**
	 * RecoveryDeliveryNotice — G40-S3 §3: the control plane issues real
	 * recovery challenges, but out-of-band SMTP transport lands with the
	 * messaging wave (G34; LogMailer no-op). The approved "check your
	 * email" copy must not imply a delivery that cannot happen yet, so
	 * this typed notice states the real state on the approved page.
	 */
</script>

<div class="s1-notice recovery-delivery-notice" data-testid="recovery-delivery-notice" role="status">
	<strong>Delivery status (real):</strong> this installation has no SMTP
	transport configured, so no email is sent. The recovery request was
	recorded by the control plane; recovery material is retrievable by the
	server operator until email delivery is enabled (G34 messaging wave).
</div>

<style>
	.recovery-delivery-notice {
		margin: 12px 0;
	}
</style>
