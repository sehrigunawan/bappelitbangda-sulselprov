import { describe, expect, it } from 'vitest';
import { isStepUpRequired, parseRetryAfterMinutes, problemDetail } from './problem';

describe('problemDetail', () => {
	it('extracts detail from RFC 9457 Problem object', () => {
		const error = {
			type: 'about:blank',
			title: 'Unauthorized',
			status: 401,
			detail: 'Kata sandi salah.'
		};
		expect(problemDetail(error)).toBe('Kata sandi salah.');
	});

	it('returns default fallback when error has no detail', () => {
		expect(problemDetail(null)).toBe('Terjadi kesalahan. Coba lagi.');
		expect(problemDetail(undefined)).toBe('Terjadi kesalahan. Coba lagi.');
		expect(problemDetail({})).toBe('Terjadi kesalahan. Coba lagi.');
		expect(problemDetail({ detail: 123 })).toBe('Terjadi kesalahan. Coba lagi.');
	});

	it('returns custom fallback when provided', () => {
		expect(problemDetail({}, 'Gagal.')).toBe('Gagal.');
	});
});

describe('isStepUpRequired', () => {
	it('returns true when detail matches step-up message', () => {
		const error = { detail: 'Verifikasi ulang dengan kode autentikator diperlukan.' };
		expect(isStepUpRequired(error)).toBe(true);
	});

	it('returns false when detail differs or missing', () => {
		expect(isStepUpRequired({ detail: 'Akses ditolak.' })).toBe(false);
		expect(isStepUpRequired(null)).toBe(false);
	});
});

describe('parseRetryAfterMinutes', () => {
	it('parses retry-after header in seconds to ceiling minutes', () => {
		const res1 = new Response(null, { headers: { 'retry-after': '120' } });
		expect(parseRetryAfterMinutes(res1)).toBe(2);

		const res2 = new Response(null, { headers: { 'retry-after': '65' } });
		expect(parseRetryAfterMinutes(res2)).toBe(2);

		const res3 = new Response(null, { headers: { 'retry-after': '1' } });
		expect(parseRetryAfterMinutes(res3)).toBe(1);
	});

	it('uses default fallback when header is missing or non-positive', () => {
		const res1 = new Response(null);
		expect(parseRetryAfterMinutes(res1)).toBe(15);

		const res2 = new Response(null, { headers: { 'retry-after': 'invalid' } });
		expect(parseRetryAfterMinutes(res2)).toBe(15);

		const res3 = new Response(null, { headers: { 'retry-after': '-5' } });
		expect(parseRetryAfterMinutes(res3)).toBe(15);
	});
});
