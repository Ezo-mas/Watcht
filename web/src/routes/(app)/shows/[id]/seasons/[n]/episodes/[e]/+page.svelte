<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { ApiError } from '#lib/api/client.js';
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import { episodeWatches, markEpisode, removeEpisodeWatches } from '#lib/api/tracking.js';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import Button from '#lib/components/Button.svelte';
	import Facts from '#lib/components/Facts.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import PeopleSection from '#lib/components/PeopleSection.svelte';
	import PickerStrip from '#lib/components/PickerStrip.svelte';
	import PlaysActions from '#lib/components/PlaysActions.svelte';
	import TitlePage from '#lib/components/TitlePage.svelte';
	import VideosSection from '#lib/components/VideosSection.svelte';
	import { catalogMessages, errorMessage } from '#lib/errors.js';
	import { formatDay } from '#lib/format.js';
	import { episodeCode, formatRuntime, seasonLabel, tvPartLinks, yearOf } from '#lib/media.js';
	import { episodeArrows } from '#lib/season-nav.js';
	import { pollSeason } from '#lib/season-poll.js';
	import { episodeMilestone, lastSeasonNumber } from '#lib/seasons.js';
	import { hasAired } from '#lib/watched.js';
	import { releaseOption } from '#lib/when.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const show = $derived(data.show);
	const showId = $derived(String(data.show.tmdb_id));
	const episode = $derived(data.episode);
	const season = $derived(data.season.season);
	const number = $derived(season.season_number);
	const episodes = $derived(data.season.episodes ?? []);
	const seasonName = $derived(season.name || seasonLabel(number));

	const now = new Date();

	// The season's episodes were still being fetched when the page loaded:
	// ask again every 3 seconds for up to 2 minutes, then load the page
	// again, which finds the episode (or says it doesn't exist).
	let problem = $state<{ kind: 'timeout' } | { kind: 'error'; message: string }>();
	let attempt = $state(0);
	$effect(() => {
		if (data.episode) return;
		void attempt;
		problem = undefined;
		const id = untrack(() => show.tmdb_id);
		return pollSeason(id, data.season.season.season_number, {
			onValue: (d) => {
				// The show is cached in the layout, and its season list still marks
				// this season pending: reload it (this page reloads with it).
				if (!d.episodes_pending) void invalidate('app:show');
			},
			onTimeout: () => (problem = { kind: 'timeout' }),
			onError: (e) => {
				// The season is gone from TMDB: reloading the show turns this
				// into the not-found page, where "Try again" would only 404 again.
				if (e instanceof ApiError && e.status === 404) {
					void invalidate('app:show');
					return;
				}
				problem = { kind: 'error', message: errorMessage(e, catalogMessages) };
			}
		});
	});

	const code = $derived(
		episode
			? episodeCode(episode.season_number, episode.episode_number)
			: episodeCode(number, data.episodeNumber)
	);
	const fullName = $derived(`${show.name} ${code}`);

	const milestone = $derived(
		episode
			? episodeMilestone(
					episode,
					episodes,
					{ lastSeason: lastSeasonNumber(show.seasons), inProduction: show.in_production },
					now
				)
			: ''
	);

	// TMDB's score for the episode, when it has one. There's no vote count
	// for episodes, so the band says just the score.
	const ratings = $derived(
		episode && episode.vote_average > 0
			? [
					{
						site: 'tmdb' as const,
						label: 'TMDB',
						score: episode.vote_average.toFixed(1),
						outOf: '10'
					}
				]
			: []
	);

	const factsList = $derived(
		episode
			? [
					{
						label: hasAired(episode.air_date, now) ? 'Aired' : 'Airs',
						value: formatDay(episode.air_date)
					},
					{ label: 'Runtime', value: formatRuntime(episode.runtime) }
				]
			: []
	);

	const sections = [
		{ id: 'overview', label: 'Overview' },
		{ id: 'people', label: 'People' },
		{ id: 'videos', label: 'Videos' }
	];

	const crumbsList = $derived([
		{ label: show.name, href: resolve('/(app)/shows/[id]', { id: showId }) },
		{
			label: seasonName,
			href: resolve('/(app)/shows/[id]/seasons/[n]', { id: showId, n: String(number) })
		}
	]);

	// Within the season only: the next season's numbers aren't known here.
	const arrows = $derived(
		episode
			? episodeArrows(show.tmdb_id, episodes, episode.tmdb_id)
			: { prev: undefined, next: undefined }
	);

	// Keyed by TMDB ID: a season can list two episodes with the same number.
	const pickerItems = $derived(
		episodes.map((ep) => ({
			key: ep.tmdb_id,
			href: resolve('/(app)/shows/[id]/seasons/[n]/episodes/[e]', {
				id: showId,
				n: String(number),
				e: String(ep.episode_number)
			}),
			label: String(ep.episode_number),
			prefix: 'Episode',
			title: ep.name,
			current: ep.tmdb_id === episode?.tmdb_id
		}))
	);
