import type { components } from '#lib/api/schema.d.ts';

type SessionState = components['schemas']['SessionState'];

/**
 * Checks if the session has `users.manage` permission.
 */
export function canManageUsers(session?: SessionState | null): boolean {
	if (!session || !session.permissions) return false;
	return session.permissions.some((p) => p.area === 'users' && p.level === 'manage');
}

/**
 * Checks if the current session has an active (non-expired) step-up window.
 */
export function hasActiveStepUp(session?: SessionState | null): boolean {
	const expiresAt = session?.stepUpExpiresAt;
	if (!expiresAt) return false;
	return new Date(expiresAt).getTime() > Date.now();
}
