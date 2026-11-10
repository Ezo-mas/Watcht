<script lang="ts" module>
	/** One source's score in the hero's ratings band. */
	export interface Rating {
		/** Which mark to show; also the key. */
		site: 'imdb' | 'tmdb';
		/** The source's name ("TMDB"), for screen readers and the tooltip. */
		label: string;
		/** The score as the source gives it ("8.3"). */
		score: string;
		/** The top of the scale ("10"), for screen readers; omit for percentages. */
		outOf?: string;
		votes?: number;
	}
</script>

<script lang="ts">
	import SourceMark from './SourceMark.svelte';

	interface Props {
		ratings: Rating[];
	}

	let { ratings }: Props = $props();

	/** What a screen reader hears: "TMDB rating 8.3 out of 10, from 29,114 votes". */
	function spoken(r: Rating): string {
		let text = `${r.label} rating ${r.score}`;
		if (r.outOf) text += ` out of ${r.outOf}`;
		if (r.votes !== undefined) text += `, from ${r.votes.toLocaleString('en')} votes`;
		return text;
	}
</script>

<!--
	The scores along the bottom of the hero, after classic Trakt: each
	source's mark and its score, nothing else; the vote count is in the
	tooltip and in what screen readers hear (with no count, as for an
	episode, the tooltip is just the score). TMDB today; MDBList's sources
	join in M8. Sits on the screen, so it uses the screen's colours in both
	themes.
-->
<ul
	aria-label="Ratings"
	class="flex min-h-14 flex-wrap items-center gap-x-8 gap-y-2 py-2.5 sm:min-h-16"
>
	{#each ratings as rating (rating.site)}
		<li
			class="flex items-center gap-2.5 whitespace-nowrap"
			title={rating.votes !== undefined
				? `${rating.votes.toLocaleString('en')} votes on ${rating.label}`
				: `${rating.label} rating ${rating.score}`}
		>
			<SourceMark site={rating.site} tone="screen" />
			<span class="sr-only">{spoken(rating)}</span>
			<span
				aria-hidden="true"
				class="text-xl leading-none font-bold text-on-screen-accent tabular-nums"
				>{rating.score}</span
			>
		</li>
	{/each}
</ul>
