import { describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import StepUpDialog from './StepUpDialog.svelte';

describe('StepUpDialog component (W10)', () => {
	it('renders dialog elements when open is true', async () => {
		render(StepUpDialog, {
			open: true,
			onverified: vi.fn(),
			oncancel: vi.fn()
		});

		await expect.element(page.getByText('Konfirmasi Identitas')).toBeInTheDocument();
		await expect
			.element(page.getByText('Masukkan kode autentikator untuk melanjutkan.'))
			.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Verifikasi' })).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Batal' })).toBeInTheDocument();
	});

	it('does not render content when open is false', async () => {
		render(StepUpDialog, {
			open: false,
			onverified: vi.fn(),
			oncancel: vi.fn()
		});

		await expect.element(page.getByText('Konfirmasi Identitas')).not.toBeInTheDocument();
	});
});
