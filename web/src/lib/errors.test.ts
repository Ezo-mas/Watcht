import { describe, expect, it } from 'vitest';
import { ApiError } from './api/client';
import { catalogMessages, errorMessage } from './errors';

describe('errorMessage', () => {
	const wrong = new ApiError(400, 'invalid_credentials', 'the password is wrong');

	it('uses the message given for the code', () => {
		expect(errorMessage(wrong, { invalid_credentials: 'The current password is wrong.' })).toBe(
			'The current password is wrong.'
		);
	});

	it("falls back to the server's message for other codes", () => {
		expect(errorMessage(wrong, { name_taken: 'Taken.' })).toBe('the password is wrong');
		expect(errorMessage(wrong)).toBe('the password is wrong');
	});

	it('gives a generic message for anything that is not an ApiError', () => {
		expect(errorMessage(new TypeError('x is undefined'))).toBe('Something went wrong.');
	});
});

describe('catalogMessages', () => {
	it('explains a missing TMDB key', () => {
		const e = new ApiError(503, 'tmdb_not_configured', 'ask the admin to set WATCHT_TMDB_API_KEY');
		expect(errorMessage(e, catalogMessages)).toContain('WATCHT_TMDB_API_KEY');
	});

	it('says TMDB is unreachable for provider_unavailable', () => {
		const e = new ApiError(502, 'provider_unavailable', 'metadata provider is unavailable');
		expect(errorMessage(e, catalogMessages)).toBe("TMDB didn't answer. Try again in a moment.");
	});
});
