<script lang="ts">
	import { tick } from 'svelte';
	import { ApiError } from '#lib/api/client.js';
	import { removeWatch } from '#lib/api/tracking.js';
	import type { MarkResult, MarkWhen, Watch } from '#lib/api/types.js';
	import { errorMessage, trackingMessages } from '#lib/errors.js';
	import { formatWatch } from '#lib/format.js';
	import { latestOnly } from '#lib/latest.js';
	import { toast } from '#lib/toast.svelte.js';
	import type { ReleaseOption } from '#lib/when.js';
	import Button from './Button.svelte';
	import MarkButton from './MarkButton.svelte';
	import Notice from './Notice.svelte';
	import PlaceholderActions from './PlaceholderActions.svelte';
	import PlaysButton from './PlaysButton.svelte';

	interface Props {
		/** The title's name, for labels ("The Matrix", "Breaking Bad S01E22"). */
		title: string;
		/** What it is, for the load error ("movie", "episode"). */
		kind: string;
		/** Ids for the two buttons, so focus can move between them: `mark-{idBase}` and `{idBase}-plays`. */
		idBase: string;
		/** The when-choice's release or air date option. */
		release: ReleaseOption;
		/** The user's plays of it, newest first (from the page load); null if they failed to load. */
		watches: Watch[] | null;
		/** Why they failed to load. */
		watchesError: string;
		/** Loads the plays again. */
		load: () => Promise<Watch[]>;
		/** Records a play. */
		mark: (when: MarkWhen) => Promise<MarkResult>;
		removeAll: () => Promise<{ removed: number }>;
	}

	let {
		title,
		kind,
		idBase,
		release,
		watches,
		watchesError,
		load,
		mark: record,
		removeAll
	}: Props = $props();

	// The page's list until a mark or removal reloads it; null while it
	// can't be loaded.
	let list = $derived(watches);
	let loadProblem = $derived(watchesError);
	let retrying = $state(false);

	// Only the newest reload may set the list, so a slow earlier answer
	// can't bring back a removed watch or hide a new one.
	const track = latestOnly<Watch[]>();

	/** Reloads the list; throws if that fails. */
	async function reload() {
		const res = await track(load());
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

	// Results are toasts (DESIGN.md, Toasts); a failed mark too, since no
	// menu is open by then. A failed removal stays in the plays menu.
	async function mark(when: MarkWhen) {
		const first = !list?.length;
		let merged: number | null;
		let at: string;
		try {
			const res = await record(when);
			merged = res.merged_into;
			at = formatWatch(res.watch);
		} catch (e) {
			toast.error(errorMessage(e, trackingMessages));
			return;
		}
		try {
			await reload();
		} catch {
			toast.error(
				'Marked, but your plays shown here are out of date. Reload the page to see them.'
			);
		}
		if (merged === null) {
			toast.success(`Added to your history: ${at}.`);
		} else {
			// A repeat within 3 hours counts as the same watch: not an error.
			const kept = list?.find((w) => w.id === merged);
			toast.info(
				kept
					? `Already in your history: you watched it ${formatWatch(kept)}.`
					: 'Already in your history.'
			);
		}
		// The first play: "Mark as watched" became the plays button.
		if (first && list?.length) {
			await tick();
			document.getElementById(playsId)?.focus();
		}
	}

	// Removing a play (from the plays menu, after its confirm). Resolves
	// with "" when done, or the message the confirm shows.
	async function remove(watch: Watch): Promise<string> {
		try {
			await removeWatch(watch.id);
		} catch (e) {
			// Already gone (removed in another tab) is what was asked for.
			if (!(e instanceof ApiError && e.code === 'not_found')) {
				return errorMessage(e, trackingMessages);
			}
		}
		try {
			await reload();
		} catch {
			if (list) list = list.filter((w) => w.id !== watch.id);
		}
		toast.success(`Removed the play from ${formatWatch(watch)}.`);
		// The last one gone: the button is "Mark as watched" again.
		if (list?.length === 0) {
			await tick();
			document.getElementById(markId)?.focus();
		}
		return '';
	}

	async function removeEveryPlay(): Promise<string> {
		let removed: number;
		try {
			({ removed } = await removeAll());
		} catch (e) {
			return errorMessage(e, trackingMessages);
		}
		try {
			await reload();
		} catch {
			list = [];
			toast.error('Removed, but your plays could not be reloaded. Reload the page to check them.');
		}
		toast.success(`Removed ${removed} ${removed === 1 ? 'play' : 'plays'} of ${title}.`);
		if (list?.length === 0) {
			await tick();
			document.getElementById(markId)?.focus();
		}
		return '';
	}

	// Where focus goes when one button replaces the other (the first play
	// added, the last one removed), so it isn't lost with the old button.
	const markId = $derived(`mark-${idBase}`);
	const playsId = $derived(`${idBase}-plays`);
</script>

<!--
	The action column of a movie or an episode: one split button that
	states and changes its watched state, then the actions still to come.
	- Not watched (or the plays can't be loaded): "Mark as watched" marks
	  it just now; the + chooses when.
	- Watched: the play count and latest play (PlaysButton); it opens the
	  plays menu (remove one, view all), and the + adds another play.
-->
<section aria-label="Track {title}" class="flex flex-col">
	{#if list && list.length > 0}
		<PlaysButton
			id={playsId}
			{title}
			plays={list}
			{release}
			onadd={mark}
			onremove={remove}
			onremoveall={removeEveryPlay}
		/>
	{:else}
		<MarkButton
			id={markId}
			label="Mark as watched"
			menuLabel="Choose when you watched {title}"
			menuIcon="plus"
			onmark={mark}
			{release}
		/>
	{/if}

	{#if list === null}
		<div class="mt-3 flex flex-col items-start gap-2">
			<Notice tone="error" message="Couldn't load your plays of this {kind}: {loadProblem}" />
			<Button variant="quiet" busy={retrying} onclick={retry}>Try again</Button>
		</div>
	{/if}

	<PlaceholderActions class="mt-4" />
</section>
