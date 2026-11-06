import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TOAST_DURATION, TOAST_LIMIT, toast } from './toast.svelte';

const messages = () => toast.items.map((t) => t.message);

describe('toast store', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		toast.clear();
	});
	afterEach(() => {
		toast.clear();
		vi.useRealTimers();
	});

	it('shows the newest first, with its tone', () => {
		toast.success('Added');
		toast.info('Already counted');
		toast.error('Failed');
		expect(toast.items.map((t) => [t.tone, t.message])).toEqual([
			['error', 'Failed'],
			['info', 'Already counted'],
			['success', 'Added']
		]);
	});

	it(`keeps at most ${TOAST_LIMIT}, dropping the oldest`, () => {
		for (const m of ['one', 'two', 'three', 'four']) toast.success(m);
		expect(messages()).toEqual(['four', 'three', 'two']);
	});

	it('takes success and info toasts away after the timeout', () => {
		toast.success('Added');
		toast.info('Already counted');
		vi.advanceTimersByTime(TOAST_DURATION - 1);
		expect(messages()).toEqual(['Already counted', 'Added']);
		vi.advanceTimersByTime(1);
		expect(messages()).toEqual([]);
	});

	it('keeps errors until they are dismissed', () => {
		const id = toast.error('Failed');
		vi.advanceTimersByTime(TOAST_DURATION * 10);
		expect(messages()).toEqual(['Failed']);
		toast.dismiss(id);
		expect(messages()).toEqual([]);
	});

	it('holds the timeout while paused, then runs only for the time left', () => {
		const id = toast.success('Added');
		vi.advanceTimersByTime(3000);
		toast.pause(id);
		vi.advanceTimersByTime(TOAST_DURATION * 4);
		expect(messages()).toEqual(['Added']);
		toast.resume(id);
		vi.advanceTimersByTime(TOAST_DURATION - 3000 - 1);
		expect(messages()).toEqual(['Added']);
		vi.advanceTimersByTime(1);
		expect(messages()).toEqual([]);
	});

	it('ignores a second pause or resume', () => {
		const id = toast.success('Added');
		toast.pause(id);
		toast.pause(id);
		toast.resume(id);
		toast.resume(id);
		vi.advanceTimersByTime(TOAST_DURATION);
		expect(messages()).toEqual([]);
	});

	it('stays held while it is hovered or focused, and runs again when it is neither', () => {
		const id = toast.success('Added');
		toast.hold(id, 'hover', true);
		toast.hold(id, 'focus', true);
		// The pointer leaves, but the × still has focus: still held.
		toast.hold(id, 'hover', false);
		vi.advanceTimersByTime(TOAST_DURATION * 2);
		expect(messages()).toEqual(['Added']);
		// Focus leaves too: it runs for the time it had left.
		toast.hold(id, 'focus', false);
		vi.advanceTimersByTime(TOAST_DURATION - 1);
		expect(messages()).toEqual(['Added']);
		vi.advanceTimersByTime(1);
		expect(messages()).toEqual([]);
	});

	it('keeps an error toast until dismissed, however it is held', () => {
		const id = toast.error('Failed');
		toast.hold(id, 'hover', true);
		toast.hold(id, 'hover', false);
		vi.advanceTimersByTime(TOAST_DURATION * 3);
		expect(messages()).toEqual(['Failed']);
	});

	it('replaces a toast shown again, on top and with a fresh timeout', () => {
		toast.info('Already in your history.');
		toast.success('Added');
		vi.advanceTimersByTime(3000);
		toast.info('Already in your history.');
		expect(messages()).toEqual(['Already in your history.', 'Added']);
		vi.advanceTimersByTime(TOAST_DURATION - 3000);
		expect(messages()).toEqual(['Already in your history.']);
		vi.advanceTimersByTime(3000);
		expect(messages()).toEqual([]);
	});

	it('dismisses one at a time, and stops its timer', () => {
		const first = toast.success('one');
		toast.success('two');
		toast.dismiss(first);
		expect(messages()).toEqual(['two']);
		vi.advanceTimersByTime(TOAST_DURATION);
		expect(messages()).toEqual([]);
	});
});
