<script lang="ts">
	// TMDB's attribution, shown on the About page: its logo and the notice
	// its terms require. Don't remove it. The footer of every signed-in page
	// carries a short credit instead (AppFooter).
	//
	// The logo is TMDB's official "Alt short" file, unedited. TMDB allows it
	// in white, black, or its brand colours, so it's drawn as a mask filled
	// with --color-tmdb-logo: pure black in light mode, pure white in dark.
	// Change only that colour, never the shape or proportions.
	import tmdbLogo from '#lib/assets/tmdb-logo.svg';
</script>

<div class="flex items-start gap-3">
	<a
		href="https://www.themoviedb.org/"
		target="_blank"
		rel="noopener noreferrer"
		aria-label="TMDB (opens in a new tab)"
		class="mt-0.5 shrink-0 rounded-sm"
	>
		<span
			aria-hidden="true"
			class="block aspect-[273.42/35.52] h-5 bg-tmdb-logo"
			style:mask={`url("${tmdbLogo}") center / contain no-repeat`}
			style:-webkit-mask={`url("${tmdbLogo}") center / contain no-repeat`}
		></span>
	</a>
	<p class="text-sm text-ink">
		This product uses the TMDB API but is not endorsed or certified by TMDB.
		<a
			href="https://www.themoviedb.org/"
			target="_blank"
			rel="noopener noreferrer"
			class="rounded-sm whitespace-nowrap text-accent-text underline-offset-2 hover:underline"
			>themoviedb.org<span class="sr-only"> (opens in a new tab)</span></a
		>
	</p>
</div>
