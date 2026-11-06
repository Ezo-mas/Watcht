<script lang="ts">
	import SourceMark from './SourceMark.svelte';
	import TitleSection from './TitleSection.svelte';

	interface Props {
		/** TMDB's vote average (0–10) and vote count; the row is left out with no votes. */
		tmdb: { average: number; votes: number };
	}

	let { tmdb }: Props = $props();

	const hasTmdb = $derived(tmdb.votes > 0 && tmdb.average > 0);

	// Household ratings and activity aren't recorded yet: the shapes are
	// shown so the layout is settled, with no numbers in them.
	const figures = [
		{ label: 'Average', caption: 'Out of 10' },
		{ label: 'Ratings', caption: 'Given in your household' },
		{ label: 'Most given', caption: 'The most common score' },
		{ label: 'Liked it', caption: 'Rated 7 or higher' }
	];
	const tiles = [
		{ label: 'Watchers', caption: 'People who watched it' },
		{ label: 'Plays', caption: 'Every play, rewatches too' },
		{ label: 'Watchlists', caption: 'People who want to watch it' }
	];
</script>

{#snippet empty()}
	<!-- A value that isn't recorded yet: a quiet shape, named for screen readers. -->
	<span aria-hidden="true" class="block h-7 w-16 rounded-sm bg-raised"></span>
	<span class="sr-only">Not available yet</span>
{/snippet}

<TitleSection id="stats" title="Stats">
	<!--
		Other sites' scores ("second opinions", after classic Trakt), straight
		under the section heading: no sub-heading, the list names itself.
	-->
	{#if hasTmdb}
		<ul
			aria-label="Ratings from other sites"
			class="mt-4 divide-y divide-line border-b border-line"
		>
			<li
				class="grid grid-cols-[minmax(0,1fr)_3.5rem] items-center gap-x-4 gap-y-2 py-3 sm:grid-cols-[11rem_3.5rem_minmax(0,1fr)]"
			>
				<div class="flex min-w-0 items-center gap-3">
					<span
						class="grid size-10 shrink-0 place-items-center rounded-md border border-line bg-surface"
					>
						<SourceMark site="tmdb" />
					</span>
					<div class="min-w-0">
						<p class="text-sm font-semibold text-ink">TMDB</p>
						<p class="text-xs text-muted tabular-nums">
							{tmdb.votes.toLocaleString('en')} votes
						</p>
					</div>
				</div>
				<p
					class="grid h-8 place-items-center rounded-md bg-raised text-sm font-bold text-ink tabular-nums"
				>
					<span aria-hidden="true">{tmdb.average.toFixed(1)}</span>
					<span class="sr-only">{tmdb.average.toFixed(1)} out of 10</span>
				</p>
				<div
					aria-hidden="true"
					class="col-span-2 h-2 overflow-hidden rounded-full bg-raised sm:col-span-1"
				>
					<div
						class="h-full rounded-full bg-accent"
						style:width="{Math.min(100, tmdb.average * 10)}%"
					></div>
				</div>
			</li>
		</ul>
	{/if}
	<p class="mt-3 text-sm text-muted">
		{hasTmdb ? 'More sites’ scores are' : 'Scores from other sites are'} coming in a later update.
	</p>

	<!-- The household's own ratings: a 1–10 distribution and four figures. -->
	<h3 class="mt-10 text-base font-semibold text-ink">Household ratings</h3>
	<p class="mt-1 text-sm text-muted">
		Your household’s ratings of it are coming in a later update.
	</p>
	<!-- Ten empty bar slots on one baseline, labelled 1–10: no counts until ratings exist. -->
	<div aria-hidden="true" class="mt-5">
		<div
			class="grid h-20 grid-cols-10 items-end gap-1.5 border-b border-line-strong/60 sm:h-28 sm:gap-3"
		>
			{#each { length: 10 }, i (i)}
				<div class="mx-auto h-full w-3/5 rounded-t-sm bg-raised/60"></div>
			{/each}
		</div>
		<div class="mt-2 grid grid-cols-10 gap-1.5 sm:gap-3">
			{#each { length: 10 }, i (i)}
				<span
					class="flex items-center justify-center gap-1 text-xs font-semibold text-muted tabular-nums"
				>
					<svg viewBox="0 0 16 16" class="size-3 max-sm:hidden" fill="currentColor"
						><path
							d="M8 14s-5.5-3.3-5.5-7.2A3 3 0 0 1 8 4.6a3 3 0 0 1 5.5 2.2C13.5 10.7 8 14 8 14z"
						/></svg
					>
					{i + 1}
				</span>
			{/each}
		</div>
	</div>
	<dl class="mt-8 grid grid-cols-2 gap-x-6 gap-y-6 lg:grid-cols-4">
		{#each figures as figure (figure.label)}
			<div class="flex flex-col gap-2 border-t-2 border-line pt-3">
				<dt class="text-[0.6875rem] font-semibold tracking-wider text-muted uppercase">
					{figure.label}
				</dt>
				<dd>{@render empty()}</dd>
				<dd class="text-xs text-muted">{figure.caption}</dd>
			</div>
		{/each}
	</dl>

	<h3 class="mt-10 text-base font-semibold text-ink">Household activity</h3>
	<p class="mt-1 text-sm text-muted">Who watched it, and how often, is coming in a later update.</p>
	<dl class="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
		{#each tiles as tile (tile.label)}
			<div class="flex flex-col gap-2 rounded-md border border-line bg-surface p-4">
				<dt class="text-[0.6875rem] font-semibold tracking-wider text-muted uppercase">
					{tile.label}
				</dt>
				<dd>{@render empty()}</dd>
				<dd class="text-xs text-muted">{tile.caption}</dd>
			</div>
		{/each}
	</dl>
</TitleSection>
