<script lang="ts">
	import type { HTMLButtonAttributes } from 'svelte/elements';

	interface Props extends HTMLButtonAttributes {
		/** "primary" for the main action of a form, "quiet" for everything else. */
		variant?: 'primary' | 'quiet';
		/** Disables the button and shows a spinner while an action runs. */
		busy?: boolean;
	}

	let {
		variant = 'primary',
		busy = false,
		disabled,
		type = 'button',
		class: className,
		children,
		...rest
	}: Props = $props();
</script>

<button
	{...rest}
	{type}
	disabled={disabled || busy}
	aria-busy={busy || undefined}
	class={[
		'inline-flex items-center justify-center gap-2 rounded-md font-medium whitespace-nowrap transition-colors disabled:cursor-not-allowed',
		variant === 'primary' &&
			'h-10 bg-accent px-4 text-sm text-on-accent hover:bg-accent-hover disabled:opacity-70 disabled:hover:bg-accent',
		variant === 'quiet' &&
			'h-8 border border-line px-3 text-sm text-ink hover:border-line-strong hover:bg-canvas disabled:opacity-60',
		className
	]}
>
	{#if busy}
		<span
			aria-hidden="true"
			class="size-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
		></span>
	{/if}
	{@render children?.()}
</button>
