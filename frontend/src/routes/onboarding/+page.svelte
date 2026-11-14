<svelte:head>
        <title>Drevanta · First-time setup</title>
</svelte:head>

<!-- Redirect stub: /onboarding forwards to /onboarding/administrator (the
     actual claim-consuming route that loads auth-flow.js). The hash fragment
     is preserved client-side so #clm_<token> handover still works. -->
<script lang="ts">
        import { onMount } from 'svelte';
        onMount(() => {
                location.replace('/onboarding/administrator' + location.hash);
        });
</script>

<p>Redirecting to first-time setup… <a href="/onboarding/administrator">Continue</a></p>
