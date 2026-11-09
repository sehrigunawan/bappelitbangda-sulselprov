import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import RoleBadge from './RoleBadge.svelte';
import type { components } from '#lib/api/schema.d.ts';

type Role = components['schemas']['Role'];

describe('RoleBadge component', () => {
	const roleExpectations: [Role, string][] = [
		['super_admin', 'Super Admin'],
		['humas_editor', 'Editor Humas'],
		['document_operator', 'Operator Dokumen'],
		['ppid_officer', 'Petugas PPID'],
		['leadership', 'Pimpinan']
	];

	it.each(roleExpectations)('renders role label "%s" as "%s"', async (role, expectedLabel) => {
		render(RoleBadge, { role });
		await expect.element(page.getByText(expectedLabel)).toBeInTheDocument();
	});
});
