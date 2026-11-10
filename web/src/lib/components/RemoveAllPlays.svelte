<script lang="ts">
	import { tick } from 'svelte';
	import { errorMessage } from '#lib/errors.js';
	import InlineConfirm from './InlineConfirm.svelte';
	import { menuItem, trashIcon } from './MenuPopover.svelte';

	interface Props {
		title: string;
		onremove: () => Promise<string>;
		ondone?: () => void;
		disabled?: boolean;
		busy?: boolean;
		open?: boolean;
	}

	let {
		title,
		onremove,
		ondone,
		disabled = false,
		busy = $bindable(false),
		open = true
	}: Props = $props();
	let confirming = $state(false);
	let error = $state('');
	let trigger = $state<HTMLButtonElement>();

	$effect(() => {
		if (!open && !busy) {
			confirming = false;
			error = '';
		}
	});

	async function cancel() {
		confirming = false;
		error = '';
		await tick();
		trigger?.focus();
	}

	async function remove() {
		if (disabled || busy) return;
		busy = true;
		try {
			error = await onremove();
		} catch (e) {
			error = errorMessage(e);
		} finally {
			busy = false;
		}
		if (!error) {
			confirming = false;
			ondone?.();
		}
	}

	function onkeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') event.preventDefault();
	}
</script>

<button
	bind:this={trigger}
	type="button"
	data-item
	aria-label="Remove all plays"
	aria-expanded={confirming}
	aria-disabled={disabled || busy || undefined}
	onclick={() => {
		if (!disabled && !busy) confirming = true;
	}}
	class={[menuItem, 'text-danger hover:bg-danger/10']}
>
	{@render trashIcon()}Remove all plays
</button>
{#if confirming}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="px-3 pb-2" {onkeydown}>
		<InlineConfirm
			confirmLabel="Remove all plays"
			onconfirm={remove}
			oncancel={cancel}
			{busy}
			{error}
		>
			Remove every play of {title} from your history? This cannot be undone.
		</InlineConfirm>
	</div>
{/if}