</script>

<svelte:head>
	<title>{code}{episode ? ` ${episode.name}` : ''} · {show.name} · watcht</title>
</svelte:head>

<TitlePage
	{code}
	title={episode?.name ?? ''}
	name={fullName}
	year={episode ? yearOf(episode.air_date) : ''}
	posterUrl={season.poster_url || show.poster_url}
	backdropUrl={episode?.still_large_url || show.backdrop_url}
	{ratings}
	links={tvPartLinks(show.tmdb_id, number, data.episodeNumber)}
	sections={episode ? sections : []}
	prev={arrows.prev}
	next={arrows.next}
>
	{#snippet crumbs()}
		<Breadcrumbs items={crumbsList} flag={milestone} />
	{/snippet}

	{#snippet picker()}
		{#if pickerItems.length > 0}
			<PickerStrip
				label="Episode"
				name="Episodes of {show.name} {seasonName}"
				items={pickerItems}
			/>
		{/if}
	{/snippet}

	{#snippet facts()}
		<Facts facts={factsList} />
	{/snippet}

	{#snippet actions()}
		{#if episode}
			<!--
				The movie's action column: "Mark as watched", or the plays button
				once watched. A fresh instance per episode: its reload guard and
				state belong to one episode.
			-->
			{#key episode.tmdb_id}
				<!--
					Read once, so a mark's later reload asks for this episode. Not a
					full guarantee: a play removed while the reader moves to another
					episode may reload once against the new one, harmlessly (the
					block is gone and the error is swallowed).
				-->
				{@const id = untrack(() => episode.tmdb_id)}
				<PlaysActions
					title={fullName}
					kind="episode"
					idBase="episode"
					release={releaseOption('Air date', episode.air_date, now)}
					watches={data.watches}
					watchesError={data.watchesError}
					load={() => episodeWatches(id)}
					mark={(when) => markEpisode(id, when)}
					removeAll={() => removeEpisodeWatches(id)}
				/>
			{/key}
		{/if}
	{/snippet}

	{#snippet below()}
		{#if episode}
			<PeopleSection />
		{/if}
	{/snippet}

	{#snippet last()}
		{#if episode}
			<VideosSection />
		{/if}
	{/snippet}

	<div class="mt-6 max-w-[72ch]">
		{#if episode}
			<p class="text-[0.9375rem] leading-relaxed text-ink">
				{episode.overview || 'TMDB has no overview for this episode yet.'}
			</p>
		{:else if problem?.kind === 'timeout'}
			<div class="flex flex-col items-start gap-3">
				<p class="text-sm text-ink">
					TMDB hasn't sent this season's episodes yet. watcht keeps trying in the background.
				</p>
				<Button variant="quiet" onclick={() => attempt++}>Check again</Button>
			</div>
		{:else if problem?.kind === 'error'}
			<div class="flex flex-col items-start gap-3">
				<Notice tone="error" message={problem.message} />
				<Button variant="quiet" onclick={() => attempt++}>Try again</Button>
			</div>
		{:else}
			<p role="status" class="flex items-center gap-2 text-sm text-ink">
				<span
					aria-hidden="true"
					class="size-3.5 animate-spin rounded-full border-2 border-accent border-r-transparent"
				></span>
				Fetching episodes from TMDB…
			</p>
		{/if}
	</div>
</TitlePage>
