import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

describe('+page.svelte', () => {
	it.each([
		['ok', 'berjalan'],
		['down', 'tidak tersedia']
	] as const)('shows the API status %s as "%s"', async (apiStatus, label) => {
		render(Page, { data: { apiStatus }, params: {} });

		await expect.element(page.getByText(label)).toBeInTheDocument();
	});
});
