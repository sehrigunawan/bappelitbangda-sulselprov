import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

describe('user detail edit component (W14)', () => {
	it('renders user details, edit controls and reset buttons', async () => {
		render(Page, {
			data: {
				user: {
					id: 12,
					username: 'sari.ppid',
					fullName: 'Sari Wulandari',
					roles: ['ppid_officer'],
					active: true,
					totpEnrolled: true,
					lastLoginAt: '2026-10-15T06:05:00Z',
					createdAt: '2026-10-01T01:00:00Z'
				}
			},
			params: { id: '12' }
		});

		await expect.element(page.getByText('Sari Wulandari')).toBeInTheDocument();
		await expect.element(page.getByText('Peran')).toBeInTheDocument();
		await expect.element(page.getByText('Status')).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Simpan' })).toBeInTheDocument();

		// Reset action buttons
		await expect
			.element(page.getByRole('button', { name: 'Atur Ulang Kata Sandi' }))
			.toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'Atur Ulang Autentikator' }))
			.toBeInTheDocument();
	});
});
