<script lang="ts">
	import TitleSection from './TitleSection.svelte';
</script>

<!--
	A show's next episode to watch and the latest ones to air, after
	classic Trakt: one card under "Up next", two under "Recently aired".
	Placeholders until slice 4's show progress fills them (maintainer
	decision): the episode cards' shape (a 16:9 still and the dark bar),
	static on `raised`, with no titles, dates, or scores.
-->
<TitleSection
	id="recent"
	title="Recent episodes"
	later="Your next episode and the latest ones to air come with show progress in a later update."
>
	<div class="mt-5 grid grid-cols-2 gap-x-3 gap-y-6 sm:gap-x-4 lg:grid-cols-3">
		<div class="col-span-2 sm:col-span-1">
			<h3 class="text-base font-semibold text-ink">Up next</h3>
			<div class="mt-3 grid grid-cols-2 gap-x-3 sm:grid-cols-1">
				{@render card()}
			</div>
		</div>
		<div class="col-span-2 sm:col-span-1 lg:col-span-2">
			<h3 class="text-base font-semibold text-ink">Recently aired</h3>
			<div class="mt-3 grid grid-cols-2 gap-x-3 sm:grid-cols-1 sm:gap-x-4 lg:grid-cols-2">
				{@render card()}
				<div class="sm:hidden lg:block">{@render card()}</div>
			</div>
		</div>
	</div>
</TitleSection>

{#snippet card()}
	<div aria-hidden="true" class="flex flex-col">
		<div class="grid aspect-video place-items-center rounded-t-sm bg-raised text-muted/50">
			<svg
				viewBox="0 0 24 24"
				class="size-8"
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linecap="round"
				stroke-linejoin="round"
				><rect x="3" y="5" width="18" height="12" rx="2" /><path d="M8 20.5h8M12 17v3.5" /></svg
			>
		</div>
		<!-- The bar's shape: a flush check square, the watchlist icon, a score. -->
		<div class="flex h-10 items-center rounded-b-sm bg-strip sm:h-8">
			<span class="size-10 rounded-bl-sm bg-on-screen-muted/15 sm:size-8"></span>
			<span class="mx-3 size-4 rounded-sm bg-on-screen-muted/15 sm:mx-2"></span>
			<span class="mr-2 ml-auto h-2.5 w-8 rounded-sm bg-on-screen-muted/15"></span>
		</div>
		<div class="flex flex-col gap-1.5 pt-2.5">
			<span class="h-3 w-3/4 rounded-sm bg-raised"></span>
			<span class="h-2.5 w-1/3 rounded-sm bg-raised/70"></span>
		</div>
	</div>
{/snippet}
