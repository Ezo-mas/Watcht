<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { searchOverlay } from '#lib/search-overlay.svelte.js';
	import Kbd from './Kbd.svelte';
	import SearchView from './SearchView.svelte';

	// A modal <dialog>: the browser keeps focus inside it, makes the page
	// behind inert, closes it on Escape, and puts focus back on whatever
	// opened it. The query stays between openings.

	let dialog = $state<HTMLDialogElement>();
	let input = $state<HTMLInputElement>();
	let query = $state('');

	$effect(() => {
		if (!dialog) return;
		if (searchOverlay.open && !dialog.open) {
			dialog.showModal();
			input?.focus();
			input?.select();
		} else if (!searchOverlay.open && dialog.open) {
			dialog.close();
		}
	});

	// Opening a result (or Back) closes it.
	afterNavigate(() => {
		searchOverlay.open = false;
	});

	// Escape fires "cancel" at once and "close" a task later; reacting to
	// both keeps the state right if "/" comes straight after Escape.
	const close = () => (searchOverlay.open = false);

	function onclick(event: MouseEvent) {
		// The dialog fills the viewport; a click on it rather than on the
		// panel is a click outside.
		if (event.target === dialog) close();
	}

	function showPage(q: string) {
		close();
		void goto(resolve(`/(app)/search?q=${encodeURIComponent(q)}`));
	}
</script>

<dialog
	bind:this={dialog}
	aria-label="Search"
	oncancel={close}
	onclose={close}
	{onclick}
	class="m-0 h-dvh max-h-none w-full max-w-none bg-transparent p-0 text-ink backdrop:bg-canvas/60 backdrop:backdrop-blur-md sm:px-6 sm:pt-[8vh] sm:pb-6"
>
	<div
		class="mx-auto flex h-full w-full max-w-3xl flex-col overflow-hidden border-line/70 bg-surface/75 shadow-[0_24px_80px_rgb(0_0_0/0.45)] backdrop-blur-2xl backdrop-saturate-150 sm:h-auto sm:max-h-full sm:rounded-xl sm:border"
	>
		<SearchView variant="overlay" bind:query bind:input onsubmit={showPage} onopen={close}>
			{#snippet trailing()}
				<button
					type="button"
					aria-label="Close search"
					onclick={close}
					class="grid size-10 shrink-0 place-items-center rounded-md text-muted transition-colors hover:bg-raised hover:text-ink"
				>
					<svg
						aria-hidden="true"
						viewBox="0 0 24 24"
						class="size-5"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18" /></svg
					>
				</button>
			{/snippet}
		</SearchView>
		<p
			class="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-line/70 px-4 py-2 text-xs text-muted max-sm:hidden"
		>
			<span><Kbd>↑</Kbd> <Kbd>↓</Kbd> choose</span>
			<span><Kbd>Enter</Kbd> open, or see all results on a page</span>
			<span><Kbd>Esc</Kbd> close</span>
		</p>
	</div>
</dialog>
