import { describe, expect, it } from 'vitest';
import { placeNear } from './anchor';

const viewport = { width: 375, height: 800 };
const menu = { width: 256, height: 240 };
const check = (top: number, left: number) => ({ top, left, width: 40, height: 40 });

describe('placeNear', () => {
	it('goes below the anchor, lined up with its left edge', () => {
		expect(placeNear(check(100, 20), menu, viewport)).toEqual({
			top: 146,
			left: 20,
			side: 'below'
		});
	});

	it('flips above when only that side has room', () => {
		expect(placeNear(check(600, 20), menu, viewport)).toEqual({
			top: 354,
			left: 20,
			side: 'above'
		});
	});

	it('stays inside the viewport on the right', () => {
		// A check in the right-hand column of a phone grid.
		expect(placeNear(check(100, 200), menu, viewport).left).toBe(375 - 8 - 256);
	});

	it('takes the roomier side, kept inside, when neither fits', () => {
		const tall = { width: 256, height: 500 };
		const at = placeNear(check(380, 20), tall, viewport);
		expect(at.side).toBe('below');
		expect(at.top).toBe(800 - 8 - 500);
	});

	it('pins a popup wider than the viewport to the left margin', () => {
		expect(placeNear(check(100, 200), { width: 400, height: 100 }, viewport).left).toBe(8);
	});

	it('prefers above and centres, for tooltips', () => {
		const tip = { width: 160, height: 44 };
		expect(placeNear(check(300, 100), tip, viewport, { prefer: 'above', align: 'center' })).toEqual(
			{ top: 250, left: 40, side: 'above' }
		);
		// No room above: below instead.
		expect(placeNear(check(20, 100), tip, viewport, { prefer: 'above' }).side).toBe('below');
	});

	it('keeps its side when told not to flip, shifted to fit', () => {
		// An open menu that grew: it moves up over the anchor rather than jumping above it.
		const at = placeNear(check(500, 20), { width: 256, height: 350 }, viewport, { flip: false });
		expect(at).toEqual({ top: 800 - 8 - 350, left: 20, side: 'below' });
	});
});
