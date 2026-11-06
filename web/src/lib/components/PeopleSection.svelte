<script lang="ts">
	import PlaceholderFilter from './PlaceholderFilter.svelte';
	import PlaceholderRow from './PlaceholderRow.svelte';
	import TitleSection from './TitleSection.svelte';
</script>

<!-- Cast and crew, after classic Trakt: a role filter and a row of people. A placeholder for now. -->
<TitleSection id="people" title="People" later="Cast and crew are coming in a later update.">
	{#snippet filter()}
		<PlaceholderFilter text="Cast" label="Show: cast" describedby="people-later" />
	{/snippet}
	<PlaceholderRow shape="person" />
</TitleSection>
