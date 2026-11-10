<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import { tick } from 'svelte';
	import { ApiError } from '#lib/api/client.js';
	import { episodeWatches, removeEpisodeWatches, removeWatch } from '#lib/api/tracking.js';
	import type { Episode, EpisodeWatched, MarkWhen } from '#lib/api/types.js';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDay, formatWatch } from '#lib/format.js';
	import { episodeCode, formatRuntime, tmdbPercent } from '#lib/media.js';
	import type { Milestone } from '#lib/seasons.js';
	import { hasAired, playsLabel } from '#lib/watched.js';
	import { toast } from '#lib/toast.svelte.js';
	import { stripBar, stripCheck } from '#lib/strip.js';
	import { releaseOption } from '#lib/when.js';
	import Artwork from './Artwork.svelte';
	import Badge from './Badge.svelte';
	import StripRating from './StripRating.svelte';
	import StripWatchlist from './StripWatchlist.svelte';
	import WhenMenu from './WhenMenu.svelte';

	interface Props {
		episode: Episode;
		/** The episode's page (a resolve()d path): the title and the still open it. */
		href: ResolvedPathname;
		/** The current time, to tell aired episodes from upcoming ones. */
		now: Date;
		/** How often the user watched it; undefined when they haven't. */
		watched?: EpisodeWatched;
		/** A premiere or finale badge (episodeMilestone); "" for none. */
		milestone?: Milestone | '';
		/**
		 * Marks the episode watched, showing its own result toast; throws an
		 * ApiError on failure (the row shows that as an error toast). Without
		 * it the card has no watched check.
		 */
		onmark?: (when: MarkWhen) => Promise<void>;
		/** Reloads the page’s watched list after removing a play. */
		onrefresh?: () => Promise<void>;
	}

	let { episode, href, now, watched, milestone = '', onmark, onrefresh }: Props = $props();

	const code = $derived(episodeCode(episode.season_number, episode.episode_number));
	const runtime = $derived(formatRuntime(episode.runtime));
	// The server's rule: an air date counts from noon UTC that day.
	const aired = $derived(hasAired(episode.air_date, now));
	const release = $derived(releaseOption('Air date', episode.air_date, now));
	const rating = $derived(tmdbPercent(episode.vote_average));

	let busy = $state(false);
	let check = $state<HTMLElement>();
	let titleLink = $state<HTMLAnchorElement>();

	async function removeNewest() {
		if (busy || !onrefresh) return;
		const name = code;
		busy = true;
		try {
			const [latest] = await episodeWatches(episode.tmdb_id);
			if (latest) {
				try {
					await removeWatch(latest.id);
				} catch (e) {
					if (!(e instanceof ApiError && e.code === 'not_found')) throw e;
				}
				toast.success(`${name}: Removed the play from ${formatWatch(latest)}.`);
			} else {
				toast.info(`${name} is already absent from your history.`);
			}
			try {
				await onrefresh();
			} catch {
				toast.error(
					`${name}: The episode list is out of date. Reload the page to see what's watched.`
				);
			}
		} catch (e) {
			toast.error(`${name}: ${errorMessage(e, trackingMessages)}`);
		} finally {
			busy = false;
		}
	}

	async function removeAll() {
		if (busy || !onrefresh) return 'A watch change is already running.';
		busy = true;
		try {
			const { removed } = await removeEpisodeWatches(episode.tmdb_id);
			toast.success(`Removed ${playsLabel(removed)} of ${code} “${episode.name}”.`);
			try {
				await onrefresh();
			} catch {
				toast.error(
					`${code}: The episode list is out of date. Reload the page to see what's watched.`
				);
			}
			return '';
		} catch (e) {
			return errorMessage(e, trackingMessages);
		} finally {
			busy = false;
		}
	}

	async function mark(when: MarkWhen) {
		if (!onmark || busy) return;
		// The when-choice hands focus back to the check, which stays (it
		// turns sage). If focus is lost anyway (the list reloaded), it moves
		// to the episode's title link rather than the page.
		const fromCheck = !!check && check.contains(document.activeElement);
		busy = true;
		try {
			await onmark(when);
		} catch (e) {
			// No menu is open by now, so the failure is a toast, named.
			toast.error(`${code}: ${errorMessage(e, trackingMessages)}`);
		} finally {
			busy = false;
		}
		if (!fromCheck) return;
		await tick();
		const active = document.activeElement;
		if (active && active !== document.body && active.isConnected) return;
		titleLink?.focus();
	}
