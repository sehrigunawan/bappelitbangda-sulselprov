import { describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

describe('forced enrollment page component (W6)', () => {
	it('renders QR code and manual key step initially', async () => {
		render(Page, {
			data: {
				enrollment: {
					secret: 'JBSWY3DPEHPK3PXP',
					otpauthUri: 'otpauth://totp/...',
					qrCodePng: 'iVBORw0KGgoAAAANSUhEUgAA...'
				}
			}
		});
		await expect.element(page.getByText('Aktifkan Autentikator')).toBeInTheDocument();
		await expect.element(page.getByText('Kunci manual')).toBeInTheDocument();
		await expect
			.element(
				page.getByText('Pindai kode QR dengan aplikasi autentikator, lalu masukkan kode 6 digit.')
			)
			.toBeInTheDocument();
	});

	it('renders recovery codes and requires confirmation checkbox to continue', async () => {
		render(Page, {
			data: {
				recoveryCodes: [
					'7k2mp-x9qrt',
					'3hv8n-c4wze',
					'q0d5j-m6tya',
					'b8r2f-n7kxs',
					'z4c6w-p1hmd',
					'e9t3g-v5jqa',
					'm2x7s-k8bnf',
					'r6d1z-w3pce',
					'h5q9v-t0gjm',
					'n1f4k-y2rsx'
				]
			}
		});
		await expect.element(page.getByText('Kode Pemulihan')).toBeInTheDocument();
		await expect
			.element(
				page.getByText(
					'Simpan kode ini di tempat aman. Setiap kode hanya dapat dipakai sekali dan tidak akan ditampilkan lagi.'
				)
			)
			.toBeInTheDocument();
		await expect.element(page.getByText('Saya sudah menyimpan kode ini')).toBeInTheDocument();
	});
});
