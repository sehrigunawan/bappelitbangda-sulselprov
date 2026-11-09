import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { fixtureSessionAuthenticated } from '#lib/admin/fixtures.ts';

describe('user list page component (W8)', () => {
	it('renders user list table headers and Indonesian role labels', async () => {
		render(Page, {
			data: {
				session: fixtureSessionAuthenticated,
				userPage: {
					items: [
						{
							id: 1,
							username: 'andi.admin',
							fullName: 'Andi Pratama',
							roles: ['super_admin'],
							active: true,
							totpEnrolled: true,
							lastLoginAt: '2026-10-15T06:05:00Z',
							createdAt: '2026-10-01T01:00:00Z'
						},
						{
							id: 2,
							username: 'sari.ppid',
							fullName: 'Sari Wulandari',
							roles: ['ppid_officer'],
							active: false,
							totpEnrolled: false,
							lastLoginAt: null,
							createdAt: '2026-10-02T01:00:00Z'
						}
					],
					page: 1,
					pageSize: 10,
					total: 2
				}
			}
		});

		await expect.element(page.getByText('Daftar Pengguna')).toBeInTheDocument();
		await expect.element(page.getByText('Tambah Pengguna')).toBeInTheDocument();
		await expect.element(page.getByText('Nama lengkap')).toBeInTheDocument();
		await expect.element(page.getByText('Peran')).toBeInTheDocument();
		await expect.element(page.getByText('Status')).toBeInTheDocument();
		await expect.element(page.getByText('Autentikator')).toBeInTheDocument();

		// Role labels
		await expect.element(page.getByText('Super Admin')).toBeInTheDocument();
		await expect.element(page.getByText('Petugas PPID')).toBeInTheDocument();

		// Status and TOTP labels
		await expect.element(page.getByText('Aktif')).toBeInTheDocument();
		await expect.element(page.getByText('Nonaktif')).toBeInTheDocument();
		await expect.element(page.getByText('Terdaftar')).toBeInTheDocument();
		await expect.element(page.getByText('Belum terdaftar')).toBeInTheDocument();

		// Pagination controls
		await expect.element(page.getByText('Sebelumnya')).toBeInTheDocument();
		await expect.element(page.getByText('Berikutnya')).toBeInTheDocument();
	});
});
