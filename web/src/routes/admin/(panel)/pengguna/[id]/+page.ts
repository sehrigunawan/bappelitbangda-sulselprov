import { error } from '@sveltejs/kit';
import { createApiClient } from '#lib/http/client.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url, parent }) => {
	const parentData = await parent();
	const session = parentData.session;
	const canManageUsers = session?.permissions?.some(
		(p: { area: string; level: string }) => p.area === 'users' && p.level === 'manage'
	);

	if (!canManageUsers) {
		return {
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

	if (response.status === 404 || !user) {
		throw error(404, 'Pengguna tidak ditemukan.');
	}

	return {
		user
	};
};
