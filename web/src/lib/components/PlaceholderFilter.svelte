<script lang="ts">
	// A filter dropdown whose feature comes later (DESIGN.md, Placeholders):
	// the filter button's shape, muted and inert, focusable with
	// aria-disabled, and described by the section's "coming later" line.
	interface Props {
		/** What it shows now ("Cast"). */
		text: string;
		/** Its accessible name ("Show: cast"). */
		label: string;
		/** The ID of the line that says it's coming later. */
		describedby: string;
	}

	let { text, label, describedby }: Props = $props();
</script>

<button
	type="button"
	aria-disabled="true"
	aria-label={label}
	aria-describedby={describedby}
	title="{label}: coming in a later update"
	class="inline-flex h-10 shrink-0 cursor-not-allowed items-center gap-1.5 rounded-md border border-line px-2.5 text-sm text-muted opacity-60 sm:h-8"
>
	{text}
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
</button>
