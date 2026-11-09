/* eslint-disable svelte/no-navigation-without-resolve */
import { goto } from '$app/navigation';
import { createAdminClient } from './client.ts';
import type { ApiClient } from '#lib/http/client.ts';

let navigating = false;

export function panelClient(fetchFn?: typeof window.fetch): ApiClient {
	return createAdminClient({
		baseUrl: typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080',
		fetch: fetchFn,
		onSessionLost: () => {
			if (navigating) return;
			navigating = true;
			Promise.resolve(goto('/admin/masuk?alasan=sesi-berakhir')).finally(() => {
				navigating = false;
			});
		}
	});
}
