<script lang="ts">
	import { tick } from 'svelte';
	import type { MarkWhen } from '#lib/api/types.js';
	import { browserTimeZone, pickedWhen, toInputValue, type ReleaseOption } from '#lib/when.js';

	interface Props {
		/** Records the mark with the chosen time. */
		onchoose: (when: MarkWhen) => void;
		/** The release-date option: the date, or why it can't be picked. */
		release: ReleaseOption;
		/** While a mark runs, the options are off. */
		busy?: boolean;
		nowLabel?: string;
	}

	let { onchoose, release, busy = false, nowLabel = 'Just now' }: Props = $props();

	// The "when" choice itself, shared by WhenMenu (the mark buttons) and
	// the history card's menu: just now, the release or air date, and
	// "Other date…", which opens a date and time field. The caller decides
	// where it sits and focuses its first option.

	const id = $props.id();
	let picking = $state(false);
	let value = $state('');
	let max = $state('');
	let error = $state('');
	let list = $state<HTMLElement>();
	let input = $state<HTMLInputElement>();

	function choose(when: MarkWhen) {
		if (busy) return;
		onchoose(when);
	}

	async function pickOther() {
		const now = new Date();
		value = toInputValue(now);
		max = value;
		error = '';
		picking = true;
		await tick();
		input?.focus();
	}

	function submit(event: SubmitEvent) {
		event.preventDefault();
		const res = pickedWhen(value, new Date());
		if ('error' in res) {
			error = res.error;
			input?.focus();
			return;
		}
		choose(res.when);
	}

	// Up and down move between the options (not while the date field is open).
	function onkeydown(event: KeyboardEvent) {
		if ((event.key !== 'ArrowDown' && event.key !== 'ArrowUp') || !list || picking) return;
		const items = [...list.querySelectorAll<HTMLButtonElement>('button')];
		const at = items.indexOf(document.activeElement as HTMLButtonElement);
		if (at === -1) return;
		event.preventDefault();
		event.stopPropagation();
		const next = (at + (event.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length;
		items[next].focus();
	}

	const zone = browserTimeZone();

	const option =
		'flex min-h-10 w-full items-center justify-between gap-3 px-3 py-1.5 text-left text-sm text-ink transition-colors hover:bg-raised focus-visible:-outline-offset-2 aria-disabled:cursor-not-allowed aria-disabled:text-muted aria-disabled:hover:bg-transparent sm:min-h-9';
</script>

<p id="{id}-q" class="px-3 pt-1.5 pb-1 text-xs text-muted">When did you watch it?</p>
<!-- The arrow keys move between the options inside the group. -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div bind:this={list} role="group" aria-labelledby="{id}-q" {onkeydown}>
	<button
		type="button"
		class={option}
		aria-disabled={busy || undefined}
		onclick={() => choose({ at: 'now' })}
	>
		{nowLabel}
	</button>
	<button
		type="button"
		class={option}
		aria-disabled={!release.enabled || busy || undefined}
		onclick={() => release.enabled && choose({ at: 'release' })}
	>
		{release.label}
		<span class="text-xs text-muted tabular-nums">{release.detail}</span>
	</button>
	<button
		type="button"
		class={option}
		aria-expanded={picking}
		aria-controls="{id}-pick"
		onclick={pickOther}
	>
		Other date…
	</button>
</div>
<form
	id="{id}-pick"
	hidden={!picking}
	onsubmit={submit}
	novalidate
	class="mx-1 mt-1 mb-0.5 flex flex-col gap-2 rounded-md border-t border-line px-2 pt-2.5 pb-1.5"
>
	<label for="{id}-time" class="text-xs font-medium text-ink">Date and time</label>
	<input
		bind:this={input}
		id="{id}-time"
		type="datetime-local"
		required
		{max}
		bind:value
		aria-describedby="{id}-zone{error ? ` ${id}-error` : ''}"
		aria-invalid={error ? true : undefined}
		class="h-9 w-full rounded-md border border-line-strong bg-field px-2 text-sm text-ink tabular-nums focus-visible:border-accent focus-visible:outline-offset-0"
	/>
	{#if error}
		<p id="{id}-error" class="text-xs text-danger">{error}</p>
	{/if}
	<p id="{id}-zone" class="text-xs text-muted">In your browser's time zone, {zone}.</p>
	<button
		type="submit"
		aria-disabled={busy || undefined}
		class="h-8 self-start rounded-md bg-accent px-3 text-sm font-medium text-on-accent transition-colors hover:bg-accent-hover aria-disabled:opacity-70"
	>
		Mark watched
	</button>
</form>
