<script lang="ts">
	import { tick } from 'svelte';
	import type { Episode, EpisodeWatched, MarkWhen } from '#lib/api/types.js';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatDay, formatWatch } from '#lib/format.js';
	import { episodeCode, formatRuntime } from '#lib/media.js';
	import { hasAired, playsLabel } from '#lib/watched.js';
	import { releaseOption } from '#lib/when.js';
	import Artwork from './Artwork.svelte';
	import Badge from './Badge.svelte';
	import WhenMenu from './WhenMenu.svelte';

	interface Props {
		episode: Episode;
		/** The current time, to tell aired episodes from upcoming ones. */
		now: Date;
		/** How often the user watched it; undefined when they haven't. */
		watched?: EpisodeWatched;
		/**
		 * Marks the episode watched. Resolves with a note to show on the row
		 * (or nothing); throws an ApiError on failure. Without it the row has
		 * no watched check.
		 */
		onmark?: (when: MarkWhen) => Promise<string | void>;
	}

	let { episode, now, watched, onmark }: Props = $props();

	const code = $derived(episodeCode(episode.season_number, episode.episode_number));
	const runtime = $derived(formatRuntime(episode.runtime));
	// The server's rule: an air date counts from noon UTC that day.
	const aired = $derived(hasAired(episode.air_date, now));
	const release = $derived(releaseOption('Air date', episode.air_date, now));

	let busy = $state(false);
	let note = $state('');
	let error = $state('');
	let actions = $state<HTMLElement>();

	async function mark(when: MarkWhen) {
		if (!onmark || busy) return;
		// Marking from the check replaces it with the watched mark, so
		// focus moves on to the row's when-choice instead of getting lost.
		const fromCheck = document.activeElement?.hasAttribute('data-check') ?? false;
		busy = true;
		note = '';
		error = '';
		try {
			note = (await onmark(when)) ?? '';
		} catch (e) {
			error = errorMessage(e, trackingMessages);
		} finally {
			busy = false;
		}
		if (fromCheck) {
			await tick();
			actions?.querySelector<HTMLElement>('button')?.focus();
		}
	}

	const box = 'grid size-10 place-items-center sm:size-8';
</script>

<!--
	One episode in the classic layout: still, code and title, date and
	runtime, overview, and the watched check on the right. Its heading is an
	h4, under a season's h3.
-->
<li
	class={[
		'grid gap-3 py-3 sm:gap-4',
		onmark
			? 'grid-cols-[7.5rem_minmax(0,1fr)_2.5rem] sm:grid-cols-[13rem_minmax(0,1fr)_3.5rem]'
			: 'grid-cols-[7.5rem_minmax(0,1fr)] sm:grid-cols-[13rem_minmax(0,1fr)]'
	]}
>
	<Artwork src={episode.still_url} kind="still" lazy class="self-start rounded-sm" />
	<div class="min-w-0">
		<h4 class="text-[0.9375rem] leading-snug font-semibold text-ink">
			<span class="mr-1 font-bold text-accent-text tabular-nums">{code}</span>
			{episode.name}
		</h4>
		<div class="mt-1 flex flex-wrap gap-1">
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
		</div>
		{#if episode.overview}
			<p class="mt-1.5 line-clamp-3 max-w-[72ch] text-sm text-muted sm:line-clamp-none">
				{episode.overview}
			</p>
		{/if}
		{#if onmark}
			{#if error}
				<p role="alert" class="mt-1.5 text-xs text-danger">{error}</p>
			{/if}
			<p role="status" class={['text-xs text-muted', note && 'mt-1.5']}>{note}</p>
		{/if}
	</div>

	{#if onmark}
		<!--
			Watched is a mark, not a button: a stray click mustn't undo it
			(watches are removed on /history). The chevron still offers
			another watch. Any episode can be marked just now or at another
			date, aired or not (maintainer decision); only "Air date" needs a
			passed date. The badge shows an unaired one.
		-->
		<div
			bind:this={actions}
			class="flex flex-col items-center gap-1 self-start sm:flex-row sm:items-start sm:gap-0"
		>
			{#if watched}
				<span
					role="img"
					aria-label="Watched"
					title="Watched"
					class={[box, 'rounded-md bg-watched text-surface sm:rounded-r-none']}
				>
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
				</span>
			{:else}
				<button
					type="button"
					data-check
					aria-label="Mark {code} “{episode.name}” watched"
					title="Mark watched just now"
					disabled={busy}
					aria-busy={busy || undefined}
					onclick={() => mark({ at: 'now' })}
					class={[
						box,
						'group rounded-md border border-line-strong text-muted transition-colors hover:border-watched hover:bg-watched/10 hover:text-watched disabled:cursor-wait sm:rounded-r-none'
					]}
				>
					{#if busy}
						<span
							aria-hidden="true"
							class="size-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
						></span>
					{:else}
						<svg
							aria-hidden="true"
							viewBox="0 0 16 16"
							class="size-4 opacity-40 group-hover:opacity-100"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"><path d="M3 8.5l3.2 3L13 4.5" /></svg
						>
					{/if}
				</button>
			{/if}
			<WhenMenu
				label={watched
					? `Mark ${code} watched again, choosing when`
					: `Choose when you watched ${code}`}
				onmark={mark}
				{release}
				{busy}
				class="h-10 w-10 rounded-md border border-line text-muted hover:border-line-strong hover:text-ink disabled:opacity-60 sm:h-8 sm:w-6 sm:rounded-l-none sm:border-l-0"
			/>
		</div>
	{/if}
</li>
