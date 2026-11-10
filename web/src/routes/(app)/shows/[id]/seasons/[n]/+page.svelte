<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { tick, untrack } from 'svelte';
	import { ApiError } from '#lib/api/client.js';
	import { markEpisode, markSeason, removeSeasonWatches, showWatched } from '#lib/api/tracking.js';
	import type { Episode, EpisodeWatched, MarkWhen, SeasonDetails } from '#lib/api/types.js';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import Button from '#lib/components/Button.svelte';
	import EpisodeRow from '#lib/components/EpisodeRow.svelte';
	import EpisodesFetching, { type FetchProblem } from '#lib/components/EpisodesFetching.svelte';
	import Facts from '#lib/components/Facts.svelte';
	import ListControls from '#lib/components/ListControls.svelte';
	import MarkButton from '#lib/components/MarkButton.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import PeopleSection from '#lib/components/PeopleSection.svelte';
	import PickerStrip from '#lib/components/PickerStrip.svelte';
	import PlaceholderActions from '#lib/components/PlaceholderActions.svelte';
	import ProgressButton from '#lib/components/ProgressButton.svelte';
	import TitlePage from '#lib/components/TitlePage.svelte';
	import TitleSection from '#lib/components/TitleSection.svelte';
	import VideosSection from '#lib/components/VideosSection.svelte';
	import { catalogMessages, errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDay, formatWatch } from '#lib/format.js';
	import { latestOnly } from '#lib/latest.js';
	import { episodeCode, formatRuntime, seasonLabel, tvPartLinks } from '#lib/media.js';
	import { seasonArrows, seasonPicker } from '#lib/season-nav.js';
	import { pollSeason } from '#lib/season-poll.js';
	import {
		airYears,
		countLabel,
		lastSeasonNumber,
		milestoneIn,
		progressBySeason,
		seasonEnds,
		seasonRating,
		totalRuntime
	} from '#lib/seasons.js';
	import { toast } from '#lib/toast.svelte.js';
	import { addedMessage, anyAired, seasonProgress, watchedById } from '#lib/watched.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const show = $derived(data.show);
	const showId = $derived(String(data.show.tmdb_id));

	// The season as loaded, or as polled while its episodes were still
	// being fetched; a problem with that polling replaces the list.
	let polled = $state<SeasonDetails | null>(null);
	let problem = $state<FetchProblem>();
	let attempt = $state(0);
	const details = $derived(polled ?? data.season);
	const number = $derived(details.season.season_number);
	const pending = $derived(details.episodes_pending && !problem);

	// While the server is still fetching the episodes from TMDB, ask again
	// every 3 seconds for up to 2 minutes. Once they're in, reload the show
	// too (reloadAfterPoll), so its season list is up to date: the picker drops
	// seasons TMDB doesn't have, and the marks see the episodes. Another
	// season or leaving the page cancels it. It runs before the render, so
	// a new season never shows the last one's polled state for a frame.
	$effect.pre(() => {
		const season = data.season;
		void attempt;
		polled = null;
		problem = undefined;
		if (!season.episodes_pending) return;
		const n = season.season.season_number;
		return pollSeason(
			untrack(() => show.tmdb_id),
			n,
			{
				onValue: (d) => {
					if (d.episodes_pending) return;
					polled = d;
					void reloadAfterPoll();
				},
				onTimeout: () => (problem = { kind: 'timeout' }),
				onError: (e) => {
					if (e instanceof ApiError && e.status === 404) {
						problem = {
							kind: 'error',
							message: `TMDB has no ${seasonLabel(n).toLowerCase()} for this show.`,
							retry: false
						};
						void invalidate('app:show');
						return;
					}
					problem = { kind: 'error', message: errorMessage(e, catalogMessages), retry: true };
				}
			}
		);
	});

	// The show is cached across seasons and episodes, so it can still mark
	// this season pending from before TMDB stored its episodes. Once the
	// season loads with them, reload the show, once per season: the picker
	// and "Mark show watched" wait on that flag.
	let reloadedFor: number | null = null;
	$effect(() => {
		const season = data.season;
		if (season.episodes_pending) return;
		const n = season.season.season_number;
		if (reloadedFor === n) return;
		if (!show.seasons.some((s) => s.season_number === n && s.episodes_pending)) return;
		reloadedFor = n;
		void reloadAfterPoll();
	});

	// The user's watched episodes: from the load, then reloaded after a
	// mark. Null while they can't be loaded: the episodes then have no
	// checks and the season no progress, so nothing looks unwatched by
	// mistake.
	let watchedList = $derived(data.watched);
	let watchedProblem = $derived(data.watchedError);
	const watched = $derived(watchedList ? watchedById(watchedList) : null);
	const track = latestOnly<EpisodeWatched[]>();

	/** Reloads the watched episodes; throws if that fails. */
	async function refreshWatched() {
		const id = show.tmdb_id;
		const res = await track(showWatched(id));
		// The page is reused across shows: drop a list for a show we've left.
		if (!res.current || show.tmdb_id !== id) return;
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

	// Once the episodes are in: reload the show, which runs this page's load
	// again with it, then the watched list. That load's list may have been
	// fetched before a mark made meanwhile, and land after the mark's reload.
	// The refresh after it is the newest, so latestOnly keeps it.
	async function reloadAfterPoll() {
		await invalidate('app:show');
		try {
			await refreshWatched();
		} catch {
			toast.error("The episode list is out of date. Reload the page to see what's watched.");
		}
	}

	// To tell aired episodes from upcoming ones, by the server's rule (an air
	// date counts from noon UTC).
	const now = new Date();

	const episodes = $derived(details.episodes_pending ? [] : (details.episodes ?? []));
	const rating = $derived(seasonRating(episodes));
	const title = $derived(details.season.name || seasonLabel(number));
	const fullName = $derived(`${show.name} ${title}`);

	// The listed episodes once they're in; until then, the show's count of
	// watched episodes for this season number.
	const progress = $derived.by(() => {
		if (!watchedList || !watched) return null;
		if (!details.episodes_pending) return seasonProgress(episodes, watched);
		return progressBySeason(show.seasons, watchedList).get(number) ?? null;
	});
	const started = $derived((progress?.watched ?? 0) > 0);
	const canMark = $derived(anyAired(episodes, now));
	// The server refuses a season mark until its episodes are stored.
	const disabledReason = $derived(
		details.episodes_pending
			? "This season's episodes are still being fetched from TMDB."
			: canMark
				? ''
				: 'Nothing in this season has aired yet.'
	);
	const release = { label: 'Air dates', detail: "Each episode's own", enabled: true };

	// Results of the season's and the episodes' marks are toasts (DESIGN.md,
	// Toasts); an episode's is named, since it floats away from its row.
	async function markWholeSeason(when: MarkWhen) {
		// Read before the awaits: the user may have moved to another season by
		// the time the mark answers, and the toast and focus are for this one.
		const name = title;
		const aired = canMark;
		const marked = number;
		const first = !started;
		let added: number;
		try {
			added = await markSeason(show.tmdb_id, marked, when);
		} catch (e) {
			toast.error(`${name}: ${errorMessage(e, trackingMessages)}`);
			return;
		}
		toast.show(added > 0 ? 'success' : 'info', `${name}: ${addedMessage(added, aired)}`);
		try {
			await refreshWatched();
		} catch {
			toast.error("The episode list is out of date. Reload the page to see what's watched.");
			return;
		}
		// The first mark: "Mark season watched" became the progress button. Only
		// on the marked season's page, where that button is.
		if (first && started && number === marked) {
			await tick();
			document.getElementById('season-progress')?.focus();
		}
	}

	async function removeWholeSeason(): Promise<string> {
		const name = fullName;
		const removedNumber = number;
		let removed: number;
		try {
			({ removed } = await removeSeasonWatches(show.tmdb_id, removedNumber));
		} catch (e) {
			return errorMessage(e, trackingMessages);
		}
		toast.success(`Removed ${removed} ${removed === 1 ? 'play' : 'plays'} of ${name}.`);
		try {
			await refreshWatched();
		} catch {
			toast.error("The episode list is out of date. Reload the page to see what's watched.");
			return '';
		}
		if (number === removedNumber) {
			await tick();
			document.getElementById('mark-season')?.focus();
		}
		return '';
	}

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
		// A repeat within 3 hours counts as the same watch: not an error.
		if (res.merged_into !== null)
			toast.info(
				`${code} is already in your history: plays within 3 hours count once, so it wasn't added again.`
			);
		else toast.success(`Added ${code} to your history: ${formatWatch(res.watch)}.`);
	}

	// The season's first and last episodes, once for every row's milestone.
	const ends = $derived(seasonEnds(episodes, now));
	const showContext = $derived({
		lastSeason: lastSeasonNumber(show.seasons),
		inProduction: show.in_production
	});

	const count = $derived(details.episodes_pending ? details.season.episode_count : episodes.length);
	const runtime = $derived(formatRuntime(totalRuntime(episodes)));

	const factsList = $derived([
		{ label: 'Premiered', value: formatDay(details.season.air_date) },
		{ label: 'Episodes', value: count ? String(count) : '' },
		{ label: 'Runtime', value: runtime },
		{ label: 'Genres', value: show.genres.map((g) => g.name).join(', ') }
	]);

	// "22 Episodes": the section list's label and the list's heading.
	const episodesLabel = $derived(countLabel(count, 'Episode'));

	const sections = $derived([
		{ id: 'overview', label: 'Overview' },
		{ id: 'people', label: 'People' },
		{ id: 'episodes', label: episodesLabel },
		{ id: 'videos', label: 'Videos' }
	]);

	const pickerItems = $derived(seasonPicker(show, number));
	const arrows = $derived(seasonArrows(show, number));
