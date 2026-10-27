<script lang="ts">
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>watcht</title>
</svelte:head>

<h1 class="text-xl font-semibold tracking-tight text-ink">
	Welcome, {data.session.user?.username}
</h1>
<p class="mt-1 text-sm text-muted">The dashboard arrives with tracking (M4).</p>
