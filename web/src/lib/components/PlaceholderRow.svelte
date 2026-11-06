<script lang="ts">
	// One row of empty tiles in the shape of content that comes later
	// (people, videos, posters): static shapes on `raised`, with no names
	// or numbers (DESIGN.md, Placeholders). The grid fills the width and
	// keeps only its first row, so the row is full at every width.
	interface Props {
		shape: 'person' | 'video' | 'poster';
	}

	let { shape }: Props = $props();

	const columns = {
		person:
			'grid-cols-[repeat(auto-fill,minmax(6.5rem,1fr))] sm:grid-cols-[repeat(auto-fill,minmax(7.5rem,1fr))]',
		video:
			'grid-cols-[repeat(auto-fill,minmax(9.5rem,1fr))] sm:grid-cols-[repeat(auto-fill,minmax(13rem,1fr))]',
		poster:
			'grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] sm:grid-cols-[repeat(auto-fill,minmax(10.5rem,1fr))]'
	};
	// Enough to fill the widest row; the rest are clipped.
	const count = { person: 10, video: 5, poster: 8 };
</script>

<ul
	aria-hidden="true"
	class={['mt-5 grid auto-rows-[0] grid-rows-1 gap-x-3 overflow-hidden sm:gap-x-4', columns[shape]]}
>
	{#each { length: count[shape] }, i (i)}
		<li class="flex flex-col gap-2">
			<div
				class={[
					'grid place-items-center rounded-sm bg-raised text-muted/50',
					shape === 'video' ? 'aspect-video' : 'aspect-[2/3]'
				]}
			>
				<svg
					viewBox="0 0 24 24"
					class="size-8"
					fill="none"
					stroke="currentColor"
					stroke-width="1.6"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					{#if shape === 'person'}
						<circle cx="12" cy="8.5" r="3.5" /><path d="M5 20c.8-3.6 3.6-5.5 7-5.5s6.2 1.9 7 5.5" />
					{:else if shape === 'video'}
						<circle cx="12" cy="12" r="8.5" /><path d="M10.25 8.75v6.5L15.5 12z" />
					{:else}
						<rect x="3" y="4" width="18" height="16" rx="2" /><path
							d="M7 4v16M17 4v16M3 8h4M3 12h4M3 16h4M17 8h4M17 12h4M17 16h4"
						/>
					{/if}
				</svg>
			</div>
			<div class="flex flex-col items-center gap-1.5 pt-0.5 pb-1">
				<span class="h-3 w-3/4 rounded-sm bg-raised"></span>
				{#if shape !== 'video'}<span class="h-2.5 w-1/2 rounded-sm bg-raised/70"></span>{/if}
			</div>
		</li>
	{/each}
</ul>
