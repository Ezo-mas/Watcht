<script lang="ts">
	import { untrack } from 'svelte';
	import { markMovie, movieWatches, removeMovieWatches } from '#lib/api/tracking.js';
	import Facts from '#lib/components/Facts.svelte';
	import PeopleSection from '#lib/components/PeopleSection.svelte';
	import PlaysActions from '#lib/components/PlaysActions.svelte';
	import RecommendationsSection from '#lib/components/RecommendationsSection.svelte';
	import StatsSection from '#lib/components/StatsSection.svelte';
	import TitlePage from '#lib/components/TitlePage.svelte';
	import VideosSection from '#lib/components/VideosSection.svelte';
	import { formatDay } from '#lib/format.js';
	import { formatRuntime, formatScore, languageName, titleLinks, yearOf } from '#lib/media.js';
	import { releaseOption } from '#lib/when.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const movie = $derived(data.movie);

	const score = $derived(formatScore(movie.vote_average, movie.vote_count));
	const runtime = $derived(formatRuntime(movie.runtime));

	// TMDB's score only until M8 adds MDBList's sources (maintainer decision).
	const ratings = $derived(
		score
			? [{ site: 'tmdb' as const, label: 'TMDB', score, outOf: '10', votes: movie.vote_count }]
			: []
	);

	const factsList = $derived([
		{ label: 'Released', value: formatDay(movie.release_date) },
		{ label: 'Runtime', value: runtime },
		{ label: 'Status', value: movie.status },
		{ label: 'Genres', value: movie.genres.map((g) => g.name).join(', ') },
		{
			label: 'Original title',
			value: movie.original_title !== movie.title ? movie.original_title : ''
		},
		{ label: 'Language', value: languageName(movie.original_language) }
	]);

	const sections = [
		{ id: 'overview', label: 'Overview' },
		{ id: 'stats', label: 'Stats' },
		{ id: 'people', label: 'People' },
		{ id: 'videos', label: 'Videos' },
		{ id: 'recommendations', label: 'Recommendations' }
	];
</script>

<svelte:head>
	<title>{movie.title} · watcht</title>
</svelte:head>

<TitlePage
	title={movie.title}
	year={yearOf(movie.release_date)}
	posterUrl={movie.poster_url}
	backdropUrl={movie.backdrop_url}
	{ratings}
	links={titleLinks('movie', movie.tmdb_id, movie.imdb_id)}
	{sections}
>
	{#snippet facts()}
		<Facts facts={factsList} />
	{/snippet}

	{#snippet actions()}
		<!--
			A fresh instance per movie: its reload guard and state belong to
			one movie, so a slow reload for the last one can't land here.
		-->
		{#key movie.tmdb_id}
			<!-- The id it was made for, not whichever movie is open when a closure runs. -->
			{@const id = untrack(() => movie.tmdb_id)}
			<PlaysActions
				title={movie.title}
				kind="movie"
				idBase="movie"
				release={releaseOption('Release date', movie.release_date, new Date())}
				watches={data.watches}
				watchesError={data.watchesError}
				load={() => movieWatches(id)}
				mark={(when) => markMovie(id, when)}
				removeAll={() => removeMovieWatches(id)}
			/>
		{/key}
	{/snippet}

	{#snippet below()}
		<StatsSection tmdb={{ average: movie.vote_average, votes: movie.vote_count }} />
		<PeopleSection />
		<VideosSection />
	{/snippet}

	{#snippet last()}
		<RecommendationsSection title={movie.title} />
	{/snippet}

	<div class="mt-6 max-w-[72ch]">
		{#if movie.tagline}
			<p class="text-[0.9375rem] text-muted italic">{movie.tagline}</p>
		{/if}
		<p class={['text-[0.9375rem] leading-relaxed text-ink', movie.tagline && 'mt-2']}>
			{movie.overview || 'TMDB has no overview for this movie yet.'}
		</p>
	</div>
</TitlePage>
