import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Layout from './+layout.svelte';
import { fixtureSessionAuthenticated } from '#lib/admin/fixtures.ts';

describe('logout functionality (W15)', () => {
	it('renders Keluar button in the admin shell header', async () => {
		render(Layout, {
			data: { session: fixtureSessionAuthenticated }
		});

		await expect.element(page.getByRole('button', { name: 'Keluar' })).toBeInTheDocument();
	});
});
