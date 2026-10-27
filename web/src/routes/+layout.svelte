<script lang="ts">
	import './layout.css';
	import favicon from '#lib/assets/favicon.svg';
	import { invalidate } from '$app/navigation';
	import { setUnauthenticatedHandler } from '#lib/api/client.js';
	import Button from '#lib/components/Button.svelte';
	import CardPage from '#lib/components/CardPage.svelte';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();

	// When an API call finds the session gone, re-run the session check,
	// which sends the user to /login and back here afterwards.
	setUnauthenticatedHandler(() => invalidate('app:session'));

	let retrying = $state(false);
	async function retry() {
		retrying = true;
		try {
			await invalidate('app:session');
		} finally {
			retrying = false;
		}
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if data.unreachable}
	<!-- The session check failed, so no page can be shown; see +layout.ts. -->
	<CardPage title="Can't reach watcht">
		<p class="text-sm text-ink">
			The watcht server didn't answer, or it ran into a problem. If it's restarting, try again in a
			moment.
		</p>
		<p class="mt-2 text-xs text-muted">
			{data.unreachable.status ? `Error ${data.unreachable.status}` : 'No connection'}
		</p>
		<div class="mt-5">
			<Button type="button" busy={retrying} onclick={retry}>Try again</Button>
		</div>
	</CardPage>
{:else}
	{@render children()}
{/if}
