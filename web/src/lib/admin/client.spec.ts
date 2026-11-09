import { describe, expect, it, vi } from 'vitest';
import { createAdminClient } from './client';

describe('createAdminClient session loss handling (W11)', () => {
	it('intercepts 401 from a regular admin call and triggers onSessionLost', async () => {
		const onSessionLost = vi.fn();
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () => new Response(JSON.stringify({ status: 401 }), { status: 401 }),
			onSessionLost
		});

		await client.GET('/api/v1/admin/auth/users');
		expect(onSessionLost).toHaveBeenCalledTimes(1);
	});

	it('does NOT trigger onSessionLost for 401 from createSession (inline handled)', async () => {
		const onSessionLost = vi.fn();
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () => new Response(JSON.stringify({ status: 401 }), { status: 401 }),
			onSessionLost
		});

		await client.POST('/api/v1/admin/auth/session', {
			body: { username: 'andi.admin', password: 'wrong-password' }
		});
		expect(onSessionLost).not.toHaveBeenCalled();
	});

	it('does NOT trigger onSessionLost for 401 from createTotpVerification (inline handled)', async () => {
		const onSessionLost = vi.fn();
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () => new Response(JSON.stringify({ status: 401 }), { status: 401 }),
			onSessionLost
		});

		await client.POST('/api/v1/admin/auth/session/totp', {
			body: { code: '123456' }
		});
		expect(onSessionLost).not.toHaveBeenCalled();
	});
	it('does NOT trigger onSessionLost for 401 from createStepUpSession (inline handled)', async () => {
		const onSessionLost = vi.fn();
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () => new Response(JSON.stringify({ status: 401 }), { status: 401 }),
			onSessionLost
		});

		await client.POST('/api/v1/admin/auth/session/step-up', {
			body: { code: '123456' }
		});
		expect(onSessionLost).not.toHaveBeenCalled();
	});

	it('does NOT trigger onSessionLost for 401 from updatePassword (inline handled)', async () => {
		const onSessionLost = vi.fn();
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () => new Response(JSON.stringify({ status: 401 }), { status: 401 }),
			onSessionLost
		});

		await client.PUT('/api/v1/admin/auth/password', {
			body: { currentPassword: 'wrong', newPassword: 'new-valid-password-123' }
		});
		expect(onSessionLost).not.toHaveBeenCalled();
	});

	it('does NOT trigger onSessionLost for 401 from deleteSession (counts as logout)', async () => {
		const onSessionLost = vi.fn();
		const client = createAdminClient({
			baseUrl: 'http://localhost:8080',
			fetch: async () => new Response(JSON.stringify({ status: 401 }), { status: 401 }),
			onSessionLost
		});

		await client.DELETE('/api/v1/admin/auth/session');
		expect(onSessionLost).not.toHaveBeenCalled();
	});
});
