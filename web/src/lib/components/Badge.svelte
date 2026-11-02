<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';

	interface Props {
		/**
		 * accent: the one thing to see first (a result's type, an air date);
		 * quiet: secondary facts (runtime, score); outline: not yet (unaired).
		 */
		tone?: 'accent' | 'quiet' | 'outline';
		title?: string;
		class?: ClassValue;
		children: Snippet;
	}

	let { tone = 'quiet', title, class: className, children }: Props = $props();
</script>

<span
	{title}
	class={[
		'inline-flex h-[1.125rem] items-center rounded-sm px-1.5 text-[0.6875rem] leading-none font-semibold whitespace-nowrap tabular-nums',
		tone === 'accent' && 'bg-accent text-on-accent',
		tone === 'quiet' && 'bg-raised text-muted',
		tone === 'outline' && 'border border-line-strong text-muted',
		className
	]}>{@render children()}</span
>
