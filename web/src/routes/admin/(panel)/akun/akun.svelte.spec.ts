import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { fixtureSessionAuthenticated } from '#lib/admin/fixtures.ts';

describe('account page component (W13)', () => {
	it('renders change password section and TOTP management options', async () => {
		render(Page, {
			data: { session: fixtureSessionAuthenticated }
		});

		await expect.element(page.getByText('Akun Saya')).toBeInTheDocument();
		await expect.element(page.getByText('Ganti Kata Sandi')).toBeInTheDocument();
		await expect.element(page.getByText('Kata sandi saat ini')).toBeInTheDocument();
		await expect.element(page.getByText('Kata sandi baru')).toBeInTheDocument();
		await expect.element(page.getByText('Ulangi kata sandi baru')).toBeInTheDocument();

		// When TOTP is already enrolled:
		await expect.element(page.getByText('Buat Ulang Kode Pemulihan')).toBeInTheDocument();
	});

	it('renders Aktifkan Autentikator button when user has not enrolled TOTP', async () => {
		const sessionNoTotp = {
			...fixtureSessionAuthenticated,
			totpEnrolled: false
		};
		render(Page, {
			data: { session: sessionNoTotp }
		});

		await expect.element(page.getByText('Aktifkan Autentikator')).toBeInTheDocument();
	});
});
