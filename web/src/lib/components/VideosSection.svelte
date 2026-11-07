<script lang="ts">
	import PlaceholderFilter from './PlaceholderFilter.svelte';
	import PlaceholderRow from './PlaceholderRow.svelte';
	import TitleSection from './TitleSection.svelte';
</script>

<!-- Trailers and clips: a type filter and a row of 16:9 thumbnails. A placeholder for now. -->
<TitleSection id="videos" title="Videos" later="Trailers and clips are coming in a later update.">
	{#snippet filter()}
		<PlaceholderFilter text="Trailers" label="Show: trailers" describedby="videos-later" />
	{/snippet}
	<PlaceholderRow shape="video" />
</TitleSection>
