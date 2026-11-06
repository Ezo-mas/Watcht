<script lang="ts">
	import { markShow, showWatched } from '#lib/api/tracking.js';
	import type { EpisodeWatched, MarkWhen } from '#lib/api/types.js';
	import Button from '#lib/components/Button.svelte';
	import Facts from '#lib/components/Facts.svelte';
	import MarkButton from '#lib/components/MarkButton.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import TitlePage from '#lib/components/TitlePage.svelte';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDay } from '#lib/format.js';
	import { latestOnly } from '#lib/latest.js';
	import { toast } from '#lib/toast.svelte.js';
	import { formatScore, languageName, titleLinks, yearOf } from '#lib/media.js';
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

	// Results are toasts (DESIGN.md, Toasts): nothing added is info, not
	// an error.
	async function markWholeShow(when: MarkWhen) {
		let added: number;
		try {
			added = await markShow(show.tmdb_id, when);
		} catch (e) {
			toast.error(errorMessage(e, trackingMessages));
			return;
		}
		toast.show(added > 0 ? 'success' : 'info', addedMessage(added, anyAired));
		try {
			await refreshWatched();
		} catch {
			toast.error("The episode list is out of date. Reload the page to see what's watched.");
		}
	}

	const regular = $derived(show.seasons.filter((s) => s.season_number > 0));
	const episodes = $derived(regular.reduce((n, s) => n + s.episode_count, 0));
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

	<div class="mt-6 max-w-[72ch]">
		{#if show.tagline}
			<p class="text-[0.9375rem] text-muted italic">{show.tagline}</p>
		{/if}
		<p class={['text-[0.9375rem] leading-relaxed text-ink', show.tagline && 'mt-2']}>
			{show.overview || 'TMDB has no overview for this show yet.'}
		</p>
	</div>

	<Seasons {show} {watched} {refreshWatched} />
</TitlePage>
