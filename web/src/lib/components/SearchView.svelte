<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onDestroy, tick, type Snippet } from 'svelte';
	import { search } from '#lib/api/catalog.js';
	import type { SearchResult } from '#lib/api/types.js';
	import { catalogMessages, errorMessage } from '#lib/errors.js';
	import {
		Searcher,
		initialSearchState,
		key,
		minQueryLength,
		normalizeQuery,
		type SearchState
	} from '#lib/search.js';
	import Button from './Button.svelte';
	import Notice from './Notice.svelte';
	import SearchResultRow from './SearchResultRow.svelte';

	interface Props {
		/**
		 * overlay: a combobox; arrow keys choose a result and Enter opens it.
		 * page: a plain search box over a list of links.
		 */
		variant: 'overlay' | 'page';
		query?: string;
		/** The input, so the caller can focus or select it. */
		input?: HTMLInputElement;
		/** Overlay: Enter with no result chosen (the caller opens /search). */
		onsubmit?: (query: string) => void;
		/** Overlay: a result is about to open (the caller closes). */
		onopen?: () => void;
		/** A control at the end of the input row (the overlay's close button). */
		trailing?: Snippet;
	}

	let {
		variant,
		query = $bindable(''),
		input = $bindable(),
		onsubmit,
		onopen,
		trailing
	}: Props = $props();

	const id = $props.id();
	const overlay = $derived(variant === 'overlay');

	let st = $state<SearchState>(initialSearchState);
	const searcher = new Searcher(
		(q, page) => search(q, page),
		(s) => (st = s)
	);
	onDestroy(() => searcher.dispose());

	// The first query (from the URL, or the overlay's last one) runs at
	// once; typing after that waits for a pause.
	let started = false;
	$effect(() => {
		const q = query;
		if (started) return searcher.set(q);
		started = true;
		searcher.now(q);
	});

	type Filter = 'all' | 'show' | 'movie';
	const filters: { value: Filter; label: string }[] = [
		{ value: 'all', label: 'Shows & movies' },
		{ value: 'show', label: 'Shows' },
		{ value: 'movie', label: 'Movies' }
	];
	let filter = $state<Filter>('all');
	const visible = $derived(
		filter === 'all' ? st.results : st.results.filter((r) => r.media_type === filter)
	);

	// The chosen result is kept by key, so it stays chosen while more load
	// and is dropped when it's filtered out or a new search replaces it.
	// Focusing the input (opening the overlay again) starts with none.
	let activeKey = $state('');
	const active = $derived(visible.findIndex((r) => key(r) === activeKey));
	const optionId = (i: number) => `${id}-option-${i}`;

	const short = $derived(st.status === 'idle' && st.query !== '');
	const hasMore = $derived(st.status === 'done' && st.page < st.totalPages);
	// The results list is on the page (the last branch below); the combobox
	// only points at it then.
	const listShown = $derived(
		st.status !== 'error' && st.query !== '' && !short && st.results.length > 0
	);

	const summary = $derived.by(() => {
		if (st.status === 'loading') return 'Searching…';
		if (st.status !== 'done') return '';
		const n = st.results.length;
		if (n === 0) return 'No results';
		return `${n} ${n === 1 ? 'result' : 'results'}${hasMore ? ' so far' : ''}`;
	});

	function open(result: SearchResult) {
		onopen?.();
		const id = String(result.tmdb_id);
		void goto(
			result.media_type === 'movie'
				? resolve('/(app)/movies/[id]', { id })
				: resolve('/(app)/shows/[id]', { id })
		);
	}

	async function onkeydown(event: KeyboardEvent) {
		if (event.isComposing) return;
		if (event.key === 'Enter') {
			event.preventDefault();
			if (overlay && active >= 0) return open(visible[active]);
			const q = normalizeQuery(query);
			if ([...q].length < minQueryLength) return;
			if (onsubmit) onsubmit(q);
			else searcher.now(q);
			return;
		}
		if (!overlay || (event.key !== 'ArrowDown' && event.key !== 'ArrowUp')) return;
		event.preventDefault();
		if (visible.length === 0) return;
		const next = Math.min(active + (event.key === 'ArrowDown' ? 1 : -1), visible.length - 1);
		activeKey = next < 0 ? '' : key(visible[next]);
		await tick();
		if (next >= 0) document.getElementById(optionId(next))?.scrollIntoView({ block: 'nearest' });
	}
</script>

