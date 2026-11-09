<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createApiClient } from '#lib/http/client.ts';
	import type { components } from '#lib/api/schema.d.ts';
	import { sanitizeKembaliUrl } from '#lib/admin/url.ts';
	import { parseRetryAfterMinutes, problemDetail } from '#lib/admin/problem.ts';
	type SessionState = components['schemas']['SessionState'];

	interface Props {
		data: {
			session?: SessionState;
			kembali?: string | null;
		};
	}

	let { data }: Props = $props();

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let errorMessage = $state('');
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		errorMessage = '';

		if (newPassword.length < 12) {
			errorMessage = 'Minimal 12 karakter.';
			return;
		}

		if (newPassword.length > 128) {
			errorMessage = 'Maksimal 128 karakter.';
			return;
		}

		if (newPassword !== confirmPassword) {
			errorMessage = 'Kata sandi baru tidak cocok.';
			return;
		}

		if (
			data.session?.user?.username &&
			newPassword.toLowerCase().includes(data.session.user.username.toLowerCase())
		) {
			errorMessage = 'Kata sandi tidak boleh memuat nama pengguna.';
			return;
		}

		loading = true;

		try {
			const client = createApiClient({ baseUrl: window.location.origin });
			const { response, error } = await client.PUT('/api/v1/admin/auth/password', {
				body: { currentPassword, newPassword }
			});

			if (response.ok) {
				const sessionRes = await client.GET('/api/v1/admin/auth/session');
				const kembaliParam = data.kembali ? `?kembali=${encodeURIComponent(data.kembali)}` : '';

				if (sessionRes.data?.stage === 'totp_enrollment_required') {
					// eslint-disable-next-line svelte/no-navigation-without-resolve
					await goto(`${resolve('/admin/(masuk)/masuk/autentikator')}${kembaliParam}`);
				} else {
					const target = sanitizeKembaliUrl(data.kembali ?? null);
					// eslint-disable-next-line svelte/no-navigation-without-resolve -- dynamic URL validated by sanitizeKembaliUrl
					await goto(target);
				}
			} else if (response.status === 400) {
				errorMessage = problemDetail(error, 'Kata sandi saat ini salah.');
			} else if (response.status === 429) {
				const minutes = parseRetryAfterMinutes(response);
				errorMessage = `Terlalu banyak percobaan gagal. Coba lagi dalam ${minutes} menit.`;
			} else if (response.status === 401) {
				const sessionRes = await client.GET('/api/v1/admin/auth/session');
				if (sessionRes.response.status === 401) {
					// eslint-disable-next-line svelte/no-navigation-without-resolve
					await goto('/admin/masuk?alasan=sesi-berakhir');
					return;
				}
				errorMessage = problemDetail(error);
			} else {
				errorMessage = problemDetail(error);
			}
		} catch {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="password-change-container">
	<h2>Ganti Kata Sandi</h2>

	{#if errorMessage}
		<div class="error-alert">{errorMessage}</div>
	{/if}

	<form onsubmit={handleSubmit} novalidate>
		<label>
			Kata sandi saat ini
			<input type="password" required bind:value={currentPassword} disabled={loading} />
		</label>

		<label>
			Kata sandi baru
			<input
				type="password"
				required
				minlength="12"
				maxlength="128"
				bind:value={newPassword}
				disabled={loading}
			/>
		</label>

		<label>
			Ulangi kata sandi baru
			<input
				type="password"
				required
				minlength="12"
				maxlength="128"
				bind:value={confirmPassword}
				disabled={loading}
			/>
		</label>

		<button type="submit" disabled={loading}>
			{loading ? 'Memuat...' : 'Simpan'}
		</button>
	</form>
</div>
