import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockGoto } = vi.hoisted(() => ({
	mockGoto: vi.fn().mockImplementation(() => new Promise(() => {}))
}));

vi.mock('$app/navigation', () => ({
	goto: mockGoto
}));

import { panelClient } from './panel-client';

describe('panelClient (C3)', () => {
	beforeEach(() => {
		mockGoto.mockClear();
	});

	it('triggers goto to login on 401 and deduplicates concurrent 401s', async () => {
		const client = panelClient(
			async () => new Response(JSON.stringify({ status: 401 }), { status: 401 })
		);

		await client.GET('/api/v1/admin/auth/users');
		await client.GET('/api/v1/admin/auth/users');

		expect(mockGoto).toHaveBeenCalledTimes(1);
		expect(mockGoto).toHaveBeenCalledWith('/admin/masuk?alasan=sesi-berakhir');
	});
});
