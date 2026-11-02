<script lang="ts" module>
	export interface Fact {
		label: string;
		value: string;
		/** An external page (IMDb, TMDB); opens in a new tab. */
		href?: string;
	}
</script>

<script lang="ts">
	interface Props {
		facts: Fact[];
	}

	let { facts }: Props = $props();

	// Facts with nothing to say aren't shown at all.
	const shown = $derived(facts.filter((f) => f.value !== ''));
</script>

<!--
	Classic label/value pairs that flow across the column and wrap; on a
	phone they stack. Labels are small caps-style captions, values plain
	text, external links in the accent.
-->
<dl class="flex flex-wrap gap-x-6 gap-y-1.5 text-sm">
	{#each shown as fact (fact.label)}
		<div class="flex max-w-full min-w-0 items-baseline gap-2">
			<dt class="shrink-0 text-[0.6875rem] font-semibold tracking-wider text-muted uppercase">
				{fact.label}
			</dt>
			<dd class="min-w-0 text-ink">
				{#if fact.href}
					<!-- External sites (IMDb, TMDB), so no resolve(). -->
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						href={fact.href}
						target="_blank"
						rel="noopener noreferrer"
						class="rounded-sm text-accent-text underline-offset-2 hover:underline"
						>{fact.value}<span class="sr-only"> (opens in a new tab)</span></a
					>
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				{:else}
					{fact.value}
				{/if}
			</dd>
		</div>
	{/each}
</dl>
