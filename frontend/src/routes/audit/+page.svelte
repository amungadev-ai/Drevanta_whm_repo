<svelte:head>
	<title>Capability Coverage · Drevanta</title>
	<link rel="stylesheet" href="/css/ui__styles.css">
	<link rel="stylesheet" href="/css/ui__accessible-palettes.css">
	<link rel="stylesheet" href="/css/capability-audit.css">
</svelte:head>

<!-- Verbatim port of the approved prototype body (directive §2). -->
<!-- Visual structure, classes and DOM ids are unchanged. -->
{@html markup}

<script lang="ts">
	import { onMount } from 'svelte';
	const markup = "<div class=\"audit-wrap\"><header class=\"audit-head\"><div><div class=\"audit-eyebrow\">DREVANTA / ARCHITECTURE REVIEW</div><h1>Capability & service coverage</h1><p>47 proposed capabilities across four separately authorized workspaces \u00b7 v0.2 review baseline</p></div><a href=\"index.html\" class=\"audit-return\">\u2190 Back to Hosting</a></header><nav class=\"audit-workspaces\" aria-label=\"Workspace navigation\"><a href=\"server-admin.html\">Server Administration</a><a href=\"index.html\">Hosting</a><a href=\"website-owner.html\">Website Owner</a><a href=\"developer.html\">Developer</a></nav><div class=\"audit-notice\"><strong>Architecture proposal \u2014 not installed functionality.</strong> Scope letters represent maximum intended boundaries, not access grants. All operations still require installed + enabled + entitled + authorized + operationally admissible checks. No service is installed by this page.</div><div class=\"audit-stats\" id=\"auditStats\"></div><section class=\"audit-panel\"><div class=\"audit-toolbar\"><label>Search capabilities<input id=\"auditSearch\" type=\"search\" placeholder=\"e.g. PHP, email, billing, shell\" autocomplete=\"off\"></label><label>Release<select id=\"auditRelease\"><option value=\"\">All releases</option><option>S0</option><option>S1</option><option>S2</option><option>S3</option><option>S4</option></select></label><label>Workspace<select id=\"auditPersona\"><option value=\"\">All workspaces</option><option value=\"4\">Server</option><option value=\"5\">Hosting</option><option value=\"6\">Website</option><option value=\"7\">Developer</option></select></label><label>Domain<select id=\"auditDomain\"><option value=\"\">All domains</option></select></label></div><div class=\"audit-table-wrap\"><table class=\"audit-table\"><thead><tr><th>Capability</th><th>Delivery</th><th>Release</th><th title=\"Server\">Server</th><th title=\"Hosting\">Hosting</th><th title=\"Website\">Website</th><th title=\"Developer\">Dev</th></tr></thead><tbody id=\"auditRows\"></tbody></table></div><div class=\"audit-foot\"><span id=\"auditCount\"></span><a href=\"Platform_Capability_Registry_v0_2.csv\" download>Download registry CSV</a></div></section><section class=\"audit-detail\" id=\"auditDetail\" aria-live=\"polite\"><h2>Choose a capability</h2><p>Select a registry row to inspect its proposed service contract and release gates.</p></section><section class=\"audit-panel audit-gates\"><h2>Release gates</h2><div class=\"audit-gate-grid\"><div><strong>S0 \u00b7 Secure foundation</strong><p>Tenant isolation, helper audit, power-loss recovery.</p></div><div><strong>S1 \u00b7 Classic hosting</strong><p>Restore drill, domain claim tests, resource-pressure benchmarks.</p></div><div><strong>S2 \u00b7 Modern hosting + commerce</strong><p>Build sandbox, payment reconciliation, support isolation. Scoped terminal remains conditional on tested isolation.</p></div><div><strong>S3 \u00b7 Optional services</strong><p>Mail deliverability prerequisites, template provenance and upgrade tests.</p></div><div><strong>S4 \u00b7 Advanced scale</strong><p>Distributed reconciliation, cross-node identity, workload admission.</p></div></div></section><footer class=\"audit-footer\">Proposed v0.2 interpretation \u00b7 The 1,284-occurrence third-party manifest index was not supplied as a standalone file, so it is not reconstructed or claimed here.</footer></div>";
	// Execute the ported modules in original order after mount, exactly as
	// the prototype does after DOM parse. Auth/onboarding modules carry
	// real API seam patches (tools/ui-migration/wire_auth.py).
	const modules: string[] = ["/js/capability-audit.js"];
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
