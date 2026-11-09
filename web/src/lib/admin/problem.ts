/**
 * Extracts the `detail` message from an RFC 9457 Problem error object.
 */
export function problemDetail(error: unknown, fallback = 'Terjadi kesalahan. Coba lagi.'): string {
	if (error && typeof error === 'object' && 'detail' in error && typeof error.detail === 'string') {
		return error.detail;
	}
	return fallback;
}

/**
 * Checks if a 403 response specifically requires step-up authentication.
 */
export function isStepUpRequired(error: unknown): boolean {
	return problemDetail(error, '') === 'Verifikasi ulang dengan kode autentikator diperlukan.';
}

/**
 * Parses the Retry-After response header into whole minutes (ceiling).
 */
export function parseRetryAfterMinutes(response: Response, defaultSeconds = 900): number {
	const header = response.headers.get('retry-after');
	const seconds = header ? parseInt(header, 10) : defaultSeconds;
	return Math.ceil((isNaN(seconds) || seconds <= 0 ? defaultSeconds : seconds) / 60);
}
