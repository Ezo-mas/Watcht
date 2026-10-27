<script lang="ts">
	interface Props {
		/** "sm" for the top bar, "lg" above the sign-in and setup cards. */
		size?: 'sm' | 'lg';
	}

	let { size = 'sm' }: Props = $props();
</script>

<!--
	The underline is a progress bar filled under "watch" and not the final
	"t": watched, almost to the end. It's decoration; the text is the name.
-->
<span
	class={[
		'inline-flex flex-col gap-[0.16em] leading-none font-extrabold tracking-[-0.04em] text-ink',
		size === 'lg' ? 'text-3xl' : 'text-lg'
	]}
>
	<span>watcht</span>
	<span aria-hidden="true" class="flex h-[0.12em] min-h-0.5 w-full overflow-hidden rounded-full">
		<span class="w-[83%] bg-accent"></span>
		<span class="flex-1 bg-line"></span>
	</span>
</span>
