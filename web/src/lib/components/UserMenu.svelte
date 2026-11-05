<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { logout } from '#lib/api/auth.js';
	import { ApiError } from '#lib/api/client.js';
	import type { User } from '#lib/api/types.js';

	interface Props {
		user: User;
	}

	let { user }: Props = $props();

	// A disclosure, not an ARIA menu: a button that shows a list of links,
	// so Tab and screen readers work as on any list of links. On a phone
	// the same button is the hamburger and the list holds the main links.

	let open = $state(false);
	let root = $state<HTMLElement>();
	let trigger = $state<HTMLButtonElement>();

	// History is in the header's nav from sm up, so only phones list it here.
	const links = $derived([
		{ href: resolve('/(app)/history'), label: 'History', phoneOnly: true },
		{ href: resolve('/(app)/settings'), label: 'Settings' },
		...(user.is_admin ? [{ href: resolve('/(app)/admin/users'), label: 'Users' }] : []),
		{ href: resolve('/(app)/about'), label: 'About' }
	]);

	function current(href: string): 'page' | undefined {
		const path = page.url.pathname;
		return path === href || path.startsWith(href + '/') ? 'page' : undefined;
	}

	afterNavigate(() => {
		open = false;
	});

	function onkeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) {
			event.stopPropagation();
			open = false;
			trigger?.focus();
		}
	}

	function onpointerdown(event: PointerEvent) {
		if (open && root && !root.contains(event.target as Node)) open = false;
	}

	function onfocusout(event: FocusEvent) {
		const next = event.relatedTarget as Node | null;
		if (next && root && !root.contains(next)) open = false;
	}

	let signingOut = $state(false);
	let signOutError = $state('');

	// Only leave once the server has ended the session (or it was already
	// gone): on a shared device, "signed out" must be true.
	async function signOut() {
		signingOut = true;
		signOutError = '';
		try {
			await logout();
		} catch (e) {
			if (!(e instanceof ApiError && e.status === 401)) {
				signOutError = "Couldn't sign out. Try again.";
				signingOut = false;
				return;
			}
		}
		await goto(resolve('/login'), { invalidate: ['app:session'] });
		signingOut = false;
	}
</script>

<svelte:document {onpointerdown} />

<!-- Escape from anywhere inside (the button or a link) closes the list. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div bind:this={root} class="relative" {onkeydown} {onfocusout}>
	<button
		bind:this={trigger}
		type="button"
		aria-expanded={open}
		aria-controls="user-menu"
		onclick={() => (open = !open)}
		class="flex h-9 items-center gap-2 rounded-md px-1.5 text-sm text-ink transition-colors hover:bg-raised max-sm:size-10 max-sm:justify-center"
	>
		<!-- Phone: a menu icon. Wider: the account's initial, name, and a chevron. -->
		<svg
			aria-hidden="true"
			viewBox="0 0 24 24"
			class="size-5 sm:hidden"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"><path d="M4 7h16M4 12h16M4 17h16" /></svg
		>
		<span
			aria-hidden="true"
			class="grid size-6 shrink-0 place-items-center rounded-full bg-accent/20 text-xs font-semibold text-accent-text uppercase max-sm:hidden"
		>
			{user.username.charAt(0)}
		</span>
		<span class="max-w-48 truncate max-sm:sr-only"
			><span class="sr-only">Menu, signed in as&nbsp;</span>{user.username}</span
		>
		<svg
			aria-hidden="true"
			viewBox="0 0 16 16"
			class={['size-3.5 text-muted transition-transform max-sm:hidden', open && 'rotate-180']}
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			stroke-linejoin="round"><path d="m4 6 4 4 4-4" /></svg
		>
	</button>

	<div
		id="user-menu"
		hidden={!open}
		class="absolute top-full right-0 z-30 mt-1.5 w-60 rounded-lg border border-line bg-surface py-1 shadow-[0_12px_32px_rgb(0_0_0/0.25)]"
	>
		<p class="truncate border-b border-line px-3 pt-1.5 pb-2 text-xs text-muted">
			Signed in as <span class="font-medium text-ink">{user.username}</span>
		</p>
		<!-- History (and Watchlist, M4) are listed here on a phone, above the account links. -->
		<ul class="py-1">
			{#each links as link (link.href)}
				<li class={[link.phoneOnly && 'sm:hidden']}>
					<a
						href={link.href}
						aria-current={current(link.href)}
						class="flex h-9 items-center px-3 text-sm text-ink transition-colors hover:bg-raised focus-visible:-outline-offset-2 aria-[current=page]:font-semibold aria-[current=page]:text-accent-text"
					>
						{link.label}
					</a>
				</li>
			{/each}
		</ul>
		<div class="border-t border-line pt-1">
			{#if signOutError}
				<p role="alert" class="px-3 py-1.5 text-xs text-danger">{signOutError}</p>
			{/if}
			<button
				type="button"
				onclick={signOut}
				disabled={signingOut}
				aria-busy={signingOut || undefined}
				class="flex h-9 w-full items-center gap-2 px-3 text-left text-sm text-ink transition-colors hover:bg-raised focus-visible:-outline-offset-2 disabled:opacity-60"
			>
				{#if signingOut}
					<span
						aria-hidden="true"
						class="size-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
					></span>
				{/if}
				Sign out
			</button>
		</div>
	</div>
</div>
