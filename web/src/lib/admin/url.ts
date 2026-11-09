/**
 * Validates and sanitizes the `?kembali=` query parameter.
 * Only accepts '/admin' or paths starting with '/admin/' that do NOT start with '/admin/masuk'.
 * Rejects external URLs, protocol-relative URLs, and non-admin paths by returning '/admin'.
 */
export function sanitizeKembaliUrl(url: string | null): string {
	if (!url) {
		return '/admin';
	}

	// Reject external, protocol-relative or non-admin URLs
	if (url.startsWith('//') || url.includes('://')) {
		return '/admin';
	}

	if (url === '/admin') {
		return '/admin';
	}

	if (url.startsWith('/admin/') && !url.startsWith('/admin/masuk')) {
		return url;
	}

	return '/admin';
}
