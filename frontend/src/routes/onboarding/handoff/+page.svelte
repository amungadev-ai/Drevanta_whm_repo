<svelte:head>
	<title>Setup complete · Drevanta</title>
	<link rel="stylesheet" href="/css/onboarding__styles.css">
	<link rel="stylesheet" href="/css/onboarding__accessible-palettes.css">
	<link rel="stylesheet" href="/css/auth-flow.css">
</svelte:head>

<!-- Verbatim port of the approved prototype body (directive §2). -->
<!-- Visual structure, classes and DOM ids are unchanged. -->
{@html markup}

<script lang="ts">
	import { onMount } from 'svelte';
	const markup = "<div id=\"authApp\"></div>";
	// Execute the ported modules in original order after mount, exactly as
	// the prototype does after DOM parse. Auth/onboarding modules carry
	// real API seam patches (tools/ui-migration/wire_auth.py).
	const modules: string[] = ["/js/auth-flow.js"];
	const bodyAttrs: Record<string,string> = {"route": "handoff"};
	onMount(() => {
		for (const [k, v] of Object.entries(bodyAttrs)) document.body.setAttribute('data-' + k, v);
		for (const src of modules) {
			const el = document.createElement('script');
			el.src = src;
			// Preserve the prototype's parser-ordered execution: dynamic
			// classic scripts default to async, which breaks modules that
			// rely on earlier scripts' top-level globals (e.g. escapeHTML).
			el.async = false;
			document.body.appendChild(el);
		}
	});
</script>
