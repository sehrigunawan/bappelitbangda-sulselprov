<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { sanitizeKembaliUrl } from '#lib/admin/url.ts';
	import { createApiClient } from '#lib/http/client.ts';

	interface Props {
		data: {
			kembali?: string | null;
		};
	}

	let { data: pageData }: Props = $props();
	let useRecoveryCode = $state(false);
	let code = $state('');
	let recoveryCode = $state('');
	let errorMessage = $state('');
	let throttleMinutes = $state<number | null>(null);
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		errorMessage = '';
		throttleMinutes = null;

		try {
			const client = createApiClient({ baseUrl: window.location.origin });
			const body = useRecoveryCode ? { recoveryCode } : { code };

			const { data, response } = await client.POST('/api/v1/admin/auth/session/totp', {
				body
			});

			if (response.ok && data) {
				switch (data.stage) {
					case 'authenticated': {
						const target = sanitizeKembaliUrl(pageData.kembali ?? null);
						await goto(target);
						break;
					}
					case 'password_change_required':
						await goto(
							`${resolve('/admin/(masuk)/masuk/kata-sandi')}${pageData.kembali ? `?kembali=${encodeURIComponent(pageData.kembali)}` : ''}`
						);
						break;
					case 'totp_enrollment_required':
						await goto(
							`${resolve('/admin/(masuk)/masuk/autentikator')}${pageData.kembali ? `?kembali=${encodeURIComponent(pageData.kembali)}` : ''}`
						);
						break;
				}
			} else if (response.status === 401) {
				errorMessage = 'Kode tidak valid.';
			} else if (response.status === 429) {
				const retryAfter = Number(response.headers.get('retry-after')) || 900;
				throttleMinutes = Math.ceil(retryAfter / 60);
			} else {
				errorMessage = 'Terjadi kesalahan. Coba lagi.';
			}
		} catch {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="verification-container">
	<h2>Verifikasi Dua Langkah</h2>

	{#if throttleMinutes !== null}
		<div class="error-alert">
			Terlalu banyak percobaan gagal. Coba lagi dalam {throttleMinutes} menit.
		</div>
	{:else if errorMessage}
		<div class="error-alert">{errorMessage}</div>
	{/if}

	<form onsubmit={handleSubmit}>
		{#if !useRecoveryCode}
			<label>
				Kode autentikator (6 digit)
				<input
					type="text"
					name="code"
					inputmode="numeric"
					pattern="[0-9]{6}"
					maxlength="6"
					required
					bind:value={code}
					disabled={loading}
					autocomplete="one-time-code"
				/>
			</label>
		{:else}
			<label>
				Kode pemulihan
				<input
					type="text"
					name="recoveryCode"
					required
					bind:value={recoveryCode}
					disabled={loading}
					autocomplete="off"
				/>
			</label>
		{/if}

		<div class="toggle-link">
			<button
				type="button"
				class="link-button"
				onclick={() => {
					useRecoveryCode = !useRecoveryCode;
					errorMessage = '';
				}}
			>
				{useRecoveryCode ? 'Gunakan kode autentikator' : 'Gunakan kode pemulihan'}
			</button>
		</div>

		<button type="submit" disabled={loading}>
			{loading ? 'Memuat...' : 'Verifikasi'}
		</button>
	</form>
</div>
