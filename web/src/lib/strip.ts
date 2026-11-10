// The dark action bar under poster and episode cards (DESIGN.md, Poster
// cards): one look for every card that has it.

/**
 * The bar itself: no padding, as tall as its controls (40 px on phones,
 * 32 px from sm), its bottom corners the card's. Above the card's
 * stretched link, so its buttons are never inside it.
 */
export const stripBar = 'relative z-10 flex h-10 items-center rounded-b-sm bg-strip sm:h-8';

/**
 * A control in the bar: the bar's full height, square corners (the check
 * adds the bottom-left one). The focus ring is drawn inside, with a strip
 * edge round it, so the card's edges never clip it and it reads on a
 * sage check as well as on the bar.
 */
export const stripControl =
	'grid h-10 shrink-0 place-items-center sm:h-8 focus-visible:outline-on-screen-accent focus-visible:-outline-offset-4 focus-visible:inset-ring-2 focus-visible:inset-ring-strip';

/** A square control (check, watchlist): 40 px on phones, 32 px from sm. */
export const stripSquare = `${stripControl} w-10 sm:w-8`;

/**
 * The card's watched check, flush in the bar's bottom-left corner, by
 * state (checkState in #lib/watched.js; episode cards use none and all):
 * - none: a check in on-screen-muted on the bar;
 * - some: partly watched, a sage tint with a light sage check;
 * - all: a solid sage square with a surface check: a button (button=true) on season
 *   and episode cards that opens the removal menu.
 * none and some are buttons that open the when-choice: hovering one fills
 * it sage, the look of what it's about to do; while its menu is open it's
 * a neutral lift (on-screen at 15%), so it never looks watched before it
 * is. Off (it can't be used now), it's faint and doesn't change on hover.
 */
export function stripCheck(state: 'none' | 'some' | 'all', off = false, button = false): string {
	const base = `${stripSquare} rounded-bl-sm transition-colors`;
	// A watched card's check opens the removal menu:
	// darker sage on hover and while its menu is open, as on history cards.
	if (state === 'all' && button) {
		return `${base} bg-watched text-surface hover:bg-watched-hover aria-expanded:bg-watched-hover`;
	}
	if (state === 'all') return `${base} bg-watched text-surface`;
	// Faded by colour, not opacity, so the focus ring stays full strength.
	if (off) {
		return state === 'some'
			? `${base} cursor-not-allowed bg-watched-part/50 text-on-watched-part/50`
			: `${base} cursor-not-allowed text-on-screen-muted/40`;
	}
	const look = state === 'some' ? 'bg-watched-part text-on-watched-part' : 'text-on-screen-muted';
	return `${base} ${look} hover:bg-watched hover:text-surface aria-expanded:bg-on-screen/15 aria-expanded:text-on-screen`;
}
