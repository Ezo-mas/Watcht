<script lang="ts">
	import { resolve } from '$app/paths';
	import type { ResolvedPathname } from '$app/types';
	import { tick } from 'svelte';
	import { ApiError } from '#lib/api/client.js';
	import { historyPage, markEpisode, markMovie, removeWatch } from '#lib/api/tracking.js';
	import type { HistoryEntry, MarkResult, MarkWhen } from '#lib/api/types.js';
	import Button from '#lib/components/Button.svelte';
	import Kbd from '#lib/components/Kbd.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import PosterCard from '#lib/components/PosterCard.svelte';
	import WatchedCheck from '#lib/components/WatchedCheck.svelte';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDateTime } from '#lib/format.js';
	import { dayHeading, dayTotal, groupByDay, insertEntry } from '#lib/history.js';
	import { episodeCode, tmdbPercent } from '#lib/media.js';
	import { searchOverlay } from '#lib/search-overlay.svelte.js';
	import { sourceLabel } from '#lib/watched.js';
	import { browserTimeZone, releaseOption, type ReleaseOption } from '#lib/when.js';
	import type { PageProps } from './$types';
	import HistoryToolbar from './HistoryToolbar.svelte';

	let { data }: PageProps = $props();

	// The first page from the load, then each further page appended.
	let entries = $derived(data.history.entries);
	let cursor = $derived(data.history.next_cursor);
	const days = $derived(groupByDay(entries));
	const zone = browserTimeZone();

	let heading = $state<HTMLElement>();
	let note = $state('');

	// More pages: loaded as the end of the list scrolls near, or with the
	// "Load more" button (keyboard users, no IntersectionObserver).

	let moreBusy = $state(false);
	let moreError = $state('');

	/** `focusNew` moves focus to the first new card (the button); scrolling never moves it. */
	async function loadMore(focusNew: boolean) {
		if (cursor === null || moreBusy) return;
		moreBusy = true;
		moreError = '';
		try {
			const next = await historyPage(cursor);
			// Pages don't overlap, but a repeated card would break the keyed
			// list, so skip any already shown.
			const seen = new Set(entries.map((e) => e.id));
			const fresh = next.entries.filter((e) => !seen.has(e.id));
			entries = [...entries, ...fresh];
			cursor = next.next_cursor;
			if (focusNew && fresh.length) {
				await tick();
				document.getElementById(`entry-${fresh[0].id}`)?.focus();
			}
		} catch (e) {
			moreError = errorMessage(e);
		} finally {
			moreBusy = false;
		}
	}

	/**
	 * Loads the next page when the sentinel under the list comes within
	 * 800 px of the viewport. It re-attaches after each page (it reads
	 * `entries`), so a page too short to push the sentinel away loads the
	 * next one too. After a failure it waits for "Try again".
	 */
	function autoLoad(node: HTMLElement) {
		if (cursor === null || moreError || !('IntersectionObserver' in window)) return;
		void entries.length;
		const observer = new IntersectionObserver(
			(items) => {
				if (items.some((i) => i.isIntersecting)) void loadMore(false);
			},
			{ rootMargin: '0px 0px 800px 0px' }
		);
		observer.observe(node);
		return () => observer.disconnect();
	}

	// The success note, fixed at the bottom of the window: the card acted on
	// may be far down the page. It clears itself after a while.
	$effect(() => {
		if (!note) return;
		const timer = setTimeout(() => (note = ''), 8000);
		return () => clearTimeout(timer);
	});

	function describe(entry: HistoryEntry): string {
		const ep = entry.episode;
		if (ep) return `${ep.show.name} ${episodeCode(ep.season_number, ep.episode_number)}`;
		return entry.movie?.title ?? '';
	}

	// The card's watched check opens a menu: remove this play, or add another.

	/** Removes one watch; "" when done, else the message for the menu. */
	async function removeOne(entry: HistoryEntry): Promise<string> {
		note = '';
		try {
			await removeWatch(entry.id);
		} catch (e) {
			// Already gone (removed in another tab) is what was asked for.
			if (!(e instanceof ApiError && e.code === 'not_found')) {
				return errorMessage(e, trackingMessages);
			}
		}
		const at = entries.findIndex((e) => e.id === entry.id);
		const next = entries[at + 1] ?? entries[at - 1];
		entries = entries.filter((e) => e.id !== entry.id);
		note = `Removed ${describe(entry)}, watched ${formatDateTime(entry.watched_at)}.`;
		await tick();
		// Focus the next card's check, so a run of removals stays in place;
		// the heading when the list is now empty.
		const target = next && document.getElementById(`check-${next.id}`);
		if (target) target.focus();
		else heading?.focus();
		return '';
	}

	/** Marks it watched again; "" when done, else the message for the menu. */
	async function addPlay(entry: HistoryEntry, when: MarkWhen): Promise<string> {
		note = '';
		let res: MarkResult;
		try {
			const ep = entry.episode;
			res = ep
				? await markEpisode(ep.tmdb_id, when)
				: await markMovie(entry.movie?.tmdb_id ?? 0, when);
		} catch (e) {
			return errorMessage(e, trackingMessages);
		}
		if (res.merged_into !== null) {
			// A repeat within 3 hours counts as the same watch: no new card.
			const kept = entries.find((e) => e.id === res.merged_into);
			note = kept
				? `Already counted: ${describe(entry)} has a play at ${formatDateTime(kept.watched_at)}, and plays within 3 hours count once.`
				: `Already counted: ${describe(entry)} has a play within 3 hours of that time.`;
			return '';
		}
		const added = { ...entry, ...res.watch };
		const placed = insertEntry(entries, added, cursor !== null);
		entries = placed.entries;
		const at = formatDateTime(added.watched_at);
		note = placed.inserted
			? `Added to your history: ${describe(entry)}, ${at}.`
			: `Added to your history: ${describe(entry)}, ${at}. It's further down; scroll to reach it.`;
		return '';
	}

	/** The menu's release or air date option. */
	function releaseOf(entry: HistoryEntry): ReleaseOption {
		if (entry.movie) return releaseOption('Release date', entry.movie.release_date, now);
		return releaseOption('Air date', entry.episode?.air_date ?? '', now);
	}

	function hrefOf(entry: HistoryEntry): ResolvedPathname {
		const ep = entry.episode;
		return ep
			? resolve(`/(app)/shows/[id]?season=${ep.season_number}`, { id: String(ep.show.tmdb_id) })
			: resolve('/(app)/movies/[id]', { id: String(entry.movie?.tmdb_id ?? 0) });
	}

	const now = new Date();
