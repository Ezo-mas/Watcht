<script lang="ts" module>
	/** Why polling for the episodes stopped. */
	export type FetchProblem =
		{ kind: 'timeout' } | { kind: 'error'; message: string; retry: boolean };
</script>

<script lang="ts">
	import Button from './Button.svelte';
	import Notice from './Notice.svelte';

	interface Props {
		/** Undefined while still fetching (spinner and skeleton rows), else why it stopped. */
		problem?: FetchProblem;
		/** Skeleton rows to show while fetching (1 to 3). */
		rows: number;
		/** What hasn't arrived, for the timeout line ("this season's episodes"). */
		what: string;
		/** Starts polling again ("Check again", "Try again"). */
		onretry: () => void;
	}

	let { problem, rows, what, onretry }: Props = $props();

	const skeleton = $derived(Array.from({ length: Math.min(Math.max(rows, 1), 3) }, (_, i) => i));
</script>

<!--
	An episode list whose episodes the server is still fetching from TMDB
	(DESIGN.md, States: Pending): a spinner line over skeleton rows in the
	episode card's shape; or, once polling stops, why, with a way to retry.
-->
{#if !problem}
	<p role="status" class="flex items-center gap-2 py-2 text-sm text-ink">
		<span
			aria-hidden="true"
			class="size-3.5 animate-spin rounded-full border-2 border-accent border-r-transparent"
		></span>
		Fetching episodes from TMDB…
		<span class="text-muted max-sm:hidden">This takes a few seconds the first time.</span>
	</p>
	<ul aria-hidden="true" class="divide-y divide-line">
		{#each skeleton as i (i)}
			<li
				class="grid gap-3 py-4 sm:grid-cols-[13rem_minmax(0,1fr)] sm:gap-5 lg:grid-cols-[15rem_minmax(0,1fr)]"
			>
				<span class="flex flex-col">
					<span class="aspect-video animate-pulse rounded-t-sm bg-raised"></span>
					<span class="h-10 rounded-b-sm bg-strip sm:h-8"></span>
				</span>
				<span class="flex flex-col gap-2 pt-1">
					<span class="h-4 w-2/5 animate-pulse rounded-sm bg-raised"></span>
					<span class="h-3 w-24 animate-pulse rounded-sm bg-raised"></span>
					<span class="h-3 w-4/5 animate-pulse rounded-sm bg-raised"></span>
				</span>
			</li>
		{/each}
	</ul>
{:else if problem.kind === 'timeout'}
	<div class="flex flex-col items-start gap-3 py-3">
		<p class="text-sm text-ink">
			TMDB hasn't sent {what} yet. watcht keeps trying in the background.
		</p>
		<Button variant="quiet" onclick={onretry}>Check again</Button>
	</div>
{:else}
	<div class="flex flex-col items-start gap-3 py-3">
		<Notice tone="error" message={problem.message} />
		{#if problem.retry}
			<Button variant="quiet" onclick={onretry}>Try again</Button>
		{/if}
	</div>
{/if}
