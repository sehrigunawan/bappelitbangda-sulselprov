import { error, redirect } from '@sveltejs/kit';
import { createApiClient } from '#lib/http/client.ts';
import { canManageUsers } from '#lib/admin/permissions.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url, parent }) => {
	const parentData = await parent();
	const canManage = canManageUsers(parentData.session);

	if (!canManage) {
		return {
			canManage: false,
			user: undefined
		};
	}

	const userId = Number(params.id);
	if (!userId || userId < 1) {
		throw error(404, 'Pengguna tidak ditemukan.');
	}

	const client = createApiClient({ baseUrl: url.origin, fetch });
	const { data: user, response } = await client.GET('/api/v1/admin/auth/users/{userId}', {
		params: { path: { userId } }
	});

	if (response.status === 401) {
		throw redirect(307, '/admin/masuk?alasan=sesi-berakhir');
	}

	if (response.status === 404 || !user) {
		throw error(404, 'Pengguna tidak ditemukan.');
	}

	if (!response.ok) {
		throw error(response.status, 'Gagal memuat pengguna.');
	}

	return {
		canManage: true,
		user
	};
};
