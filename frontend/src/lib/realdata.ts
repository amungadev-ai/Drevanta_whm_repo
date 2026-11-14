/**
 * Real-data section rendering (G40 full-persona wave).
 *
 * These builders REPLICATE the approved prototype markup helpers exactly
 * (head/panel/table/badge/note/metric/link/btn — same classes, same
 * structure) so the real-data sections are visually indistinguishable from
 * the approved design. The DATA is real: every row comes from the Go control
 * plane; empty tables render the approved "No records" row; mutations are
 * real API calls whose outcomes re-render the section.
 *
 * Nothing here invents records. Sections whose backing capability is not
 * implemented keep the approved prototype content (which self-labels as
 * simulated).
 */

export const esc = (v: unknown): string =>
	String(v ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string);

export const head = (title: string, sub: string, org: string, orgType: string, actions = ''): string =>
	`<div class="page-heading"><div><div class="eyebrow">${esc(org)} · ${esc(orgType)}</div><h1>${esc(title)}</h1><p class="subtitle">${esc(sub)}</p></div><div class="actions">${actions}</div></div>`;

export const panel = (headTxt: string, body: string, foot = ''): string =>
	`<section class="card host-card"><div class="card-header"><h3>${esc(headTxt)}</h3>${foot}</div>${body}</section>`;

export const table = (cols: string[], rows: string[][]): string =>
	`<div class="table-scroll"><table><thead><tr>${cols.map((x) => `<th>${esc(x)}</th>`).join('')}</tr></thead><tbody>${
		rows.length
			? rows.map((r) => `<tr>${r.map((x) => `<td>${x}</td>`).join('')}</tr>`).join('')
			: `<tr><td colspan="${cols.length}">No records · this workspace has none yet</td></tr>`
	}</tbody></table></div>`;

export const badge = (s: string): string => {
	const c = /active|succeeded|healthy|verified|published|listed/i.test(s)
		? 'success'
		: /pending|invited|draft|restricted|warning|queued|building|issued/i.test(s)
			? 'warning'
			: /suspended|failed|down|denied|rejected|uncertain/i.test(s)
				? 'danger'
				: 'neutral';
	return `<span class="badge ${c}">${esc(s)}</span>`;
};

export const note = (s: string): string => `<div class="host-notice">${s}</div>`;

export const btn = (label: string, action: string, style = 'secondary'): string =>
	`<button class="btn ${style} sm" data-real-action="${esc(action)}">${esc(label)}</button>`;

export const field = (label: string, id: string, type = 'text', placeholder = ''): string =>
	`<label class="field"><span>${esc(label)}</span><input class="field-input" id="${esc(id)}" type="${esc(type)}" placeholder="${esc(placeholder)}" /></label>`;

export const money = (cents: number, currency: string): string =>
	`${(cents / 100).toFixed(2)} ${esc(currency)}`;

/** Inline mutation form panel (used by every create/apply action). */
export const formPanel = (title: string, fieldsHtml: string, action: string, submitLabel: string): string =>
	`${panel(
		title,
		`<div class="real-form" data-real-form="${esc(action)}">${fieldsHtml}
			<div class="form-row"><button class="btn primary sm" data-real-submit="${esc(action)}">${esc(submitLabel)}</button>
			<span class="real-form-msg" aria-live="polite"></span></div></div>`
	)}`;

export const fmtWhen = (iso: string | null | undefined): string => {
	if (!iso) return '—';
	const d = new Date(iso);
	return isNaN(d.getTime()) ? esc(iso) : d.toLocaleString();
};
