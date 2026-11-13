import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter({ out: 'build' }),
		// The Go control plane remains the API authority; the frontend server
		// proxies /api/v1 with cookie-borne credentials (BFF pattern).
		paths: {},
		alias: {
			$lib: 'src/lib',
			$components: 'src/lib/components'
		}
	}
};

export default config;
