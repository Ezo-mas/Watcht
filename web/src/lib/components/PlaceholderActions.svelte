<script lang="ts">
	import type { ClassValue } from 'svelte/elements';

	// The action column's other actions, shown so the column has its final
	// shape: watchlist and rating (M4 slice 3) and where to watch (in the
	// backlog). Placeholders (DESIGN.md): quiet full-width buttons, muted and
	// inert, focusable with aria-disabled and described as coming later.
	// Nothing for features that aren't planned.

	let { class: className }: { class?: ClassValue } = $props();

	const id = $props.id();

	const actions = [
		{
			label: 'Add to watchlist',
			d: 'M6 4h7.5M6 8h7.5M6 12h7.5M2.5 4h.01M2.5 8h.01M2.5 12h.01'
		},
		{
			label: 'Rate it',
			d: 'M8 13.5s-5.25-3.1-5.25-6.95A2.9 2.9 0 0 1 8 4.8a2.9 2.9 0 0 1 5.25 1.75C13.25 10.4 8 13.5 8 13.5z'
		},
		{
			label: 'Where to watch',
			d: 'M2.25 3.25h11.5v8H2.25zM5.5 13.75h5M7 5.75v3l2.5-1.5z'
		}
	];
</script>

<div class={['flex flex-col gap-2', className]}>
	<span id="{id}-later" hidden>Coming in a later update</span>
	{#each actions as action (action.label)}
		<button
			type="button"
			aria-disabled="true"
			aria-describedby="{id}-later"
			title="{action.label}: coming in a later update"
			class="inline-flex h-10 w-full cursor-not-allowed items-center gap-2.5 rounded-md border border-line px-4 text-sm font-medium text-muted opacity-60"
		>
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class="size-4 shrink-0"
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"
				stroke-linejoin="round"><path d={action.d} /></svg
			>
			{action.label}
		</button>
	{/each}
</div>
