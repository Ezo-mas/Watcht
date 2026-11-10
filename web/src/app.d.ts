// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		interface Error {
			message: string;
			/**
			 * What a 404 is about, so the error page can name it and link to
			 * what's still there: a show, or a season or episode of one.
			 */
			missing?: 'show' | 'season' | 'episode';
		}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
