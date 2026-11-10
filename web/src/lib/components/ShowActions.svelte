<script lang="ts">
	import { tick } from 'svelte';
	import { markShow, removeShowWatches } from '#lib/api/tracking.js';
	import type { EpisodeWatched, MarkWhen, Show } from '#lib/api/types.js';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { showProgress } from '#lib/seasons.js';
	import { toast } from '#lib/toast.svelte.js';
	import { addedMessage, hasAired } from '#lib/watched.js';
	import Button from './Button.svelte';
	import MarkButton from './MarkButton.svelte';
	import Notice from './Notice.svelte';
	import PlaceholderActions from './PlaceholderActions.svelte';
	import ProgressButton from './ProgressButton.svelte';

	interface Props {
		show: Show;
		/** The user's watched episodes; null while they can't be loaded. */
		watched: EpisodeWatched[] | null;
		/** Why they couldn't be loaded, shown with "Try again". */
		watchedError: string;
		/** Reloads the watched episodes (the page's list); throws if that fails. */
		refresh: () => Promise<void>;
	}

	let { show, watched, watchedError, refresh }: Props = $props();

	// The show's action column (the show page and its All episodes page):
	// - Nothing watched (or the watched list can't be loaded): "Mark show
	//   watched" marks every aired episode just now; the + chooses when.
	// - Something watched: the progress button; its + marks the rest, and
	//   its menu watches the rest, or views the history.
	// Then the actions still to come. Results are toasts (DESIGN.md).

	const now = new Date();
	// The server refuses a show mark until the episodes of every regular
	// season it covers are stored (one season per number, lowest TMDB ID;
	// specials aren't part of it). This checks every listed regular season,
	// which can only be stricter.
	const pending = $derived(show.seasons.some((s) => s.season_number > 0 && s.episodes_pending));
	const pendingReason = $derived(
		pending ? 'Some episodes are still being fetched from TMDB. Reload the page in a moment.' : ''
	);
	// Whether anything has aired, as far as the show's dates say, for the
	// wording of a mark that added nothing.
	const anyAired = $derived(
		hasAired(show.first_air_date, now) ||
			show.seasons.some((s) => s.season_number > 0 && hasAired(s.air_date, now))
	);

	// Regular seasons only; null while the watched list can't be loaded.
	const progress = $derived(watched ? showProgress(show.seasons, watched) : null);
	const started = $derived((progress?.watched ?? 0) > 0);
	const release = { label: 'Air dates', detail: "Each episode's own", enabled: true };

	async function markWholeShow(when: MarkWhen) {
		const first = !started;
		let added: number;
		try {
			added = await markShow(show.tmdb_id, when);
		} catch (e) {
			toast.error(errorMessage(e, trackingMessages));
			return;
		}
		toast.show(added > 0 ? 'success' : 'info', addedMessage(added, anyAired));
		try {
			await refresh();
		} catch {
			toast.error("Your progress here is out of date. Reload the page to see what's watched.");
			return;
		}
		// The first mark: "Mark show watched" became the progress button.
		if (first && started) {
			await tick();
			document.getElementById('show-progress')?.focus();
		}
	}

	async function removeWholeShow(): Promise<string> {
		const name = show.name;
		let removed: number;
		try {
			({ removed } = await removeShowWatches(show.tmdb_id));
		} catch (e) {
			return errorMessage(e, trackingMessages);
		}
		toast.success(`Removed ${removed} ${removed === 1 ? 'play' : 'plays'} of ${name}.`);
		try {
			await refresh();
		} catch {
			toast.error("Your progress here is out of date. Reload the page to see what's watched.");
			return '';
		}
		await tick();
		document.getElementById('mark-show')?.focus();
		return '';
	}

	let retrying = $state(false);
	let retryError = $state('');
	async function retry() {
		retrying = true;
		try {
			await refresh();
			retryError = '';
		} catch (e) {
			retryError = errorMessage(e);
		} finally {
			retrying = false;
		}
	}
</script>

<section aria-label="Track {show.name}" class="flex flex-col">
	{#if progress && started}
		<ProgressButton
			id="show-progress"
			title={show.name}
			{progress}
			{release}
			onmark={markWholeShow}
			onremoveall={removeWholeShow}
			disabledReason={pendingReason}
		/>
	{:else}
		<MarkButton
			id="mark-show"
			label="Mark show watched"
			menuLabel="Choose when you watched {show.name}"
			menuIcon="plus"
			onmark={markWholeShow}
			{release}
			disabledReason={pendingReason}
		/>
		<p class="mt-2 text-xs text-muted">
			Adds every aired episode you haven't watched yet, except specials.
		</p>
	{/if}
	{#if watched === null}
		<div class="mt-3 flex flex-col items-start gap-2">
			<Notice
				tone="error"
				message="Couldn't load which episodes you've watched: {retryError || watchedError}"
			/>
			<Button variant="quiet" busy={retrying} onclick={retry}>Try again</Button>
		</div>
	{/if}
	<PlaceholderActions class="mt-4" />
</section>
