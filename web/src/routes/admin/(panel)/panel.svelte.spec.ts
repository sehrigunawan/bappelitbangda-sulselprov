import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Layout from './+layout.svelte';
import PenggunaPage from './pengguna/+page.svelte';
import { fixtureSessionAuthenticated } from '#lib/admin/fixtures.ts';

describe('admin panel shell and permission navigation (W7)', () => {
	it('shows Beranda and Akun Saya to all sessions, and Pengguna only with users.manage', async () => {
		// Session with users.manage (Super Admin)
		render(Layout, {
			data: { session: fixtureSessionAuthenticated }
		});
		await expect.element(page.getByText('Beranda')).toBeInTheDocument();
		await expect.element(page.getByText('Akun Saya')).toBeInTheDocument();
		await expect.element(page.getByText('Pengguna')).toBeInTheDocument();
		await expect.element(page.getByText('Keluar')).toBeInTheDocument();
		await expect.element(page.getByText('Andi Pratama')).toBeInTheDocument();
	});

	it('hides Pengguna nav link when user lacks users.manage', async () => {
		const sessionWithoutUsersManage = {
			...fixtureSessionAuthenticated,
			permissions: [{ area: 'publications' as const, level: 'manage' as const }]
		};
		render(Layout, {
			data: { session: sessionWithoutUsersManage }
		});
		await expect.element(page.getByText('Beranda')).toBeInTheDocument();
		await expect.element(page.getByText('Akun Saya')).toBeInTheDocument();
		await expect.element(page.getByText('Pengguna')).not.toBeInTheDocument();
	});

	it('/admin/pengguna shows access denied message when session lacks users.manage', async () => {
		const sessionWithoutUsersManage = {
			...fixtureSessionAuthenticated,
			permissions: []
		};
		render(PenggunaPage, {
			data: { session: sessionWithoutUsersManage }
		});
		await expect
			.element(page.getByText('Anda tidak memiliki akses ke halaman ini.'))
			.toBeInTheDocument();
	});
});
