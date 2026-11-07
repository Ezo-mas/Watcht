<script lang="ts">
	import PlaceholderRow from './PlaceholderRow.svelte';
	import TitleSection from './TitleSection.svelte';

	interface Props {
		/** The title's name, for the heading. */
		title: string;
	}

	let { title }: Props = $props();
</script>

<!-- "If you like X, check out…": a row of poster cards. A placeholder for now. -->
<TitleSection
	id="recommendations"
	title="If you like {title}, check out…"
	later="Recommendations are coming in a later update."
>
	<PlaceholderRow shape="poster" />
</TitleSection>
