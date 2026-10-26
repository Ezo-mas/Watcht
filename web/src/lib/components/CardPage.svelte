<script lang="ts">
	import type { Snippet } from 'svelte';
	import Wordmark from './Wordmark.svelte';

	interface Props {
		/** The page heading, shown at the top of the card. */
		title: string;
		/** One line under the heading. */
		intro?: string;
		children: Snippet;
	}

	let { title, intro, children }: Props = $props();
</script>

<!-- A page with the wordmark and one narrow card: sign-in, setup, errors. -->
<main class="flex min-h-dvh justify-center px-4 pt-[12vh] pb-12">
	<div class="flex w-full max-w-88 flex-col gap-6">
		<div>
			<Wordmark size="lg" />
		</div>
		<section
			aria-labelledby="card-title"
			class="rounded-lg border border-line bg-surface p-6 shadow-[0_1px_2px_rgb(0_0_0/0.06)]"
		>
			<h1 id="card-title" class="text-lg font-semibold tracking-tight text-ink">{title}</h1>
			{#if intro}
				<p class="mt-1 text-sm text-muted">{intro}</p>
			{/if}
			<div class="mt-5">
				{@render children()}
			</div>
		</section>
	</div>
</main>
