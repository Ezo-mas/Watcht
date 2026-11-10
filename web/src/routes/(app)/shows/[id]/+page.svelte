<script lang="ts">
	import { markSeason, showWatched } from '#lib/api/tracking.js';
	import type { EpisodeWatched, MarkWhen } from '#lib/api/types.js';
	import Facts from '#lib/components/Facts.svelte';
	import PeopleSection from '#lib/components/PeopleSection.svelte';
	import RecentEpisodesSection from '#lib/components/RecentEpisodesSection.svelte';
	import RecommendationsSection from '#lib/components/RecommendationsSection.svelte';
	import ShowActions from '#lib/components/ShowActions.svelte';
	import StatsSection from '#lib/components/StatsSection.svelte';
	import TitlePage from '#lib/components/TitlePage.svelte';
	import VideosSection from '#lib/components/VideosSection.svelte';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDay } from '#lib/format.js';
	import { latestOnly } from '#lib/latest.js';
	import { formatScore, languageName, seasonLabel, titleLinks, yearOf } from '#lib/media.js';
	import { seasonList, seasonsLabel } from '#lib/seasons.js';
	import { toast } from '#lib/toast.svelte.js';
	import { addedMessage, hasAired } from '#lib/watched.js';
	import SeasonGrid from './SeasonGrid.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const show = $derived(data.show);

	// The user's watched episodes: from the load, then reloaded after a
	// mark. Null while they can't be loaded: the page then shows no
	// progress rather than everything as unwatched.
	let watchedList = $derived(data.watched);

	// Only the newest reload may set the list, so two quick marks can't
	// leave an episode looking unwatched.
	const track = latestOnly<EpisodeWatched[]>();

	/** Reloads the watched episodes; throws if that fails. */
	async function refreshWatched() {
		const id = show.tmdb_id;
		const res = await track(showWatched(id));
		// The page is reused across shows: drop a list for a show we've left.
		if (!res.current || show.tmdb_id !== id) return;
		watchedList = res.value;
	}

	const now = new Date();

	// A season card's mark: its aired episodes not watched yet. Results
	// are toasts named for the season, since they float away from the card.
	async function markOneSeason(n: number, when: MarkWhen) {
		const season = seasonList(show.seasons).find((s) => s.season_number === n);
		const name = season?.name || seasonLabel(n);
		const aired = !!season && hasAired(season.air_date, now);
		let added: number;
		try {
			added = await markSeason(show.tmdb_id, n, when);
		} catch (e) {
			toast.error(`${name}: ${errorMessage(e, trackingMessages)}`);
			return;
		}
		toast.show(added > 0 ? 'success' : 'info', `${name}: ${addedMessage(added, aired)}`);
		try {
			await refreshWatched();
		} catch {
			toast.error("Your progress here is out of date. Reload the page to see what's watched.");
		}
	}

	const episodes = $derived(
		seasonList(show.seasons)
			.filter((s) => s.season_number > 0)
			.reduce((n, s) => n + s.episode_count, 0)
	);
	const score = $derived(formatScore(show.vote_average, show.vote_count));

	// TMDB's score only until M8 adds MDBList's sources (maintainer decision).
	const ratings = $derived(
		score
			? [{ site: 'tmdb' as const, label: 'TMDB', score, outOf: '10', votes: show.vote_count }]
			: []
	);

	const aired = $derived.by(() => {
		const first = formatDay(show.first_air_date);
		if (!first) return '';
		if (show.in_production) return `${first} – now`;
		const last = formatDay(show.last_air_date);
		return last && last !== first ? `${first} – ${last}` : first;
	});

	const factsList = $derived([
		{ label: 'Aired', value: aired },
		{ label: 'Status', value: show.status },
		{ label: 'Episodes', value: episodes ? String(episodes) : '' },
		{ label: 'Genres', value: show.genres.map((g) => g.name).join(', ') },
		{
			label: 'Original title',
			value: show.original_name !== show.name ? show.original_name : ''
		},
		{ label: 'Language', value: languageName(show.original_language) }
	]);

	const sections = $derived([
		{ id: 'overview', label: 'Overview' },
		{ id: 'recent', label: 'Recent episodes' },
		{ id: 'people', label: 'People' },
		{ id: 'seasons', label: seasonsLabel(show.seasons) },
		{ id: 'stats', label: 'Stats' },
		{ id: 'videos', label: 'Videos' },
		{ id: 'recommendations', label: 'Recommendations' }
	]);
</script>

<svelte:head>
	<title>{show.name} · watcht</title>
</svelte:head>

<TitlePage
	title={show.name}
	year={yearOf(show.first_air_date)}
	posterUrl={show.poster_url}
	backdropUrl={show.backdrop_url}
	{ratings}
	links={titleLinks('tv', show.tmdb_id, show.imdb_id)}
	{sections}
>
	{#snippet facts()}
		<Facts facts={factsList} />
	{/snippet}

	{#snippet actions()}
		<ShowActions
			{show}
			watched={watchedList}
			watchedError={data.watchedError}
			refresh={refreshWatched}
		/>
	{/snippet}

	{#snippet below()}
		<RecentEpisodesSection />
		<PeopleSection />
		<!-- Keyed by show, so a season mark still running can't spin or take focus on another show's grid. -->
		{#key show.tmdb_id}
			<SeasonGrid
				{show}
				watched={watchedList}
				{now}
				onmark={markOneSeason}
				onrefresh={refreshWatched}
			/>
		{/key}
		<StatsSection tmdb={{ average: show.vote_average, votes: show.vote_count }} />
		<VideosSection />
	{/snippet}

	{#snippet last()}
		<RecommendationsSection title={show.name} />
	{/snippet}

	<div class="mt-6 max-w-[72ch]">
		{#if show.tagline}
			<p class="text-[0.9375rem] text-muted italic">{show.tagline}</p>
		{/if}
		<p class={['text-[0.9375rem] leading-relaxed text-ink', show.tagline && 'mt-2']}>
			{show.overview || 'TMDB has no overview for this show yet.'}
		</p>
	</div>
</TitlePage>
