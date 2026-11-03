<script lang="ts">
	import { markShow, showWatched } from '#lib/api/tracking.js';
	import type { EpisodeWatched, MarkWhen } from '#lib/api/types.js';
	import Button from '#lib/components/Button.svelte';
	import Facts from '#lib/components/Facts.svelte';
	import MarkButton from '#lib/components/MarkButton.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import TitlePage, { type HeroChip } from '#lib/components/TitlePage.svelte';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDay } from '#lib/format.js';
	import { latestOnly } from '#lib/latest.js';
	import { formatScore, languageName, yearOf } from '#lib/media.js';
	import { addedMessage, hasAired, watchedById } from '#lib/watched.js';
	import Seasons from './Seasons.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const show = $derived(data.show);

	// The user's watched episodes: from the load, then reloaded after a
	// mark. Null while they can't be loaded: the page then hides the
	// episode checks rather than show everything as unwatched.
	let watchedList = $derived(data.watched);
	let watchedProblem = $derived(data.watchedError);
	const watched = $derived(watchedList ? watchedById(watchedList) : null);

	// Only the newest reload may set the list, so two quick marks can't
	// leave an episode looking unwatched.
	const track = latestOnly<EpisodeWatched[]>();

	/** Reloads the watched episodes; throws if that fails. */
	async function refreshWatched() {
		const res = await track(showWatched(show.tmdb_id));
		if (!res.current) return;
		watchedList = res.value;
		watchedProblem = '';
	}

	let retrying = $state(false);
	async function retryWatched() {
		retrying = true;
		try {
			await refreshWatched();
		} catch (e) {
			watchedProblem = errorMessage(e);
		} finally {
			retrying = false;
		}
	}

	const now = new Date();
	// The server refuses a show mark until the episodes of every regular
	// season it covers are stored (one season per number, lowest TMDB ID;
	// specials aren't part of it). This checks every listed regular season,
	// which can only be stricter.
	const pending = $derived(show.seasons.some((s) => s.season_number > 0 && s.episodes_pending));
	// Whether anything has aired, as far as the show's dates say, for the
	// wording of a mark that added nothing.
	const anyAired = $derived(
		hasAired(show.first_air_date, now) ||
			show.seasons.some((s) => s.season_number > 0 && hasAired(s.air_date, now))
	);

	let showNote = $state('');
	let showProblem = $state('');

	async function markWholeShow(when: MarkWhen) {
		showNote = '';
		showProblem = '';
		let added: number;
		try {
			added = await markShow(show.tmdb_id, when);
		} catch (e) {
			showProblem = errorMessage(e, trackingMessages);
			return;
		}
		showNote = addedMessage(added, anyAired);
		try {
			await refreshWatched();
		} catch {
			showProblem = "The episode list is out of date. Reload the page to see what's watched.";
		}
	}

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

	{#snippet actions()}
		<div class="flex flex-col">
			<MarkButton
				label="Mark show watched"
				menuLabel="Choose when you watched {show.name}"
				onmark={markWholeShow}
				release={{ label: 'Air dates', detail: "Each episode's own", enabled: true }}
				disabledReason={pending
					? 'Some episodes are still being fetched from TMDB. Reload the page in a moment.'
					: ''}
			/>
			<p class="mt-2 text-xs text-muted">
				Adds every aired episode you haven't watched yet, except specials.
			</p>
			<Notice tone="success" message={showNote} class="mt-3" />
			<Notice tone="error" message={showProblem} class="mt-3" />
			{#if watched === null}
				<div class="mt-3 flex flex-col items-start gap-2">
					<Notice
						tone="error"
						message="Couldn't load which episodes you've watched: {watchedProblem}"
					/>
					<Button variant="quiet" busy={retrying} onclick={retryWatched}>Try again</Button>
				</div>
			{/if}
		</div>
	{/snippet}

	<div class="mt-5 max-w-[72ch]">
		{#if show.tagline}
			<p class="text-[0.9375rem] text-muted italic">{show.tagline}</p>
		{/if}
		<p class={['text-[0.9375rem] leading-relaxed text-ink', show.tagline && 'mt-2']}>
			{show.overview || 'TMDB has no overview for this show yet.'}
		</p>
	</div>

	<Seasons {show} {watched} {refreshWatched} />
</TitlePage>
