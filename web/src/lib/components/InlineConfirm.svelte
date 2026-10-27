<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import Button from './Button.svelte';
	import Notice from './Notice.svelte';

	interface Props {
		/** What will happen, named plainly ("Delete alice? …"). */
		children: Snippet;
		/** The destructive button's label ("Delete alice"). */
		confirmLabel: string;
		onconfirm: () => void;
		/** Also runs on Escape. The caller moves focus back to the trigger. */
		oncancel: () => void;
		busy?: boolean;
		/** A failure of the confirmed action, shown under the question. */
		error?: string;
		class?: ClassValue;
	}

	let {
		children,
		confirmLabel,
		onconfirm,
		oncancel,
		busy = false,
		error = '',
		class: className
	}: Props = $props();

	const id = $props.id();

	// Focus the question itself, not the destructive button: the warning is
	// read first, and a second Enter can't confirm by accident.
	function focusOnMount(node: HTMLElement) {
		node.focus();
	}

	function onkeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && !busy) {
			event.stopPropagation();
			oncancel();
		}
	}
</script>

<!-- Escape from anywhere inside (the question or its buttons) cancels. -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	role="group"
	aria-labelledby="{id}-question"
	tabindex="-1"
	{onkeydown}
	{@attach focusOnMount}
	class={['flex flex-col gap-3 rounded-md border border-danger/40 bg-danger/5 p-3', className]}
>
	<p id="{id}-question" class="text-sm text-ink">{@render children()}</p>
	<Notice tone="error" message={error} />
	<div class="flex flex-wrap gap-2">
		<Button variant="danger" {busy} onclick={onconfirm}>{confirmLabel}</Button>
		<Button variant="quiet" disabled={busy} onclick={oncancel}>Cancel</Button>
	</div>
</div>
