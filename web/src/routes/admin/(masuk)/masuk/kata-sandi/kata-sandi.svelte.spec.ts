import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockGoto } = vi.hoisted(() => ({
	mockGoto: vi.fn().mockResolvedValue(undefined)
}));

vi.mock('$app/navigation', () => ({
	goto: mockGoto
}));

import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

describe('kata-sandi page component', () => {
	beforeEach(() => {
		mockGoto.mockClear();
		vi.restoreAllMocks();
	});

	it('renders change password form elements', async () => {
		render(Page, { data: {} });
		await expect.element(page.getByText('Ganti Kata Sandi')).toBeInTheDocument();
		await expect.element(page.getByLabelText('Kata sandi saat ini')).toBeInTheDocument();
		await expect.element(page.getByLabelText('Kata sandi baru')).toBeInTheDocument();
		await expect.element(page.getByLabelText('Ulangi kata sandi baru')).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Simpan' })).toBeInTheDocument();
	});

	it('validates password length < 12 characters', async () => {
		render(Page, { data: {} });

		await page.getByLabelText('Kata sandi saat ini').fill('old-password-123');
		await page.getByLabelText('Kata sandi baru').fill('short');
		await page.getByLabelText('Ulangi kata sandi baru').fill('short');
		await page.getByRole('button', { name: 'Simpan' }).click();

		await expect.element(page.getByText('Minimal 12 karakter.')).toBeInTheDocument();
	});

	it('validates mismatched confirm password', async () => {
		render(Page, { data: {} });

		await page.getByLabelText('Kata sandi saat ini').fill('old-password-123');
		await page.getByLabelText('Kata sandi baru').fill('new-valid-password-1');
		await page.getByLabelText('Ulangi kata sandi baru').fill('new-valid-password-2');
		await page.getByRole('button', { name: 'Simpan' }).click();

		await expect.element(page.getByText('Kata sandi baru tidak cocok.')).toBeInTheDocument();
	});

	it('rejects password containing username', async () => {
		render(Page, {
			data: {
				session: {
					user: { id: 1, username: 'andi.admin', fullName: 'Andi Admin', roles: ['super_admin'] },
					permissions: [],
					totpEnrolled: false,
					stepUpExpiresAt: null,
					stage: 'password_change_required'
				}
			}
		});

		await page.getByLabelText('Kata sandi saat ini').fill('old-password-123');
		await page.getByLabelText('Kata sandi baru').fill('secret-andi.admin-pass');
		await page.getByLabelText('Ulangi kata sandi baru').fill('secret-andi.admin-pass');
		await page.getByRole('button', { name: 'Simpan' }).click();

		await expect
			.element(page.getByText('Kata sandi tidak boleh memuat nama pengguna.'))
			.toBeInTheDocument();
	});

	it('displays 400 error detail from server', async () => {
		vi.spyOn(window, 'fetch').mockResolvedValue(
			new Response(JSON.stringify({ status: 400, detail: 'Kata sandi terlalu umum.' }), {
				status: 400,
				headers: { 'content-type': 'application/json' }
			})
		);

		render(Page, { data: {} });

		await page.getByLabelText('Kata sandi saat ini').fill('old-password-123');
		await page.getByLabelText('Kata sandi baru').fill('password123456');
		await page.getByLabelText('Ulangi kata sandi baru').fill('password123456');
		await page.getByRole('button', { name: 'Simpan' }).click();

		await expect.element(page.getByText('Kata sandi terlalu umum.')).toBeInTheDocument();
	});

	it('displays 429 throttle message with minutes calculated from Retry-After', async () => {
		vi.spyOn(window, 'fetch').mockResolvedValue(
			new Response(null, {
				status: 429,
				headers: { 'retry-after': '180' }
			})
		);

		render(Page, { data: {} });

		await page.getByLabelText('Kata sandi saat ini').fill('old-password-123');
		await page.getByLabelText('Kata sandi baru').fill('new-valid-password-123');
		await page.getByLabelText('Ulangi kata sandi baru').fill('new-valid-password-123');
		await page.getByRole('button', { name: 'Simpan' }).click();

		await expect
			.element(page.getByText('Terlalu banyak percobaan gagal. Coba lagi dalam 3 menit.'))
			.toBeInTheDocument();
	});

	it('navigates to autentikator when stage is totp_enrollment_required', async () => {
		const fetchSpy = vi.spyOn(window, 'fetch');
		// 1st call: PUT /password
		fetchSpy.mockResolvedValueOnce(
			new Response(JSON.stringify({ status: 200 }), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			})
		);
		// 2nd call: GET /session
		fetchSpy.mockResolvedValueOnce(
			new Response(JSON.stringify({ stage: 'totp_enrollment_required' }), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			})
		);

		render(Page, { data: {} });

		await page.getByLabelText('Kata sandi saat ini').fill('old-password-123');
		await page.getByLabelText('Kata sandi baru').fill('new-valid-password-123');
		await page.getByLabelText('Ulangi kata sandi baru').fill('new-valid-password-123');
		await page.getByRole('button', { name: 'Simpan' }).click();

		await expect
			.poll(() => mockGoto)
			.toHaveBeenCalledWith(expect.stringContaining('/admin/masuk/autentikator'));
	});
});
