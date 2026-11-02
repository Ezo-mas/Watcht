import { describe, expect, it } from 'vitest';
import { QuerySync } from './search-url';

describe('QuerySync', () => {
	it('keeps what was typed while its own URL write lands', () => {
		const s = new QuerySync();
		s.wrote('matrix');
		// The box has moved on by the time ?q=matrix lands.
		expect(s.arrived('matrix', 'matrix reloaded')).toBeNull();
	});

	it('only treats the last write as its own', () => {
		const s = new QuerySync();
		s.wrote('ab');
		s.wrote('abc');
		s.wrote('ab');
		// Only the newest goto lands; the superseded ones never do.
		expect(s.arrived('ab', 'abcd')).toBeNull();
		// Nothing is left over to hide a later Back to ?q=abc.
		expect(s.arrived('abc', 'zz')).toBe('abc');
	});

	it('forgets its write when another navigation lands first', () => {
		const s = new QuerySync();
		s.wrote('alien');
		// Back cancels the pending write and lands elsewhere.
		expect(s.arrived('matrix', 'alien')).toBe('matrix');
		// A later navigation to ?q=alien isn't mistaken for the old write.
		expect(s.arrived('alien', 'matrix')).toBe('alien');
	});

	it('puts the query of any other navigation in the box', () => {
		const s = new QuerySync();
		s.wrote('alien');
		expect(s.arrived('alien', 'alien')).toBeNull();
		// Back to an earlier search.
		expect(s.arrived('matrix', 'alien')).toBe('matrix');
		// A link to the page with no query.
		expect(s.arrived('', 'alien')).toBe('');
	});

	it('leaves the box alone when it already says the same', () => {
		const s = new QuerySync();
		expect(s.arrived('the  matrix', 'the matrix ')).toBeNull();
	});

	it('compares against the pending write, not the stale URL', () => {
		const s = new QuerySync();
		expect(s.target('matrix')).toBe('matrix');
		s.wrote('ab');
		// Typing "ab" then deleting it before the write lands: the box ("")
		// equals the stale URL (""), but the URL is about to say "ab".
		expect(s.target('')).toBe('ab');
		s.arrived('ab', '');
		expect(s.target('ab')).toBe('ab');
	});
});
