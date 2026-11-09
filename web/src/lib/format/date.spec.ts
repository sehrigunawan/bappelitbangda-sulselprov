import { describe, expect, it } from 'vitest';
import { formatDateTimeWita } from './date';

describe('formatDateTimeWita (W12)', () => {
	it('formats ISO instant to Indonesian WITA string', () => {
		const formatted = formatDateTimeWita('2026-10-15T06:05:00Z');
		expect(formatted).toBe('Kamis, 15 Oktober 2026 14.05 WITA');
	});
});
