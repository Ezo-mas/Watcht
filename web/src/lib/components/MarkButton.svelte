<script lang="ts">
	import type { ClassValue } from 'svelte/elements';
	import type { MarkWhen } from '#lib/api/types.js';
	import type { ReleaseOption } from '#lib/when.js';
	import WhenMenu from './WhenMenu.svelte';

	interface Props {
		/** The one-click action, which marks it just now ("Mark as watched"). */
		label: string;
		/** The menu button's accessible name ("Choose when you watched The Matrix"). */
		menuLabel: string;
		/** Records the mark; the button is busy until it settles. */
		onmark: (when: MarkWhen) => Promise<unknown>;
		release: ReleaseOption;
		/** primary: the page's main action (40 px); quiet: a section's (32 px). */
		variant?: 'primary' | 'quiet';
		/** Turns the whole control off; the reason is shown under it. */
		disabledReason?: string;
		class?: ClassValue;
	}

	let {
		label,
		menuLabel,
		onmark,
		release,
		variant = 'primary',
		disabledReason = '',
		class: className
	}: Props = $props();

	let busy = $state(false);

	async function run(when: MarkWhen) {
		if (busy) return;
		busy = true;
		try {
			await onmark(when);
		} finally {
			busy = false;
		}
	}

	const primary = $derived(variant === 'primary');
	const id = $props.id();
	const reasonId = $derived(disabledReason ? `${id}-reason` : undefined);
</script>

<!--
	A split button: the main part marks it watched just now in one click;
	the chevron opens the when-choice (just now, release date, other date).
-->
<div class={['flex flex-col', !primary && 'items-start', className]}>
	<div class="flex">
		<button
			type="button"
			aria-describedby={reasonId}
			disabled={busy || !!disabledReason}
			aria-busy={busy || undefined}
			onclick={() => run({ at: 'now' })}
			class={[
				'inline-flex min-w-0 flex-1 items-center justify-center gap-2 rounded-l-md font-medium whitespace-nowrap transition-colors disabled:cursor-not-allowed',
				primary
					? 'h-10 bg-accent px-4 text-sm text-on-accent hover:bg-accent-hover disabled:opacity-70 disabled:hover:bg-accent'
					: 'h-8 border border-line px-3 text-sm text-ink hover:border-line-strong hover:bg-canvas disabled:opacity-60 disabled:hover:border-line disabled:hover:bg-transparent'
			]}
		>
			{#if busy}
				<span
					aria-hidden="true"
					class="size-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
				></span>
			{:else}
				<svg
					aria-hidden="true"
					viewBox="0 0 16 16"
					class="size-4 shrink-0"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"><path d="M3 8.5l3.2 3L13 4.5" /></svg
				>
			{/if}
			<span class="truncate">{label}</span>
		</button>
		<WhenMenu
			label={menuLabel}
			onmark={run}
			{release}
			busy={busy || !!disabledReason}
			describedby={reasonId}
			class={[
				'rounded-r-md',
				primary
					? 'h-10 w-10 border-l border-on-accent/25 bg-accent text-on-accent hover:bg-accent-hover disabled:opacity-70 disabled:hover:bg-accent'
					: 'h-8 w-8 border border-l-0 border-line text-ink hover:border-line-strong hover:bg-canvas disabled:opacity-60 disabled:hover:border-line disabled:hover:bg-transparent'
			]}
		/>
	</div>
	{#if disabledReason}
		<!-- Visible, and the buttons' description, so screen readers hear why. -->
		<p id={reasonId} class="mt-1.5 text-xs text-muted">{disabledReason}</p>
	{/if}
</div>
