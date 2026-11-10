<script lang="ts">
	import { tick, type Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import type { MarkWhen } from '#lib/api/types.js';
	import type { ReleaseOption } from '#lib/when.js';
	import { keepIt, menuItem, trashIcon } from './MenuPopover.svelte';
	import WhenChoices from './WhenChoices.svelte';
	import RemoveAllPlays from './RemoveAllPlays.svelte';

	interface Props {
		/** The trigger's accessible name ("More ways to mark The Matrix watched"). */
		label: string;
		/** A watched check first offers removal; the menu closes before removing. */
		onremove?: () => unknown;
		onremoveall?: () => Promise<string>;
		removeTitle?: string;
		remaining?: boolean;
		watchAgainDisabledReason?: string;
		/** Records the mark; the menu closes first. */
		onmark: (when: MarkWhen) => unknown;
		/** The release-date option: the date, or why it can't be picked. */
		release: ReleaseOption;
		/**
		 * Disables the trigger (the mark can't be used now). Not for a mark
		 * that's running: focus returns to the trigger after a choice.
		 */
		busy?: boolean;
		/** A mark started from here is running: aria-busy, the trigger stays usable. */
		working?: boolean;
		/** The ID of text that describes the trigger (why it's disabled). */
		describedby?: string;
		/**
		 * Which edge of the trigger the menu lines up with; auto picks start
		 * when the menu fits to the right of it, else end (a card in a grid).
		 */
		align?: 'start' | 'end' | 'auto';
		/**
		 * chevron: flips while open; plus ("add a play", the movie's action
		 * column): turns 45° into an × while open.
		 */
		icon?: 'chevron' | 'plus';
		/**
		 * The trigger's content instead of the icon: a card's watched check,
		 * which opens the when-choice itself (DESIGN.md, Poster cards).
		 */
		trigger?: Snippet;
		/** The trigger's tooltip. */
		title?: string;
		/**
		 * Can't be used now (the reason is the trigger's description and
		 * tooltip): aria-disabled, so it stays focusable and says why, and it
		 * doesn't open.
		 */
		off?: boolean;
		/**
		 * The trigger says it opens a dialog (aria-haspopup), and the panel is
		 * one, named for its question: for a trigger whose name alone
		 * ("Mark Season 1 watched…") doesn't say a choice follows.
		 */
		popup?: boolean;
		/** The trigger's id, so the page can move focus to it. */
		triggerId?: string;
		/** The trigger's look; the caller joins it to its own button. */
		class?: ClassValue;
	}

	let {
		label,
		onmark,
		onremove,
		onremoveall,
		removeTitle = label,
		remaining = false,
		watchAgainDisabledReason = '',
		release,
		busy = false,
		working = false,
		describedby,
		align = 'end',
		icon = 'chevron',
		trigger: content,
		title,
		off = false,
		popup = false,
		triggerId,
		class: className
	}: Props = $props();

	// A disclosure (like the user menu), not an ARIA menu: a button that
	// shows a short list of buttons, and a date form for "Other date…"
	// (WhenChoices, which the history card's menu shares).

	const id = $props.id();
	let open = $state(false);
	let adding = $state(false);
	let root = $state<HTMLElement>();
	let trigger = $state<HTMLButtonElement>();
	let panel = $state<HTMLElement>();

	// auto: the panel's left edge, relative to the trigger's, measured as
	// it opens: lined up with the trigger, shifted left just enough to
	// stay 1 rem inside the window.
	let shift = $state<number | null>(null);

	async function toggle() {
		if (off && !open) return;
		open = !open;
		if (!open) return;
		adding = false;
		if (align === 'auto' && root) {
			const left = root.getBoundingClientRect().left;
			const vw = document.documentElement.clientWidth;
			const rem = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
			const width = Math.min(18 * rem, vw - 2 * rem);
			shift = Math.max(rem - left, Math.min(0, vw - rem - width - left));
		}
		await tick();
		panel?.querySelector<HTMLButtonElement>('button')?.focus();
	}

	function close(refocus: boolean) {
		open = false;
		if (refocus) trigger?.focus();
	}

	function choose(when: MarkWhen) {
		close(true);
		void onmark(when);
	}

	async function showWhen() {
		if (working || watchAgainDisabledReason) return;
		adding = true;
		await tick();
		panel?.querySelector<HTMLButtonElement>('button')?.focus();
	}

	function remove() {
		if (working) return;
		close(true);
		void onremove?.();
	}

	// Escape closes; WhenChoices handles its own arrow keys.
	function onkeydown(event: KeyboardEvent) {
		if (
			open &&
			(onremove || onremoveall) &&
			!remaining &&
			!adding &&
			(event.key === 'ArrowDown' || event.key === 'ArrowUp')
		) {
			const items = [...(panel?.querySelectorAll<HTMLButtonElement>('[data-item]') ?? [])];
			const at = items.indexOf(document.activeElement as HTMLButtonElement);
			if (at >= 0) {
				event.preventDefault();
				items[(at + (event.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length]?.focus();
			}
		}
		if (open && event.key === 'Escape') {
			event.stopPropagation();
			close(true);
		}
	}

	function onpointerdown(event: PointerEvent) {
		if (open && root && !root.contains(event.target as Node)) close(false);
	}

	function onfocusout(event: FocusEvent) {
		const next = event.relatedTarget as Node | null;
		if (next && root && !root.contains(next)) close(false);
	}
</script>

<svelte:document {onpointerdown} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div bind:this={root} class="relative flex" {onkeydown} {onfocusout}>
	<button
		bind:this={trigger}
		id={triggerId}
		type="button"
		aria-label={label}
		aria-describedby={describedby}
		aria-haspopup={popup ? 'dialog' : undefined}
		aria-expanded={open}
		aria-controls="{id}-when"
		aria-busy={working || undefined}
		aria-disabled={off ? 'true' : undefined}
		{title}
		disabled={busy}
		onclick={toggle}
		class={['grid place-items-center transition-colors disabled:cursor-not-allowed', className]}
	>
		{#if content}
			{@render content()}
		{:else if icon === 'plus'}
			<!-- Turns into an × while open (instantly with reduced motion: layout.css). -->
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class={['size-4 transition-transform duration-200', open && 'rotate-45']}
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"><path d="M8 2.75v10.5M2.75 8h10.5" /></svg
			>
		{:else}
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class={['size-3.5 transition-transform', open && 'rotate-180']}
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"><path d="m4 6 4 4 4-4" /></svg
			>
		{/if}
	</button>

	<div
		bind:this={panel}
		id="{id}-when"
		role={popup ? 'dialog' : undefined}
		aria-label={popup
			? (onremove || onremoveall) && !remaining && !adding
				? 'Remove from history?'
				: 'When did you watch it?'
			: undefined}
		hidden={!open}
		style:left={align === 'auto' && shift !== null ? `${shift}px` : undefined}
		class={[
			'absolute top-full z-30 mt-1.5 w-72 max-w-[calc(100vw-2rem)] rounded-lg border border-line bg-surface py-1 shadow-[0_12px_32px_rgb(0_0_0/0.25)]',
			align === 'end' ? 'right-0' : 'left-0'
		]}
	>
		{#if (onremove || onremoveall) && !remaining && !adding}
			<p class="border-b border-line px-3 py-2 text-sm font-semibold text-ink">
				Remove from history?
			</p>
			{#if onremove}
				<button
					type="button"
					data-item
					aria-disabled={working || undefined}
					onclick={remove}
					class={[menuItem, 'text-danger hover:bg-danger/10']}
				>
					{@render trashIcon()}Only this play
				</button>
			{/if}
			{#if onremoveall}
				<RemoveAllPlays
					title={removeTitle}
					onremove={onremoveall}
					disabled={working}
					{open}
					ondone={() => close(true)}
				/>
			{/if}
			{@render keepIt(() => close(true))}
			<hr class="border-line" />
			<button
				type="button"
				data-item
				aria-disabled={working || !!watchAgainDisabledReason || undefined}
				title={watchAgainDisabledReason || undefined}
				onclick={showWhen}
				class={[
					menuItem,
					watchAgainDisabledReason ? 'cursor-not-allowed text-muted' : 'text-ink hover:bg-raised'
				]}>{onremove ? 'Add another play' : 'Watch again'}</button
			>
		{:else}
			<!-- While a mark from here runs, the options are off: a second choice would be dropped. -->
			<WhenChoices
				onchoose={choose}
				{release}
				busy={working}
				nowLabel={remaining ? 'Watch remaining' : 'Just now'}
			/>
			{#if remaining && onremoveall}
				<hr class="border-line" />
				<RemoveAllPlays
					title={removeTitle}
					onremove={onremoveall}
					disabled={working}
					{open}
					ondone={() => close(true)}
				/>
			{/if}
		{/if}
	</div>
</div>
