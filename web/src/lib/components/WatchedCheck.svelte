<script lang="ts">
	import { tick } from 'svelte';
	import type { MarkWhen } from '#lib/api/types.js';
	import { coveredTop, outOfView, placeNear } from '#lib/anchor.js';
	import { formatWatchDay } from '#lib/format.js';
	import { formatRuntime } from '#lib/media.js';
	import { stripSquare } from '#lib/strip.js';
	import type { ReleaseOption } from '#lib/when.js';
	import MenuPopover, {
		keepIt,
		menuIcon,
		menuItem,
		spinner,
		trashIcon
	} from './MenuPopover.svelte';
	import RemoveAllPlays from './RemoveAllPlays.svelte';
	import WhenChoices from './WhenChoices.svelte';

	interface Props {
		/** What was watched, for the labels ("Breaking Bad S01E02"). */
		name: string;
		/** The watch's time (RFC 3339), shown in the tooltip. */
		watchedAt: string;
		/** The watch's date when it has no time (YYYY-MM-DD), shown instead. */
		watchedOn: string | null;
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
		onremoveall: () => Promise<string>;
		/** Marks it watched again; resolves like onremove. The menu closes on success. */
		onadd: (when: MarkWhen) => Promise<string>;
	}

	let {
		name,
		watchedAt,
		watchedOn,
		runtime,
		release,
		buttonId,
		zoneNote,
		onremove,
		onremoveall,
		onadd
	}: Props = $props();

	// A watched check on a poster card's bar: a sage square with a check,
	// flush in the bar's bottom-left corner.
	// Hover or keyboard focus shows a tooltip with when it was watched; a
	// click opens a small dialog: remove this play, keep it, or add another.
	// Both popups use the Popover API, so they sit in the top layer: the
	// card, the next card, and the sticky day heading can't cover or clip
	// them. The menu is the shared MenuPopover shell (placement, focus,
	// keys, light dismiss).

	const uid = $props.id();
	let trigger = $state<HTMLButtonElement>();
	let tip = $state<HTMLElement>();
	let menu = $state<MenuPopover>();
	let whenBox = $state<HTMLElement>();

	let menuOpen = $state(false);
	let tipOpen = $state(false);
	let adding = $state(false);
	let busy = $state<'remove' | 'add' | null>(null);
	let removeAllBusy = $state(false);
	let error = $state('');

	const date = $derived(formatWatchDay({ watched_at: watchedAt, watched_on: watchedOn }));
	const length = $derived(formatRuntime(runtime));

	function viewport() {
		return { width: document.documentElement.clientWidth, height: window.innerHeight };
	}

	function placeTip() {
		if (!tip || !trigger || !tip.matches(':popover-open')) return;
		const box = trigger.getBoundingClientRect();
		const inset = coveredTop();
		// Like the menu: the check scrolled out of view, so the tooltip goes.
		if (outOfView(box, viewport(), inset)) {
			hideTip();
			return;
		}
		const at = placeNear(box, { width: tip.offsetWidth, height: tip.offsetHeight }, viewport(), {
			prefer: 'above',
			insetTop: inset
		});
		tip.style.top = `${at.top}px`;
		tip.style.left = `${at.left}px`;
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
		placeTip();
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

	// The menu (MenuPopover): as it opens the tooltip goes; once it has
	// closed (and focus is back on the check), the "add another" choice
	// folds away, errors clear, and the next focus may show the tooltip
	// again: quietFocus only covers focus coming back from the menu.
	function onMenuChange(open: boolean) {
		if (open) hideTip();
		else {
			adding = false;
			error = '';
			quietFocus = false;
		}
	}

	function close() {
		menu?.close();
	}

	async function removeOne() {
		if (busy || removeAllBusy) return;
		busy = 'remove';
		error = '';
		const problem = await onremove();
		busy = null;
		if (problem) error = problem;
		else close();
	}

	async function toggleAdding() {
		if (busy || removeAllBusy) return;
		adding = !adding;
		if (!adding) return;
		await tick();
		whenBox?.querySelector<HTMLButtonElement>('button')?.focus();
	}

	async function add(when: MarkWhen) {
		if (busy || removeAllBusy) return;
		busy = 'add';
		error = '';
		const problem = await onadd(when);
		busy = null;
		if (problem) error = problem;
		else close();
	}

	// The tooltip follows the check while the page scrolls or resizes.
	$effect(() => {
		if (!tipOpen) return;
		window.addEventListener('scroll', placeTip, { capture: true, passive: true });
		window.addEventListener('resize', placeTip);
		return () => {
			window.removeEventListener('scroll', placeTip, { capture: true });
			window.removeEventListener('resize', placeTip);
		};
	});

	$effect(() => () => clearTimeout(hideTimer));
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
	class={[
		stripSquare,
		'rounded-bl-sm bg-watched text-surface transition-colors hover:bg-watched-hover focus-visible:bg-watched-hover aria-expanded:bg-watched-hover'
	]}
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

<MenuPopover
	bind:this={menu}
	bind:open={menuOpen}
	id="{uid}-menu"
	title="Remove from history?"
	anchor={trigger}
	onchange={onMenuChange}
	onreturnfocus={() => (quietFocus = true)}
	{error}
	class="w-64"
>
	<div class="py-1">
		<button
			type="button"
			data-item
			aria-busy={busy === 'remove' || undefined}
			onclick={removeOne}
			class={[menuItem, 'text-danger hover:bg-danger/10']}
		>
			{#if busy === 'remove'}{@render spinner()}{:else}{@render trashIcon()}{/if}
			Only this play
		</button>
		<RemoveAllPlays
			title={name}
			onremove={onremoveall}
			disabled={!!busy}
			bind:busy={removeAllBusy}
			open={menuOpen}
			ondone={close}
		/>
		{@render keepIt(close)}
	</div>

	<hr class="border-line" />

	<div class="py-1">
		<button
			type="button"
			data-item
			aria-expanded={adding}
			aria-controls="{uid}-when"
			onclick={toggleAdding}
			class={[menuItem, 'hover:bg-raised']}
		>
			{#if busy === 'add'}
				{@render spinner()}
			{:else}
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
			{/if}
			Add another play
		</button>
		<div bind:this={whenBox} id="{uid}-when" hidden={!adding} class="border-t border-line">
			{#if adding}
				<WhenChoices onchoose={add} {release} busy={busy === 'add'} />
			{/if}
		</div>
	</div>
</MenuPopover>
