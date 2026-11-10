// Long lists rendered a chunk at a time (the All episodes page): a show
// like General Hospital lists some 15,000 episodes, far too many rows to
// put on the page at once.

/** How many rows a long list shows first, and adds each time. */
export const CHUNK = 100;

/**
 * How many rows to show after one more chunk: the current count plus
 * `step`, never past the total (and never fewer than shown already).
 */
export function nextLimit(shown: number, total: number, step = CHUNK): number {
	const at = Math.max(0, Math.min(shown, total));
	return Math.min(total, at + Math.max(1, step));
}

/** The "Show more" button's label: "Show 100 more" or, near the end, "Show the last 12". */
export function moreLabel(shown: number, total: number, step = CHUNK): string {
	const next = nextLimit(shown, total, step) - Math.min(shown, total);
	const n = next.toLocaleString('en');
	return nextLimit(shown, total, step) >= total ? `Show the last ${n}` : `Show ${n} more`;
}
