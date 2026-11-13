<svelte:head>
	<title>S1 Service Center · Drevanta</title>
	<link rel="stylesheet" href="/css/ui__styles.css">
	<link rel="stylesheet" href="/css/ui__accessible-palettes.css">
	<link rel="stylesheet" href="/css/s1-services.css">
</svelte:head>

<!-- Verbatim port of the approved prototype body (directive §2). -->
<!-- Visual structure, classes and DOM ids are unchanged. -->
{@html markup}

<script lang="ts">
	import { onMount } from 'svelte';
	const markup = "<main class=\"s1-layout\"><div class=\"s1-top\"><div><div class=\"s1-crumb\"><a href=\"index.html\">Drevanta</a> / <a href=\"capability-audit.html\">Capability audit</a> / S1 services</div><h1>S1 \u00b7 Classic Hosting Service Center</h1><p class=\"s1-muted\">Persona-specific workflows, prerequisites and service lifecycle previews</p></div><a href=\"capability-audit.html\">View all 47 capabilities</a></div><nav id=\"s1Personas\" class=\"s1-links\" aria-label=\"Workspace context\"></nav><div class=\"s1-notice\"><strong>Architecture prototype \u00b7 no live operations.</strong> These cards describe proposed S1 service contracts. No package installation, DNS change, certificate issuance, file access, database creation, backup or job execution occurs. Workspace switching here previews scopes; it does not grant permissions.</div><div id=\"s1Summary\" aria-live=\"polite\"></div><div class=\"s1-filter\"><input id=\"s1Search\" type=\"search\" aria-label=\"Search S1 capabilities\" placeholder=\"Search S1 capabilities, e.g. PHP or backup\"><button type=\"button\" id=\"s1All\" aria-pressed=\"true\">All S1</button><button type=\"button\" id=\"s1Action\" aria-pressed=\"false\">My operational scope</button></div><div id=\"s1Grid\" class=\"s1-grid\"></div><section id=\"s1Detail\" class=\"s1-detail\" hidden tabindex=\"-1\" aria-live=\"polite\"></section></main>";
	// Execute the ported modules in original order after mount, exactly as
	// the prototype does after DOM parse. Auth/onboarding modules carry
	// real API seam patches (tools/ui-migration/wire_auth.py).
	const modules: string[] = ["/js/s1-services.js"];
	const bodyAttrs: Record<string,string> = {};
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
