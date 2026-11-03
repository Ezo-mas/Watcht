<script lang="ts">
	import Facts from '#lib/components/Facts.svelte';
	import TitlePage, { type HeroChip } from '#lib/components/TitlePage.svelte';
	import { formatDay } from '#lib/format.js';
	import { formatRuntime, formatScore, languageName, yearOf } from '#lib/media.js';
	import MovieActions from './MovieActions.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const movie = $derived(data.movie);

	const score = $derived(formatScore(movie.vote_average, movie.vote_count));
	const runtime = $derived(formatRuntime(movie.runtime));

	const chips = $derived<HeroChip[]>([
		...(score
			? [
					{
						label: 'TMDB',
						text: score,
						strong: true,
						title: `${movie.vote_count.toLocaleString()} votes on TMDB`
					}
				]
			: []),
		...(runtime ? [{ text: runtime }] : []),
		// "Released" is the usual case; anything else (Planned, In Production) is worth a chip.
		...(movie.status && movie.status !== 'Released' ? [{ text: movie.status }] : [])
	]);

	const factsList = $derived([
		{ label: 'Released', value: formatDay(movie.release_date) },
		{ label: 'Runtime', value: runtime },
		{ label: 'Status', value: movie.status },
		{ label: 'Genres', value: movie.genres.map((g) => g.name).join(', ') },
		{
			label: 'Original title',
			value: movie.original_title !== movie.title ? movie.original_title : ''
		},
		{ label: 'Language', value: languageName(movie.original_language) },
		{
			label: 'IMDb',
			value: movie.imdb_id,
			href: movie.imdb_id
				? `https://www.imdb.com/title/${encodeURIComponent(movie.imdb_id)}/`
				: undefined
		},
		{
			label: 'TMDB',
			value: String(movie.tmdb_id),
			href: `https://www.themoviedb.org/movie/${movie.tmdb_id}`
		}
	]);
</script>

<svelte:head>
	<title>{movie.title} · watcht</title>
</svelte:head>

<TitlePage
	title={movie.title}
	year={yearOf(movie.release_date)}
	posterUrl={movie.poster_url}
	backdropUrl={movie.backdrop_url}
	{chips}
	sections={[{ id: 'overview', label: 'Overview' }]}
>
	{#snippet facts()}
		<Facts facts={factsList} />
	{/snippet}

	{#snippet actions()}
		<MovieActions {movie} watches={data.watches} watchesError={data.watchesError} />
	{/snippet}

	<div class="mt-5 max-w-[72ch]">
		{#if movie.tagline}
			<p class="text-[0.9375rem] text-muted italic">{movie.tagline}</p>
		{/if}
		<p class={['text-[0.9375rem] leading-relaxed text-ink', movie.tagline && 'mt-2']}>
			{movie.overview || 'TMDB has no overview for this movie yet.'}
		</p>
	</div>
</TitlePage>
