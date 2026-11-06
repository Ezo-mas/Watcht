<script lang="ts">
	import { tick } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import type { MarkWhen } from '#lib/api/types.js';
	import type { ReleaseOption } from '#lib/when.js';
	import WhenChoices from './WhenChoices.svelte';

	interface Props {
		/** The trigger's accessible name ("More ways to mark The Matrix watched"). */
		label: string;
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
		/** Which edge of the trigger the menu lines up with. */
		align?: 'start' | 'end';
		/**
		 * chevron: flips while open; plus ("add a play", the movie's action
		 * column): turns 45° into an × while open.
		 */
		icon?: 'chevron' | 'plus';
		/** The trigger's look; the caller joins it to its own button. */
		class?: ClassValue;
	}

	let {
		label,
		onmark,
		release,
		busy = false,
		working = false,
		describedby,
		align = 'end',
		icon = 'chevron',
		class: className
	}: Props = $props();

	// A disclosure (like the user menu), not an ARIA menu: a button that
	// shows a short list of buttons, and a date form for "Other date…"
	// (WhenChoices, which the history card's menu shares).

	const id = $props.id();
	let open = $state(false);
	let root = $state<HTMLElement>();
	let trigger = $state<HTMLButtonElement>();
	let panel = $state<HTMLElement>();

	async function toggle() {
		open = !open;
		if (!open) return;
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

	// Escape closes; WhenChoices handles the arrow keys.
	function onkeydown(event: KeyboardEvent) {
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
		type="button"
		aria-label={label}
		aria-describedby={describedby}
		aria-expanded={open}
		aria-controls="{id}-when"
		aria-busy={working || undefined}
		disabled={busy}
		onclick={toggle}
		class={['grid place-items-center transition-colors disabled:cursor-not-allowed', className]}
	>
		{#if icon === 'plus'}
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
		hidden={!open}
		class={[
			'absolute top-full z-30 mt-1.5 w-72 max-w-[calc(100vw-2rem)] rounded-lg border border-line bg-surface py-1 shadow-[0_12px_32px_rgb(0_0_0/0.25)]',
			align === 'end' ? 'right-0' : 'left-0'
		]}
	>
		{#if open}
			<WhenChoices onchoose={choose} {release} />
		{/if}
	</div>
</div>
