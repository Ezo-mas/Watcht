// Placing a popup (a menu or tooltip in the top layer, position: fixed)
// next to the control that opened it, inside the viewport.

export interface Box {
	top: number;
	left: number;
	width: number;
	height: number;
}

export interface Placement {
	top: number;
	left: number;
	/** Which side of the anchor it ended up on. */
	side: 'below' | 'above';
}

/**
 * Where to put a popup of `size` next to `anchor` (both in viewport
 * pixels, as getBoundingClientRect gives them). It goes on the preferred
 * side when it fits, flips when only the other side fits, and otherwise
 * takes the roomier side. It lines up with the anchor's left edge (or
 * centre), and is kept `margin` px inside the viewport on every edge.
 */
export function placeNear(
	anchor: Box,
	size: { width: number; height: number },
	viewport: { width: number; height: number },
	{
		prefer = 'below',
		align = 'start',
		gap = 6,
		margin = 8,
		flip = true,
		insetTop = 0
	}: {
		prefer?: 'below' | 'above';
		align?: 'start' | 'center';
		gap?: number;
		margin?: number;
		/** false keeps it on `prefer`, shifted to fit: for a popup that grows while open. */
		flip?: boolean;
		/** How much of the top is covered (the sticky header): it stays below that. */
		insetTop?: number;
	} = {}
): Placement {
	const room = {
		below: viewport.height - margin - (anchor.top + anchor.height + gap),
		above: anchor.top - gap - margin - insetTop
	};
	const other = prefer === 'below' ? 'above' : 'below';
	let side: 'below' | 'above' = prefer;
	if (flip && room[prefer] < size.height) {
		side = room[other] >= size.height || room[other] > room[prefer] ? other : prefer;
	}
	let top = side === 'below' ? anchor.top + anchor.height + gap : anchor.top - gap - size.height;
	top = clamp(top, insetTop + margin, viewport.height - margin - size.height);

	let left = align === 'center' ? anchor.left + (anchor.width - size.width) / 2 : anchor.left;
	left = clamp(left, margin, viewport.width - margin - size.width);
	return { top: Math.round(top), left: Math.round(left), side };
}

/** Within [min, max]; min wins when the range is empty (too big to fit). */
function clamp(n: number, min: number, max: number): number {
	return Math.max(min, Math.min(n, max));
}

/**
 * Whether the anchor has left the visible part of the window: scrolled
 * up under the sticky header (`coveredTop`, its bottom edge), below the
 * bottom, or off either side. A popup following it then closes, rather
 * than being held at the window's edge over the header.
 */
export function outOfView(
	anchor: Box,
	viewport: { width: number; height: number },
	coveredTop = 0
): boolean {
	return (
		anchor.top + anchor.height <= coveredTop ||
		anchor.top >= viewport.height ||
		anchor.left + anchor.width <= 0 ||
		anchor.left >= viewport.width
	);
}

/** The bottom edge of the sticky app header (`data-app-header`), or 0 without one. */
export function coveredTop(): number {
	const header = document.querySelector('[data-app-header]');
	return header ? Math.max(0, header.getBoundingClientRect().bottom) : 0;
}
