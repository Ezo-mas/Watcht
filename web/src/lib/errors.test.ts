import { describe, expect, it } from 'vitest';
import { ApiError } from './api/client';
import { errorMessage } from './errors';

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
