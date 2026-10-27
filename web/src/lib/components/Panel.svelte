<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';

	interface Props {
		/** The section heading (an h2). */
		title: string;
		/** One line under the heading. */
		intro?: string;
		/** Lets the page move focus to the heading, e.g. after a row is deleted. */
		headingRef?: HTMLElement;
		/** Content that should reach the panel's edges, such as a table. */
		flush?: boolean;
		class?: ClassValue;
		children: Snippet;
	}

	let {
		title,
		intro,
		headingRef = $bindable(),
		flush = false,
		class: className,
		children
	}: Props = $props();

	const id = $props.id();
</script>

<!-- A titled section of a signed-in page: settings groups, the user list. -->
<section
	aria-labelledby="{id}-title"
	class={['rounded-lg border border-line bg-surface shadow-[0_1px_2px_rgb(0_0_0/0.06)]', className]}
>
	<header class="border-b border-line px-4 py-3">
		<h2
			id="{id}-title"
			bind:this={headingRef}
			tabindex="-1"
			class="text-base font-semibold tracking-tight text-ink"
		>
			{title}
		</h2>
		{#if intro}
			<p class="mt-0.5 text-sm text-muted">{intro}</p>
		{/if}
	</header>
	<div class={flush ? '' : 'p-4'}>
		{@render children()}
	</div>
</section>
