/**
 * Polls until a value is ready: used for a season whose episodes the
 * server is still fetching from TMDB (episodes_pending).
 *
 * `load` runs at once, then every `interval` ms while `done` says no, until
 * `timeout` ms have passed since the start (then `onTimeout`). A failed
 * load stops polling and goes to `onError`. The returned function cancels:
 * it aborts a request in flight (through the signal) and nothing is
 * reported after it.
 */
export interface PollOptions<T> {
	load: (signal: AbortSignal) => Promise<T>;
	done: (value: T) => boolean;
	/** Every loaded value, including the last one. */
	onValue: (value: T) => void;
	onTimeout: () => void;
	onError: (error: unknown) => void;
	/** Milliseconds between the end of one load and the next; default 3 s. */
	interval?: number;
	/** Milliseconds from the start after which polling gives up; default 2 min. */
	timeout?: number;
	now?: () => number;
}

export function poll<T>(options: PollOptions<T>): () => void {
	const {
		load,
		done,
		onValue,
		onTimeout,
		onError,
		interval = 3000,
		timeout = 120_000,
		now = Date.now
	} = options;
	const controller = new AbortController();
	const start = now();
	let timer: ReturnType<typeof setTimeout> | undefined;
	let cancelled = false;

	async function run() {
		timer = undefined;
		let value: T;
		try {
			value = await load(controller.signal);
		} catch (e) {
			if (!cancelled) onError(e);
			return;
		}
		if (cancelled) return;
		onValue(value);
		if (done(value)) return;
		if (now() - start + interval > timeout) {
			onTimeout();
			return;
		}
		timer = setTimeout(run, interval);
	}

	void run();

	return () => {
		cancelled = true;
		if (timer !== undefined) clearTimeout(timer);
		controller.abort();
	};
}
