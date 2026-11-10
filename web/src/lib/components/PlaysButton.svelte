<script lang="ts">
	import { tick } from 'svelte';
	import { resolve } from '$app/paths';
	import type { MarkWhen, Watch } from '#lib/api/types.js';
	import { formatWatch } from '#lib/format.js';
	import { playsLabel, sourceLabel } from '#lib/watched.js';
	import type { ReleaseOption } from '#lib/when.js';
	import Badge from './Badge.svelte';
	import InlineConfirm from './InlineConfirm.svelte';
	import MenuPopover, { menuIcon, menuItem, trashIcon } from './MenuPopover.svelte';
	import RemoveAllPlays from './RemoveAllPlays.svelte';
	import WhenChoices from './WhenChoices.svelte';
	import WhenMenu from './WhenMenu.svelte';

	interface Props {
		/** An id for the main button, so the page can move focus to it. */
		id?: string;
		/** The title's name, for labels ("Choose when you watched The Matrix"). */
		title: string;
		/** The user's plays of it, newest first; at least one. */
		plays: Watch[];
		/** The release-date option for the + menu. */
		release: ReleaseOption;
		/** Adds a play; the + is busy until it settles. */
		onadd: (when: MarkWhen) => Promise<unknown>;
		/**
		 * Removes a play. Resolves with "" when done (the caller reloads the
		 * list), or the message to show in the confirm.
		 */
		onremove: (watch: Watch) => Promise<string>;
		onremoveall: () => Promise<string>;
	}

	let { id, title, plays, release, onadd, onremove, onremoveall }: Props = $props();

	// The watched state of the action column, after classic Trakt's play
	// button: a sage split button. The main part shows the play count and
	// the latest play and opens the plays menu; the + adds another play
	// (the when-choice). The menu is the shared MenuPopover shell, like the
	// history card's: every play (each removable after a confirm) in a list
	// that scrolls past about five rows, then "Remove all plays" (confirmed inline), "View all plays", and "Add another play" under it. No header: the
	// button already says how many plays there are, and the dialog is
	// named "Plays of …". Esc, a click outside, focus leaving, or the
	// button itself close it.

	const uid = $props.id();
	let trigger = $state<HTMLButtonElement>();
	let menu = $state<MenuPopover>();
	let menuOpen = $state(false);
	let adding = $state(false);
	let choosingWhen = $state(false);
	let whenBox = $state<HTMLElement>();

	const latest = $derived(plays[0]);
	const count = $derived(plays.length);

	// The + stays enabled while a play is added (a disabled button would
	// drop the focus WhenMenu returns to it); a second choice is ignored.
	async function add(when: MarkWhen) {
		if (adding || removeBusy || removeAllBusy) return;
		adding = true;
		try {
			await onadd(when);
		} finally {
			adding = false;
		}
	}

	// Removing one play, confirmed inline in the menu.

	let confirming = $state<number | null>(null);
	let removeBusy = $state(false);
	let removeAllBusy = $state(false);
	let removeError = $state('');

	function askRemove(watch: Watch) {
		if (adding || removeBusy || removeAllBusy) return;
		confirming = confirming === watch.id ? null : watch.id;
		removeError = '';
	}

	async function cancelRemove() {
		const id = confirming;
		confirming = null;
		removeError = '';
		await tick();
		if (id !== null) document.getElementById(`${uid}-remove-${id}`)?.focus();
	}

	async function remove(watch: Watch) {
		if (adding || removeBusy || removeAllBusy) return;
		const at = plays.findIndex((w) => w.id === watch.id);
		removeBusy = true;
		removeError = '';
		const problem = await onremove(watch);
		removeBusy = false;
		if (problem) {
			removeError = problem;
			return;
		}
		confirming = null;
		// The last play gone: the caller swaps this for "Mark as watched"
		// and moves focus there. Otherwise the next play's remove button,
		// or the menu itself.
		await tick();
		if (plays.length === 0) return;
		const next = plays[Math.min(at, plays.length - 1)];
		const target = next && document.getElementById(`${uid}-remove-${next.id}`);
		(target ?? menu?.element())?.focus();
	}

	async function showWhen() {
		if (adding || removeBusy || removeAllBusy) return;
		confirming = null;
		removeError = '';
		choosingWhen = true;
		await tick();
		whenBox?.querySelector<HTMLButtonElement>('button')?.focus();
	}

	function chooseWhen(when: MarkWhen) {
		if (adding || removeBusy || removeAllBusy) return;
		menu?.close();
		void add(when);
	}

	function onMenuChange(open: boolean) {
		if (!open) {
			choosingWhen = false;
			confirming = null;
			removeError = '';
		}
	}

	// Escape inside a confirm cancels it (InlineConfirm), not the whole
	// menu: preventing it stops the popover's light dismiss. Elsewhere in
	// the menu, Escape closes it as usual.
	function onConfirmKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') event.preventDefault();
	}
</script>

