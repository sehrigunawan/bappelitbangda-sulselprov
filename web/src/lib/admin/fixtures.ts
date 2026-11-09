import type { components } from '#lib/api/schema.d.ts';

type SessionState = components['schemas']['SessionState'];

export const fixtureSessionAuthenticated: SessionState = {
	stage: 'authenticated',
	user: {
		id: 1,
		username: 'andi.admin',
		fullName: 'Andi Pratama',
		roles: ['super_admin']
	},
	permissions: [
		{ area: 'publications', level: 'manage' },
		{ area: 'pages', level: 'manage' },
		{ area: 'documents', level: 'manage' },
		{ area: 'site_content', level: 'manage' },
		{ area: 'media', level: 'manage' },
		{ area: 'ppid', level: 'manage' },
		{ area: 'users', level: 'manage' },
		{ area: 'activity_log', level: 'view' },
		{ area: 'statistics', level: 'view' }
	],
	totpEnrolled: true,
	stepUpExpiresAt: '2026-10-15T06:15:00Z'
};

export const fixtureSessionTotpRequired: SessionState = {
	stage: 'totp_required',
	user: {
		id: 1,
		username: 'andi.admin',
		fullName: 'Andi Pratama',
		roles: ['super_admin']
	},
	permissions: [],
	totpEnrolled: true,
	stepUpExpiresAt: null
};

export const fixtureSessionPasswordChangeRequired: SessionState = {
	stage: 'password_change_required',
	user: {
		id: 12,
		username: 'sari.ppid',
		fullName: 'Sari Wulandari',
		roles: ['ppid_officer']
	},
	permissions: [],
	totpEnrolled: false,
	stepUpExpiresAt: null
};

export const fixtureSessionTotpEnrollmentRequired: SessionState = {
	stage: 'totp_enrollment_required',
	user: {
		id: 12,
		username: 'sari.ppid',
		fullName: 'Sari Wulandari',
		roles: ['ppid_officer']
	},
	permissions: [],
	totpEnrolled: false,
	stepUpExpiresAt: null
};
