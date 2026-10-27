<script lang="ts">
	import type { ClassValue } from 'svelte/elements';

	interface Props {
		/** "error" for a problem (role="alert"), "success" for a done note (role="status"). */
		tone: 'error' | 'success';
		/** The text; nothing is shown while it's empty. */
		message: string;
		/** Classes for the visible note, e.g. a margin. */
		class?: ClassValue;
	}

	let { tone, message, class: className }: Props = $props();
</script>

<!--
	Errors appear with their alert role, which screen readers announce on
	insertion. A status region must already exist when its text changes to
	be announced, so the success wrapper is always rendered (and takes no
	space while empty): keep it out of flex or grid gaps.
-->
{#if tone === 'error'}
	{#if message}
		<p
			role="alert"
			class={[
				'rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger first-letter:uppercase',
				className
			]}
		>
			{message}
		</p>
	{/if}
{:else}
	<div role="status">
		{#if message}
			<p
				class={[
					'flex items-start gap-2 rounded-md border border-ok/40 bg-ok/10 px-3 py-2 text-sm text-ok',
					className
				]}
			>
				<svg
					aria-hidden="true"
					viewBox="0 0 16 16"
					class="mt-0.5 size-3.5 shrink-0"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"><path d="M3 8.5l3.2 3L13 4.5" /></svg
				>
				<span class="first-letter:uppercase">{message}</span>
			</p>
		{/if}
	</div>
{/if}
