<script lang="ts" module>
	export interface HeroChip {
		text: string;
		/** A small caption before the text ("TMDB" before a score). */
		label?: string;
		/** The score: bold, in the accent. */
		strong?: boolean;
		title?: string;
	}

	export interface PageSection {
		/** The element ID the link scrolls to. */
		id: string;
		label: string;
		/** A count shown at the end of the row (episodes, seasons). */
		count?: number;
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import Artwork from './Artwork.svelte';

	interface Props {
		title: string;
		year: string;
		posterUrl: string;
		backdropUrl: string;
		chips: HeroChip[];
		/** Listed under the poster when there are two or more. */
		sections: PageSection[];
		/** Label/value facts beside the poster (a Facts list). */
		facts: Snippet;
		/** Everything below: tagline, overview, seasons. */
		children: Snippet;
		/**
		 * The tracking actions: a right-hand column on desktop, a full-width
		 * row under the facts below that.
		 */
		actions?: Snippet;
	}

	let { title, year, posterUrl, backdropUrl, chips, sections, facts, children, actions }: Props =
		$props();

	// The hero and the body share one grid, so the title lines up with the
	// facts and the poster overlaps the hero in its own column. With
	// actions, desktop adds a third column for them.
	const grid = $derived([
		'mx-auto grid w-full max-w-7xl grid-cols-[6rem_minmax(0,1fr)] gap-x-4 px-4 sm:grid-cols-[8.5rem_minmax(0,1fr)] sm:gap-x-6 sm:px-6 lg:gap-x-8',
		actions ? 'lg:grid-cols-[11rem_minmax(0,1fr)_16rem]' : 'lg:grid-cols-[11rem_minmax(0,1fr)]'
	]);
</script>

<article>
	<!--
		The hero is a dark "screen" in both themes (see DESIGN.md): the
		backdrop, a scrim that fades to the screen colour at the bottom and
		left, and the title on top.
	-->
	<header class="relative isolate overflow-hidden bg-screen text-on-screen">
		{#if backdropUrl}
			<img
				src={backdropUrl}
				alt=""
				width="1280"
				height="720"
				fetchpriority="high"
				referrerpolicy="no-referrer"
				class="absolute inset-0 -z-10 size-full object-cover object-[50%_25%]"
			/>
			<div
				aria-hidden="true"
				class="absolute inset-0 -z-10 bg-linear-to-t from-screen from-2% via-screen/70 via-40% to-screen/5"
			></div>
			<div
				aria-hidden="true"
				class="absolute inset-0 -z-10 bg-linear-to-r from-screen/70 to-screen/0 to-70%"
			></div>
		{:else}
			<!-- No backdrop: a quiet ember glow instead of an empty band. -->
			<div
				aria-hidden="true"
				class="absolute inset-0 -z-10 bg-radial-[at_85%_0%] from-on-screen-accent/20 to-screen to-60%"
			></div>
		{/if}
		<div
			class={[...grid, backdropUrl ? 'pt-28 sm:pt-44 lg:pt-56' : 'pt-14 sm:pt-20 lg:pt-24', 'pb-4']}
		>
			<div class={['col-start-2 min-w-0', actions && 'lg:col-end-4']}>
				<h1
					class="text-2xl leading-tight font-bold tracking-tight text-balance [text-shadow:0_1px_12px_rgb(0_0_0/0.5)] sm:text-3xl lg:text-4xl"
				>
					{title}{#if year}&nbsp;<span class="font-normal text-on-screen-muted">{year}</span>{/if}
				</h1>
				{#if chips.length}
					<ul class="mt-2 flex flex-wrap items-center gap-1.5 text-xs sm:mt-3">
						{#each chips as chip (chip.text)}
							<li
								title={chip.title}
								class="inline-flex h-6 items-center gap-1.5 rounded-sm border border-on-screen/20 bg-screen/50 px-2 whitespace-nowrap tabular-nums"
							>
								{#if chip.label}<span class="text-on-screen-muted">{chip.label}</span>{/if}
								<span class={chip.strong ? 'font-bold text-on-screen-accent' : ''}>{chip.text}</span
								>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	</header>

	<!--
		With actions, a third, flexible row takes up the action column's (or
		poster's) extra height on desktop, so the overview doesn't move down
		when the column grows.
	-->
	<div class={[...grid, actions && 'lg:grid-rows-[auto_auto_1fr]', 'pb-12']}>
		<aside
			class={[
				actions ? 'lg:row-span-3' : 'lg:row-span-2',
				'relative z-10 col-start-1 row-start-1 -mt-16 self-start sm:-mt-24 lg:sticky lg:top-16 lg:-mt-36'
			]}
		>
			<Artwork
				src={posterUrl}
				kind="poster"
				label={title}
				class="rounded-md shadow-[0_10px_30px_rgb(0_0_0/0.45)] ring-1 ring-black/25"
			/>
			{#if sections.length > 1}
				<nav aria-label="Sections" class="mt-4 max-sm:hidden">
					<ul class="border-t border-line">
						{#each sections as section (section.id)}
							<li>
								<a
									href="#{section.id}"
									class="flex items-center justify-between gap-2 border-b border-line py-2 text-[0.6875rem] font-semibold tracking-wider text-muted uppercase transition-colors hover:text-ink focus-visible:-outline-offset-2"
								>
									{section.label}
									{#if section.count !== undefined}
										<span class="font-medium tabular-nums">{section.count}</span>
									{/if}
								</a>
							</li>
						{/each}
					</ul>
				</nav>
			{/if}
		</aside>

		<div id="overview" class="col-start-2 row-start-1 min-w-0 pt-3 sm:pt-4">
			{@render facts()}
		</div>

		{#if actions}
			<div
				class="col-span-2 mt-5 min-w-0 lg:sticky lg:top-16 lg:col-span-1 lg:col-start-3 lg:row-span-3 lg:row-start-1 lg:mt-0 lg:self-start lg:pt-4"
			>
				{@render actions()}
			</div>
		{/if}

		<div class="col-span-2 min-w-0 lg:col-span-1 lg:col-start-2">
			{@render children()}
		</div>
	</div>
</article>
