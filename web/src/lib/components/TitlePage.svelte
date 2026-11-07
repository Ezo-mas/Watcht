<script lang="ts" module>
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
	import type { TitleLink } from '#lib/media.js';
	import { currentSection } from '#lib/scroll-spy.js';
	import Artwork from './Artwork.svelte';
	import ExternalLinks from './ExternalLinks.svelte';
	import RatingsBand, { type Rating } from './RatingsBand.svelte';
	import SectionNav from './SectionNav.svelte';

	interface Props {
		title: string;
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
	}

	let {
		title,
		year,
		posterUrl,
		backdropUrl,
		ratings,
		links,
		sections,
		facts,
		children,
		actions,
		below
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
		backdrop, a scrim that fades to the screen colour at the bottom and
		left, the title, and the ratings band. It starts under the
		translucent header, so its top padding clears it.
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
				class="absolute inset-0 -z-10 bg-linear-to-t from-screen from-2% via-screen/60 via-40% to-screen/5"
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
			class={[
				...grid,
				backdropUrl ? 'pt-40 sm:pt-60 lg:pt-72 xl:pt-80' : 'pt-24 sm:pt-28 lg:pt-32',
				ratings.length ? 'pb-4' : 'pb-6'
			]}
		>
			<div class={['col-start-2 min-w-0', actions && 'lg:col-end-4']}>
				<h1
					class="text-2xl leading-tight font-bold tracking-tight text-balance [text-shadow:0_1px_12px_rgb(0_0_0/0.5)] sm:text-3xl lg:text-4xl"
				>
					{title}{#if year}&nbsp;<span class="font-normal text-on-screen-muted">{year}</span>{/if}
				</h1>
			</div>
		</div>
		{#if ratings.length}
			<div class="bg-screen/55 backdrop-blur-sm">
				<div class={grid}>
					<div class={['col-start-2 min-w-0', actions && 'lg:col-end-4']}>
						<RatingsBand {ratings} />
					</div>
				</div>
			</div>
		{/if}
	</header>

	<!--
		Rows (lg): facts | overview | a flexible row | the sections below.
		The poster column spans them all, so it can stay in view (sticky
		from sm) as the page scrolls. The actions span the first three
		rows; the flexible one takes up their extra height, so the overview
		doesn't move down when the column grows.
	-->
	<div class={[...grid, 'pb-16 lg:grid-rows-[auto_auto_1fr_auto]']}>
		<aside
			aria-label="Poster and sections"
			bind:clientHeight={asideHeight}
			style:--stick-top="{stickTop}px"
			class="relative z-10 col-start-1 row-start-1 -mt-20 self-start sm:sticky sm:top-(--stick-top) sm:row-span-4 sm:-mt-28 lg:-mt-40 xl:-mt-44"
		>
			<Artwork
				src={posterUrl}
				kind="poster"
				label={title}
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
				<ExternalLinks {title} {links} />
			</div>
		</aside>

		<div id="overview" class="col-start-2 row-start-1 min-w-0 pt-4 sm:pt-5">
			<h2 class="sr-only">Overview</h2>
			{@render facts()}
		</div>

		<div class="col-span-2 row-start-2 min-w-0 sm:col-span-1 sm:col-start-2">
			{@render children()}
		</div>

		{#if actions}
			<div
				class="col-span-2 row-start-3 mt-8 min-w-0 sm:col-span-1 sm:col-start-2 lg:col-start-3 lg:row-span-3 lg:row-start-1 lg:mt-0 lg:self-start lg:pt-5"
			>
				{@render actions()}
			</div>
		{/if}

		{#if below}
			<div
				class="col-span-2 row-start-4 min-w-0 sm:col-span-1 sm:col-start-2 lg:col-span-2 lg:col-start-2"
			>
				{#if showNav}
					<SectionNav
						{sections}
						{current}
						onpick={pick}
						variant="strip"
						class="mt-10 -mb-4 sm:hidden"
					/>
				{/if}
				{@render below()}
			</div>
		{/if}
	</div>
</article>
