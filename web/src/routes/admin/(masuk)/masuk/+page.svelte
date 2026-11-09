<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createApiClient } from '#lib/http/client.ts';
	import { sanitizeKembaliUrl } from '#lib/admin/url.ts';

	interface Props {
		data: {
			alasan?: string | null;
			kembali?: string | null;
			errorMessage?: string | null;
			throttleMinutes?: number | null;
		};
	}

	let { data }: Props = $props();

	let username = $state('');
	let password = $state('');
	let manualError = $state('');
	let manualThrottle = $state<number | null>(null);
	let errorMessage = $derived(manualError || (data.errorMessage ?? ''));
	let throttleMinutes = $derived(manualThrottle ?? data.throttleMinutes ?? null);
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		manualError = '';
		manualThrottle = null;

		try {
			const client = createApiClient({ baseUrl: window.location.origin });
			const { data: sessionData, response } = await client.POST('/api/v1/admin/auth/session', {
				body: { username, password }
			});

			if (response.ok && sessionData) {
				const kembaliParam = data.kembali ? `?kembali=${encodeURIComponent(data.kembali)}` : '';
				switch (sessionData.stage) {
					case 'authenticated': {
						const target = sanitizeKembaliUrl(data.kembali ?? null);
						// eslint-disable-next-line svelte/no-navigation-without-resolve -- dynamic URL validated by sanitizeKembaliUrl
						await goto(target);
						break;
					}
					case 'totp_required':
						// eslint-disable-next-line svelte/no-navigation-without-resolve
						await goto(`${resolve('/admin/(masuk)/masuk/verifikasi')}${kembaliParam}`);
						break;
					case 'password_change_required':
						// eslint-disable-next-line svelte/no-navigation-without-resolve
						await goto(`${resolve('/admin/(masuk)/masuk/kata-sandi')}${kembaliParam}`);
						break;
					case 'totp_enrollment_required':
						// eslint-disable-next-line svelte/no-navigation-without-resolve
						await goto(`${resolve('/admin/(masuk)/masuk/autentikator')}${kembaliParam}`);
						break;
				}
			} else if (response.status === 401) {
				manualError = 'Nama pengguna atau kata sandi salah.';
			} else if (response.status === 429) {
				const retryAfter = Number(response.headers.get('retry-after')) || 900;
				manualThrottle = Math.ceil(retryAfter / 60);
			} else {
				manualError = 'Terjadi kesalahan. Coba lagi.';
			}
		} catch {
			manualError = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="login-container">
	<h2>Masuk Panel Admin</h2>

	{#if data.alasan === 'sesi-berakhir'}
		<div class="info-alert">Sesi Anda berakhir. Silakan masuk kembali.</div>
	{/if}

	{#if throttleMinutes !== null}
		<div class="error-alert">
			Terlalu banyak percobaan gagal. Coba lagi dalam {throttleMinutes} menit.
		</div>
	{:else if errorMessage}
		<div class="error-alert">{errorMessage}</div>
	{/if}

	<form onsubmit={handleSubmit}>
		<label>
			Nama pengguna
			<input
				type="text"
				name="username"
				bind:value={username}
				required
				autocomplete="username"
				disabled={loading}
			/>
		</label>

		<label>
			Kata sandi
			<input
				type="password"
				name="password"
				bind:value={password}
				required
				autocomplete="current-password"
				disabled={loading}
			/>
		</label>

		<button type="submit" disabled={loading}>
			{loading ? 'Memuat...' : 'Masuk'}
		</button>
	</form>
</div>
