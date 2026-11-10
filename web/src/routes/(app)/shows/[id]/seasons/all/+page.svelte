<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { tick, untrack } from 'svelte';
	import { markEpisode, showWatched } from '#lib/api/tracking.js';
	import type { Episode, EpisodeWatched, MarkWhen, ShowEpisodes } from '#lib/api/types.js';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import Button from '#lib/components/Button.svelte';
	import EpisodeRow from '#lib/components/EpisodeRow.svelte';
	import EpisodesFetching, { type FetchProblem } from '#lib/components/EpisodesFetching.svelte';
	import Facts from '#lib/components/Facts.svelte';
	import ListControls from '#lib/components/ListControls.svelte';
	import PickerStrip from '#lib/components/PickerStrip.svelte';
	import ShowActions from '#lib/components/ShowActions.svelte';
	import TitlePage from '#lib/components/TitlePage.svelte';
	import TitleSection from '#lib/components/TitleSection.svelte';
	import { CHUNK, moreLabel, nextLimit } from '#lib/chunk.js';
	import { catalogMessages, errorMessage } from '#lib/errors.js';
	import { formatWatch } from '#lib/format.js';
	import { latestOnly } from '#lib/latest.js';
	import { episodeCode, formatRuntime, titleLinks } from '#lib/media.js';
	import { ALL, seasonArrows, seasonPicker } from '#lib/season-nav.js';
	import { pollShowEpisodes } from '#lib/season-poll.js';
	import {
		airYears,
		bySeason,
		countLabel,
		lastSeasonNumber,
		milestoneIn,
		seasonEnds,
		seasonList,
		showOrder,
		totalRuntime
	} from '#lib/seasons.js';
	import { toast } from '#lib/toast.svelte.js';
	import { watchedById } from '#lib/watched.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const show = $derived(data.show);
	const showId = $derived(String(data.show.tmdb_id));

	// The episodes as loaded, or as polled while some season was still
	// being fetched; a problem with that polling replaces the list.
	let polled = $state<ShowEpisodes | null>(null);
	let problem = $state<FetchProblem>();
	let attempt = $state(0);
	const details = $derived(polled ?? data.episodes);
	const pending = $derived(details.episodes_pending && !problem);

	// As on the season page: every 3 seconds for up to 2 minutes; once
	// they're in, reload the show (its seasons' pending marks) and the
	// watched list. Leaving the page cancels it.
	$effect.pre(() => {
		const loaded = data.episodes;
		void attempt;
		polled = null;
		problem = undefined;
		if (!loaded.episodes_pending) return;
		return pollShowEpisodes(
			untrack(() => show.tmdb_id),
			{
				onValue: (d) => {
					if (d.episodes_pending) return;
					polled = d;
					void reloadAfterPoll();
				},
				onTimeout: () => (problem = { kind: 'timeout' }),
				onError: (e) =>
					(problem = { kind: 'error', message: errorMessage(e, catalogMessages), retry: true })
			}
		);
	});

	// The cached show may still mark seasons pending that are stored by
	// now (a season page was open while they synced): when the episodes
	// load with none pending, reload the show once, as the season page does,
	// so the picker and the show's mark don't wait on a stale flag.
	// Once per show: the page is reused when another show's All page opens.
	let reloadedStale: number | null = null;
	$effect(() => {
		if (data.episodes.episodes_pending || reloadedStale === show.tmdb_id) return;
		if (!show.seasons.some((s) => s.episodes_pending)) return;
		reloadedStale = show.tmdb_id;
		void reloadAfterPoll();
	});

	// The user's watched episodes: from the load, then reloaded after a
	// mark. Null while they can't be loaded: the rows then have no checks.
	let watchedList = $derived(data.watched);
	const watched = $derived(watchedList ? watchedById(watchedList) : null);
	const track = latestOnly<EpisodeWatched[]>();

	/** Reloads the watched episodes; throws if that fails. */
	async function refreshWatched() {
		const id = show.tmdb_id;
		const res = await track(showWatched(id));
		if (!res.current || show.tmdb_id !== id) return;
		watchedList = res.value;
	}

	async function reloadAfterPoll() {
		await invalidate('app:show');
		try {
			await refreshWatched();
		} catch {
			toast.error("The episode list is out of date. Reload the page to see what's watched.");
		}
	}

	const now = new Date();

	const episodes = $derived(details.episodes_pending ? [] : showOrder(details.episodes ?? []));
	// Each season's first and last episodes, worked out once, so a row's
	// milestone is a lookup (a long-running show lists thousands).
	const ends = $derived(
		new Map([...bySeason(episodes)].map(([n, list]) => [n, seasonEnds(list, now)]))
	);
	const showContext = $derived({
		lastSeason: lastSeasonNumber(show.seasons),
		inProduction: show.in_production
	});

	/** Marks one episode, with a toast for the result; throws an ApiError if it fails. */
	async function markOne(episode: Episode, when: MarkWhen): Promise<void> {
		const code = episodeCode(episode.season_number, episode.episode_number);
		const res = await markEpisode(episode.tmdb_id, when);
		try {
			await refreshWatched();
		} catch {
			toast.error(`${code} marked, but this list is out of date. Reload the page to see it.`);
			return;
		}
		if (res.merged_into !== null)
			toast.info(
				`${code} is already in your history: plays within 3 hours count once, so it wasn't added again.`
			);
		else toast.success(`Added ${code} to your history: ${formatWatch(res.watch)}.`);
	}

	// While pending, the show's own counts of its listed seasons.
	const count = $derived(
		details.episodes_pending
			? seasonList(show.seasons).reduce((n, s) => n + s.episode_count, 0)
			: episodes.length
	);
	const episodesLabel = $derived(countLabel(count, 'Episode'));
	const runtime = $derived(formatRuntime(totalRuntime(episodes)));

	const factsList = $derived([
		{ label: 'Episodes', value: count ? count.toLocaleString('en') : '' },
		{ label: 'Runtime', value: runtime }
	]);

	const sections = $derived([
		{ id: 'overview', label: 'Overview' },
		{ id: 'episodes', label: episodesLabel }
	]);

	const pickerItems = $derived(seasonPicker(show, ALL));
	const arrows = $derived(seasonArrows(show, ALL));
	const fullName = $derived(`${show.name} all episodes`);

	// A long-running show lists thousands of episodes (General Hospital
	// about 15,000), so the list renders a chunk at a time: the first 100,
	// then more as the reader nears its end, or with "Show more" (keyboard,
	// or no IntersectionObserver). Back to the first chunk for another show.
	// Keyed on the ID, not the show object: a reload of the same show
	// (invalidate) makes a new object and mustn't drop the rows shown.
	const showKey = $derived(show.tmdb_id);
	let limit = $derived.by(() => {
		void showKey;
		return CHUNK;
	});
	const shown = $derived(episodes.slice(0, limit));
	let list = $state<HTMLElement>();
	let sentinel = $state<HTMLElement>();

	// Reading the limit re-creates the observer after each chunk, so it
	// reports again if the end is still near (a very tall window).
	$effect(() => {
		const at = limit;
		const total = episodes.length;
		if (!sentinel || at >= total || typeof IntersectionObserver === 'undefined') return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries.some((e) => e.isIntersecting)) limit = nextLimit(at, total);
			},
			{ rootMargin: '0px 0px 1500px 0px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});

	// "Show more" moves focus to the first new episode, so the keyboard
	// carries on where the list did.
	async function showMore() {
		const from = limit;
		limit = nextLimit(from, episodes.length);
		await tick();
		list?.children[from]?.querySelector<HTMLElement>('h3 a')?.focus();
	}
</script>

<svelte:head>
	<title>All episodes · {show.name} · watcht</title>
</svelte:head>

<TitlePage
	title="All episodes"
	name={fullName}
	year={airYears([{ air_date: show.first_air_date }, { air_date: show.last_air_date }])}
	posterUrl={show.poster_url}
	backdropUrl={show.backdrop_url}
	ratings={[]}
	links={titleLinks('tv', show.tmdb_id, show.imdb_id)}
	{sections}
	prev={arrows.prev}
	next={arrows.next}
>
	{#snippet crumbs()}
		<Breadcrumbs
			items={[{ label: show.name, href: resolve('/(app)/shows/[id]', { id: showId }) }]}
		/>
	{/snippet}

	{#snippet picker()}
		<PickerStrip label="Season" name="Seasons of {show.name}" items={pickerItems} />
	{/snippet}

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
		<TitleSection id="episodes" title={episodesLabel}>
			{#snippet filter()}
				<ListControls items="episodes" />
			{/snippet}
			<div class="mt-2">
				{#if pending || problem}
					<EpisodesFetching
						{problem}
						rows={count}
						what="this show's episodes"
						onretry={() => attempt++}
					/>
				{:else if episodes.length === 0}
					<p class="py-3 text-sm text-muted">TMDB lists no episodes for this show yet.</p>
				{:else}
					<ul bind:this={list} class="divide-y divide-line">
						{#each shown as episode (episode.tmdb_id)}
							<EpisodeRow
								{episode}
								{now}
								href={resolve('/(app)/shows/[id]/seasons/[n]/episodes/[e]', {
									id: showId,
									n: String(episode.season_number),
									e: String(episode.episode_number)
								})}
								milestone={milestoneIn(
									episode,
									ends.get(episode.season_number) ?? null,
									showContext
								)}
								watched={watched?.get(episode.tmdb_id)}
								onmark={watched ? (when) => markOne(episode, when) : undefined}
								onrefresh={refreshWatched}
							/>
						{/each}
					</ul>
					{#if shown.length < episodes.length}
						<div
							bind:this={sentinel}
							class="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-line pt-4"
						>
							<Button variant="quiet" class="max-sm:h-10" onclick={showMore}>
								{moreLabel(shown.length, episodes.length)}
							</Button>
							<p class="text-sm text-muted tabular-nums">
								Showing {shown.length.toLocaleString('en')} of {episodes.length.toLocaleString(
									'en'
								)} episodes
							</p>
						</div>
					{/if}
				{/if}
			</div>
		</TitleSection>
	{/snippet}

	<div class="mt-6 max-w-[72ch]">
		<p class="text-[0.9375rem] leading-relaxed text-ink">
			{show.overview || 'TMDB has no overview for this show yet.'}
		</p>
	</div>
</TitlePage>
