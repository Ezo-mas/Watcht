// Toasts: the results of actions ("Added to your history: …"), shown
// floating by the Toaster in the (app) layout and gone after a while.
// Errors that belong to a control or a state (a load failure with "Try
// again", a disabled reason, a field, an open menu) stay inline instead
// (DESIGN.md, Toasts).

export type ToastTone = 'success' | 'info' | 'error';

export interface Toast {
	id: number;
	tone: ToastTone;
	message: string;
}

/** How long success and info toasts stay, in ms; errors stay until dismissed. */
export const TOAST_DURATION = 5000;
/** How many are shown at once; older ones drop off. */
export const TOAST_LIMIT = 3;

interface Timer {
	/** ms left before it goes. */
	remaining: number;
	/** When the running timer started (Date.now()), or null while paused. */
	started: number | null;
	handle: ReturnType<typeof setTimeout> | undefined;
}

class Toaster {
	/** Shown toasts, newest first. */
	items = $state<Toast[]>([]);
	#next = 1;
	#timers = new Map<number, Timer>();
	/** Why each toast is held (hovered, focused); it runs again when none is left. */
	#holds = new Map<number, { hover: boolean; focus: boolean }>();

	success(message: string): number {
		return this.show('success', message);
	}

	info(message: string): number {
		return this.show('info', message);
	}

	error(message: string): number {
		return this.show('error', message);
	}

	/** Shows a toast; returns its id. */
	show(tone: ToastTone, message: string): number {
		// The same toast again (a repeated action) replaces the shown one:
		// back on top, with a fresh timeout, instead of a stack of copies.
		const same = this.items.find((t) => t.tone === tone && t.message === message);
		if (same) this.#stop(same.id);
		const id = this.#next++;
		const items = [{ id, tone, message }, ...this.items.filter((t) => t !== same)];
		for (const old of items.slice(TOAST_LIMIT)) this.#stop(old.id);
		this.items = items.slice(0, TOAST_LIMIT);
		if (tone !== 'error') {
			this.#timers.set(id, { remaining: TOAST_DURATION, started: null, handle: undefined });
			this.resume(id);
		}
		return id;
	}

	dismiss(id: number) {
		this.#stop(id);
		this.items = this.items.filter((t) => t.id !== id);
	}

	/**
	 * Holds a toast while it's hovered or focused, and lets it run again
	 * only when it's neither: the pointer leaving while a × still has focus
	 * (or the reverse) doesn't restart it.
	 */
	hold(id: number, reason: 'hover' | 'focus', held: boolean) {
		const holds = this.#holds.get(id) ?? { hover: false, focus: false };
		holds[reason] = held;
		this.#holds.set(id, holds);
		if (holds.hover || holds.focus) this.pause(id);
		else this.resume(id);
	}

	/** Holds its timeout. */
	pause(id: number) {
		const timer = this.#timers.get(id);
		if (!timer || timer.started === null) return;
		clearTimeout(timer.handle);
		timer.remaining -= Date.now() - timer.started;
		timer.started = null;
	}

	/** Lets its timeout run again, for the time it had left. */
	resume(id: number) {
		const timer = this.#timers.get(id);
		if (!timer || timer.started !== null) return;
		timer.started = Date.now();
		timer.handle = setTimeout(() => this.dismiss(id), Math.max(0, timer.remaining));
	}

	/** Removes them all (signing out, leaving the signed-in pages, tests). */
	clear() {
		for (const id of this.#timers.keys()) this.#stop(id);
		this.items = [];
	}

	#stop(id: number) {
		const timer = this.#timers.get(id);
		if (timer) clearTimeout(timer.handle);
		this.#timers.delete(id);
		this.#holds.delete(id);
	}
}

export const toast = new Toaster();