</script>

<svelte:head>
	<title>History · watcht</title>
</svelte:head>

<!--
	The nav says where you are, so the heading is for screen readers (and
	the focus target when the list empties); the day headings lead visually.
-->
<h1 bind:this={heading} tabindex="-1" class="sr-only">History</h1>

<HistoryToolbar />

<!--
	Results of the cards' menus, announced, and shown at the bottom of the
	window; the [&_p] variant puts the note on a solid surface over the grid.
-->
<div
	class="pointer-events-none fixed inset-x-4 bottom-4 z-30 flex justify-center [&_p]:pointer-events-auto [&_p]:bg-surface [&_p]:shadow-lg"
>
	<Notice tone="success" message={note} class="max-w-md" />
</div>

{#if entries.length === 0 && cursor === null}
	<div class="mt-6 flex max-w-4xl flex-col items-start gap-3 border-t border-line pt-5">
		<p class="max-w-[60ch] text-sm text-ink">
			Nothing in your history yet. Find a movie or show, then mark it watched from its page.
		</p>
		<Button variant="quiet" onclick={() => (searchOverlay.open = true)}>
			Search
			<span aria-hidden="true" class="max-sm:hidden"><Kbd>/</Kbd></span>
		</Button>
	</div>
{:else}
	{#each days as day (day.key)}
		{@const name = dayHeading(day.key)}
		{@const total = dayTotal(day.entries)}
		<section aria-labelledby="day-{day.key}" class="mt-6">
			<!-- Sticky under the header, so the day stays in view while scrolling it. -->
			<div
				class="sticky top-12 z-10 flex items-baseline justify-between gap-3 border-b border-line bg-canvas pt-3 pb-2"
			>
				<h2 id="day-{day.key}" class="text-lg text-ink sm:text-xl">
					<span class="font-bold">{name.weekday}</span>
					{name.date}
				</h2>
				<p
					title={total.label}
					class="flex shrink-0 items-center gap-1.5 text-sm text-muted sm:text-[0.9375rem]"
				>
					{#if total.minutes > 0}
						<svg
							aria-hidden="true"
							viewBox="0 0 16 16"
							class="size-4 self-center"
							fill="none"
							stroke="currentColor"
							stroke-width="1.5"
							stroke-linecap="round"
							><circle cx="8" cy="8" r="6.25" /><path d="M8 4.75V8l2.25 1.5" /></svg
						>
					{/if}
					<span aria-hidden="true" class="tabular-nums">{total.text}</span>
					<span class="sr-only">{total.label}</span>
				</p>
			</div>
			<ul
				class="mt-4 grid grid-cols-2 gap-x-3 gap-y-5 sm:grid-cols-[repeat(auto-fill,minmax(10.5rem,1fr))] sm:gap-x-4 sm:gap-y-6"
			>
				{#each day.entries as entry (entry.id)}
					{@const ep = entry.episode}
					{@const code = ep ? episodeCode(ep.season_number, ep.episode_number) : ''}
					{@const title = ep ? ep.name : (entry.movie?.title ?? '')}
					{@const source = sourceLabel(entry.source)}
					{@const rating = tmdbPercent(ep ? ep.vote_average : (entry.movie?.vote_average ?? 0))}
					{@const when = formatDateTime(entry.watched_at)}
					<li>
						<PosterCard
							href={hrefOf(entry)}
							poster={ep ? ep.show.poster_url || ep.still_url : (entry.movie?.poster_url ?? '')}
							label={ep ? ep.show.name : title}
							linkId="entry-{entry.id}"
							corner={source}
						>
							<!-- One line each, so every card is the same height; the tooltip has the rest. -->
							<span
								title={ep ? `${code} ${title}` : title}
								class="block truncate text-[0.9375rem] leading-snug font-semibold text-ink group-hover/link:underline"
							>
								{#if ep}
									<span class="sr-only">{ep.show.name}</span>
									<span class="font-bold text-accent-text tabular-nums">{code}</span>
								{/if}
								{title}
							</span>
							<span class="mt-0.5 block truncate text-xs text-muted tabular-nums">
								<time datetime={entry.watched_at}>{when}</time>{#if source}<span class="sr-only"
										>, recorded by {source}</span
									>{/if}
							</span>

							{#snippet bar()}
								<!-- Every card here is watched; the check's menu removes it or adds another play. -->
								<WatchedCheck
									name={describe(entry)}
									watchedAt={entry.watched_at}
									runtime={ep ? ep.runtime : (entry.movie?.runtime ?? 0)}
									release={releaseOf(entry)}
									buttonId="check-{entry.id}"
									zoneNote="Browser time zone: {zone}"
									onremove={() => removeOne(entry)}
									onadd={(w) => addPlay(entry, w)}
								/>
								<!-- The watchlist arrives in slice 3; until then this does nothing and says so. -->
								<button
									type="button"
									aria-disabled="true"
									aria-label="Add to watchlist (coming in a later update)"
									title="Add to watchlist (coming in a later update)"
									class="grid size-10 shrink-0 cursor-not-allowed place-items-center rounded-sm text-on-screen-muted/40 focus-visible:outline-on-screen-accent sm:size-8"
								>
									<svg
										aria-hidden="true"
										viewBox="0 0 16 16"
										class="size-4"
										fill="none"
										stroke="currentColor"
										stroke-width="1.8"
										stroke-linecap="round"
										><path d="M6 4h7.5M6 8h7.5M6 12h7.5M2.5 4h.01M2.5 8h.01M2.5 12h.01" /></svg
									>
								</button>
								{#if rating}
									<span
										title="TMDB rating"
										class="ml-auto flex items-center gap-1 pr-1.5 text-xs font-semibold text-on-screen tabular-nums"
									>
										<svg
											aria-hidden="true"
											viewBox="0 0 16 16"
											class="size-3.5 text-on-screen-accent"
											fill="currentColor"
											><path
												d="M8 14s-5.75-3.4-5.75-7.6A3.15 3.15 0 0 1 8 4.5a3.15 3.15 0 0 1 5.75 1.9C13.75 10.6 8 14 8 14z"
											/></svg
										>
										<span class="sr-only">TMDB rating</span>
										{rating}
									</span>
								{/if}
							{/snippet}
						</PosterCard>
					</li>
				{/each}
			</ul>
		</section>
	{/each}

	{#if cursor !== null}
		<!-- Scrolling near this loads the next page. -->
		<div {@attach autoLoad}></div>
	{/if}
	<div class="mt-8 flex flex-col items-start gap-3 border-t border-line pt-4">
		{#if cursor !== null}
			<Notice tone="error" message={moreError} />
			<!-- One button throughout, so focus stays on it from "Try again" to "Loading…". -->
			<Button variant="quiet" busy={moreBusy} onclick={() => loadMore(true)}>
				{moreBusy ? 'Loading…' : moreError ? 'Try again' : 'Load more'}
			</Button>
		{:else}
			<p class="text-sm text-muted">That's the start of your history.</p>
		{/if}
	</div>
{/if}
