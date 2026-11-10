import { describe, expect, it } from 'vitest';
import { CHUNK, moreLabel, nextLimit } from './chunk';

describe('nextLimit', () => {
	it('adds a chunk of 100 at a time', () => {
		expect(CHUNK).toBe(100);
		expect(nextLimit(100, 15012)).toBe(200);
		expect(nextLimit(0, 15012)).toBe(100);
	});

	it('stops at the total', () => {
		expect(nextLimit(15000, 15012)).toBe(15012);
		expect(nextLimit(15012, 15012)).toBe(15012);
		expect(nextLimit(100, 40)).toBe(40);
		expect(nextLimit(0, 0)).toBe(0);
	});

	it('takes another step size, but always moves on', () => {
		expect(nextLimit(10, 100, 25)).toBe(35);
		expect(nextLimit(10, 100, 0)).toBe(11);
		expect(nextLimit(-5, 100, 25)).toBe(25);
	});
});

describe('moreLabel', () => {
	it('says how many more a click shows', () => {
		expect(moreLabel(100, 15012)).toBe('Show 100 more');
		expect(moreLabel(14900, 15012)).toBe('Show 100 more');
		expect(moreLabel(14950, 15012)).toBe('Show the last 62');
		expect(moreLabel(15000, 15012)).toBe('Show the last 12');
		expect(moreLabel(1000, 2500, 2000)).toBe('Show the last 1,500');
		expect(moreLabel(100, 5000, 1000)).toBe('Show 1,000 more');
	});
});
