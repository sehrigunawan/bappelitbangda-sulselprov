import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockGoto } = vi.hoisted(() => ({
	mockGoto: vi.fn().mockResolvedValue(undefined)
}));

vi.mock('$app/navigation', () => ({
	goto: mockGoto
}));

import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Layout from './+layout.svelte';
import { fixtureSessionAuthenticated } from '#lib/admin/fixtures.ts';

describe('logout functionality (W15)', () => {
	beforeEach(() => {
		mockGoto.mockClear();
		vi.restoreAllMocks();
	});

	it('renders Keluar button in the admin shell header', async () => {
		render(Layout, {
			data: { session: fixtureSessionAuthenticated }
		});

		await expect.element(page.getByRole('button', { name: 'Keluar' })).toBeInTheDocument();
	});

	it('calls DELETE /session and navigates to login when Keluar is clicked', async () => {
		const fetchSpy = vi
			.spyOn(window, 'fetch')
			.mockResolvedValue(new Response(null, { status: 204 }));

		render(Layout, {
			data: { session: fixtureSessionAuthenticated }
		});

		const logoutButton = page.getByRole('button', { name: 'Keluar' });
		await logoutButton.click();

		expect(fetchSpy).toHaveBeenCalled();
		const firstCall = fetchSpy.mock.calls[0];
		const req = firstCall[0] as Request;
		expect(req.url).toContain('/api/v1/admin/auth/session');
		expect(req.method).toBe('DELETE');
		expect(mockGoto).toHaveBeenCalled();
	});
});
