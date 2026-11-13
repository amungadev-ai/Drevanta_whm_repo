import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 5173,
		strictPort: true
	},
	// Go control plane URL for dev/prod proxying (BFF). Set via env.
	define: {}
});
