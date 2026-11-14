<svelte:head>
	<title>Developer Workspace · Drevanta</title>
	<link rel="stylesheet" href="/css/ui__styles.css">
	<link rel="stylesheet" href="/css/ui__accessible-palettes.css">
	<link rel="stylesheet" href="/css/phase2.css">
	<link rel="stylesheet" href="/css/hosting.css">
	<link rel="stylesheet" href="/css/personas.css">
	<link rel="stylesheet" href="/css/capability-audit.css">
</svelte:head>

<!-- Verbatim port of the approved prototype body (directive §2). -->
<!-- Visual structure, classes and DOM ids are unchanged. -->
{@html markup}

<RealDataBridge preferredKind="developer" />
<RealOpsDock />

<script lang="ts">
	import RealDataBridge from '$components/RealDataBridge.svelte';
	import RealOpsDock from '$components/RealOpsDock.svelte';
	import { onMount } from 'svelte';
	const markup = "<a class=\"skip-link\" href=\"#main\">Skip to main content</a><div id=\"app\"><aside class=\"sidebar\" id=\"sidebar\"><div class=\"brand\"><span class=\"brand-mark\" aria-hidden=\"true\">\u25a6</span><span>dre<span class=\"brand-accent\">vanta</span><small>SELF-HOSTED CONSOLE</small></span></div><div class=\"workspace\"><span class=\"workspace-icon\">D</span><span><strong>Atlas Web App</strong><small>Developer project</small></span><span class=\"chevron\">\u2304</span></div><div class=\"nav-label\">DEVELOPER WORKSPACE</div><nav id=\"nav\" aria-label=\"Main navigation\"></nav><div class=\"sidebar-bottom\"><div class=\"connection\"><span class=\"dot success\"></span><span>Demo only \u00b7 no live services</span></div><button class=\"user-card\" id=\"userMenu\"><span class=\"avatar\">AM</span><span><strong>Alex Morgan</strong><small>Project Maintainer</small></span><span class=\"chevron\">\u00b7\u00b7\u00b7</span></button></div></aside><div class=\"drawer-backdrop\" id=\"drawerBackdrop\" hidden></div><div class=\"shell\"><header class=\"topbar\"><div class=\"top-left\"><button class=\"icon-btn mobile-menu\" id=\"menuButton\" aria-label=\"Open navigation\" aria-expanded=\"false\" aria-controls=\"sidebar\" data-icon=\"menu\"></button><div class=\"crumb\">Developer Workspace <span>/</span> <strong id=\"crumbTitle\">Dashboard</strong></div></div><div class=\"top-actions\"><label class=\"palette-select-wrap\" for=\"paletteSelect\"><span>Palette</span><select id=\"paletteSelect\" class=\"palette-select\" aria-label=\"Palette — choose dashboard color palette\"><option value=\"default\">Original \u00b7 Teal / Slate</option><option value=\"slate\">01 \u00b7 Slate & Charcoal</option><option value=\"sand\">02 \u00b7 Sand & Saddle</option><option value=\"sage\">04 \u00b7 Sage & Burgundy</option><option value=\"mauve\">12 \u00b7 Mauve Meadow</option></select></label><a class=\"audit-top-link\" href=\"service-center.html?persona=developer\" title=\"S1 service capability workflows\">S1 services</a><a class=\"audit-top-link\" href=\"capability-audit.html\" title=\"Proposed cross-persona capability coverage\">Capability audit</a><span class=\"demo-tag\">DEMO ENVIRONMENT</span><button class=\"icon-btn\" id=\"searchButton\" aria-label=\"Search\" title=\"Search (Ctrl/\u2318 K)\" data-icon=\"search\"></button><button class=\"icon-btn\" id=\"notificationButton\" aria-label=\"Notifications\" title=\"Notifications\" data-icon=\"bell\"><span class=\"notification-dot\"></span></button><button class=\"icon-btn\" id=\"themeButton\" aria-label=\"Toggle color theme\" title=\"Toggle theme\" data-icon=\"moon\"></button><span class=\"avatar top-avatar\">AM</span></div></header><main id=\"main\" tabindex=\"-1\"><div id=\"page\"></div></main></div></div><div id=\"toastRegion\" class=\"toast-region\" aria-live=\"polite\" aria-atomic=\"false\"></div><div id=\"modalRoot\"></div><div id=\"commandRoot\"></div>";
	// Execute the ported modules in original order after mount, exactly as
	// the prototype does after DOM parse. Auth/onboarding modules carry
	// real API seam patches (tools/ui-migration/wire_auth.py).
	const modules: string[] = ["/js/developer.js"];
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
