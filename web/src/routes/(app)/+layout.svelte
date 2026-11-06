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

	const navLinks = [{ href: resolve('/(app)/history'), label: 'History' }];

	function current(href: string): 'page' | undefined {
		const path = page.url.pathname;
		return path === href || path.startsWith(href + '/') ? 'page' : undefined;
	}

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
	<!--
		Glass: the surface at 85% with a blur, so the page (and a title's
		hero) shows through; no hairline under it. 85% keeps muted text at
		AA over black or white underneath (DESIGN.md, Navigation).
	-->
	<header
		data-app-header
		class="sticky top-0 z-20 bg-surface/85 backdrop-blur-xl backdrop-saturate-150"
	>
		<div class="mx-auto flex h-12 w-full max-w-7xl items-center gap-3 px-4 sm:gap-6 sm:px-6">
			<a href={resolve('/(app)')} class="-mx-1 rounded-sm px-1 py-1">
				<Wordmark />
			</a>
			<!--
				Page links (History; Watchlist joins in M4), with the wordmark's
				accent bar on the header's bottom edge for the current page. On a
				phone they're in the menu until the tab bar arrives.
			-->
			<nav aria-label="Main" class="flex flex-1 items-center gap-1 self-stretch">
				<ul class="flex items-center self-stretch max-sm:hidden">
					{#each navLinks as link (link.href)}
						<li class="flex self-stretch">
							<a
								href={link.href}
								aria-current={current(link.href)}
								class="relative flex items-center px-2.5 text-sm font-medium text-muted transition-colors after:absolute after:inset-x-2.5 after:-bottom-px after:h-0.5 after:rounded-full hover:text-ink focus-visible:-outline-offset-2 aria-[current=page]:text-ink aria-[current=page]:after:bg-accent"
								>{link.label}</a
							>
						</li>
					{/each}
				</ul>
				<button
					type="button"
					aria-haspopup="dialog"
					aria-keyshortcuts="/"
					onclick={() => (searchOverlay.open = true)}
					class="ml-auto flex h-8 items-center gap-2 rounded-md text-sm text-muted transition-colors hover:text-ink max-sm:size-10 max-sm:justify-center sm:w-56 sm:border sm:border-line sm:bg-field sm:px-2.5 sm:hover:border-line-strong lg:w-72"
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

	<!-- A title's hero starts under the header. -->
	<main class={fullBleed ? '-mt-12 flex-1' : 'mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6'}>
		{@render children()}
	</main>

	<AppFooter />
</div>

<SearchOverlay />
