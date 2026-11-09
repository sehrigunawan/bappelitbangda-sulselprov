<script lang="ts">
	import type { ApiClient } from '#lib/http/client.ts';

	interface Props {
		open?: boolean;
		client?: ApiClient;
		onverified?: () => void;
		oncancel?: () => void;
	}

	let { open = false, client, onverified, oncancel }: Props = $props();

	let code = $state('');
	let errorMessage = $state('');
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!client) {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
			return;
		}

		loading = true;
		errorMessage = '';

		try {
			const { data, response } = await client.POST('/api/v1/admin/auth/session/step-up', {
				body: { code }
			});

			if (response.ok && data) {
				code = '';
				onverified?.();
			} else if (response.status === 401) {
				const sessionRes = await client.GET('/api/v1/admin/auth/session');
				if (sessionRes.response.status === 401) {
					window.location.href = '/admin/masuk?alasan=sesi-berakhir';
				} else {
					errorMessage = 'Kode tidak valid.';
				}
			} else if (response.status === 429) {
				const retryAfter = Number(response.headers.get('retry-after')) || 900;
				const minutes = Math.ceil(retryAfter / 60);
				errorMessage = `Terlalu banyak percobaan gagal. Coba lagi dalam ${minutes} menit.`;
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

{#if open}
	<dialog open class="step-up-dialog">
		<form onsubmit={handleSubmit}>
			<h3>Konfirmasi Identitas</h3>
			<p>Masukkan kode autentikator untuk melanjutkan.</p>

			{#if errorMessage}
				<div class="error-alert">{errorMessage}</div>
			{/if}

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

			<div class="dialog-actions">
				<button type="button" onclick={() => oncancel?.()} disabled={loading}> Batal </button>
				<button type="submit" disabled={loading || code.length !== 6}>
					{loading ? 'Memuat...' : 'Verifikasi'}
				</button>
			</div>
		</form>
	</dialog>
{/if}
