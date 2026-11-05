<script lang="ts">
	import { tick } from 'svelte';
	import { ApiError } from '#lib/api/client.js';
	import { markMovie, movieWatches, removeWatch } from '#lib/api/tracking.js';
	import type { MarkWhen, Movie, Watch } from '#lib/api/types.js';
	import Badge from '#lib/components/Badge.svelte';
	import Button from '#lib/components/Button.svelte';
	import InlineConfirm from '#lib/components/InlineConfirm.svelte';
	import MarkButton from '#lib/components/MarkButton.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatWatch } from '#lib/format.js';
	import { latestOnly } from '#lib/latest.js';
	import { sourceLabel } from '#lib/watched.js';
	import { releaseOption } from '#lib/when.js';

	interface Props {
		movie: Movie;
		/** The user's watches of it, newest first (from the page load); null if they failed to load. */
		watches: Watch[] | null;
		/** Why they failed to load. */
		watchesError: string;
	}

	let { movie, watches, watchesError }: Props = $props();

	// The page's list until a mark or removal reloads it; null while it
	// can't be loaded.
	let list = $derived(watches);
	let loadProblem = $derived(watchesError);
	let retrying = $state(false);
	const release = $derived(releaseOption('Release date', movie.release_date, new Date()));

	let note = $state('');
	let problem = $state('');
	let heading = $state<HTMLElement>();

	// Only the newest reload may set the list, so a slow earlier answer
	// can't bring back a removed watch or hide a new one.
	const track = latestOnly<Watch[]>();

	/** Reloads the list; throws if that fails. */
	async function reload() {
		const res = await track(movieWatches(movie.tmdb_id));
		if (!res.current) return;
		list = res.value;
		loadProblem = '';
	}

	async function retry() {
		retrying = true;
		try {
			await reload();
		} catch (e) {
			loadProblem = errorMessage(e);
		} finally {
			retrying = false;
		}
	}

	async function mark(when: MarkWhen) {
		note = '';
		problem = '';
		cancelRemove(false);
		let merged: number | null;
		let at: string;
		try {
			const res = await markMovie(movie.tmdb_id, when);
			merged = res.merged_into;
			at = formatWatch(res.watch);
		} catch (e) {
			problem = errorMessage(e, trackingMessages);
			return;
		}
		try {
			await reload();
		} catch {
			problem = 'Marked, but the list below is out of date. Reload the page to see it.';
		}
		if (merged === null) {
			note = `Added to your history: ${at}.`;
		} else {
			// A repeat within 3 hours counts as the same watch.
			const kept = list?.find((w) => w.id === merged);
			note = kept
				? `Already in your history: you watched it ${formatWatch(kept)}.`
				: 'Already in your history.';
		}
	}

	// Removing one watch, confirmed inline.

	let confirming = $state<Watch | null>(null);
	let removeBusy = $state(false);
	let removeError = $state('');

	function askRemove(watch: Watch) {
		if (removeBusy) return;
		if (confirming?.id === watch.id) return cancelRemove(true);
		confirming = watch;
		removeError = '';
		note = '';
	}

	async function cancelRemove(refocus: boolean) {
		const id = confirming?.id;
		confirming = null;
		removeError = '';
		if (!refocus || id === undefined) return;
		await tick();
		document.getElementById(`remove-watch-${id}`)?.focus();
	}

	async function remove() {
		if (!confirming) return;
		const watch = confirming;
		removeBusy = true;
		removeError = '';
		try {
			await removeWatch(watch.id);
		} catch (e) {
			// Already gone (removed in another tab) is what was asked for.
			if (!(e instanceof ApiError && e.code === 'not_found')) {
				removeBusy = false;
				removeError = errorMessage(e, trackingMessages);
				return;
			}
		}
		try {
			await reload();
		} catch {
			if (list) list = list.filter((w) => w.id !== watch.id);
		}
		confirming = null;
		removeBusy = false;
		note = `Removed the watch from ${formatWatch(watch)}.`;
		heading?.focus();
	}

	const count = $derived(list?.length ?? 0);

	// The newest few, so a much-rewatched film doesn't push the page down;
	// the rest on request.
	const shortList = 5;
	let showAll = $state(false);
	const shown = $derived(!list ? [] : showAll ? list : list.slice(0, shortList));
