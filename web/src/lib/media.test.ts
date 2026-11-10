import { describe, expect, it } from 'vitest';
import {
	episodeCode,
	formatRuntime,
	formatScore,
	languageName,
	parseId,
	parseSeasonParam,
	seasonLabel,
	titleLinks,
	tvPartLinks,
	tmdbPercent,
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
		expect(episodeCode(1, 1)).toBe('S01E01');
		expect(episodeCode(0, 12)).toBe('S00E12');
		expect(episodeCode(3, 110)).toBe('S03E110');
	});

	it('shows a score only when there are votes', () => {
		expect(formatScore(8.27, 120)).toBe('8.3');
		expect(formatScore(8, 0)).toBe('');
		expect(formatScore(0, 10)).toBe('');
	});

	it('turns a vote average into a percentage, none without votes', () => {
		expect(tmdbPercent(6.66)).toBe('67%');
		expect(tmdbPercent(10)).toBe('100%');
		expect(tmdbPercent(8.24)).toBe('82%');
		expect(tmdbPercent(0)).toBe('');
	});

	it('names languages', () => {
		expect(languageName('en')).toBe('English');
		expect(languageName('')).toBe('');
	});

	it('labels specials', () => {
		expect(seasonLabel(0)).toBe('Specials');
		expect(seasonLabel(2)).toBe('Season 2');
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

	it('links a title to IMDb and TMDB', () => {
		expect(titleLinks('movie', 603, 'tt0133093')).toEqual([
			{ site: 'imdb', label: 'IMDb', href: 'https://www.imdb.com/title/tt0133093/' },
			{ site: 'tmdb', label: 'TMDB', href: 'https://www.themoviedb.org/movie/603' }
		]);
		expect(titleLinks('tv', 1396, '')).toEqual([
			{ site: 'tmdb', label: 'TMDB', href: 'https://www.themoviedb.org/tv/1396' }
		]);
	});

	it('links a season or an episode to its TMDB page', () => {
		expect(tvPartLinks(1396, 1)).toEqual([
			{ site: 'tmdb', label: 'TMDB', href: 'https://www.themoviedb.org/tv/1396/season/1' }
		]);
		expect(tvPartLinks(1396, 0, 3)).toEqual([
			{
				site: 'tmdb',
				label: 'TMDB',
				href: 'https://www.themoviedb.org/tv/1396/season/0/episode/3'
			}
		]);
	});
});
