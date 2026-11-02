<script lang="ts">
	import Facts from '#lib/components/Facts.svelte';
	import TitlePage, { type HeroChip } from '#lib/components/TitlePage.svelte';
	import { formatDay } from '#lib/format.js';
	import { formatScore, languageName, yearOf } from '#lib/media.js';
	import Seasons from './Seasons.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const show = $derived(data.show);

	const regular = $derived(show.seasons.filter((s) => s.season_number > 0));
	const episodes = $derived(regular.reduce((n, s) => n + s.episode_count, 0));
	const score = $derived(formatScore(show.vote_average, show.vote_count));

	const chips = $derived<HeroChip[]>([
		...(score
			? [
					{
						label: 'TMDB',
						text: score,
						strong: true,
						title: `${show.vote_count.toLocaleString()} votes on TMDB`
					}
				]
			: []),
		...(show.status ? [{ text: show.status }] : []),
		...(regular.length
			? [{ text: `${regular.length} ${regular.length === 1 ? 'season' : 'seasons'}` }]
			: [])
	]);

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
		{ label: 'Language', value: languageName(show.original_language) },
		{
			label: 'IMDb',
			value: show.imdb_id,
			href: show.imdb_id
				? `https://www.imdb.com/title/${encodeURIComponent(show.imdb_id)}/`
				: undefined
		},
		{
			label: 'TMDB',
			value: String(show.tmdb_id),
			href: `https://www.themoviedb.org/tv/${show.tmdb_id}`
		}
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
	{chips}
	sections={[
		{ id: 'overview', label: 'Overview' },
		{ id: 'seasons', label: 'Seasons', count: show.seasons.length }
	]}
>
	{#snippet facts()}
		<Facts facts={factsList} />
	{/snippet}

	<div class="mt-5 max-w-[72ch]">
		{#if show.tagline}
			<p class="text-[0.9375rem] text-muted italic">{show.tagline}</p>
		{/if}
		<p class={['text-[0.9375rem] leading-relaxed text-ink', show.tagline && 'mt-2']}>
			{show.overview || 'TMDB has no overview for this show yet.'}
		</p>
	</div>

	<Seasons {show} />
</TitlePage>
