<script lang="ts">
	// History's toolbar, after classic Trakt's: type and date range on the
	// left; sort, sort direction, source, and fade/hide filters on the
	// right. None of it works yet (the filter backend comes later), so every
	// control is a placeholder: muted, aria-disabled, and described as
	// "Coming in a later update". No counts until they can be real.

	const id = $props.id();
	const later = `${id}-later`;

	// The placeholder look (DESIGN.md, Placeholders): a filter button's
	// shape, quiet and inert. 40 px on phones, 32 px from sm.
	const control =
		'inline-flex h-10 shrink-0 cursor-not-allowed items-center justify-center gap-1 rounded-md border border-line px-2.5 text-sm text-muted opacity-60 sm:h-8';
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

{#snippet icon(d: string, label: string, withChevron = true)}
	<button
		type="button"
		aria-disabled="true"
		aria-label={label}
		aria-describedby={later}
		title="{label}: coming in a later update"
		class={[control, 'min-w-10 sm:min-w-8', !withChevron && 'px-0']}
	>
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
		{#if withChevron}{@render chevron()}{/if}
	</button>
{/snippet}

<div
	role="group"
	aria-label="History filters and sorting"
	class="flex flex-wrap items-center justify-between gap-x-2 gap-y-2"
>
	<span id={later} hidden>Coming in a later update</span>
	<div class="flex items-center gap-1.5">
		<button
			type="button"
			aria-disabled="true"
			aria-label="Type: all"
			aria-describedby={later}
			title="Type: coming in a later update"
			class={control}
		>
			All {@render chevron()}
		</button>
		{@render icon('M2.75 4.25h10.5v9H2.75zM2.75 7h10.5M5.5 2.5v3M10.5 2.5v3', 'Date range')}
	</div>
	<div class="flex items-center gap-1.5">
		<!-- The sort's name is a label from sm up; an icon on phones, so the row fits. -->
		<button
			type="button"
			aria-disabled="true"
			aria-label="Sort by watched date"
			aria-describedby={later}
			title="Sort: coming in a later update"
			class={[control, 'min-w-10 sm:min-w-8']}
		>
			<svg
				aria-hidden="true"
				viewBox="0 0 16 16"
				class="size-4 shrink-0 sm:hidden"
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"><path d="M2.75 4h10.5M2.75 8h7M2.75 12h3.5" /></svg
			>
			<span class="max-sm:hidden">Watched date</span>
			{@render chevron()}
		</button>
		{@render icon(
			'M5 2.75v10.5M2.75 11 5 13.25 7.25 11M11 13.25V2.75M8.75 5 11 2.75 13.25 5',
			'Sort direction: newest first',
			false
		)}
		{@render icon(
			'M5.75 2.5v3M10.25 2.5v3M4 5.5h8v2.75a4 4 0 0 1-8 0zM8 12.25v1.25',
			'Source filter'
		)}
		{@render icon(
			'M1.75 8S4 3.75 8 3.75 14.25 8 14.25 8 12 12.25 8 12.25 1.75 8 1.75 8zM8 9.75a1.75 1.75 0 1 0 0-3.5 1.75 1.75 0 0 0 0 3.5z',
			'Fade or hide filters'
		)}
	</div>
</div>
