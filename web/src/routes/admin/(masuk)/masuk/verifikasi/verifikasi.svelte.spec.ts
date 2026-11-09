import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

describe('verification page component (W5)', () => {
	it('renders TOTP verification elements by default', async () => {
		render(Page, {});
		await expect.element(page.getByText('Verifikasi Dua Langkah')).toBeInTheDocument();
		await expect.element(page.getByText('Kode autentikator (6 digit)')).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Verifikasi' })).toBeInTheDocument();
		await expect.element(page.getByText('Gunakan kode pemulihan')).toBeInTheDocument();
	});

	it('toggles to recovery code input when clicking Gunakan kode pemulihan', async () => {
		render(Page, {});
		const toggleBtn = page.getByText('Gunakan kode pemulihan');
		await toggleBtn.click();

		await expect.element(page.getByText('Kode pemulihan')).toBeInTheDocument();
	});
});
