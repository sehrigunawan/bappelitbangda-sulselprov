import { canManageUsers } from '#lib/admin/permissions.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const parentData = await parent();
	const canManage = canManageUsers(parentData.session);
	return {
		canManage
	};
};
