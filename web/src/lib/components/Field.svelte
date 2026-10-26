<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLInputAttributes, 'value' | 'id'> {
		label: string;
		value?: string;
		/** A rule shown under the field before anything is typed. */
		hint?: string;
		/** A problem with this field, shown under it; also marks the input invalid. */
		error?: string;
	}

	let { label, value = $bindable(''), hint, error, class: className, ...rest }: Props = $props();

	const id = $props.id();
	const describedBy = $derived(
		[hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined
	);
</script>

<div class={['flex flex-col gap-1.5', className]}>
	<label for={id} class="text-sm font-medium text-ink">{label}</label>
	<input
		{...rest}
		{id}
		bind:value
		aria-describedby={describedBy}
		aria-invalid={error ? true : undefined}
		class="h-10 w-full rounded-md border border-line-strong bg-field px-3 text-base text-ink transition-colors placeholder:text-muted focus-visible:border-focus focus-visible:outline-offset-0 aria-invalid:border-danger sm:text-sm"
	/>
	{#if hint}
		<p id="{id}-hint" class="text-xs text-muted">{hint}</p>
	{/if}
	{#if error}
		<p id="{id}-error" class="text-xs font-medium text-danger">{error}</p>
	{/if}
</div>
