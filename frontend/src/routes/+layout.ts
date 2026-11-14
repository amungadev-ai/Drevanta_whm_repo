// The app is client-rendered: the approved UI is defined by the ported
// prototype markup + persona modules (see tools/ui-migration/port.py).
// SSR is disabled to mirror prototype behaviour exactly and avoid
// hydration conflicts with DOM-manipulating modules.
export const ssr = false;
export const prerender = false;
export const trailingSlash = 'never';
