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

describe('login page component (W4)', () => {
	beforeEach(() => {
		mockGoto.mockClear();
		vi.restoreAllMocks();
	});

	it('renders login form elements and labels', async () => {
		render(Page, { data: {} });
		await expect.element(page.getByText('Masuk Panel Admin')).toBeInTheDocument();
		await expect.element(page.getByText('Nama pengguna')).toBeInTheDocument();
		await expect.element(page.getByText('Kata sandi')).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Masuk' })).toBeInTheDocument();
	});

	it('shows session expired message when alasan=sesi-berakhir', async () => {
		render(Page, { data: { alasan: 'sesi-berakhir' } });
		await expect
			.element(page.getByText('Sesi Anda berakhir. Silakan masuk kembali.'))
			.toBeInTheDocument();
	});

	it('submits credentials and displays 401 error message when login fails', async () => {
		vi.spyOn(window, 'fetch').mockResolvedValue(
			new Response(
				JSON.stringify({ status: 401, detail: 'Nama pengguna atau kata sandi salah.' }),
				{
					status: 401,
					headers: { 'content-type': 'application/json' }
				}
			)
		);

		render(Page, { data: {} });

		await page.getByLabelText('Nama pengguna').fill('andi.admin');
		await page.getByLabelText('Kata sandi').fill('wrong-password');
		await page.getByRole('button', { name: 'Masuk' }).click();

		await expect
			.element(page.getByText('Nama pengguna atau kata sandi salah.'))
			.toBeInTheDocument();
	});

	it('submits credentials and displays 429 throttle message with minutes from Retry-After', async () => {
		vi.spyOn(window, 'fetch').mockResolvedValue(
			new Response(null, {
				status: 429,
				headers: { 'retry-after': '180' }
			})
		);

		render(Page, { data: {} });

		await page.getByLabelText('Nama pengguna').fill('andi.admin');
		await page.getByLabelText('Kata sandi').fill('wrong-password');
		await page.getByRole('button', { name: 'Masuk' }).click();

		await expect
			.element(page.getByText('Terlalu banyak percobaan gagal. Coba lagi dalam 3 menit.'))
			.toBeInTheDocument();
	});

	it('submits valid credentials and navigates on success', async () => {
		vi.spyOn(window, 'fetch').mockResolvedValue(
			new Response(JSON.stringify({ stage: 'authenticated' }), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			})
		);

		render(Page, { data: {} });

		await page.getByLabelText('Nama pengguna').fill('andi.admin');
		await page.getByLabelText('Kata sandi').fill('valid-password-123');
		await page.getByRole('button', { name: 'Masuk' }).click();

		expect(mockGoto).toHaveBeenCalledWith('/admin');
	});
});
