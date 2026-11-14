import { writable } from 'svelte/store';
import { api, type Me, type WorkspaceItem, routeForWorkspaceKind } from '$lib/api';

/** Live session state, loaded from the real control plane. */
export interface SessionState {
	me: Me | null;
	workspaces: WorkspaceItem[];
	active: WorkspaceItem | null;
	loading: boolean;
	error: string | null;
}

export const session = writable<SessionState>({
	me: null,
	workspaces: [],
	active: null,
	loading: true,
	error: null
});

/** Load the real session: /me + /workspaces; pick the active workspace. */
const KIND_ALIASES: Record<string, string> = {
	server_admin: 'server',
	hosting: 'hosting_org',
	developer: 'developer_project'
};

export async function loadSession(preferredKind?: string): Promise<SessionState> {
	session.update((s) => ({ ...s, loading: true, error: null }));
	try {
		const me = await api.me();
		const ws = await api.workspaces();
		const wanted = preferredKind ? (KIND_ALIASES[preferredKind] ?? preferredKind) : preferredKind;
		const active =
			(wanted && ws.items.find((w) => w.kind === wanted)) ||
			ws.items.find((w) => w.kind === 'hosting') ||
			ws.items[0] ||
			null;
		const state: SessionState = { me, workspaces: ws.items, active, loading: false, error: null };
		session.set(state);
		return state;
	} catch (e) {
		const state: SessionState = {
			me: null,
			workspaces: [],
			active: null,
			loading: false,
			error: e instanceof Error ? e.message : 'session unavailable'
		};
		session.set(state);
		return state;
	}
}

export { routeForWorkspaceKind };
