<script lang="ts">
	import { tick } from 'svelte';
	import type { MarkWhen } from '#lib/api/types.js';
	import { placeNear } from '#lib/anchor.js';
	import { formatWatchDate } from '#lib/format.js';
	import { formatRuntime } from '#lib/media.js';
	import type { ReleaseOption } from '#lib/when.js';
	import Notice from './Notice.svelte';
	import WhenChoices from './WhenChoices.svelte';

	interface Props {
		/** What was watched, for the labels ("Breaking Bad S01E02"). */
		name: string;
		/** The watch's time (RFC 3339), shown in the tooltip. */
		watchedAt: string;
		/** Minutes; 0 when unknown (the tooltip leaves it out). */
		runtime: number;
		/** The release-date option for "Add another play". */
		release: ReleaseOption;
		/** The check button's id, so the page can move focus to it. */
		buttonId?: string;
		/** Which time zone times are shown in, as the tooltip's last line. */
		zoneNote?: string;
		/**
		 * Removes this watch. Resolves with "" when done (the caller updates
		 * the list and moves focus), or the message to show in the menu.
		 */
		onremove: () => Promise<string>;
		/** Marks it watched again; resolves like onremove. The menu closes on success. */
		onadd: (when: MarkWhen) => Promise<string>;
	}

	let { name, watchedAt, runtime, release, buttonId, zoneNote, onremove, onadd }: Props = $props();

	// A watched check on a poster card's bar: a sage square with a check.
	// Hover or keyboard focus shows a tooltip with when it was watched; a
	// click opens a small dialog: remove this play, keep it, or add another.
	// Both popups use the Popover API, so they sit in the top layer: the
	// card, the next card, and the sticky day heading can't cover or clip
	// them. Only one menu ("auto" popover) is open at a time; Esc and a
	// click outside close it (the browser's light dismiss).

	const uid = $props.id();
	let trigger = $state<HTMLButtonElement>();
	let tip = $state<HTMLElement>();
	let menu = $state<HTMLElement>();
	let whenBox = $state<HTMLElement>();

	let menuOpen = $state(false);
	let tipOpen = $state(false);
	let adding = $state(false);
	let busy = $state<'remove' | 'add' | null>(null);
	let error = $state('');

	const date = $derived(formatWatchDate(watchedAt));
	const length = $derived(formatRuntime(runtime));

	function viewport() {
		return { width: document.documentElement.clientWidth, height: window.innerHeight };
	}

	// The side the menu opened on; it keeps it while open (shifting to fit
	// as it grows) instead of jumping across the check.
	let menuSide: 'below' | 'above' = 'below';

	function place(el: HTMLElement | undefined, prefer: 'below' | 'above', flip = true) {
		if (!el || !trigger || !el.matches(':popover-open')) return 'below';
		const at = placeNear(
			trigger.getBoundingClientRect(),
			{ width: el.offsetWidth, height: el.offsetHeight },
			viewport(),
			{ prefer, flip }
		);
		el.style.top = `${at.top}px`;
		el.style.left = `${at.left}px`;
		return at.side;
	}

	// Set as it opens: the first placement (often the ResizeObserver's,
	// before the toggle event) picks the side.
	let fresh = false;

	function placeMenu() {
		if (fresh) {
			fresh = false;
			menuSide = place(menu, 'below');
		} else place(menu, menuSide, false);
	}

	// Runs as the menu opens or closes (toggle comes later). Closing with
	// focus inside, the browser puts focus back on the check right away:
	// that focus is quiet, so the tooltip doesn't pop up again.
	function onbeforetoggle(event: ToggleEvent) {
		menuOpen = event.newState === 'open';
		if (menuOpen) fresh = true;
		else if (menu?.contains(document.activeElement)) quietFocus = true;
	}

	// The tooltip: on hover (not touch) and keyboard focus. It stays while
	// the pointer moves onto it, and Escape hides it.

	let hideTimer: ReturnType<typeof setTimeout> | undefined;
	// Focus coming back from the menu doesn't bring the tooltip up again.
	let quietFocus = false;

	function showTip() {
		clearTimeout(hideTimer);
		if (menuOpen || !tip) return;
		try {
			if (!tip.matches(':popover-open')) tip.showPopover();
		} catch {
			// Mid-way through the menu opening or closing: no tooltip now.
			return;
		}
		tipOpen = true;
		place(tip, 'above');
	}

	function hideTip() {
		clearTimeout(hideTimer);
		try {
			if (tip?.matches(':popover-open')) tip.hidePopover();
		} catch {
			// As in showTip; the tooltip closes with the next hide.
		}
		tipOpen = false;
	}

	function hideTipSoon() {
		clearTimeout(hideTimer);
		hideTimer = setTimeout(hideTip, 150);
	}

	function onfocus() {
		if (quietFocus) quietFocus = false;
		else if (trigger?.matches(':focus-visible')) showTip();
	}

	function onTriggerKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && tipOpen) {
			event.preventDefault();
			hideTip();
		}
	}

	// The menu.

	function ontoggle(event: ToggleEvent) {
		menuOpen = event.newState === 'open';
		if (menuOpen) {
			hideTip();
			placeMenu();
			// The dialog itself, not "Only this play": a second Enter mustn't remove.
			menu?.focus({ preventScroll: true });
			return;
		}
		adding = false;
		error = '';
		// Back to the check when focus was in the menu (Esc, Keep it, a click
		// on empty page), not when it moved somewhere on purpose.
		const active = document.activeElement;
		if (trigger?.isConnected && (!active || active === document.body || menu?.contains(active))) {
			quietFocus = true;
			trigger.focus();
		}
		quietFocus = false;
	}

	function close() {
		if (menu?.isConnected && menu.matches(':popover-open')) menu.hidePopover();
	}

	// Up and down move between the items; Tab works as usual. Focus leaving
	// for somewhere else closes it.
	function onMenuKeydown(event: KeyboardEvent) {
		if ((event.key !== 'ArrowDown' && event.key !== 'ArrowUp') || !menu) return;
		const items = [...menu.querySelectorAll<HTMLElement>('[data-item]')];
		const at = items.indexOf(document.activeElement as HTMLElement);
		if (at === -1 && document.activeElement !== menu) return;
		event.preventDefault();
		const step = event.key === 'ArrowDown' ? 1 : items.length - 1;
		const next =
			at === -1 ? (event.key === 'ArrowDown' ? 0 : items.length - 1) : (at + step) % items.length;
		items[next].focus();
	}

	function onfocusout(event: FocusEvent) {
		const next = event.relatedTarget as Node | null;
		if (next && menu && !menu.contains(next) && next !== trigger) close();
	}

	async function removeOne() {
		if (busy) return;
		busy = 'remove';
		error = '';
		const problem = await onremove();
		busy = null;
		if (problem) error = problem;
		else close();
	}

	async function toggleAdding() {
		adding = !adding;
		if (!adding) return;
		await tick();
		whenBox?.querySelector<HTMLButtonElement>('button')?.focus();
	}

	async function add(when: MarkWhen) {
		if (busy) return;
		busy = 'add';
		error = '';
		const problem = await onadd(when);
		busy = null;
		if (problem) error = problem;
		else close();
	}

	// Follow the check while the page scrolls or resizes, and re-place the
	// menu when its size changes (opening it, or the date field).
	$effect(() => {
		if (!menuOpen && !tipOpen) return;
		const follow = () => {
			placeMenu();
			place(tip, 'above');
		};
		window.addEventListener('scroll', follow, { capture: true, passive: true });
		window.addEventListener('resize', follow);
		return () => {
			window.removeEventListener('scroll', follow, { capture: true });
			window.removeEventListener('resize', follow);
		};
	});

	$effect(() => {
		if (!menu) return;
		const observer = new ResizeObserver(placeMenu);
		observer.observe(menu);
		return () => observer.disconnect();
	});

	$effect(() => () => clearTimeout(hideTimer));

	const item =
		'flex min-h-10 w-full items-center gap-2.5 px-3 py-1.5 text-left text-sm transition-colors focus-visible:-outline-offset-2 sm:min-h-9';
	const icon = 'size-4 shrink-0';
