<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { flip } from 'svelte/animate';
	import { fly } from 'svelte/transition';
	import { toast, type Toast } from '#lib/toast.svelte.js';

	// The app's toasts (one Toaster, in the (app) layout): action results
	// that float under the header and time out (DESIGN.md, Toasts). Both
	// live regions are always mounted, so the first toast is announced:
	// errors in an alert region (stacked on top, since they stay until
	// dismissed), everything else in a polite status region. Nothing here
	// takes focus.
	//
	// Menus are popovers in the top layer, where z-index doesn't reach, so
	// the stack is a manual popover too, always open. When a menu opens
	// over it, it's raised (hidden and shown again: the newest top-layer
	// element is on top), at once if toasts are showing, else just before
	// the next toast goes in, so the live regions are shown when the new
	// text arrives. Usually nothing is open and nothing is raised.
	// Tooltips don't count, and a modal dialog (the search overlay) is
	// never covered: everything outside it is inert, so toasts there
	// couldn't be dismissed; they wait under it.

	const errors = $derived(toast.items.filter((t) => t.tone === 'error'));
	const notes = $derived(toast.items.filter((t) => t.tone !== 'error'));

	// Svelte's transitions run in script, so the global reduced-motion
	// rule in layout.css doesn't reach them: check it here.
	function duration(ms: number) {
		return typeof matchMedia === 'function' &&
			matchMedia('(prefers-reduced-motion: reduce)').matches
			? 0
			: ms;
	}

	function enter(node: Element) {
		return fly(node, { y: -8, duration: duration(160) });
	}

	function leave(node: Element) {
		return fly(node, { y: -8, duration: duration(120) });
	}

	function move(node: Element, rects: { from: DOMRect; to: DOMRect }) {
		return flip(node, rects, { duration: duration(160) });
	}

	let host = $state<HTMLElement>();

	// Dismissed from the keyboard (a click with no pointer, detail 0),
	// focus moves to the next toast's ×, so it isn't dropped to the page.
	// Not for the mouse: that would hold the next toast until focus left.
	async function dismiss(event: MouseEvent, item: Toast) {
		toast.dismiss(item.id);
		if (event.detail !== 0) return;
		await tick();
		// Skip the one still fading out (Svelte makes it inert).
		const left = host?.querySelectorAll<HTMLElement>('[data-dismiss]') ?? [];
		[...left].find((b) => !b.closest('[inert]'))?.focus();
	}

	const modalOpen = () => !!document.querySelector('dialog:modal');

	/** Another menu or dialog is in the top layer, maybe over the stack. */
	function othersOpen() {
		return [...document.querySelectorAll(':popover-open, dialog[open]')].some(
			(el) => el !== host && el.getAttribute('role') !== 'tooltip'
		);
	}

	// Set when a menu opened while no toast was showing: the next toast
	// raises the stack first.
	let covered = false;

	function raise() {
		covered = false;
		if (!host?.isConnected || modalOpen()) return;
		const active = document.activeElement as HTMLElement | null;
		const hadFocus = !!active && host.contains(active);
		try {
			if (host.matches(':popover-open')) host.hidePopover();
			host.showPopover();
		} catch {
			// Not supported, or mid-way through another toggle: the toasts
			// still show, just possibly under an open menu.
			return;
		}
		// Hiding drops focus from a toast's ×; put it back where it was.
		if (hadFocus && document.activeElement !== active) active?.focus({ preventScroll: true });
	}

	// Before the DOM gets the new toast (see above), and only if a menu
	// opened since the last raise and is still open.
	const newest = $derived(toast.items[0]?.id);
	$effect.pre(() => {
		if (newest === undefined || !covered) return;
		if (othersOpen()) raise();
		else covered = false;
	});

	onMount(() => {
		// Toasts belong to the signed-in session that made them: none carry
		// over into or out of the signed-in pages (sign-out, an expired
		// session sending you to /login, the next person signing in).
		toast.clear();
		try {
			host?.showPopover();
		} catch {
			// No Popover API: the stack is still drawn, just not over menus.
		}
		function ontoggle(event: Event) {
			const target = event.target as Element;
			if (target === host || (event as ToggleEvent).newState !== 'open') return;
			if (target.getAttribute('role') === 'tooltip' || target.matches('dialog:modal')) return;
			if (toast.items.length) raise();
			else covered = true;
		}
		document.addEventListener('toggle', ontoggle, true);
		return () => {
			document.removeEventListener('toggle', ontoggle, true);
			toast.clear();
		};
	});

	// Hovered or focused, a toast holds its timeout; it runs again only
	// when it's neither.
	function onfocusout(event: FocusEvent, item: Toast) {
		const next = event.relatedTarget as Node | null;
		if (!next || !(event.currentTarget as HTMLElement).contains(next)) {
			toast.hold(item.id, 'focus', false);
		}
	}
