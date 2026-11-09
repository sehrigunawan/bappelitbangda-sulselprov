import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

describe('login page component (W4)', () => {
	it('renders login form elements and labels', async () => {
		render(Page, { data: {} });
		await expect.element(page.getByText('Masuk Panel Admin')).toBeInTheDocument();
		await expect.element(page.getByText('Nama pengguna')).toBeInTheDocument();
		await expect.element(page.getByText('Kata sandi')).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Masuk' })).toBeInTheDocument();
	});

	it('shows 401 error message when login fails', async () => {
		render(Page, { data: { errorMessage: 'Nama pengguna atau kata sandi salah.' } });
		await expect
			.element(page.getByText('Nama pengguna atau kata sandi salah.'))
			.toBeInTheDocument();
	});

	it('shows 429 throttle message with minutes calculated from Retry-After', async () => {
		render(Page, { data: { throttleMinutes: 15 } });
		await expect
			.element(page.getByText('Terlalu banyak percobaan gagal. Coba lagi dalam 15 menit.'))
			.toBeInTheDocument();
	});

	it('shows session expired message when alasan=sesi-berakhir', async () => {
		render(Page, { data: { alasan: 'sesi-berakhir' } });
		await expect
			.element(page.getByText('Sesi Anda berakhir. Silakan masuk kembali.'))
			.toBeInTheDocument();
	});
});
