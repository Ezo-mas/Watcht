import { afterEach, describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import Toaster from './components/Toaster.svelte';
import { toast } from './toast.svelte';

const html = () => render(Toaster).body.replace(/<!--[^]*?-->/g, '');

describe('Toaster', () => {
	afterEach(() => toast.clear());

	it('has both live regions before the first toast, so it is announced', () => {
		const body = html();
		expect(body).toMatch(/<div role="alert"[^>]*>\s*<\/div>/);
		expect(body).toMatch(/<div role="status" aria-live="polite"[^>]*>\s*<\/div>/);
	});

	it('puts errors in the alert region and the rest in the status region, newest first', () => {
		toast.success('Added to your history: 3 Nov 2026.');
		toast.error('Could not reach watcht.');
		toast.info('Already in your history.');
		const body = html();
		const alert = body.slice(body.indexOf('role="alert"'), body.indexOf('role="status"'));
		const status = body.slice(body.indexOf('role="status"'));
		expect(alert).toContain('Could not reach watcht.');
		expect(alert).not.toContain('Already in your history.');
		expect(status.indexOf('Already in your history.')).toBeLessThan(
			status.indexOf('Added to your history: 3 Nov 2026.')
		);
		expect(status).not.toContain('Could not reach watcht.');
		expect(body.match(/aria-label="Dismiss"/g)).toHaveLength(3);
	});
});
