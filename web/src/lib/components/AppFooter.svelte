<script lang="ts">
	import { resolve } from '$app/paths';
</script>

<!--
	A short credit, as Letterboxd does. TMDB's logo and its full notice are
	on the About page (TmdbAttribution); both must stay.
-->
<footer class="border-t border-line">
	<div
		class="mx-auto flex w-full max-w-7xl flex-wrap items-center justify-between gap-x-6 gap-y-1 px-4 py-4 text-xs text-muted sm:px-6"
	>
		<p>
			Film and TV data from
			<a
				href="https://www.themoviedb.org/"
				target="_blank"
				rel="noopener noreferrer"
				class="rounded-sm text-accent-text underline-offset-2 hover:underline"
				>TMDB<span class="sr-only"> (opens in a new tab)</span></a
			>
		</p>
		<a
			href={resolve('/(app)/about')}
			class="rounded-sm underline-offset-2 hover:text-ink hover:underline">About watcht</a
		>
	</div>
</footer>
