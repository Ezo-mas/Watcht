import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { Watch } from '#lib/api/types.js';
import { formatWatch } from '#lib/format.js';
import PlaysButton from './PlaysButton.svelte';

const plays: Watch[] = [
	{ id: 7, watched_at: '2026-11-09T23:44:00Z', watched_on: null, source: 'manual' },
	{ id: 6, watched_at: '2026-11-08T12:00:00Z', watched_on: '2026-11-08', source: 'manual' }
];

function html() {
	return render(PlaysButton, {
		props: {
			title: 'The Matrix',
			plays,
			release: { label: 'Release date', detail: '31 Mar 1999', enabled: true },
			onadd: async () => {},
			onremove: async () => '',
			onremoveall: async () => ''
		}
	}).body.replace(/<!--[^]*?-->/g, '');
}

describe('plays button menu', () => {
	it('labels each compact remove button visibly and names its play for screen readers', () => {
		const body = html();
		for (const play of plays) {
			expect(body).toMatch(
				new RegExp(
					`<button[^>]*aria-label="Remove the play from ${formatWatch(play)}"[^>]*>[^]*?<svg[^]*?</svg>\\s*Remove\\s*</button>`
				)
			);
		}
	});

	it('offers adding another play after the history link and a divider', () => {
		const body = html();
		expect(body).toMatch(
			/View all plays[^]*?<\/a>\s*<\/div>\s*<hr[^>]*>\s*<div[^>]*>\s*<button[^>]*aria-expanded="false"[^>]*aria-controls="[^"]*-when"[^>]*>[^]*?Add another play\s*<\/button>/
		);
		expect(body).toMatch(/<button[^>]*aria-label="Remove all plays"[^>]*aria-expanded="false"/);
		expect(body).not.toContain('Coming in a later update');
		// The existing separate + still opens the same add flow.
		expect(body).toContain('aria-label="Add another play of The Matrix"');
	});
});