</script>

<section aria-labelledby="watches-title" class="flex flex-col">
	<MarkButton
		label="Mark as watched"
		menuLabel="Choose when you watched {movie.title}"
		onmark={mark}
		{release}
	/>
	<Notice tone="success" message={note} class="mt-3" />
	<Notice tone="error" message={problem} class="mt-3" />

	<div class="mt-5 flex items-center gap-2 border-b border-line pb-2">
		{#if count > 0}
			<span
				aria-hidden="true"
				class="grid size-5 shrink-0 place-items-center rounded-sm bg-watched text-surface"
			>
				<svg
					viewBox="0 0 16 16"
					class="size-3.5"
					fill="none"
					stroke="currentColor"
					stroke-width="2.4"
					stroke-linecap="round"
					stroke-linejoin="round"><path d="M3 8.5l3.2 3L13 4.5" /></svg
				>
			</span>
		{/if}
		<h2
			id="watches-title"
			bind:this={heading}
			tabindex="-1"
			class="text-base font-semibold tracking-tight text-ink"
		>
			{#if list === null}Your watches{:else if count === 0}Not watched yet{:else if count === 1}Watched
				once{:else}Watched {count} times{/if}
		</h2>
	</div>

	{#if list === null}
		<div class="flex flex-col items-start gap-2 py-2.5">
			<Notice tone="error" message="Couldn't load your watches of this movie: {loadProblem}" />
			<Button variant="quiet" busy={retrying} onclick={retry}>Try again</Button>
		</div>
	{:else if count === 0}
		<p class="py-2.5 text-sm text-muted">Your watches of this movie will be listed here.</p>
	{:else}
		<ul class="divide-y divide-line">
			{#each shown as watch (watch.id)}
				{@const source = sourceLabel(watch.source)}
				<li class="py-1">
					<div class="flex items-center gap-2">
						<time
							datetime={watch.watched_on ?? watch.watched_at}
							class="min-w-0 flex-1 text-sm text-ink tabular-nums"
						>
							{formatWatch(watch)}
						</time>
						{#if source}<Badge title="Recorded by {source}">{source}</Badge>{/if}
						<button
							type="button"
							id="remove-watch-{watch.id}"
							aria-label="Remove the watch from {formatWatch(watch)}"
							aria-expanded={confirming?.id === watch.id}
							title="Remove from history"
							onclick={() => askRemove(watch)}
							class="grid size-10 shrink-0 place-items-center rounded-md text-muted transition-colors hover:bg-raised hover:text-danger sm:size-8"
						>
							<svg
								aria-hidden="true"
								viewBox="0 0 16 16"
								class="size-3.5"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"><path d="M4 4l8 8M12 4l-8 8" /></svg
							>
						</button>
					</div>
					{#if confirming?.id === watch.id}
						<InlineConfirm
							class="mt-1 mb-2"
							confirmLabel="Remove watch"
							busy={removeBusy}
							error={removeError}
							onconfirm={remove}
							oncancel={() => cancelRemove(true)}
						>
							Remove the watch from <strong class="font-semibold">{formatWatch(watch)}</strong> from your
							history?
						</InlineConfirm>
					{/if}
				</li>
			{/each}
		</ul>
		{#if count > shortList}
			<button
				type="button"
				aria-expanded={showAll}
				onclick={() => (showAll = !showAll)}
				class="mt-1 self-start rounded-sm py-1 text-sm text-accent-text underline-offset-2 hover:underline"
			>
				{showAll ? `Show only the newest ${shortList}` : `Show all ${count} watches`}
			</button>
		{/if}
	{/if}
</section>