</script>

<svelte:head>
	<title>{title} · {show.name} · watcht</title>
</svelte:head>

<TitlePage
	{title}
	name={fullName}
	year={airYears(episodes, details.season.air_date)}
	posterUrl={details.season.poster_url || show.poster_url}
	backdropUrl={show.backdrop_url}
	ratings={rating > 0
		? [{ site: 'tmdb', label: 'TMDB', score: rating.toFixed(1), outOf: '10' }]
		: []}
	links={tvPartLinks(show.tmdb_id, number)}
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
		<section aria-label="Track {fullName}" class="flex flex-col">
			{#if progress && started}
				<ProgressButton
					id="season-progress"
					title={fullName}
					{progress}
					{release}
					onmark={markWholeSeason}
					onremoveall={removeWholeSeason}
					{disabledReason}
				/>
			{:else}
				<MarkButton
					id="mark-season"
					label="Mark season watched"
					menuLabel="Choose when you watched {fullName}"
					menuIcon="plus"
					onmark={markWholeSeason}
					{release}
					{disabledReason}
				/>
			{/if}
			{#if watchedList === null}
				<div class="mt-3 flex flex-col items-start gap-2">
					<Notice
						tone="error"
						message="Couldn't load which episodes you've watched: {watchedProblem}"
					/>
					<Button variant="quiet" busy={retrying} onclick={retryWatched}>Try again</Button>
				</div>
			{/if}
			<PlaceholderActions class="mt-4" />
		</section>
	{/snippet}

	{#snippet below()}
		<PeopleSection />
		<TitleSection id="episodes" title={episodesLabel}>
			{#snippet filter()}
				<ListControls items="episodes" />
			{/snippet}
			<div class="mt-2">
				{#if pending || problem}
					<EpisodesFetching
						{problem}
						rows={details.season.episode_count}
						what="this season's episodes"
						onretry={() => attempt++}
					/>
				{:else if episodes.length === 0}
					<p class="py-3 text-sm text-muted">TMDB lists no episodes for this season yet.</p>
				{:else}
					<ul class="divide-y divide-line">
						{#each episodes as episode (episode.tmdb_id)}
							<EpisodeRow
								{episode}
								{now}
								href={resolve('/(app)/shows/[id]/seasons/[n]/episodes/[e]', {
									id: showId,
									n: String(number),
									e: String(episode.episode_number)
								})}
								milestone={milestoneIn(episode, ends, showContext)}
								watched={watched?.get(episode.tmdb_id)}
								onmark={watched ? (when) => markOne(episode, when) : undefined}
								onrefresh={refreshWatched}
							/>
						{/each}
					</ul>
				{/if}
			</div>
		</TitleSection>
	{/snippet}

	{#snippet last()}
		<VideosSection />
	{/snippet}

	<div class="mt-6 max-w-[72ch]">
		<p class="text-[0.9375rem] leading-relaxed text-ink">
			{details.season.overview || show.overview || 'TMDB has no overview for this season yet.'}
		</p>
	</div>
</TitlePage>
