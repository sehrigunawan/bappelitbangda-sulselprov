import { describe, expect, it } from 'vitest';
import { sanitizeKembaliUrl } from './url';

describe('sanitizeKembaliUrl (W3)', () => {
	it.each([
		['//evil', '/admin'],
		['https://x', '/admin'],
		['/publik', '/admin'],
		['/admin/masuk', '/admin'],
		['/admin/masuk?x=1', '/admin'],
		['', '/admin'],
		[null, '/admin'],
		['/admin', '/admin'],
		['/admin/pengguna', '/admin/pengguna'],
		['/admin/pengguna?halaman=2', '/admin/pengguna?halaman=2'],
		['/admin/pengguna?redirect=https://example.com', '/admin/pengguna?redirect=https://example.com']
	])('resolves %s to %s', (input, expected) => {
		expect(sanitizeKembaliUrl(input)).toBe(expected);
	});
});