</script>

{#snippet card(item: Toast)}
	<div
		role="group"
		aria-label={item.tone === 'error' ? 'Error' : 'Notification'}
		onpointerenter={() => toast.hold(item.id, 'hover', true)}
		onpointerleave={() => toast.hold(item.id, 'hover', false)}
		onfocusin={() => toast.hold(item.id, 'focus', true)}
		onfocusout={(e) => onfocusout(e, item)}
		class={[
			'pointer-events-auto flex items-start gap-3 rounded-lg border bg-surface py-2 pr-1.5 pl-3 text-sm text-ink shadow-[0_12px_32px_rgb(0_0_0/0.25)]',
			item.tone === 'error' ? 'border-danger/40' : 'border-line'
		]}
	>
		<svg
			aria-hidden="true"
			viewBox="0 0 16 16"
			class={[
				'mt-2 size-4 shrink-0',
				item.tone === 'success' && 'text-watched',
				item.tone === 'error' && 'text-danger',
				item.tone === 'info' && 'text-muted'
			]}
			fill="none"
			stroke="currentColor"
			stroke-width="1.8"
			stroke-linecap="round"
			stroke-linejoin="round"
		>
			{#if item.tone === 'success'}
				<circle cx="8" cy="8" r="6.5" /><path d="M5.25 8.25l1.9 1.8 3.6-3.8" />
			{:else if item.tone === 'error'}
				<circle cx="8" cy="8" r="6.5" /><path d="M8 4.75v3.75M8 11.1v.15" />
			{:else}
				<circle cx="8" cy="8" r="6.5" /><path d="M8 7.25v3.75M8 4.9v.15" />
			{/if}
		</svg>
		<p class="min-w-0 flex-1 py-1.5 leading-snug first-letter:uppercase">{item.message}</p>
		<button
			type="button"
			aria-label="Dismiss"
			data-dismiss
			onclick={(e) => dismiss(e, item)}
			class="grid size-10 shrink-0 place-items-center rounded-md text-muted transition-colors hover:bg-raised hover:text-ink sm:size-8"
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
{/snippet}

<!--
	Under the header: full width on phones (the planned tab bar sits at
	the bottom), top right on wider screens, lined up with the column's
	right edge. Clicks pass through the empty area.
-->
<div
	bind:this={host}
	popover="manual"
	class="pointer-events-none fixed inset-auto top-14 right-2 left-2 m-0 h-auto max-h-none w-auto max-w-none overflow-visible border-0 bg-transparent p-0 text-ink sm:right-[max(1.5rem,calc((100vw-80rem)/2+1.5rem))] sm:left-auto sm:w-[22rem]"
>
	<!-- Display utilities go on this wrapper, never on the popover itself. -->
	<div class="flex flex-col">
		<div role="alert" class="flex flex-col gap-2">
			{#each errors as item (item.id)}
				<div in:enter out:leave animate:move>{@render card(item)}</div>
			{/each}
		</div>
		<div
			role="status"
			aria-live="polite"
			class={['flex flex-col gap-2', errors.length > 0 && notes.length > 0 && 'mt-2']}
		>
			{#each notes as item (item.id)}
				<div in:enter out:leave animate:move>{@render card(item)}</div>
			{/each}
		</div>
	</div>
</div>
