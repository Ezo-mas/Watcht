<script lang="ts">
	import { resolve } from '$app/paths';
	import { tick } from 'svelte';
	import { removeSeasonWatches } from '#lib/api/tracking.js';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { toast } from '#lib/toast.svelte.js';
	import { playsLabel } from '#lib/watched.js';
	import type { EpisodeWatched, MarkWhen, Show } from '#lib/api/types.js';
	import ListControls from '#lib/components/ListControls.svelte';
	import PosterCard from '#lib/components/PosterCard.svelte';
	import StripWatchlist from '#lib/components/StripWatchlist.svelte';
	import TitleSection from '#lib/components/TitleSection.svelte';
	import WhenMenu from '#lib/components/WhenMenu.svelte';
	import { seasonLabel } from '#lib/media.js';
	import { progressBySeason, seasonList, seasonMark, seasonsLabel } from '#lib/seasons.js';
	import { stripCheck } from '#lib/strip.js';
	import { checkState } from '#lib/watched.js';

	interface Props {
		show: Show;
		/** The user's watched episodes; null while they can't be loaded (then no progress or marks). */
		watched: EpisodeWatched[] | null;
		/** The current time, to tell whether a season has started airing. */
		now: Date;
		/**
		 * Marks a season's aired, unwatched episodes, with its own result
		 * toasts, then reloads the watched list; resolves when that's done.
		 */
		onmark: (seasonNumber: number, when: MarkWhen) => Promise<void>;
		onrefresh: () => Promise<void>;
	}

	let { show, watched, now, onmark, onrefresh }: Props = $props();

	const seasons = $derived(seasonList(show.seasons));
	const progress = $derived(watched ? progressBySeason(show.seasons, watched) : null);
	const heading = $derived(seasonsLabel(show.seasons));
	const release = { label: 'Air dates', detail: "Each episode's own", enabled: true };
	const uid = $props.id();

	// The seasons being marked: their checks spin, and a second mark is ignored.
	let marking = $state<number[]>([]);

	async function removeAll(n: number, name: string) {
		if (marking.includes(n)) return 'A watch change is already running.';
		marking = [...marking, n];
		try {
			const { removed } = await removeSeasonWatches(show.tmdb_id, n);
			toast.success(`Removed ${playsLabel(removed)} of ${show.name} – ${name}.`);
			try {
				await onrefresh();
			} catch {
				toast.error("Your progress here is out of date. Reload the page to see what's watched.");
			}
			return '';
		} catch (e) {
			return errorMessage(e, trackingMessages);
		} finally {
			marking = marking.filter((m) => m !== n);
		}
	}

	async function mark(n: number, when: MarkWhen) {
		if (marking.includes(n)) return;
		// Focus in this card's bar (the check, which the when-choice returns
		// it to).
		const card = document.getElementById(`${uid}-card-${n}`);
		const fromCard = !!card && card.contains(document.activeElement);
		marking = [...marking, n];
		try {
			await onmark(n, when);
		} finally {
			marking = marking.filter((m) => m !== n);
		}
		// Keep focus on the card if reloading replaced its check.
		if (!fromCard) return;
		await tick();
		const active = document.activeElement;
		if (active && active !== document.body && active.isConnected) return;
		document.getElementById(`${uid}-link-${n}`)?.focus();
	}
</script>

<!--
	The show's seasons as poster cards, after classic Trakt: regular
	seasons in order, specials last, each opening its season page. A thin
	sage bar along the poster shows how much of it is watched, and the
	caption says it. The bar's check shows it too (none, some, all) and
	marks the season: its aired episodes not watched yet, so on a partly
	watched season, the rest. Episodes are listed on the season page
	(maintainer decision).
-->
<TitleSection id="seasons" title={heading}>
	{#snippet filter()}
		<ListControls items="seasons" field="name" />
	{/snippet}
	{#if seasons.length === 0}
		<p class="mt-3 text-sm text-muted">TMDB lists no seasons for this show yet.</p>
	{:else}
		<ul
			class="mt-5 grid grid-cols-2 gap-x-3 gap-y-5 sm:grid-cols-[repeat(auto-fill,minmax(10.5rem,1fr))] sm:gap-x-4 sm:gap-y-6"
		>
			{#each seasons as season (season.tmdb_id)}
				{@const n = season.season_number}
				{@const name = season.name || seasonLabel(n)}
				{@const p = progress?.get(n)}
				{@const state = p ? seasonMark(season, p, now) : null}
				{@const busy = marking.includes(n)}
				{@const episodes = `${season.episode_count} ${season.episode_count === 1 ? 'episode' : 'episodes'}`}
				<li id="{uid}-card-{n}">
					<PosterCard
						href={resolve('/(app)/shows/[id]/seasons/[n]', {
							id: String(show.tmdb_id),
							n: String(n)
						})}
						linkId="{uid}-link-{n}"
						poster={season.poster_url || show.poster_url}
						label="{show.name}: {name}"
						tooltip="{show.name} – {name}"
						progress={p && p.total > 0 ? p.watched / p.total : p ? 0 : undefined}
					>
						<span
							class="block truncate text-[0.9375rem] leading-snug font-semibold text-ink group-hover/link:underline"
							>{name}</span
						>
						<span class="mt-0.5 block truncate text-xs text-muted tabular-nums">
							{#if p && p.watched > 0}
								{p.watched} of {p.total} watched
							{:else}
								{episodes}
							{/if}
						</span>

						{#snippet bar()}
							{#if state && p}
								{@const off = state.kind === 'off'}
								{@const rest = p.watched > 0 && state.kind !== 'done'}
								{@const reasonId = `${uid}-reason-${n}`}
								<!--
									The check opens the when-choice (just now, air dates, other
									date): one click and Enter marks it just now. Partly watched,
									it's a sage tint and marks the rest.
								-->
								<WhenMenu
									triggerId="{uid}-check-{n}"
									label={state.kind === 'done'
										? `Remove or add plays of ${name}…`
										: rest
											? `Watch remaining episodes of ${name}…`
											: `Mark ${name} watched…`}
									title={state.kind === 'off'
										? state.reason
										: state.kind === 'done'
											? 'Remove or add plays…'
											: rest
												? 'Mark the rest watched…'
												: 'Mark watched…'}
									onmark={(when) => mark(n, when)}
									onremoveall={p.watched > 0 ? () => removeAll(n, name) : undefined}
									removeTitle={`${show.name} – ${name}`}
									remaining={rest}
									watchAgainDisabledReason={state.kind === 'done'
										? 'Season rewatching is pending: the current mark only adds unwatched episodes.'
										: ''}
									{release}
									{off}
									working={busy}
									describedby={off ? reasonId : undefined}
									popup
									align="auto"
									class={stripCheck(checkState(p), off, true)}
								>
									{#snippet trigger()}
										{#if busy}
											<span
												aria-hidden="true"
												class="size-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
											></span>
										{:else}
											{@render check()}
										{/if}
									{/snippet}
								</WhenMenu>
								{#if state.kind === 'off'}
									<span id={reasonId} hidden>{state.reason}</span>
								{/if}
							{/if}
							<StripWatchlist what={name} />
						{/snippet}
					</PosterCard>
				</li>
			{/each}
		</ul>
	{/if}
</TitleSection>

{#snippet check()}
	<svg
		aria-hidden="true"
		viewBox="0 0 16 16"
		class="size-4"
		fill="none"
		stroke="currentColor"
		stroke-width="2.4"
		stroke-linecap="round"
		stroke-linejoin="round"><path d="M3 8.5l3.2 3L13 4.5" /></svg
	>
{/snippet}
