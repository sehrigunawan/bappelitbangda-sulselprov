/**
 * Validates and sanitizes the `?kembali=` query parameter.
 * Only accepts '/admin' or paths starting with '/admin/' that do NOT start with '/admin/masuk'.
 * Rejects external URLs, protocol-relative URLs, and non-admin paths by returning '/admin'.
 */
export function sanitizeKembaliUrl(url: string | null): string {
	if (!url || typeof url !== 'string' || !url.startsWith('/') || url.startsWith('//')) {
		return '/admin';
	}

	try {
		const parsed = new URL(url, 'http://localhost');
		if (
			parsed.pathname === '/admin' ||
			(parsed.pathname.startsWith('/admin/') && !parsed.pathname.startsWith('/admin/masuk'))
		) {
			return parsed.pathname + parsed.search + parsed.hash;
		}
	} catch {
		return '/admin';
	}

	return '/admin';
}