<div class="flex">
	<button
		bind:this={trigger}
		{id}
		type="button"
		popovertarget="{uid}-plays"
		aria-haspopup="dialog"
		aria-expanded={menuOpen}
		aria-busy={adding || undefined}
		aria-label="{playsLabel(count)}, latest {formatWatch(latest)}. Your plays of {title}"
		class="flex h-12 min-w-0 flex-1 items-center gap-3 rounded-l-md bg-watched px-4 text-left text-surface transition-colors hover:bg-watched-hover aria-expanded:bg-watched-hover"
	>
		{#if adding}
			<span
				aria-hidden="true"
				class="size-5 shrink-0 animate-spin rounded-full border-2 border-current border-r-transparent"
			></span>
		{:else}
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class="size-5 shrink-0"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"><path d="M3 8.5l3.2 3L13 4.5" /></svg
			>
		{/if}
		<span class="flex min-w-0 flex-col leading-tight">
			<span class="truncate text-sm font-semibold">{playsLabel(count)}</span>
			<span class="truncate text-xs tabular-nums">{formatWatch(latest)}</span>
		</span>
	</button>
	<WhenMenu
		label="Add another play of {title}"
		onmark={add}
		{release}
		icon="plus"
		working={adding || removeBusy || removeAllBusy}
		class="h-12 w-12 rounded-r-md border-l border-surface/30 bg-watched text-surface hover:bg-watched-hover"
	/>
</div>

<MenuPopover
	bind:this={menu}
	bind:open={menuOpen}
	id="{uid}-plays"
	label="Plays of {title}"
	anchor={trigger}
	onchange={onMenuChange}
	class="w-72"
>
	<!--
		Every play, newest first. Past about five rows the list scrolls
		inside the menu (never the page behind it), so the menu stays inside
		the window; focus moving to a row (arrow keys, a confirm) scrolls it
		into view. The trash buttons keep the list's 12 px right padding
		clear, so a scrollbar (an overlay one too) never covers them. The rows exist while the menu is closed too, hidden with
		the popover.
	-->
	<ul
		aria-label="Plays"
		class="max-h-[min(17rem,calc(100dvh-13rem))] [scrollbar-width:thin] divide-y divide-line overflow-y-auto overscroll-contain px-3 py-1 sm:max-h-[min(14rem,calc(100dvh-13rem))]"
	>
		{#each plays as watch (watch.id)}
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
						data-item
						id="{uid}-remove-{watch.id}"
						aria-label="Remove the play from {formatWatch(watch)}"
						aria-expanded={confirming === watch.id}
						aria-disabled={adding || removeBusy || removeAllBusy || undefined}
						title="Remove from history"
						onclick={() => askRemove(watch)}
						class="flex h-10 shrink-0 items-center gap-1 rounded-md px-1.5 text-xs text-muted transition-colors hover:bg-raised hover:text-danger focus-visible:-outline-offset-2 sm:h-8"
					>
						{@render trashIcon()}
						Remove
					</button>
				</div>
				{#if confirming === watch.id}
					<div onkeydowncapture={onConfirmKeydown}>
						<InlineConfirm
							class="mt-1 mb-2"
							confirmLabel="Remove play"
							busy={removeBusy}
							error={removeError}
							onconfirm={() => remove(watch)}
							oncancel={cancelRemove}
						>
							Remove the play from <strong class="font-semibold">{formatWatch(watch)}</strong> from your
							history?
						</InlineConfirm>
					</div>
				{/if}
			</li>
		{/each}
	</ul>

	<!-- Fixed under the list, not inside it. -->
	<div class="border-t border-line py-1">
		<RemoveAllPlays
			{title}
			onremove={onremoveall}
			disabled={adding || removeBusy}
			bind:busy={removeAllBusy}
			open={menuOpen}
			ondone={() => menu?.close()}
		/>
		<!-- History has no filters yet, so this is the whole history, and it says so. -->
		<a href={resolve('/(app)/history')} data-item class={[menuItem, 'hover:bg-raised']}>
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class={menuIcon}
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"
				stroke-linejoin="round"
				><circle cx="8" cy="8" r="6.25" /><path d="M8 4.75V8l2.25 1.5" /></svg
			>
			<span>
				View all plays
				<span class="block text-xs text-muted">Your whole history; filters come later</span>
			</span>
		</a>
	</div>
	<hr class="border-line" />
	<div class="py-1">
		<button
			type="button"
			data-item
			aria-expanded={choosingWhen}
			aria-controls="{uid}-when"
			aria-disabled={adding || removeBusy || removeAllBusy || undefined}
			onclick={showWhen}
			class={[menuItem, 'hover:bg-raised']}
		>
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class={menuIcon}
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"
				><circle cx="8" cy="8" r="6.25" /><path d="M8 5.25v5.5M5.25 8h5.5" /></svg
			>
			Add another play
		</button>
		<div bind:this={whenBox} id="{uid}-when" hidden={!choosingWhen} class="border-t border-line">
			{#if choosingWhen}
				<WhenChoices onchoose={chooseWhen} {release} busy={adding || removeBusy || removeAllBusy} />
			{/if}
		</div>
	</div>
</MenuPopover>
