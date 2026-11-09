import { redirect } from '@sveltejs/kit';
import type { components } from '#lib/api/schema.d.ts';
import { sanitizeKembaliUrl } from './url';

type SessionStage = components['schemas']['SessionStage'];
type SessionState = components['schemas']['SessionState'];

export function handleStageRedirect(
	expectedStage: SessionStage | 'unauthenticated',
	session: SessionState | null,
	kembaliUrl?: string | null
): void {
	if (!session) {
		if (expectedStage !== 'unauthenticated') {
			throw redirect(307, '/admin/masuk');
		}
		return;
	}

	if (session.stage === 'authenticated') {
		const target = sanitizeKembaliUrl(kembaliUrl ?? null);
		throw redirect(307, target);
	}

	if (session.stage !== expectedStage) {
		switch (session.stage) {
			case 'totp_required':
				throw redirect(307, '/admin/masuk/verifikasi');
			case 'password_change_required':
				throw redirect(307, '/admin/masuk/kata-sandi');
			case 'totp_enrollment_required':
				throw redirect(307, '/admin/masuk/autentikator');
			default:
				throw redirect(307, '/admin/masuk');
		}
	}
}
