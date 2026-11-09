import { createApiClient } from '#lib/http/client.ts';
import { handleStageRedirect } from '#lib/admin/stage.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const client = createApiClient({ baseUrl: url.origin, fetch });
	const { data } = await client.GET('/api/v1/admin/auth/session');

	handleStageRedirect('totp_enrollment_required', data ?? null, url.searchParams.get('kembali'));

	return {
		session: data,
		kembali: url.searchParams.get('kembali')
	};
};
