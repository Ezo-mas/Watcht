<script lang="ts">
	import type { PageSection } from './TitlePage.svelte';

	interface Props {
		sections: PageSection[];
		/** The section in view (TitlePage's scroll-spy); marked aria-current. */
		current: string | undefined;
		/** Called with the section a link was used for, before the page scrolls. */
		onpick: (id: string) => void;
		/**
		 * column: the classic list under the poster (sm up);
		 * strip: a row of filter-style links that scrolls sideways (phones).
		 */
		variant: 'column' | 'strip';
		class?: string;
	}

	let { sections, current, onpick, variant, class: className }: Props = $props();
</script>

<nav aria-label="Sections" class={className}>
	{#if variant === 'column'}
		<ul class="border-t border-line">
			{#each sections as section (section.id)}
				<li>
					<a
						href="#{section.id}"
						aria-current={current === section.id ? 'true' : undefined}
						onclick={() => onpick(section.id)}
						class="group relative flex items-center gap-2 border-b border-line py-2.5 pr-0.5 text-[0.6875rem] font-semibold tracking-wider text-muted uppercase transition-colors before:absolute before:inset-y-1.5 before:-left-3 before:w-0.5 before:rounded-full hover:text-ink focus-visible:-outline-offset-2 aria-[current=true]:text-ink aria-[current=true]:before:bg-accent"
					>
						<span class="min-w-0 flex-1 truncate">{section.label}</span>
						{#if section.count !== undefined}
							<span class="font-medium tabular-nums">{section.count}</span>
						{/if}
						<svg
							aria-hidden="true"
							viewBox="0 0 16 16"
							class="size-3.5 shrink-0 opacity-50 transition-opacity group-hover:opacity-100 group-aria-[current=true]:text-accent-text group-aria-[current=true]:opacity-100"
							fill="none"
							stroke="currentColor"
							stroke-width="1.6"
							stroke-linecap="round"
							stroke-linejoin="round"><path d="M2.5 8h11M9.5 4l4 4-4 4" /></svg
						>
					</a>
				</li>
			{/each}
		</ul>
	{:else}
		<ul class="-mx-4 flex [scrollbar-width:none] gap-2 overflow-x-auto px-4 py-1">
			{#each sections as section (section.id)}
				<li class="shrink-0">
					<a
						href="#{section.id}"
						aria-current={current === section.id ? 'true' : undefined}
						onclick={() => onpick(section.id)}
						class="flex h-10 items-center gap-1.5 rounded-md border border-line bg-surface px-3 text-sm font-medium whitespace-nowrap text-muted transition-colors hover:border-line-strong hover:text-ink aria-[current=true]:border-accent aria-[current=true]:bg-accent/15 aria-[current=true]:text-accent-text"
					>
						{section.label}
						{#if section.count !== undefined}
							<span class="tabular-nums">{section.count}</span>
						{/if}
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</nav>
