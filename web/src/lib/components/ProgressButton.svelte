<script lang="ts">
	import { resolve } from '$app/paths';
	import type { MarkWhen } from '#lib/api/types.js';
	import { progressPercent, progressSummary } from '#lib/seasons.js';
	import type { Progress } from '#lib/watched.js';
	import type { ReleaseOption } from '#lib/when.js';
	import MenuPopover, { menuIcon, menuItem } from './MenuPopover.svelte';
	import RemoveAllPlays from './RemoveAllPlays.svelte';
	import WhenMenu from './WhenMenu.svelte';

	interface Props {
		/** An id for the main button, so the page can move focus to it. */
		id?: string;
		/** What it's the progress of, for labels ("Breaking Bad", "Breaking Bad Season 1"). */
		title: string;
		/** Episodes watched of the total, and their plays; something is watched. */
		progress: Progress;
		/** The + menu's air-date option. */
		release: ReleaseOption;
		/** Marks the rest watched (every aired episode not watched yet); the + is busy until it settles. */
		onmark: (when: MarkWhen) => Promise<unknown>;
		onremoveall: () => Promise<string>;
		/** Turns the + off; the reason is shown under the bar. */
		disabledReason?: string;
	}

	let { id, title, progress, release, onmark, onremoveall, disabledReason = '' }: Props = $props();

	// The watched state of a show's or season's action column, after
	// classic Trakt: a sage split button, the same shape as PlaysButton.
	// The main part says how much is watched and opens a short menu (watch
	// the remaining episodes, view history; rewatch and drop as placeholders, and remove all
	// plays after confirmation); the + marks the rest with the when-choice. A
	// thin sage bar under it shows the same progress.

	const uid = $props.id();
	let trigger = $state<HTMLButtonElement>();
	let menu = $state<MenuPopover>();
	let menuOpen = $state(false);
	let marking = $state(false);
	let removing = $state(false);

	const percent = $derived(progressPercent(progress));
	const summary = $derived(progressSummary(progress));
	const reasonId = $derived(disabledReason ? `${uid}-reason` : undefined);

	// As on PlaysButton: the + stays enabled while marking (focus returns
	// to it), and a second choice is ignored.
	async function mark(when: MarkWhen) {
		if (marking || removing) return;
		marking = true;
		try {
			await onmark(when);
		} finally {
			marking = false;
		}
	}

	// "Watch remaining": the +'s "Just now", from the menu. The menu closes
	// first, so focus is back on the button while the mark runs.
	// Off once everything counted is watched: nothing would be added.
	const remainingOff = $derived(
		disabledReason ||
			(progress.total > 0 && progress.watched >= progress.total ? 'Every episode is watched.' : '')
	);

	function watchRemaining() {
		if (remainingOff || marking || removing) return;
		menu?.close();
		void mark({ at: 'now' });
	}
</script>

<div class="flex flex-col">
	<div class="flex">
		<button
			bind:this={trigger}
			{id}
			type="button"
			popovertarget="{uid}-menu"
			aria-haspopup="dialog"
			aria-expanded={menuOpen}
			aria-busy={marking || undefined}
			aria-label="{percent}% watched: {summary.replace(' · ', ', ')}. Your history of {title}"
			class="flex min-h-12 min-w-0 flex-1 items-center gap-3 rounded-l-md bg-watched py-1.5 pr-3 pl-4 text-left text-surface transition-colors hover:bg-watched-hover aria-expanded:bg-watched-hover"
		>
			{#if marking}
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
			<span class="flex min-w-0 flex-col gap-0.5">
				<span class="text-sm leading-none font-semibold"
					><span class="text-xl leading-none font-bold tabular-nums">{percent}%</span> watched</span
				>
				<span class="text-xs leading-snug tabular-nums">{summary}</span>
			</span>
		</button>
		<WhenMenu
			label="Mark the rest of {title} watched, choosing when"
			onmark={mark}
			{release}
			icon="plus"
			busy={!!disabledReason}
			working={marking || removing}
			describedby={reasonId}
			class="min-h-12 w-12 rounded-r-md border-l border-surface/30 bg-watched text-surface hover:bg-watched-hover disabled:opacity-70 disabled:hover:bg-watched"
		/>
	</div>
	<!-- The same progress as the button says, so it's decoration for screen readers. -->
	<div aria-hidden="true" class="mt-2 h-1 overflow-hidden rounded-full bg-line">
		<div class="h-full rounded-full bg-watched" style:width="{percent}%"></div>
	</div>
	{#if disabledReason}
		<p id={reasonId} class="mt-1.5 text-xs text-muted">{disabledReason}</p>
	{/if}
</div>

<MenuPopover
	bind:this={menu}
	bind:open={menuOpen}
	id="{uid}-menu"
	label="Your history of {title}"
	anchor={trigger}
	class="w-72"
>
	<div class="py-1">
		<!-- The +'s "Just now", one step closer; off for the same reason as the +. -->
		<button
			type="button"
			data-item
			aria-disabled={remainingOff ? 'true' : undefined}
			aria-describedby="{uid}-remaining"
			onclick={watchRemaining}
			class={[menuItem, remainingOff ? 'cursor-not-allowed text-muted' : 'hover:bg-raised']}
		>
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class={menuIcon}
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"><path d="M3 8.5l3.2 3L13 4.5" /></svg
			>
			<span>
				Watch remaining
				<span id="{uid}-remaining" class="block text-xs text-muted"
					>{remainingOff || 'Marks the rest watched just now'}</span
				>
			</span>
		</button>
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
				View history
				<span class="block text-xs text-muted">Your whole history; filters come later</span>
			</span>
		</a>
	</div>
	<!-- Planned (see the plan's Backlog), so shown as placeholders. -->
	<div class="border-t border-line py-1">
		{@render later(
			'Rewatch',
			'M2.75 8a5.25 5.25 0 1 0 1.6-3.78M2.75 2.75v2.5h2.5',
			`${uid}-rewatch`
		)}
		{@render later(
			'Drop',
			'M8 1.75a6.25 6.25 0 1 0 0 12.5 6.25 6.25 0 0 0 0-12.5zM5.25 8h5.5',
			`${uid}-drop`
		)}
		<RemoveAllPlays
			{title}
			onremove={onremoveall}
			disabled={marking}
			bind:busy={removing}
			open={menuOpen}
			ondone={() => menu?.close()}
		/>
	</div>
</MenuPopover>

<!-- A menu row whose feature comes later (DESIGN.md, Placeholders). -->
{#snippet later(label: string, d: string, describedId: string)}
	<button
		type="button"
		data-item
		aria-disabled="true"
		aria-describedby={describedId}
		title="Coming in a later update"
		class={[menuItem, 'cursor-not-allowed text-muted']}
	>
		<svg
			aria-hidden="true"
			viewBox="0 0 16 16"
			class={menuIcon}
			fill="none"
			stroke="currentColor"
			stroke-width="1.6"
			stroke-linecap="round"
			stroke-linejoin="round"><path {d} /></svg
		>
		<span>
			{label}
			<span id={describedId} class="block text-xs">Coming in a later update</span>
		</span>
	</button>
{/snippet}
