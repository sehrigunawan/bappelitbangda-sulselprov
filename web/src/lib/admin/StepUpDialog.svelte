<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import type { ApiClient } from '#lib/http/client.ts';
	import { parseRetryAfterMinutes, problemDetail } from '#lib/admin/problem.ts';

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
	function setupModal(node: HTMLDialogElement) {
		if (!node.open) {
			node.showModal();
		}
		const input = node.querySelector<HTMLInputElement>('input[name="code"]');
		input?.focus();

		return {
			destroy() {
				if (node.open) {
					node.close();
				}
			}
		};
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!client) {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
			return;
		}

		loading = true;
		errorMessage = '';

		try {
			const { data, response, error } = await client.POST('/api/v1/admin/auth/session/step-up', {
				body: { code }
			});

			if (response.ok && data) {
				code = '';
				await invalidateAll();
				onverified?.();
			} else if (response.status === 401) {
				const sessionRes = await client.GET('/api/v1/admin/auth/session');
				if (sessionRes.response.status === 401) {
					window.location.href = '/admin/masuk?alasan=sesi-berakhir';
				} else {
					errorMessage = problemDetail(error, 'Kode tidak valid.');
				}
			} else if (response.status === 429) {
				const minutes = parseRetryAfterMinutes(response);
				errorMessage = `Terlalu banyak percobaan gagal. Coba lagi dalam ${minutes} menit.`;
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

{#if open}
	<dialog
		use:setupModal
		class="step-up-dialog"
		aria-labelledby="step-up-title"
		oncancel={(e) => {
			e.preventDefault();
			oncancel?.();
		}}
	>
		<form onsubmit={handleSubmit}>
			<h3 id="step-up-title">Konfirmasi Identitas</h3>
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
					pattern={'[0-9]{6}'}
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
