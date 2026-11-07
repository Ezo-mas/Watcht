<script lang="ts" module>
	// Shared by the menus built on this shell (WatchedCheck's history
	// menu, PlaysButton's plays menu): one look for their rows.

	/** A menu row: 40 px on phones, 36 px from sm. */
	export const menuItem =
		'flex min-h-10 w-full items-center gap-2.5 px-3 py-1.5 text-left text-sm transition-colors focus-visible:-outline-offset-2 sm:min-h-9';
	/** A row's leading icon. */
	export const menuIcon = 'size-4 shrink-0';

	export { trashIcon, removeAllPlaceholder, keepIt, spinner };
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import { coveredTop, outOfView, placeNear } from '#lib/anchor.js';
	import Notice from './Notice.svelte';

	interface Props {
		/** The popover's id; the trigger opens it with popovertarget. */
		id: string;
		/**
		 * A header row at the top ("Remove from history?", with a × to close),
		 * which also names the dialog. Without it there's no header; then
		 * `label` names the dialog.
		 */
		title?: string;
		/** The dialog's name when it has no title ("Plays of The Matrix"). */
		label?: string;
		/** The trigger: the menu is placed next to it, and focus returns to it. */
		anchor: HTMLElement | undefined;
		/** Whether it's open; kept in step by the menu (read it, don't set it). */
		open?: boolean;
		/** Runs as it opens (after placing and focusing) or closes (after focus returns). */
		onchange?: (open: boolean) => void;
		/** Runs right before focus goes back to the anchor (WatchedCheck keeps its tooltip down). */
		onreturnfocus?: () => void;
		/** A failure to show at the foot; the menu stays open. */
		error?: string;
		class?: ClassValue;
		children: Snippet;
	}

	let {
		id,
		title,
		label,
		anchor,
		open = $bindable(false),
		onchange,
		onreturnfocus,
		error = '',
		class: className,
		children
	}: Props = $props();

	// A non-modal dialog on the Popover API ("auto"), so it sits in the top
	// layer: no card, sticky heading, or overflow can cover or clip it, only
	// one is open at a time, and Esc or a click outside closes it (the
	// browser's light dismiss). Placed in script next to its anchor.

	let menu = $state<HTMLElement>();

	function viewport() {
		return { width: document.documentElement.clientWidth, height: window.innerHeight };
	}

	// The side it opened on; it keeps it while open (shifting to fit as it
	// grows) instead of jumping across the anchor.
	let side: 'below' | 'above' = 'below';
	// Set as it opens: the first placement (often the ResizeObserver's,
	// before the toggle event) picks the side.
	let fresh = false;

	function place() {
		if (!menu || !anchor || !menu.matches(':popover-open')) return;
		const box = anchor.getBoundingClientRect();
		const inset = coveredTop();
		// Its anchor scrolled out of view (under the header, or off the
		// window): close rather than hold it at the window's edge over the
		// header. Focus inside goes back to the anchor without scrolling.
		if (outOfView(box, viewport(), inset)) {
			close();
			return;
		}
		const at = placeNear(box, { width: menu.offsetWidth, height: menu.offsetHeight }, viewport(), {
			prefer: fresh ? 'below' : side,
			flip: fresh,
			insetTop: inset
		});
		menu.style.top = `${at.top}px`;
		menu.style.left = `${at.left}px`;
		if (fresh) side = at.side;
		fresh = false;
	}

	// Runs as it opens or closes (toggle comes later). Closing with focus
	// inside, the browser puts focus back on the anchor right away.
	function onbeforetoggle(event: ToggleEvent) {
		open = event.newState === 'open';
		if (open) fresh = true;
		else if (menu?.contains(document.activeElement)) onreturnfocus?.();
	}

	function ontoggle(event: ToggleEvent) {
		open = event.newState === 'open';
		if (open) {
			place();
			onchange?.(true);
			// The dialog itself, not its first item: a second Enter mustn't act.
			menu?.focus({ preventScroll: true });
			return;
		}
		// Back to the anchor when focus was in the menu (Esc, Keep it, a click
		// on empty page), not when it moved somewhere on purpose.
		const active = document.activeElement;
		if (anchor?.isConnected && (!active || active === document.body || menu?.contains(active))) {
			onreturnfocus?.();
			// Without scrolling: it may have closed because the anchor scrolled away.
			anchor.focus({ preventScroll: true });
		}
		onchange?.(false);
	}

	/** Closes it (focus goes back to the anchor if it was inside). */
	export function close() {
		if (menu?.isConnected && menu.matches(':popover-open')) menu.hidePopover();
	}

	/** The dialog element, to move focus into it (after removing a row). */
	export function element() {
		return menu;
	}

	// Up and down move between the rows (data-item); Tab works as usual.
	function onkeydown(event: KeyboardEvent) {
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

	// Focus leaving for somewhere else closes it.
	function onfocusout(event: FocusEvent) {
		const next = event.relatedTarget as Node | null;
		if (next && menu && !menu.contains(next) && next !== anchor) close();
	}

	// Follow the anchor while the page scrolls or resizes.
	$effect(() => {
		if (!open) return;
		window.addEventListener('scroll', place, { capture: true, passive: true });
		window.addEventListener('resize', place);
		return () => {
			window.removeEventListener('scroll', place, { capture: true });
			window.removeEventListener('resize', place);
		};
	});

	// Re-place it when its size changes (opening, a confirm, the date field).
	$effect(() => {
		if (!menu) return;
		const observer = new ResizeObserver(place);
		observer.observe(menu);
		return () => observer.disconnect();
	});
</script>

<!-- Popovers keep their own display rules: no display utilities on this element. -->
<div
	bind:this={menu}
	{id}
	popover="auto"
	role="dialog"
	aria-labelledby={title ? `${id}-title` : undefined}
	aria-label={title ? undefined : label}
	tabindex="-1"
	{onbeforetoggle}
	{ontoggle}
	{onkeydown}
	{onfocusout}
	class={[
		'inset-auto m-0 max-h-[calc(100dvh-1rem)] max-w-[calc(100vw-1rem)] overflow-y-auto rounded-lg border border-line bg-surface text-ink shadow-[0_12px_32px_rgb(0_0_0/0.25)] focus-visible:outline-offset-0',
		className
	]}
>
	{#if title}
		<div class="flex items-center justify-between gap-2 border-b border-line py-1 pr-1 pl-3">
			<p id="{id}-title" class="text-sm font-semibold">{title}</p>
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
	{/if}

	{@render children()}

	<Notice tone="error" message={error} class="mx-2 mb-2" />
</div>

{#snippet trashIcon()}
	<svg
		aria-hidden="true"
		viewBox="0 0 16 16"
		class={menuIcon}
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

{#snippet spinner()}
	<span
		aria-hidden="true"
		class="size-4 shrink-0 animate-spin rounded-full border-2 border-current border-r-transparent"
	></span>
{/snippet}

<!--
	Removing every play of a title needs a new endpoint (later); until then
	the row says so (DESIGN.md, Placeholders). `describedId` names its
	"coming later" line.
-->
{#snippet removeAllPlaceholder(describedId: string)}
	<button
		type="button"
		data-item
		aria-disabled="true"
		aria-label="Remove all plays"
		aria-describedby={describedId}
		title="Coming in a later update"
		class={[menuItem, 'cursor-not-allowed text-muted']}
	>
		{@render trashIcon()}
		<span>
			Remove all plays
			<span id={describedId} class="block text-xs">Coming in a later update</span>
		</span>
	</button>
{/snippet}

{#snippet keepIt(onclick: () => void)}
	<button type="button" data-item {onclick} class={[menuItem, 'hover:bg-raised']}>
		<svg
			aria-hidden="true"
			viewBox="0 0 16 16"
			class={menuIcon}
			fill="none"
			stroke="currentColor"
			stroke-width="1.8"
			stroke-linecap="round"><path d="M4.5 4.5l7 7M11.5 4.5l-7 7" /></svg
		>
		Keep it
	</button>
{/snippet}
