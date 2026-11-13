// Head-redirect stub ported from the prototype (approved navigation).
import { redirect } from '@sveltejs/kit';
export const load = () => { throw redirect(302, '/auth/login'); };
