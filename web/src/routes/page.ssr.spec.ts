import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import Page from './+page.svelte';

// NFR-04: the page content must be in the server-rendered HTML, without client JavaScript.
describe('+page.svelte (server render)', () => {
	it.each([
		['ok', 'berjalan'],
		['down', 'tidak tersedia']
	] as const)('renders the API status %s as "%s" in HTML', (apiStatus, label) => {
		const { body } = render(Page, { props: { data: { apiStatus }, params: {} } });

		expect(body).toContain(label);
		expect(body).toContain('Bappelitbangda Sulawesi Selatan');
	});
});
