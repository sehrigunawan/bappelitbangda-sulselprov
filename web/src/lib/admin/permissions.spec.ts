import { describe, expect, it } from 'vitest';
import { canManageUsers, hasActiveStepUp } from './permissions';
import type { components } from '#lib/api/schema.d.ts';

type SessionState = components['schemas']['SessionState'];

describe('canManageUsers', () => {
	it('returns false for empty/null session or missing permissions', () => {
		expect(canManageUsers(null)).toBe(false);
		expect(canManageUsers(undefined)).toBe(false);
		expect(canManageUsers({} as SessionState)).toBe(false);
	});

	it('returns true when users.manage permission is present', () => {
		const session: SessionState = {
			user: { id: 1, username: 'admin', fullName: 'Admin', roles: ['super_admin'] },
			permissions: [
				{ area: 'users', level: 'manage' },
				{ area: 'documents', level: 'manage' }
			],
			totpEnrolled: true,
			stepUpExpiresAt: null,
			stage: 'authenticated'
		};
		expect(canManageUsers(session)).toBe(true);
	});

	it('returns false when users permission is only view', () => {
		const session: SessionState = {
			user: { id: 1, username: 'viewer', fullName: 'Viewer', roles: ['leadership'] },
			permissions: [{ area: 'users', level: 'view' }],
			totpEnrolled: true,
			stepUpExpiresAt: null,
			stage: 'authenticated'
		};
		expect(canManageUsers(session)).toBe(false);
	});
});

describe('hasActiveStepUp', () => {
	it('returns false when stepUpExpiresAt is missing', () => {
		expect(hasActiveStepUp(null)).toBe(false);
		expect(hasActiveStepUp(undefined)).toBe(false);
		expect(hasActiveStepUp({} as SessionState)).toBe(false);
	});

	it('returns true when stepUpExpiresAt is in the future', () => {
		const future = new Date(Date.now() + 60_000).toISOString();
		expect(hasActiveStepUp({ stepUpExpiresAt: future } as SessionState)).toBe(true);
	});

	it('returns false when stepUpExpiresAt is in the past', () => {
		const past = new Date(Date.now() - 60_000).toISOString();
		expect(hasActiveStepUp({ stepUpExpiresAt: past } as SessionState)).toBe(false);
	});
});
