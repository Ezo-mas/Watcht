<script lang="ts">
	import type { ResolvedPathname } from '$app/types';

	interface Crumb {
		label: string;
		/** A resolve()d path. */
		href: ResolvedPathname;
	}

	interface Props {
		/** The pages above this one, outermost first ("Breaking Bad", "Season 1"). */
		items: Crumb[];
		/** A note joined to the end of the pill ("Season finale"); not a link. */
		flag?: string;
	}

	let { items, flag = '' }: Props = $props();
</script>

<!--
	The way back up, after classic Trakt: a pill on the hero, above the
	title. It sits on the screen, so it uses the screen's colours in both
	themes, on a translucent screen fill so it reads over any backdrop; its
	links draw their focus ring in on-screen-accent, inside the pill. The
	flag (a premiere or finale) is joined on the end, inverted; on phones,
	where the hero's column is narrow, it wraps under the pill as a pill of
	its own, so the links keep their names. A long show name is cut first.
-->
<div class="flex max-w-full flex-wrap items-stretch gap-1.5 sm:flex-nowrap sm:gap-0">
	<nav
		aria-label="Breadcrumb"
		class={[
			'flex min-w-0 overflow-hidden rounded-full border border-on-screen/20 bg-screen/70 text-sm backdrop-blur-sm',
			flag && 'sm:rounded-r-none sm:border-r-0'
		]}
	>
		<ol class="flex min-w-0 items-stretch">
			{#each items as item, i (item.href)}
				<li class={['flex min-w-0 items-center', i > 0 && 'shrink-0']}>
					{#if i > 0}
						<span aria-hidden="true" class="text-on-screen-muted">/</span>
					{/if}
					<!-- The type makes callers pass a resolve()d path; the rule can't see through props. -->
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						href={item.href}
						data-sveltekit-preload-data="tap"
						title={item.label}
						class="block h-10 min-w-0 truncate rounded-full px-3 leading-10 text-on-screen underline-offset-2 hover:underline focus-visible:-outline-offset-2 focus-visible:outline-on-screen-accent sm:h-8 sm:leading-8"
						>{item.label}</a
					>
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				</li>
			{/each}
		</ol>
	</nav>
	{#if flag}
		<span
			class="flex h-7 shrink-0 items-center rounded-full bg-on-screen px-3 text-[0.8125rem] font-semibold whitespace-nowrap text-screen sm:h-auto sm:rounded-l-none"
			><span class="sr-only">,&nbsp;</span>{flag}</span
		>
	{/if}
</div>
