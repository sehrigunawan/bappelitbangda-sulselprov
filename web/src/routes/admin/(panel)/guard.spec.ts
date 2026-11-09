import { describe, expect, it } from 'vitest';
import { load } from './+layout';
import {
	fixtureSessionAuthenticated,
	fixtureSessionTotpRequired,
	fixtureSessionPasswordChangeRequired,
	fixtureSessionTotpEnrollmentRequired
} from '#lib/admin/fixtures.ts';

function mockLoadEvent(handler: (req: Request) => Response) {
	return {
		fetch: async (input: RequestInfo | URL, init?: RequestInit) => {
			const req = input instanceof Request ? input : new Request(input, init);
			return handler(req);
		},
		url: new URL('http://localhost:8080/admin/pengguna')
	} as unknown as Parameters<typeof load>[0];
}

describe('admin panel layout guard (W1)', () => {
	it('redirects 401 unauthenticated session to /admin/masuk?kembali=...', async () => {
		const event = mockLoadEvent(
			() => new Response(JSON.stringify({ status: 401 }), { status: 401 })
		);
		await expect(async () => {
			await load(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk?kembali=%2Fadmin%2Fpengguna'
		});
	});

	it('redirects totp_required stage to /admin/masuk/verifikasi', async () => {
		const event = mockLoadEvent(() => Response.json(fixtureSessionTotpRequired));
		await expect(async () => {
			await load(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk/verifikasi'
		});
	});

	it('redirects password_change_required stage to /admin/masuk/kata-sandi', async () => {
		const event = mockLoadEvent(() => Response.json(fixtureSessionPasswordChangeRequired));
		await expect(async () => {
			await load(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk/kata-sandi'
		});
	});

	it('redirects totp_enrollment_required stage to /admin/masuk/autentikator', async () => {
		const event = mockLoadEvent(() => Response.json(fixtureSessionTotpEnrollmentRequired));
		await expect(async () => {
			await load(event);
		}).rejects.toMatchObject({
			status: 307,
			location: '/admin/masuk/autentikator'
		});
	});

	it('allows authenticated session and returns session data', async () => {
		const event = mockLoadEvent(() => Response.json(fixtureSessionAuthenticated));
		const data = await load(event);
		expect(data).toMatchObject({
			session: fixtureSessionAuthenticated
		});
	});
});
