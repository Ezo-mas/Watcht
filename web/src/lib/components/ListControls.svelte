<script lang="ts">
	interface Props {
		/** What the list holds, for the names ("episodes", "seasons"). */
		items: string;
		/** What the filter field matches ("title", "name"). */
		field?: string;
	}

	let { items, field = 'title' }: Props = $props();

	// The controls over a season grid or an episode list, after classic
	// Trakt's: sort (by number) with its direction, a filter field, and a
	// hide-watched (eye) toggle. None of it works yet, so every control is
	// a placeholder (DESIGN.md, Placeholders): muted, aria-disabled, and
	// described as "Coming in a later update". The field can't be typed in
	// (readonly) but stays focusable, so it can say so.

	const id = $props.id();
	const later = `${id}-later`;

	// The placeholder look: a filter button's shape, quiet and inert. 40 px
	// on phones, 32 px from sm.
	const control =
		'inline-flex h-10 shrink-0 cursor-not-allowed items-center justify-center gap-1 border border-line text-sm text-muted sm:h-8';
</script>

{#snippet chevron()}
	<svg
		aria-hidden="true"
		viewBox="0 0 16 16"
		class="size-3 shrink-0"
		fill="none"
		stroke="currentColor"
		stroke-width="2"
		stroke-linecap="round"
		stroke-linejoin="round"><path d="m4 6 4 4 4-4" /></svg
	>
{/snippet}

{#snippet icon(d: string)}
	<svg
		aria-hidden="true"
		viewBox="0 0 16 16"
		class="size-4 shrink-0"
		fill="none"
		stroke="currentColor"
		stroke-width="1.6"
		stroke-linecap="round"
		stroke-linejoin="round"><path {d} /></svg
	>
{/snippet}

<div
	role="group"
	aria-label="Sort and filter {items}"
	class="flex w-full items-center gap-1.5 opacity-60 sm:w-auto"
>
	<span id={later} hidden>Coming in a later update</span>
	<!-- Sort and its direction, joined. -->
	<div class="flex">
		<button
			type="button"
			aria-disabled="true"
			aria-label="Sort {items} by number"
			aria-describedby={later}
			title="Sort: coming in a later update"
			class={[control, 'rounded-l-md px-2.5']}
		>
			<span
				><span class="max-sm:hidden">Sort:&nbsp;</span><span class="font-medium">Number</span></span
			>
			{@render chevron()}
		</button>
		<button
			type="button"
			aria-disabled="true"
			aria-label="Sort direction: first to last"
			aria-describedby={later}
			title="Sort direction: coming in a later update"
			class={[control, 'w-10 rounded-r-md border-l-0 sm:w-8']}
		>
			{@render icon('M5 2.75v10.5M2.75 11 5 13.25 7.25 11M11 13.25V2.75M8.75 5 11 2.75 13.25 5')}
		</button>
	</div>
	<label
		class="relative flex h-10 min-w-0 flex-1 cursor-not-allowed items-center sm:h-8 sm:w-44 sm:flex-none"
	>
		<span class="sr-only">Filter {items} by {field}</span>
		<span class="pointer-events-none absolute left-2.5 text-muted">
			{@render icon('M7 12a5 5 0 1 0 0-10 5 5 0 0 0 0 10zM10.75 10.75l2.75 2.75')}
		</span>
		<input
			type="search"
			readonly
			aria-disabled="true"
			aria-describedby={later}
			placeholder="Filter by {field}"
			title="Filter: coming in a later update"
			class="size-full min-w-0 cursor-not-allowed rounded-md border border-line bg-field pr-2.5 pl-8 text-sm text-muted placeholder:text-muted"
		/>
	</label>
	<button
		type="button"
		aria-disabled="true"
		aria-label="Hide watched {items}"
		aria-describedby={later}
		title="Hide watched: coming in a later update"
		class={[control, 'rounded-md px-2']}
	>
		{@render icon(
			'M1.75 8S4 3.75 8 3.75 14.25 8 14.25 8 12 12.25 8 12.25 1.75 8 1.75 8zM8 9.75a1.75 1.75 0 1 0 0-3.5 1.75 1.75 0 0 0 0 3.5z'
		)}
		{@render chevron()}
	</button>
</div>