</script>

<!--
	One episode of a season, after classic Trakt's season page: an episode
	card on the left (the still, and the dark bar with the watched check,
	the watchlist placeholder, and TMDB's score), and the code and title,
	badges, and overview on the right. On phones the card sits above. Its
	heading is an h3, under the Episodes h2.
-->
<li
	class="relative isolate grid gap-3 py-4 has-[[aria-expanded=true]]:z-20 sm:grid-cols-[13rem_minmax(0,1fr)] sm:gap-5 lg:grid-cols-[15rem_minmax(0,1fr)]"
>
	<div class="self-start">
		<!-- The title is the link; the still repeats it for the mouse only. -->
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<a {href} tabindex="-1" aria-hidden="true" data-sveltekit-preload-data="tap" class="block">
			<Artwork src={episode.still_url} kind="still" lazy class="rounded-t-sm" />
		</a>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
		<!--
			Dark in both themes: the screen's text colours on `strip`. No
			padding: the check sits flush in the card's bottom-left corner.
		-->
		<div class={stripBar}>
			{#if onmark}
				<div bind:this={check} class="flex">
					<WhenMenu
						label={watched
							? `Remove or add plays of ${code} “${episode.name}”…`
							: `Mark ${code} “${episode.name}” watched…`}
						title={watched ? 'Remove or add plays…' : 'Mark watched…'}
						onremove={watched && onrefresh ? removeNewest : undefined}
						onremoveall={watched && onrefresh ? removeAll : undefined}
						removeTitle={`${code} “${episode.name}”`}
						onmark={mark}
						{release}
						working={busy}
						popup
						align="start"
						class={watched ? stripCheck('all', false, true) : stripCheck('none')}
					>
						{#snippet trigger()}
							{#if busy}
								<span
									aria-hidden="true"
									class="size-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
								></span>
							{:else}
								{@render checkIcon()}
							{/if}
						{/snippet}
					</WhenMenu>
				</div>
			{/if}
			<StripWatchlist what={code} />
			{#if rating}<StripRating {rating} />{/if}
		</div>
	</div>

	<div class="min-w-0">
		<h3 class="text-base leading-snug font-semibold text-ink">
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a
				bind:this={titleLink}
				{href}
				data-sveltekit-preload-data="tap"
				class="underline-offset-2 hover:underline"
				><span class="mr-1 font-bold text-accent-text tabular-nums">{code}</span>
				{episode.name}</a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		</h3>
		<div class="mt-1.5 flex flex-wrap gap-1">
			{#if episode.air_date === ''}
				<Badge tone="outline">No air date</Badge>
			{:else if !aired}
				<Badge tone="outline">Airs {formatDay(episode.air_date)}</Badge>
			{:else}
				<Badge tone="accent"
					><time datetime={episode.air_date}>{formatDay(episode.air_date)}</time></Badge
				>
			{/if}
			{#if runtime}
				<Badge><span class="sr-only">Runtime </span>{runtime}</Badge>
			{/if}
			{#if watched}
				<Badge
					title="Last watched {formatWatch({
						watched_at: watched.last_watched_at,
						watched_on: watched.last_watched_on
					})}"><span class="sr-only">Watched,&nbsp;</span>{playsLabel(watched.plays)}</Badge
				>
			{/if}
			{#if milestone}
				<Badge tone="strong">{milestone}</Badge>
			{/if}
		</div>
		{#if episode.overview}
			<p class="mt-2 line-clamp-3 max-w-[72ch] text-sm leading-relaxed text-muted sm:line-clamp-4">
				{episode.overview}
			</p>
		{/if}
	</div>
</li>

{#snippet checkIcon()}
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
