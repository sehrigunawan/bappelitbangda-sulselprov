import { error, redirect } from '@sveltejs/kit';
import { createApiClient } from '#lib/http/client.ts';
import { canManageUsers } from '#lib/admin/permissions.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url, parent }) => {
	const parentData = await parent();
	const canManage = canManageUsers(parentData.session);

	if (!canManage) {
		return {
			canManage: false,
			userPage: undefined
		};
	}

	const client = createApiClient({ baseUrl: url.origin, fetch });
	const page = Math.max(1, Math.floor(Number(url.searchParams.get('halaman')) || 1));
	const { data: userPage, response } = await client.GET('/api/v1/admin/auth/users', {
		params: {
			query: {
				page,
				pageSize: 10
			}
		}
	});

	if (response.status === 401) {
		throw redirect(307, '/admin/masuk?alasan=sesi-berakhir');
	}

	if (!response.ok) {
		throw error(response.status, 'Gagal memuat daftar pengguna.');
	}

	return {
		canManage: true,
		userPage
	};
};
