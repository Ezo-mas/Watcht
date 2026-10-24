import { describe, it, expect } from 'vitest';
import { appName } from './version';

describe('appName', () => {
	it('is watcht', () => {
		expect(appName).toBe('watcht');
	});
});