</script>

<button
	bind:this={trigger}
	id={buttonId}
	type="button"
	popovertarget="{uid}-menu"
	aria-haspopup="dialog"
	aria-expanded={menuOpen}
	aria-label="{name}: watched. History options"
	aria-describedby="{uid}-tip"
	onpointerenter={(e) => e.pointerType !== 'touch' && showTip()}
	onpointerleave={hideTipSoon}
	{onfocus}
	onblur={hideTip}
	onkeydown={onTriggerKeydown}
	class="grid size-10 shrink-0 place-items-center rounded-sm bg-watched text-surface transition-colors hover:bg-watched-hover focus-visible:bg-watched-hover focus-visible:outline-on-screen-accent aria-expanded:bg-watched-hover sm:size-8"
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
</button>

<!--
	The tooltip. A described-by target, so screen readers hear it with the
	button even while it's hidden. Popovers keep their own display rules,
	so no display utilities on the popover elements themselves.
-->
<div
	bind:this={tip}
	id="{uid}-tip"
	popover="manual"
	role="tooltip"
	onpointerenter={() => clearTimeout(hideTimer)}
	onpointerleave={hideTipSoon}
	class="inset-auto m-0 w-max max-w-60 rounded-md bg-ink px-2.5 py-1.5 text-xs leading-snug text-canvas shadow-lg"
