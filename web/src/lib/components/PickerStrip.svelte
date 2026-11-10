<script lang="ts" module>
	import type { ResolvedPathname } from '$app/types';

	/** One page in the strip: a season or an episode. */
	export interface PickerItem {
		key: number;
		/** A resolve()d path. */
		href: ResolvedPathname;
		/** What the chip shows ("1", "Specials"). */
		label: string;
		/** Said before the label to screen readers ("Season", "Episode"). */
		prefix?: string;
		/** The link's whole accessible name, when the label alone isn't one ("All episodes"). */
		name?: string;
		/** The tooltip (an episode's title, a season's episode count). */
		title?: string;
		/** This page: accent-filled, aria-current="page". */
		current: boolean;
		/** Its episodes are still being fetched: a dashed border. */
		pending?: boolean;
	}
</script>

<script lang="ts">
	import type { ClassValue } from 'svelte/elements';

	interface Props {
		/** The small label before the chips ("Season"), shown in capitals. */
		label: string;
		/** The navigation's accessible name ("Seasons of Breaking Bad"). */
		name: string;
		items: PickerItem[];
		class?: ClassValue;
	}

	let { label, name, items, class: className }: Props = $props();

	let list = $state<HTMLElement>();
	const currentKey = $derived(items.find((i) => i.current)?.key);

	// Bring the current chip into view inside the strip (not the page) when
	// it's off to the side: on load, and when another one is picked.
	$effect(() => {
		void currentKey;
		if (!list || list.scrollWidth <= list.clientWidth) return;
		const chip = list.querySelector<HTMLElement>('[aria-current="page"]');
		if (!chip) return;
		// The list is positioned, so it's the chip's offset parent.
		const left = chip.offsetLeft;
		if (left < list.scrollLeft || left + chip.offsetWidth > list.scrollLeft + list.clientWidth) {
			list.scrollLeft = left - (list.clientWidth - chip.offsetWidth) / 2;
		}
	});
</script>

<!--
	Sibling pages (a show's seasons, a season's episodes), after classic
	Trakt: a label, then a row of links that scrolls sideways when it
	doesn't fit. Picking one keeps the scroll position and the focus, so
	it works like switching tabs. Padding inside the scroller keeps focus
	rings from being clipped, and the deeper bottom padding keeps the
	scrollbar (an overlay one too) below the chips, never over them; the
	negative margin takes it back out of the layout.
-->
<nav aria-label={name} class={['flex min-w-0 items-center gap-2 sm:gap-3', className]}>
	<span
		aria-hidden="true"
		class="shrink-0 text-[0.6875rem] font-semibold tracking-wider text-muted uppercase"
		>{label}</span
	>
	<ul
		bind:this={list}
		class="relative -mt-1 -mb-3 flex min-w-0 [scrollbar-width:thin] gap-1.5 overflow-x-auto overscroll-x-contain p-1 pb-3"
	>
		{#each items as item (item.key)}
			<li class="shrink-0">
				<!-- The type makes callers pass a resolve()d path; the rule can't see through props. -->
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					href={item.href}
					aria-current={item.current ? 'page' : undefined}
					aria-label={item.name}
					title={item.title}
					data-sveltekit-reset="false"
					class={[
						'flex h-10 min-w-10 items-center justify-center rounded-md border px-2.5 text-sm font-semibold whitespace-nowrap tabular-nums transition-colors sm:h-8 sm:min-w-8',
						item.current
							? 'border-accent bg-accent text-on-accent'
							: 'border-line bg-surface text-ink hover:border-line-strong',
						item.pending && !item.current && 'border-dashed text-muted'
					]}
				>
					{#if item.prefix}<span class="sr-only">{item.prefix}&nbsp;</span>{/if}{item.label}
				</a>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			</li>
		{/each}
	</ul>
</nav>
