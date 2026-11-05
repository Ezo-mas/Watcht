<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { getSeason } from '#lib/api/catalog.js';
	import { ApiError } from '#lib/api/client.js';
	import { markEpisode, markSeason } from '#lib/api/tracking.js';
	import type { Episode, EpisodeWatched, MarkWhen, SeasonDetails, Show } from '#lib/api/types.js';
	import Button from '#lib/components/Button.svelte';
	import EpisodeRow from '#lib/components/EpisodeRow.svelte';
	import MarkButton from '#lib/components/MarkButton.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import { catalogMessages, errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDay, formatWatch } from '#lib/format.js';
	import { defaultSeason, parseSeasonParam, seasonLabel } from '#lib/media.js';
	import { poll } from '#lib/poll.js';
	import { addedMessage, anyAired, seasonProgress } from '#lib/watched.js';

	interface Props {
		show: Show;
		/**
		 * The user's watched episodes, by TMDB episode ID; null while they
		 * can't be loaded (the rows then have no checks, and there's no
		 * progress).
		 */
		watched: Map<number, EpisodeWatched> | null;
		/** Reloads `watched` after a mark; throws if that fails. */
		refreshWatched: () => Promise<void>;
	}

	let { show, watched, refreshWatched }: Props = $props();

	// Regular seasons in order, specials last.
	const seasons = $derived(
		[...show.seasons].sort(
			(a, b) =>
				(a.season_number === 0 ? Infinity : a.season_number) -
				(b.season_number === 0 ? Infinity : b.season_number)
		)
	);

	// ?season=N picks the season; without it, the first regular one.
	const requested = $derived(parseSeasonParam(page.url.searchParams.get('season')));
	const number = $derived(requested ?? defaultSeason(show.seasons));
	const showId = $derived(show.tmdb_id);
	const listed = $derived(seasons.find((s) => s.season_number === number));

	type View =
		| { kind: 'loading' }
		| { kind: 'pending' }
		| { kind: 'ready'; details: SeasonDetails }
		| { kind: 'timeout' }
		| { kind: 'error'; message: string; retry: boolean };
	let view = $state<View>({ kind: 'loading' });
	let attempt = $state(0);

	// Load the season; while the server is still fetching its episodes from
	// TMDB, ask again every 3 seconds for up to 2 minutes. Switching season
	// or leaving the page cancels it.
	$effect(() => {
		const id = showId;
		const n = number;
		void attempt;
		if (n === null) return;
		view = { kind: 'loading' };
		// When the season was pending, the show's season list is out of date
		// once it's done: reload it so the marks are right and seasons TMDB
		// doesn't have drop out.
		let wasPending = untrack(
			() => show.seasons.find((s) => s.season_number === n)?.episodes_pending ?? false
		);
		return poll<SeasonDetails>({
			load: (signal) =>
				getSeason(id, n, ((input, init) => fetch(input, { ...init, signal })) as typeof fetch),
			done: (d) => !d.episodes_pending,
			onValue: (d) => {
				if (d.episodes_pending) {
					wasPending = true;
					view = { kind: 'pending' };
					return;
				}
				view = { kind: 'ready', details: d };
				if (wasPending) void invalidate('app:show');
			},
			onTimeout: () => (view = { kind: 'timeout' }),
			onError: (e) => {
				if (e instanceof ApiError && e.status === 404) {
					view = {
						kind: 'error',
						message: `TMDB has no ${n === 0 ? 'specials' : `season ${n}`} for this show.`,
						retry: false
					};
					if (wasPending) void invalidate('app:show');
					return;
				}
				view = { kind: 'error', message: errorMessage(e, catalogMessages), retry: true };
			}
		});
	});

	// To tell aired episodes from upcoming ones, by the server's rule (an air
	// date counts from noon UTC).
	const now = new Date();

	const episodes = $derived(view.kind === 'ready' ? (view.details.episodes ?? []) : []);
	const progress = $derived(watched ? seasonProgress(episodes, watched) : null);
	const canMarkSeason = $derived(anyAired(episodes, now));
	// The server refuses a season mark until its episodes are stored.
	const seasonDisabled = $derived(
		view.kind === 'pending' || view.kind === 'loading'
			? "This season's episodes are still being fetched from TMDB."
			: canMarkSeason
				? ''
				: 'Nothing in this season has aired yet.'
	);

	// Feedback for the season's mark; cleared when the season changes.
	let seasonNote = $state('');
	let seasonProblem = $state('');
	$effect(() => {
		void number;
		seasonNote = '';
		seasonProblem = '';
	});

	async function markWholeSeason(when: MarkWhen) {
		if (number === null) return;
		seasonNote = '';
		seasonProblem = '';
		let added: number;
		try {
			added = await markSeason(showId, number, when);
		} catch (e) {
			seasonProblem = errorMessage(e, trackingMessages);
			return;
		}
		seasonNote = addedMessage(added, canMarkSeason);
		try {
			await refreshWatched();
		} catch {
			seasonProblem = "The episode list is out of date. Reload the page to see what's watched.";
		}
	}

	/** Marks one episode; the row shows the note returned, or the error thrown. */
	async function markOne(episode: Episode, when: MarkWhen): Promise<string> {
		const res = await markEpisode(episode.tmdb_id, when);
		try {
			await refreshWatched();
		} catch {
			return 'Marked, but this list is out of date. Reload the page to see it.';
		}
		// A repeat within 3 hours counts as the same watch.
		if (res.merged_into !== null) return "Already in your history, so it wasn't added again.";
		return `Added to your history: ${formatWatch(res.watch)}.`;
	}

	// Up to three placeholder rows while a season loads.
	const skeletonRows = $derived(
		Array.from({ length: Math.min(Math.max(listed?.episode_count ?? 3, 1), 3) }, (_, i) => i)
	);
