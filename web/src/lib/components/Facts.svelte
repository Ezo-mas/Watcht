<script lang="ts" module>
	export interface Fact {
		label: string;
		value: string;
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
	text.
-->
<dl class="flex flex-wrap gap-x-6 gap-y-1.5 text-sm">
	{#each shown as fact (fact.label)}
		<div class="flex max-w-full min-w-0 items-baseline gap-2">
			<dt class="shrink-0 text-[0.6875rem] font-semibold tracking-wider text-muted uppercase">
				{fact.label}
			</dt>
			<dd class="min-w-0 text-ink">{fact.value}</dd>
		</div>
	{/each}
</dl>
