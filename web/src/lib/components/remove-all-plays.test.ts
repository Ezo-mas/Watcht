import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import RemoveAllPlays from './RemoveAllPlays.svelte';
import WatchedCheck from './WatchedCheck.svelte';

describe('remove all plays menu action', () => {
	it('offers a confirmation disclosure instead of a disabled placeholder', () => {
		const body = render(RemoveAllPlays, {
			props: { title: 'The Matrix', onremove: async () => '' }
		}).body;
		expect(body).toMatch(/<button[^>]*aria-label="Remove all plays"[^>]*aria-expanded="false"/);
		expect(body).not.toContain('aria-disabled');
		expect(body).not.toContain('Coming in a later update');
	});

	it('keeps both single-play and confirmed all-play removal in the history card menu', () => {
		const body = render(WatchedCheck, {
			props: {
				name: 'The Matrix',
				watchedAt: '2026-11-09T20:00:00Z',
				watchedOn: null,
				runtime: 136,
				release: { label: 'Release date', detail: '31 Mar 1999', enabled: true },
				onremove: async () => '',
				onremoveall: async () => '',
				onadd: async () => ''
			}
		}).body;
		expect(body).toContain('Only this play');
		expect(body).toMatch(/<button[^>]*aria-label="Remove all plays"[^>]*aria-expanded="false"/);
		expect(body).toContain('Keep it');
		expect(body).not.toContain('Coming in a later update');
	});
});
