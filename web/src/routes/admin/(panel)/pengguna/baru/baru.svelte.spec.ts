import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

describe('create user page component (W9)', () => {
	it('renders create user form fields', async () => {
		render(Page, {
			data: {}
		});

		await expect.element(page.getByText('Tambah Pengguna')).toBeInTheDocument();
		await expect.element(page.getByText('Nama pengguna')).toBeInTheDocument();
		await expect.element(page.getByText('Nama lengkap')).toBeInTheDocument();
		await expect.element(page.getByText('Peran')).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Simpan' })).toBeInTheDocument();
	});

	it('shows temporary password notice once created', async () => {
		render(Page, {
			data: {
				createdResult: {
					user: {
						id: 12,
						username: 'sari.ppid',
						fullName: 'Sari Wulandari',
						roles: ['ppid_officer'],
						active: true,
						totpEnrolled: false,
						lastLoginAt: null,
						createdAt: '2026-10-01T01:00:00Z'
					},
					temporaryPassword: 'k7m2x9qrt3hv8nc4'
				}
			}
		});

		await expect
			.element(
				page.getByText(
					'Kata sandi sementara: k7m2x9qrt3hv8nc4. Berikan langsung kepada pengguna; kata sandi ini tidak akan ditampilkan lagi.'
				)
			)
			.toBeInTheDocument();
	});
});
