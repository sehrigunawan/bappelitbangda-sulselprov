import { describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import RecoveryCodesPanel from './RecoveryCodesPanel.svelte';

describe('RecoveryCodesPanel component', () => {
	it('disables Lanjutkan button until checkbox is checked, then invokes onconfirmed', async () => {
		const onconfirmed = vi.fn();
		const codes = ['code-1', 'code-2', 'code-3'];

		render(RecoveryCodesPanel, {
			codes,
			onconfirmed
		});

		await expect.element(page.getByText('code-1')).toBeInTheDocument();
		await expect.element(page.getByText('code-2')).toBeInTheDocument();
		await expect.element(page.getByText('code-3')).toBeInTheDocument();

		const button = page.getByRole('button', { name: 'Lanjutkan' });
		const checkbox = page.getByRole('checkbox', { name: 'Saya sudah menyimpan kode ini' });

		await expect.element(button).toBeDisabled();

		await checkbox.click();
		await expect.element(button).toBeEnabled();

		await button.click();
		expect(onconfirmed).toHaveBeenCalledTimes(1);
	});
});