<div class={['flex flex-col', overlay && 'min-h-0 flex-1']}>
	<div class={['flex flex-col gap-3', overlay ? 'border-b border-line/70 p-3 sm:p-4' : '']}>
		<div class="flex items-center gap-2">
			<div class="relative min-w-0 flex-1">
				<label for="{id}-input" class="sr-only">Search shows and movies</label>
				<svg
					aria-hidden="true"
					viewBox="0 0 24 24"
					class="pointer-events-none absolute top-1/2 left-3.5 size-5 -translate-y-1/2 text-muted"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg
				>
				<input
					id="{id}-input"
					bind:this={input}
					bind:value={query}
					{onkeydown}
					onfocus={() => (activeKey = '')}
					type="text"
					role={overlay ? 'combobox' : undefined}
					aria-expanded={overlay ? listShown && visible.length > 0 : undefined}
					aria-controls={overlay && listShown ? `${id}-list` : undefined}
					aria-autocomplete={overlay ? 'list' : undefined}
					aria-activedescendant={overlay && active >= 0 ? optionId(active) : undefined}
					aria-describedby="{id}-summary"
					autocomplete="off"
					autocapitalize="off"
					spellcheck="false"
					enterkeyhint="search"
					maxlength={100}
					placeholder="Search for a show or movie"
					class="h-12 w-full rounded-lg border-2 border-line-strong bg-field/80 pr-10 pl-11 text-base text-ink transition-colors placeholder:text-muted focus:border-accent focus-visible:outline-none"
				/>
				{#if st.status === 'loading'}
					<span
						aria-hidden="true"
						class="absolute top-1/2 right-3.5 size-4 -translate-y-1/2 animate-spin rounded-full border-2 border-muted border-r-transparent"
					></span>
				{/if}
			</div>
			{@render trailing?.()}
		</div>

		<div class="flex flex-wrap items-center gap-x-3 gap-y-2">
			<div role="group" aria-label="Show" class="flex flex-wrap gap-1.5">
				{#each filters as f (f.value)}
					<button
						type="button"
						aria-pressed={filter === f.value}
						onclick={() => (filter = f.value)}
						class="h-8 rounded-md border border-line bg-surface/50 px-3 text-sm font-medium text-muted transition-colors hover:border-line-strong hover:text-ink aria-pressed:border-accent aria-pressed:bg-accent/15 aria-pressed:text-ink"
					>
						{f.label}
					</button>
				{/each}
			</div>
			<p id="{id}-summary" role="status" class="ml-auto text-xs text-muted tabular-nums">
				{summary}
			</p>
		</div>
	</div>

	<div
		class={overlay ? 'min-h-0 flex-1 overflow-y-auto overscroll-contain p-2 sm:p-3' : '-mx-2 mt-4'}
	>
		{#if st.status === 'error'}
			<div class="flex flex-col items-start gap-3 p-2">
				<Notice tone="error" message={errorMessage(st.error, catalogMessages)} />
				<Button variant="quiet" onclick={() => searcher.now(query)}>Try again</Button>
			</div>
		{:else if st.query === '' || short}
			<p class="px-2 py-6 text-sm text-muted">
				{short
					? 'Type one more character to search.'
					: 'Find a show or movie on TMDB by its title, in any language TMDB knows it by.'}
			</p>
		{:else if st.status === 'done' && st.results.length === 0}
			<div class="px-2 py-6">
				<p class="text-sm font-medium text-ink">No shows or movies match “{st.query}”.</p>
				<p class="mt-1 text-sm text-muted">Check the spelling, or try the original title.</p>
			</div>
		{:else if st.status === 'loading' && st.results.length === 0}
			<ul aria-hidden="true" class="flex flex-col gap-1">
				{#each [0, 1, 2] as i (i)}
					<li class="flex items-center gap-3 px-2 py-1.5">
						<span
							class={[
								'aspect-[2/3] animate-pulse rounded-sm bg-raised',
								overlay ? 'w-10' : 'w-14 sm:w-16'
							]}
						></span>
						<span class="flex flex-1 flex-col gap-2">
							<span class="h-3.5 w-1/3 animate-pulse rounded-sm bg-raised"></span>
							<span class="h-3 w-16 animate-pulse rounded-sm bg-raised"></span>
						</span>
					</li>
				{/each}
			</ul>
		{:else if listShown}
			<ul
				id="{id}-list"
				role={overlay ? 'listbox' : undefined}
				aria-label="Results"
				aria-busy={st.status === 'loading' || undefined}
				class={[
					'flex flex-col gap-0.5 transition-opacity',
					st.status === 'loading' && 'opacity-60'
				]}
			>
				{#each visible as result, i (key(result))}
					<li role={overlay ? 'none' : undefined}>
						<SearchResultRow
							{result}
							{variant}
							id={overlay ? optionId(i) : undefined}
							active={overlay && i === active}
							onclick={onopen}
						/>
					</li>
				{/each}
			</ul>
			{#if visible.length === 0}
				<p class="px-2 py-4 text-sm text-muted">
					None of {hasMore ? 'these' : 'the'} results {filter === 'show'
						? 'are shows'
						: 'are movies'}.
				</p>
			{/if}
			{#if hasMore || st.more === 'error'}
				<div class="flex flex-col items-start gap-2 px-2 pt-3 pb-1">
					<Notice
						tone="error"
						message={st.more === 'error' ? errorMessage(st.error, catalogMessages) : ''}
					/>
					<Button variant="quiet" busy={st.more === 'loading'} onclick={() => searcher.more()}>
						{st.more === 'error' ? 'Try loading more again' : 'Load more results'}
					</Button>
				</div>
			{/if}
		{/if}
	</div>
</div>
