import { redirect } from '@sveltejs/kit';
import { createApiClient } from '#lib/http/client.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ fetch, url }) => {
	const client = createApiClient({ baseUrl: url.origin, fetch });
	const { data, response } = await client.GET('/api/v1/admin/auth/session');

	if (response.status === 401 || !data) {
		const target = url.pathname + url.search;
		throw redirect(307, `/admin/masuk?kembali=${encodeURIComponent(target)}`);
	}

	switch (data.stage) {
		case 'totp_required':
			throw redirect(307, '/admin/masuk/verifikasi');
		case 'password_change_required':
			throw redirect(307, '/admin/masuk/kata-sandi');
		case 'totp_enrollment_required':
			throw redirect(307, '/admin/masuk/autentikator');
		case 'authenticated':
			return { session: data };
		default:
			throw redirect(307, '/admin/masuk');
	}
};