>
	<span class="block">Watched on <strong class="font-semibold">{date}</strong></span>
	{#if length}<span class="sr-only">, </span><span class="block text-canvas/75 tabular-nums"
			>{length}</span
		>{/if}
	{#if zoneNote}<span class="sr-only">. </span><span
			class="mt-1 block border-t border-canvas/20 pt-1 text-[0.6875rem] text-canvas/75"
			>{zoneNote}</span
		>{/if}
</div>

<div
	bind:this={menu}
	id="{uid}-menu"
	popover="auto"
	role="dialog"
	aria-labelledby="{uid}-title"
	tabindex="-1"
	{onbeforetoggle}
	{ontoggle}
	onkeydown={onMenuKeydown}
	{onfocusout}
	class="inset-auto m-0 max-h-[calc(100dvh-1rem)] w-64 max-w-[calc(100vw-1rem)] overflow-y-auto rounded-lg border border-line bg-surface text-ink shadow-[0_12px_32px_rgb(0_0_0/0.25)] focus-visible:outline-offset-0"
>
	<div class="flex items-center justify-between gap-2 border-b border-line py-1 pr-1 pl-3">
		<p id="{uid}-title" class="text-sm font-semibold">Remove from history?</p>
		<button
			type="button"
			aria-label="Close"
			onclick={close}
			class="grid size-10 place-items-center rounded-md text-muted transition-colors hover:bg-raised hover:text-ink sm:size-8"
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

	<div class="py-1">
		<button
			type="button"
			data-item
			aria-busy={busy === 'remove' || undefined}
			onclick={removeOne}
			class={[item, 'text-danger hover:bg-danger/10']}
		>
			{#if busy === 'remove'}
				<span
					aria-hidden="true"
					class="size-4 shrink-0 animate-spin rounded-full border-2 border-current border-r-transparent"
				></span>
			{:else}
				{@render trash()}
			{/if}
			Only this play
		</button>
		<!-- Removing every play of it needs a new endpoint (later); until then it says so. -->
		<button
			type="button"
			data-item
			aria-disabled="true"
			aria-label="Remove all plays"
			aria-describedby="{uid}-all"
			title="Coming in a later update"
			class={[item, 'cursor-not-allowed text-muted']}
		>
			{@render trash()}
			<span>
				Remove all plays
				<span id="{uid}-all" class="block text-xs">Coming in a later update</span>
			</span>
		</button>
		<button type="button" data-item onclick={close} class={[item, 'hover:bg-raised']}>
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class={icon}
				fill="none"
				stroke="currentColor"
				stroke-width="1.8"
				stroke-linecap="round"><path d="M4.5 4.5l7 7M11.5 4.5l-7 7" /></svg
			>
			Keep it
		</button>
	</div>

	<hr class="border-line" />

	<div class="py-1">
		<button
			type="button"
			data-item
			aria-expanded={adding}
			aria-controls="{uid}-when"
			onclick={toggleAdding}
			class={[item, 'hover:bg-raised']}
		>
			{#if busy === 'add'}
				<span
					aria-hidden="true"
					class="size-4 shrink-0 animate-spin rounded-full border-2 border-current border-r-transparent"
				></span>
			{:else}
				<svg
					aria-hidden="true"
					viewBox="0 0 16 16"
					class={icon}
					fill="none"
					stroke="currentColor"
					stroke-width="1.6"
					stroke-linecap="round"
					><circle cx="8" cy="8" r="6.25" /><path d="M8 5.25v5.5M5.25 8h5.5" /></svg
				>
			{/if}
			Add another play
		</button>
		<div bind:this={whenBox} id="{uid}-when" hidden={!adding} class="border-t border-line">
			{#if adding}
				<WhenChoices onchoose={add} {release} busy={busy === 'add'} />
			{/if}
		</div>
	</div>

	<Notice tone="error" message={error} class="mx-2 mb-2" />
</div>

{#snippet trash()}
	<svg
		aria-hidden="true"
		viewBox="0 0 16 16"
		class={icon}
		fill="none"
		stroke="currentColor"
		stroke-width="1.6"
		stroke-linecap="round"
		stroke-linejoin="round"
		><path
			d="M2.75 4.25h10.5M6.25 4.25V2.75h3.5v1.5M4 4.25l.65 9h6.7l.65-9M6.75 6.75v4M9.25 6.75v4"
		/></svg
	>
{/snippet}
