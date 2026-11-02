<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import AppFooter from '#lib/components/AppFooter.svelte';
	import Kbd from '#lib/components/Kbd.svelte';
	import SearchOverlay from '#lib/components/SearchOverlay.svelte';
	import UserMenu from '#lib/components/UserMenu.svelte';
	import Wordmark from '#lib/components/Wordmark.svelte';
	import { searchOverlay } from '#lib/search-overlay.svelte.js';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
	const user = $derived(data.session.user);

	// Title pages start with a full-width hero, so they lay out their own
	// width; every other page gets the usual centred column.
	const fullBleed = $derived(
		!page.error && (page.route.id === '/(app)/movies/[id]' || page.route.id === '/(app)/shows/[id]')
	);

	// "/" opens search from anywhere, except while typing. On the search
	// page it goes to that page's own box instead.
	function onkeydown(event: KeyboardEvent) {
		if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey) return;
		if (event.defaultPrevented || event.isComposing) return;
		const target = event.target;
		if (target instanceof Element && target.closest('input, textarea, select, [contenteditable]'))
			return;
		if (document.querySelector('dialog[open]')) return;
		event.preventDefault();
		const pageInput = document.querySelector<HTMLInputElement>('[data-search-page] input');
		if (pageInput) pageInput.focus();
		else searchOverlay.open = true;
	}
</script>

<svelte:window {onkeydown} />

<div class="flex min-h-dvh flex-col">
	<header class="sticky top-0 z-20 border-b border-line bg-surface">
		<div class="mx-auto flex h-12 w-full max-w-7xl items-center gap-3 px-4 sm:gap-6 sm:px-6">
			<a href={resolve('/(app)')} class="-mx-1 rounded-sm px-1 py-1">
				<Wordmark />
			</a>
			<!--
				History and Watchlist (M4) join Search here, as links with the
				wordmark's accent bar on the header's bottom edge for the current
				page. On a phone they move into the menu.
			-->
			<nav aria-label="Main" class="flex flex-1 items-center justify-end gap-1">
				<button
					type="button"
					aria-haspopup="dialog"
					aria-keyshortcuts="/"
					onclick={() => (searchOverlay.open = true)}
					class="flex h-8 items-center gap-2 rounded-md text-sm text-muted transition-colors hover:text-ink max-sm:size-10 max-sm:justify-center sm:w-56 sm:border sm:border-line sm:bg-field sm:px-2.5 sm:hover:border-line-strong lg:w-72"
				>
					<svg
						aria-hidden="true"
						viewBox="0 0 24 24"
						class="size-5 shrink-0 sm:size-4"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg
					>
					<span class="flex-1 text-left max-sm:sr-only">Search</span>
					<span aria-hidden="true" class="max-sm:hidden"><Kbd>/</Kbd></span>
				</button>
			</nav>
			{#if user}
				<UserMenu {user} />
			{/if}
		</div>
	</header>

	<main class={fullBleed ? 'flex-1' : 'mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6'}>
		{@render children()}
	</main>

	<AppFooter />
</div>

<SearchOverlay />
