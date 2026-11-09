import { describe, expect, it } from 'vitest';
import { load as loadMasuk } from './masuk/+page';
import { load as loadVerifikasi } from './masuk/verifikasi/+page';
import { load as loadKataSandi } from './masuk/kata-sandi/+page';
import { load as loadAutentikator } from './masuk/autentikator/+page';
import {
	fixtureSessionAuthenticated,
	fixtureSessionTotpRequired,
	fixtureSessionPasswordChangeRequired
} from '#lib/admin/fixtures.ts';

function mockEvent<T>(handler: (req: Request) => Response, path = '/admin/masuk'): T {
	return {
		fetch: async (input: RequestInfo | URL, init?: RequestInit) => {
			const req = input instanceof Request ? input : new Request(input, init);
			return handler(req);
		},
		url: new URL(`http://localhost:8080${path}`)
	} as unknown as T;
}

describe('stage pages mismatch guard (W2)', () => {
	it('redirects already authenticated session from /admin/masuk to /admin', async () => {
		const event = mockEvent<Parameters<typeof loadMasuk>[0]>(() =>
			Response.json(fixtureSessionAuthenticated)
		);
		await expect(async () => {
			await loadMasuk(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin'
		});
	});

	it('redirects unauthenticated (401) from /admin/masuk/verifikasi to /admin/masuk', async () => {
		const event = mockEvent<Parameters<typeof loadVerifikasi>[0]>(
			() => new Response(JSON.stringify({ status: 401 }), { status: 401 }),
			'/admin/masuk/verifikasi'
		);
		await expect(async () => {
			await loadVerifikasi(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk'
		});
	});

	it('redirects password_change_required session from /admin/masuk/verifikasi to /admin/masuk/kata-sandi', async () => {
		const event = mockEvent<Parameters<typeof loadVerifikasi>[0]>(
			() => Response.json(fixtureSessionPasswordChangeRequired),
			'/admin/masuk/verifikasi'
		);
		await expect(async () => {
			await loadVerifikasi(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk/kata-sandi'
		});
	});

	it('redirects totp_required session from /admin/masuk/kata-sandi to /admin/masuk/verifikasi', async () => {
		const event = mockEvent<Parameters<typeof loadKataSandi>[0]>(
			() => Response.json(fixtureSessionTotpRequired),
			'/admin/masuk/kata-sandi'
		);
		await expect(async () => {
			await loadKataSandi(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk/verifikasi'
		});
	});

	it('redirects totp_required session from /admin/masuk/autentikator to /admin/masuk/verifikasi', async () => {
		const event = mockEvent<Parameters<typeof loadAutentikator>[0]>(
			() => Response.json(fixtureSessionTotpRequired),
			'/admin/masuk/autentikator'
		);
		await expect(async () => {
			await loadAutentikator(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk/verifikasi'
		});
	});

	it('preserves kembali parameter across stage redirects', async () => {
		const event = mockEvent<Parameters<typeof loadAutentikator>[0]>(
			() => Response.json(fixtureSessionTotpRequired),
			'/admin/masuk/autentikator?kembali=%2Fadmin%2Fpengguna'
		);
		await expect(async () => {
			await loadAutentikator(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk/verifikasi?kembali=%2Fadmin%2Fpengguna'
		});
	});
});