</script>

<section id="seasons" aria-labelledby="seasons-title" class="mt-8 scroll-mt-16">
	<h2 id="seasons-title" class="border-b border-line pb-2 text-lg font-semibold tracking-tight">
		Seasons
	</h2>

	{#if seasons.length === 0}
		<p class="py-4 text-sm text-muted">TMDB lists no seasons for this show yet.</p>
	{:else}
		<!--
			Links, so a season can be opened in a new tab; they replace the
			history entry, so switching seasons doesn't fill Back.
		-->
		<nav aria-label="Seasons" class="mt-3">
			<ul class="flex flex-wrap gap-1.5">
				{#each seasons as s (s.tmdb_id)}
					<li>
						<a
							href={resolve(`/(app)/shows/[id]?season=${s.season_number}`, { id: String(showId) })}
							data-sveltekit-replacestate
							data-sveltekit-reset="false"
							aria-current={s.season_number === number ? 'true' : undefined}
							title={s.episodes_pending
								? 'Episodes still loading from TMDB'
								: `${s.episode_count} episodes`}
							class={[
								'flex h-8 min-w-8 items-center justify-center rounded-md border px-2.5 text-sm font-semibold tabular-nums transition-colors',
								s.season_number === number
									? 'border-accent bg-accent text-on-accent'
									: 'border-line bg-surface text-ink hover:border-line-strong',
								s.episodes_pending && s.season_number !== number && 'border-dashed text-muted'
							]}
						>
							{#if s.season_number === 0}Specials{:else}<span class="sr-only"
									>Season
								</span>{s.season_number}{/if}
						</a>
					</li>
				{/each}
			</ul>
		</nav>

		{#if number !== null}
			<div class="mt-5 flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
				<div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
					<h3 class="text-base font-semibold text-ink">
						{listed?.name || seasonLabel(number)}
					</h3>
					{#if listed}
						<p class="flex flex-wrap gap-x-3 text-sm text-muted tabular-nums">
							<span
								>{listed.episode_count} {listed.episode_count === 1 ? 'episode' : 'episodes'}</span
							>
							{#if listed.air_date}<span>Premiered {formatDay(listed.air_date)}</span>{/if}
						</p>
					{/if}
				</div>
				{#if episodes.length > 0 || view.kind === 'pending'}
					<div class="flex flex-wrap items-start gap-x-4 gap-y-2">
						{#if progress && progress.total > 0}
							<p class="flex h-8 items-center gap-2 text-sm text-muted tabular-nums">
								<span
									aria-hidden="true"
									class="block h-1.5 w-16 overflow-hidden rounded-full bg-line"
								>
									<span
										class="block h-full rounded-full bg-watched"
										style:width="{(progress.watched / progress.total) * 100}%"
									></span>
								</span>
								<span
									><span class="font-semibold text-ink">{progress.watched}</span> of {progress.total}
									watched</span
								>
							</p>
						{/if}
						<MarkButton
							variant="quiet"
							label="Mark season watched"
							menuLabel="Choose when you watched {listed?.name || seasonLabel(number)}"
							onmark={markWholeSeason}
							release={{ label: 'Air dates', detail: "Each episode's own", enabled: true }}
							disabledReason={seasonDisabled}
						/>
					</div>
				{/if}
			</div>
			{#if listed?.overview}
				<p class="mt-1.5 max-w-[72ch] text-sm text-muted">{listed.overview}</p>
			{/if}
			<Notice tone="success" message={seasonNote} class="mt-3" />
			<Notice tone="error" message={seasonProblem} class="mt-3" />

			<div class="mt-2">
				{#if view.kind === 'loading' || view.kind === 'pending'}
					{#if view.kind === 'pending'}
						<p role="status" class="flex items-center gap-2 py-2 text-sm text-ink">
							<span
								aria-hidden="true"
								class="size-3.5 animate-spin rounded-full border-2 border-accent border-r-transparent"
							></span>
							Fetching episodes from TMDB…
							<span class="text-muted max-sm:hidden">This takes a few seconds the first time.</span>
						</p>
					{/if}
					<ul aria-hidden="true" class="divide-y divide-line">
						{#each skeletonRows as i (i)}
							<li
								class="grid grid-cols-[7.5rem_minmax(0,1fr)] gap-3 py-3 sm:grid-cols-[13rem_minmax(0,1fr)] sm:gap-4"
							>
								<span class="aspect-video animate-pulse rounded-sm bg-raised"></span>
								<span class="flex flex-col gap-2 pt-1">
									<span class="h-4 w-2/5 animate-pulse rounded-sm bg-raised"></span>
									<span class="h-3 w-24 animate-pulse rounded-sm bg-raised"></span>
									<span class="h-3 w-4/5 animate-pulse rounded-sm bg-raised"></span>
								</span>
							</li>
						{/each}
					</ul>
				{:else if view.kind === 'timeout'}
					<div class="flex flex-col items-start gap-3 py-3">
						<p class="text-sm text-ink">
							TMDB hasn't sent this season's episodes yet. watcht keeps trying in the background.
						</p>
						<Button variant="quiet" onclick={() => attempt++}>Check again</Button>
					</div>
				{:else if view.kind === 'error'}
					<div class="flex flex-col items-start gap-3 py-3">
						<Notice tone="error" message={view.message} />
						{#if view.retry}
							<Button variant="quiet" onclick={() => attempt++}>Try again</Button>
						{/if}
					</div>
				{:else if episodes.length === 0}
					<p class="py-3 text-sm text-muted">TMDB lists no episodes for this season yet.</p>
				{:else}
					<ul class="divide-y divide-line">
						{#each episodes as episode (episode.tmdb_id)}
							<EpisodeRow
								{episode}
								{now}
								watched={watched?.get(episode.tmdb_id)}
								onmark={watched ? (when) => markOne(episode, when) : undefined}
							/>
						{/each}
					</ul>
				{/if}
			</div>
		{/if}
	{/if}
</section>
