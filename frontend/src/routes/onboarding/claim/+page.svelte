<svelte:head>
        <title>Continue setup · Drevanta</title>
</svelte:head>

<!-- Redirect stub: /onboarding/claim forwards to /onboarding/administrator
     (the actual claim-consuming route). Hash preserved client-side. -->
<script lang="ts">
        import { onMount } from 'svelte';
        onMount(() => {
                location.replace('/onboarding/administrator' + location.hash);
        });
</script>

<p>Redirecting to first-time setup… <a href="/onboarding/administrator">Continue</a></p>
