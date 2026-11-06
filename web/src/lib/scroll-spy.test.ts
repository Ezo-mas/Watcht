import { describe, expect, it } from 'vitest';
import { currentSection } from './scroll-spy';

const tops = (...values: number[]) =>
	['overview', 'stats', 'people', 'videos'].slice(0, values.length).map((id, i) => ({
		id,
		top: values[i]
	}));

describe('currentSection', () => {
	it('is the first section before any has reached the line', () => {
		expect(currentSection(tops(400, 1200, 2000, 2600), 250, false, 900)).toBe('overview');
	});

	it('is the last section whose top has passed the line', () => {
		expect(currentSection(tops(-900, -100, 240, 900), 250, false, 900)).toBe('people');
		expect(currentSection(tops(-900, -100, 600, 900), 250, false, 900)).toBe('stats');
	});

	it('is the last section showing at the bottom of the page', () => {
		expect(currentSection(tops(-2000, -900, 300, 700), 250, true, 900)).toBe('videos');
	});

	it('is nothing without sections', () => {
		expect(currentSection([], 250, false, 900)).toBeUndefined();
	});
});
