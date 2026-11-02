<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<!-- A key in a keyboard hint ("/", "Esc"). -->
<kbd
	class="inline-grid h-[1.125rem] min-w-5 place-items-center rounded-sm border border-line-strong px-1 font-sans text-[0.6875rem] leading-none text-muted"
	>{@render children()}</kbd
>
