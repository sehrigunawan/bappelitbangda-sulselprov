import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockInvalidateAll } = vi.hoisted(() => ({
	mockInvalidateAll: vi.fn().mockResolvedValue(undefined)
}));

vi.mock('$app/navigation', () => ({
	invalidateAll: mockInvalidateAll
}));

import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import StepUpDialog from './StepUpDialog.svelte';
import { createAdminClient } from './client';

describe('StepUpDialog component (W10)', () => {
	beforeEach(() => {
		mockInvalidateAll.mockClear();
		vi.restoreAllMocks();
	});

	it('renders accessible modal dialog elements when open is true', async () => {
		render(StepUpDialog, {
			open: true,
			onverified: vi.fn(),
			oncancel: vi.fn()
		});

		const heading = page.getByText('Konfirmasi Identitas');
		await expect.element(heading).toBeInTheDocument();
		await expect
			.element(page.getByText('Masukkan kode autentikator untuk melanjutkan.'))
			.toBeInTheDocument();

		const codeInput = page.getByLabelText('Kode autentikator (6 digit)');
		await expect.element(codeInput).toBeInTheDocument();
		expect(codeInput.element().getAttribute('pattern')).toBe('[0-9]{6}');
		expect(codeInput.element().getAttribute('maxlength')).toBe('6');

		const verifyButton = page.getByRole('button', { name: 'Verifikasi' });
		const cancelButton = page.getByRole('button', { name: 'Batal' });

		await expect.element(verifyButton).toBeInTheDocument();
		await expect.element(verifyButton).toBeDisabled();
		await expect.element(cancelButton).toBeInTheDocument();
	});

	it('does not render content when open is false', async () => {
		render(StepUpDialog, {
			open: false,
			onverified: vi.fn(),
			oncancel: vi.fn()
		});

		await expect.element(page.getByText('Konfirmasi Identitas')).not.toBeInTheDocument();
	});

	it('enables submit button only when 6 digits are entered', async () => {
		render(StepUpDialog, {
			open: true,
			onverified: vi.fn(),
			oncancel: vi.fn()
		});

		const codeInput = page.getByLabelText('Kode autentikator (6 digit)');
		const verifyButton = page.getByRole('button', { name: 'Verifikasi' });

		await codeInput.fill('12345');
		await expect.element(verifyButton).toBeDisabled();

		await codeInput.fill('123456');
		await expect.element(verifyButton).toBeEnabled();
	});

	it('submits code, calls invalidateAll, and triggers onverified on success', async () => {
		const onverified = vi.fn();
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () =>
				new Response(JSON.stringify({ stepUpExpiresAt: '2026-10-15T06:15:00Z' }), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				})
		});

		render(StepUpDialog, {
			open: true,
			client,
			onverified
		});

		await page.getByLabelText('Kode autentikator (6 digit)').fill('123456');
		await page.getByRole('button', { name: 'Verifikasi' }).click();

		await expect.poll(() => onverified).toHaveBeenCalledTimes(1);
		expect(mockInvalidateAll).toHaveBeenCalledTimes(1);
	});

	it('displays error message when verification code is invalid (401)', async () => {
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async (input) => {
				const url = typeof input === 'string' ? input : input instanceof Request ? input.url : '';
				if (url.includes('/session/step-up')) {
					return new Response(JSON.stringify({ status: 401, detail: 'Kode tidak valid.' }), {
						status: 401,
						headers: { 'content-type': 'application/json' }
					});
				}
				// session check
				return new Response(JSON.stringify({ stage: 'authenticated' }), { status: 200 });
			}
		});

		render(StepUpDialog, {
			open: true,
			client
		});

		await page.getByLabelText('Kode autentikator (6 digit)').fill('000000');
		await page.getByRole('button', { name: 'Verifikasi' }).click();

		await expect.element(page.getByText('Kode tidak valid.')).toBeInTheDocument();
	});

	it('displays throttle error message on 429 response', async () => {
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () =>
				new Response(null, {
					status: 429,
					headers: { 'retry-after': '300' }
				})
		});

		render(StepUpDialog, {
			open: true,
			client
		});

		await page.getByLabelText('Kode autentikator (6 digit)').fill('999999');
		await page.getByRole('button', { name: 'Verifikasi' }).click();

		await expect
			.element(page.getByText('Terlalu banyak percobaan gagal. Coba lagi dalam 5 menit.'))
			.toBeInTheDocument();
	});

	it('invokes oncancel when Batal button is clicked', async () => {
		const oncancel = vi.fn();
		render(StepUpDialog, {
			open: true,
			oncancel
		});

		await page.getByRole('button', { name: 'Batal' }).click();
		expect(oncancel).toHaveBeenCalledTimes(1);
	});
});
