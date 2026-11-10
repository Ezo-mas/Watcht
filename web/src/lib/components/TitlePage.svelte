<script lang="ts" module>
	import type { ResolvedPathname } from '$app/types';

	export interface PageSection {
		/** The element ID the link scrolls to. */
		id: string;
		/** With its count first when it has one ("7 Seasons", "22 Episodes"; countLabel). */
		label: string;
	}

	/** A previous or next page, as an arrow on the hero's side. */
	export interface HeroLink {
		/** A resolve()d path. */
		href: ResolvedPathname;
		/** Its accessible name and tooltip ("Next episode: S01E04 Cancer Man"). */
		label: string;
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { TitleLink } from '#lib/media.js';
	import { currentSection } from '#lib/scroll-spy.js';
	import Artwork from './Artwork.svelte';
	import ExternalLinks from './ExternalLinks.svelte';
	import RatingsBand, { type Rating } from './RatingsBand.svelte';
	import SectionNav from './SectionNav.svelte';

	interface Props {
		title: string;
		/** An episode's code ("S01E22"), shown before the title in the codes' accent. */
		code?: string;
		/**
		 * The full name, for the poster placeholder and the link labels, when
		 * the title alone is ambiguous ("Breaking Bad Season 1"); default the title.
		 */
		name?: string;
		year: string;
		posterUrl: string;
		backdropUrl: string;
		/** Scores for the band along the bottom of the hero; no band when empty. */
		ratings: Rating[];
		/** The title on other sites, as icon links under the poster. */
		links: TitleLink[];
		/**
		 * The section list under the poster (shown with two or more). The
		 * first is the overview.
		 */
		sections: PageSection[];
		/** Above the title on the hero: the breadcrumb pill (Breadcrumbs). */
		crumbs?: Snippet;
		/** The previous and next season or episode, as arrows on the hero's sides; none at the ends. */
		prev?: HeroLink;
		next?: HeroLink;
		/**
		 * Right under the hero, above the facts: the strip of sibling pages
		 * (PickerStrip), as wide as the facts and actions together.
		 */
		picker?: Snippet;
		/** Label/value facts beside the poster (a Facts list). */
		facts: Snippet;
		/** Under the facts, beside the actions: tagline, overview, seasons. */
		children: Snippet;
		/**
		 * The tracking actions: a right-hand column on desktop, under the
		 * overview below that.
		 */
		actions?: Snippet;
		/** Sections under everything, as wide as the overview and actions together. */
		below?: Snippet;
		/**
		 * The last section, under those: the full width of the page, under
		 * the poster column too, which stops sticking where it starts.
		 */
		last?: Snippet;
	}

	let {
		title,
		code = '',
		name,
		year,
		posterUrl,
		backdropUrl,
		ratings,
		links,
		sections,
		crumbs,
		prev,
		next,
		picker,
		facts,
		children,
		actions,
		below,
		last
	}: Props = $props();

	// The hero and the body share one grid, so the title lines up with the
	// facts and the poster overlaps the hero in its own column. With
	// actions, desktop adds a third column for them.
	const grid = $derived([
		'mx-auto grid w-full max-w-7xl grid-cols-[7rem_minmax(0,1fr)] gap-x-4 px-4 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-x-6 sm:px-6 lg:gap-x-8 xl:gap-x-10',
		actions
			? 'lg:grid-cols-[12rem_minmax(0,1fr)_16rem] xl:grid-cols-[14rem_minmax(0,1fr)_18rem]'
			: 'lg:grid-cols-[12rem_minmax(0,1fr)] xl:grid-cols-[14rem_minmax(0,1fr)]'
	]);

	const showNav = $derived(sections.length > 1);
	const fullName = $derived(name ?? title);

	// Where each part of the body sits. With a picker it takes the first
	// row (beside the poster, and over the actions on lg), so every other
	// part moves down one; the poster spans the picker and the facts on
	// phones, so the facts still sit beside it. The poster column spans
	// every row but the last section's, which runs under it.
	const cells = $derived(
		picker
			? {
					rows: 'lg:grid-rows-[auto_auto_auto_1fr_auto_auto]',
					aside: 'row-span-2 sm:row-span-5',
					facts: 'row-start-2',
					children: 'row-start-3',
					actions: 'row-start-4 lg:row-start-2',
					below: 'row-start-5',
					last: 'row-start-6'
				}
			: {
					rows: 'lg:grid-rows-[auto_auto_1fr_auto_auto]',
					aside: 'sm:row-span-4',
					facts: 'row-start-1',
					children: 'row-start-2',
					actions: 'row-start-3 lg:row-start-1',
					below: 'row-start-4',
					last: 'row-start-5'
				}
	);

	// Breadcrumb pages align the poster with the title block, even when it wraps.
	let titleHeight = $state(0);

	// The poster column sticks 4 rem from the top; when it's taller than
	// the window (a short laptop screen), it sticks by its bottom edge
	// instead, so the section list and links stay reachable.
	let asideHeight = $state(0);
	let viewportHeight = $state(0);
	const stickTop = $derived(
		asideHeight && viewportHeight ? Math.min(64, viewportHeight - asideHeight - 16) : 64
	);

	// Scroll-spy: the section in view is highlighted in the list. A link
	// that was just used stays highlighted while the page scrolls to it
	// (a short section at the end can't reach the top); the next scroll
	// after that one has settled is the reader's own and drops it.
	let spied = $state<string>();
	let picked = $state<string | null>(null);
	const current = $derived(picked ?? spied ?? sections[0]?.id);
	let settled = true;
	let settleTimer: ReturnType<typeof setTimeout> | undefined;

	function settleSoon() {
		clearTimeout(settleTimer);
		settleTimer = setTimeout(() => (settled = true), 200);
	}

	function pick(id: string) {
		picked = id;
		settled = false;
		settleSoon();
	}

	$effect(() => {
		if (!showNav) return;
		const ids = sections.map((s) => s.id);
		let frame = 0;
		function measure() {
			frame = 0;
			const tops = [];
			for (const id of ids) {
				const el = document.getElementById(id);
				if (el) tops.push({ id, top: el.getBoundingClientRect().top });
			}
			const root = document.documentElement;
			const atBottom = window.innerHeight + window.scrollY >= root.scrollHeight - 2;
			// The line sits a third of the way down, well below the header.
			spied = currentSection(tops, window.innerHeight / 3, atBottom, window.innerHeight);
		}
		function onscroll() {
			if (picked !== null) {
				if (settled) picked = null;
				else settleSoon();
			}
			if (!frame) frame = requestAnimationFrame(measure);
		}
		function onresize() {
			if (!frame) frame = requestAnimationFrame(measure);
		}
		measure();
		window.addEventListener('scroll', onscroll, { passive: true });
		window.addEventListener('resize', onresize, { passive: true });
		return () => {
			cancelAnimationFrame(frame);
			clearTimeout(settleTimer);
			window.removeEventListener('scroll', onscroll);
			window.removeEventListener('resize', onresize);
		};
	});
</script>

<svelte:window bind:innerHeight={viewportHeight} />

<article>
	<!--
		The hero is a dark "screen" in both themes (see DESIGN.md): the
		backdrop in full colour, and at the bottom the title and the ratings
		band over a short fade to the screen colour, which keeps their text
		readable over the brightest picture (maintainer decision). It starts
		under the translucent header, so the space above the title clears it.
		The arrows to the previous and next season or episode sit on its
		sides, in the middle of the picture above the title.
	-->
	<header
		class={[
			'relative isolate flex flex-col overflow-hidden bg-screen text-on-screen',
			backdropUrl
				? 'min-h-96 sm:min-h-[32rem] lg:min-h-[min(42rem,78vh)]'
				: 'min-h-56 sm:min-h-64 lg:min-h-72'
		]}
	>
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
		{:else}
			<!-- No backdrop: a quiet ember glow instead of an empty band. -->
			<div
				aria-hidden="true"
				class="absolute inset-0 -z-10 bg-radial-[at_85%_0%] from-on-screen-accent/20 to-screen to-60%"
			></div>
		{/if}

		<!-- The picture's free space, under the header: the arrows, centred in it. -->
		<div class="flex flex-1 items-center justify-between gap-4 px-4 pt-12 pb-2 sm:px-6">
			{#if prev}{@render arrow(prev, 'prev')}{:else}<span></span>{/if}
			{#if next}{@render arrow(next, 'next')}{/if}
		</div>

		<div>
			{#if backdropUrl}
				<!-- The fade's soft top edge, above the title. -->
				<div
					aria-hidden="true"
					class="h-16 bg-linear-to-t from-screen/65 to-screen/0 sm:h-24"
				></div>
			{/if}
			<div
				bind:clientHeight={titleHeight}
				class={backdropUrl && 'bg-linear-to-t from-screen/90 to-screen/65'}
			>
				<div class={[...grid, ratings.length ? 'pb-4' : 'pb-6']}>
					<div class={['col-start-2 min-w-0', actions && 'lg:col-end-4']}>
						{#if crumbs}
							<div class="mb-2 sm:mb-3">{@render crumbs()}</div>
						{/if}
						<h1
							class="text-2xl leading-tight font-bold tracking-tight text-balance [text-shadow:0_1px_12px_rgb(0_0_0/0.5)] sm:text-3xl lg:text-4xl"
						>
							<!-- The code stays on the title's first line. -->
							{#if code}<span class="text-on-screen-accent tabular-nums">{code}</span
								>&nbsp;{/if}{title}{#if year}&nbsp;<span class="font-normal text-on-screen-muted"
									>{year}</span
								>{/if}
						</h1>
					</div>
				</div>
				{#if ratings.length}
					<div class={grid}>
						<div class={['col-start-2 min-w-0', actions && 'lg:col-end-4']}>
							<RatingsBand {ratings} />
						</div>
					</div>
				{/if}
			</div>
		</div>
	</header>

	<!--
		Rows (lg): facts | overview | a flexible row | the sections below |
		the last section. The poster column spans all but the last, so it
		stays in view (sticky from sm) as the page scrolls, and stops where
		the last section, as wide as the page, starts under it. The actions
		span the first three rows; the flexible one takes up their extra
		height, so the overview doesn't move down when the column grows.
	-->
	<div class={[...grid, 'pb-16', cells.rows]}>
		<!--
			The poster column's own cell, spanning every row but the last
			section's: the sticky aside inside it stops where that section
			starts (a sticky grid item itself would be held by the whole grid).
		-->
		<div
			class={[
				'relative z-10 col-start-1 row-start-1 -mt-20 sm:-mt-28 lg:-mt-40 xl:-mt-44',
				cells.aside
			]}
			style:margin-top={crumbs && titleHeight ? `-${titleHeight}px` : undefined}
		>
			<aside
				aria-label="Poster and sections"
				bind:clientHeight={asideHeight}
				style:--stick-top="{stickTop}px"
				class="sm:sticky sm:top-(--stick-top)"
			>
				<Artwork
					src={posterUrl}
					kind="poster"
					label={fullName}
					class="rounded-md shadow-[0_12px_32px_rgb(0_0_0/0.45)] ring-1 ring-black/25"
				/>
				{#if showNav}
					<SectionNav
						{sections}
						{current}
						onpick={pick}
						variant="column"
						class="mt-6 max-sm:hidden"
					/>
				{/if}
				<div class="mt-4 sm:mt-6">
					<ExternalLinks title={fullName} {links} />
				</div>
			</aside>
		</div>

		{#if picker}
			<div class={['col-start-2 row-start-1 min-w-0 pt-4 sm:pt-5', actions && 'lg:col-end-4']}>
				{@render picker()}
			</div>
		{/if}

		<div id="overview" class={['col-start-2 min-w-0 pt-4 sm:pt-5', cells.facts]}>
			<h2 class="sr-only">Overview</h2>
			{@render facts()}
		</div>

		<div class={['col-span-2 min-w-0 sm:col-span-1 sm:col-start-2', cells.children]}>
			{@render children()}
		</div>

		{#if actions}
			<div
				class={[
					'col-span-2 mt-8 min-w-0 sm:col-span-1 sm:col-start-2 lg:col-start-3 lg:row-span-3 lg:mt-0 lg:self-start lg:pt-5',
					cells.actions
				]}
			>
				{@render actions()}
			</div>
		{/if}

		{#if below}
			<div
				class={[
					'col-span-2 min-w-0 sm:col-span-1 sm:col-start-2 lg:col-span-2 lg:col-start-2',
					cells.below
				]}
			>
				{@render phoneNav()}
				{@render below()}
			</div>
		{/if}

		{#if last}
			<div class={['col-span-full min-w-0', cells.last]}>
				{#if !below}{@render phoneNav()}{/if}
				{@render last()}
			</div>
		{/if}
	</div>
</article>

<!-- Phones: the section list as a strip of links, just above the first section. -->
{#snippet phoneNav()}
	{#if showNav}
		<SectionNav {sections} {current} onpick={pick} variant="strip" class="mt-10 -mb-4 sm:hidden" />
	{/if}
{/snippet}

<!--
	A round arrow on the hero's side, after classic Trakt: the screen's
	translucent fill (as the breadcrumb pill) so it reads over any picture,
	and its focus ring inside it in on-screen-accent.
-->
{#snippet arrow(link: HeroLink, side: 'prev' | 'next')}
	<!-- The type makes callers pass a resolve()d path; the rule can't see through props. -->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		href={link.href}
		aria-label={link.label}
		title={link.label}
		data-sveltekit-preload-data="tap"
		class="grid size-10 shrink-0 place-items-center rounded-full border border-on-screen/20 bg-screen/60 text-on-screen backdrop-blur-sm transition-colors hover:bg-screen/85 focus-visible:-outline-offset-4 focus-visible:outline-on-screen-accent sm:size-11"
	>
		<svg
			aria-hidden="true"
			viewBox="0 0 16 16"
			class="size-4 sm:size-5"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			stroke-linecap="round"
			stroke-linejoin="round"
			><path d={side === 'prev' ? 'M10 3.5 5.5 8l4.5 4.5' : 'M6 3.5 10.5 8 6 12.5'} /></svg
		>
	</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
{/snippet}
