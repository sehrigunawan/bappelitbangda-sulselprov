import { createApiClient, type ApiClient } from '#lib/http/client.ts';

export interface AdminClientOptions {
	baseUrl: string;
	fetch?: typeof window.fetch;
	onSessionLost?: () => void;
}

const INLINE_401_OPERATIONS: Record<string, true> = {
	'POST /api/v1/admin/auth/session': true,
	'POST /api/v1/admin/auth/session/totp': true,
	'POST /api/v1/admin/auth/session/step-up': true,
	'PUT /api/v1/admin/auth/password': true,
	'DELETE /api/v1/admin/auth/session': true
};

export function createAdminClient(options: AdminClientOptions): ApiClient {
	const originalFetch = options.fetch ?? window.fetch.bind(window);

	const interceptingFetch: typeof window.fetch = async (input, init) => {
		const response = await originalFetch(input, init);

		if (response.status === 401) {
			const method = (
				init?.method ?? (input instanceof Request ? input.method : 'GET')
			).toUpperCase();
			let pathname = '';
			if (typeof input === 'string') {
				pathname = new URL(input, options.baseUrl).pathname;
			} else if (input instanceof URL) {
				pathname = input.pathname;
			} else if (input instanceof Request) {
				pathname = new URL(input.url).pathname;
			}

			const opKey = `${method} ${pathname}`;
			if (!INLINE_401_OPERATIONS[opKey]) {
				options.onSessionLost?.();
			}
		}

		return response;
	};

	return createApiClient({
		baseUrl: options.baseUrl,
		fetch: interceptingFetch
	});
}
