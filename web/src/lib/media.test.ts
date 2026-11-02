import { describe, expect, it } from 'vitest';
import {
	defaultSeason,
	episodeCode,
	formatRuntime,
	formatScore,
	languageName,
	parseId,
	parseSeasonParam,
	seasonLabel,
	yearOf
} from './media';

describe('media helpers', () => {
	it('takes the year from a date', () => {
		expect(yearOf('1999-03-30')).toBe('1999');
		expect(yearOf('')).toBe('');
	});

	it('formats runtimes', () => {
		expect(formatRuntime(58)).toBe('58m');
		expect(formatRuntime(136)).toBe('2h 16m');
		expect(formatRuntime(120)).toBe('2h');
		expect(formatRuntime(0)).toBe('');
	});

	it('builds episode codes', () => {
		expect(episodeCode(1, 1)).toBe('1x01');
		expect(episodeCode(0, 12)).toBe('0x12');
		expect(episodeCode(3, 110)).toBe('3x110');
	});

	it('shows a score only when there are votes', () => {
		expect(formatScore(8.27, 120)).toBe('8.3');
		expect(formatScore(8, 0)).toBe('');
		expect(formatScore(0, 10)).toBe('');
	});

	it('names languages', () => {
		expect(languageName('en')).toBe('English');
		expect(languageName('')).toBe('');
	});

	it('labels specials', () => {
		expect(seasonLabel(0)).toBe('Specials');
		expect(seasonLabel(2)).toBe('Season 2');
	});

	it('opens on the first regular season, else specials', () => {
		const s = (...n: number[]) => n.map((season_number) => ({ season_number }));
		expect(defaultSeason(s(0, 1, 2))).toBe(1);
		expect(defaultSeason(s(3, 2, 0))).toBe(2);
		expect(defaultSeason(s(0))).toBe(0);
		expect(defaultSeason([])).toBeNull();
	});

	it('reads the season parameter', () => {
		expect(parseSeasonParam('0')).toBe(0);
		expect(parseSeasonParam('12')).toBe(12);
		for (const bad of [null, '', 'abc', '-1', '1.5', ' 1'])
			expect(parseSeasonParam(bad)).toBeNull();
	});

	it('reads route IDs', () => {
		expect(parseId('603')).toBe(603);
		for (const bad of ['0', '-1', '01', 'abc', '1e3', '2147483648'])
			expect(parseId(bad)).toBeNull();
	});
});
