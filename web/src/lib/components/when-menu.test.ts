import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import WhenMenu from './WhenMenu.svelte';

const props = {
	label: 'Episode plays',
	removeTitle: 'S01E01',
	release: { label: 'Air dates', detail: "Each episode's own", enabled: true },
	onmark: () => {},
	onremoveall: async () => '',
	popup: true
};

const html = (extra = {}) => render(WhenMenu, { props: { ...props, ...extra } }).body;

describe('card watch menu', () => {
	it('shares season removal and watch-again choices', () => {
		const body = html();
		expect(body).toContain('Remove from history?');
		expect(body).toMatch(/<button[^>]*aria-label="Remove all plays"[^>]*aria-expanded="false"/);
		expect(body).toContain('Keep it');
		expect(body).toContain('Watch again');
		expect(body).not.toContain('Only this play');
		expect(body).not.toContain('Coming in a later update');
	});

	it('explains why season rewatching is pending without offering a no-op', () => {
		const body = html({
			watchAgainDisabledReason: 'The current mark only adds unwatched episodes.'
		});
		expect(body).toMatch(
			/<button[^>]*aria-disabled="true"[^>]*title="The current mark only adds unwatched episodes."[^>]*>\s*Watch again/
		);
	});

	it('keeps the newest-play action for episode cards', () => {
		const body = html({ onremove: () => {} });
		expect(body).toContain('Only this play');
		expect(body).toContain('Remove all plays');
		expect(body).toContain('Add another play');
	});

	it('leads partial seasons with Watch remaining, then dates and removal', () => {
		const body = html({ remaining: true });
		expect(body).toContain('Watch remaining');
		expect(body.indexOf('Watch remaining')).toBeLessThan(body.indexOf('Air dates'));
		expect(body.indexOf('Air dates')).toBeLessThan(body.indexOf('Other date…'));
		expect(body.indexOf('Other date…')).toBeLessThan(body.indexOf('Remove all plays'));
		expect(body).not.toContain('Just now');
		expect(body).not.toContain('Remove from history?');
	});
});
