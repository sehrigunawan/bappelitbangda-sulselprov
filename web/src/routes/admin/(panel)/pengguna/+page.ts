import { createApiClient } from '#lib/http/client.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url, parent }) => {
	const parentData = await parent();
	const session = parentData.session;
	const canManageUsers = session?.permissions?.some(
		(p: { area: string; level: string }) => p.area === 'users' && p.level === 'manage'
	);

	if (!canManageUsers) {
		return {
			userPage: undefined
		};
	}

	const client = createApiClient({ baseUrl: url.origin, fetch });
	const page = Math.max(1, Number(url.searchParams.get('halaman')) || 1);
	const { data: userPage } = await client.GET('/api/v1/admin/auth/users', {
		params: {
			query: {
				page,
				pageSize: 10
			}
		}
	});

	return {
		userPage
	};
};
