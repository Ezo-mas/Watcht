<script lang="ts">
	import type { TitleLink } from '#lib/media.js';
	import SourceMark from './SourceMark.svelte';

	interface Props {
		/** The title's name, for each link's accessible name ("The Matrix on IMDb"). */
		title: string;
		links: TitleLink[];
	}

	let { title, links }: Props = $props();
</script>

<!--
	The title on other sites, as square icon links under the poster (after
	classic Trakt): 40 px tiles that wrap, each opening a new tab.
-->
{#if links.length}
	<ul aria-label="{title} on other sites" class="flex flex-wrap gap-2">
		{#each links as link (link.site)}
			<li>
				<!-- External sites, so no resolve(). -->
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					href={link.href}
					target="_blank"
					rel="noopener noreferrer"
					title={link.label}
					class="grid size-10 place-items-center rounded-md border border-line bg-surface transition-colors hover:border-line-strong hover:bg-raised"
				>
					<SourceMark site={link.site} />
					<span class="sr-only">{title} on {link.label} (opens in a new tab)</span>
				</a>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			</li>
		{/each}
	</ul>
{/if}
