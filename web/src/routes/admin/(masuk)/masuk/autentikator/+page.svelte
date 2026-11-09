<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { sanitizeKembaliUrl } from '#lib/admin/url.ts';
	import { createApiClient } from '#lib/http/client.ts';
	import RecoveryCodesPanel from '#lib/admin/RecoveryCodesPanel.svelte';

	interface Props {
		data: {
			enrollment?: {
				secret: string;
				otpauthUri: string;
				qrCodePng: string;
			} | null;
			recoveryCodes?: string[] | null;
			kembali?: string | null;
		};
	}

	let { data }: Props = $props();

	let fetchedEnrollment = $state<{
		secret: string;
		otpauthUri: string;
		qrCodePng: string;
	} | null>(null);
	let confirmedCodes = $state<string[] | null>(null);
	let enrollment = $derived(fetchedEnrollment ?? data.enrollment ?? null);
	let recoveryCodes = $derived(confirmedCodes ?? data.recoveryCodes ?? null);

	let code = $state('');
	let errorMessage = $state('');
	let loading = $state(false);

	onMount(() => {
		if (!enrollment && !recoveryCodes) {
			loadEnrollment();
		}
	});

	async function loadEnrollment() {
		try {
			const client = createApiClient({ baseUrl: window.location.origin });
			const { data: enrollData, response } = await client.POST('/api/v1/admin/auth/totp');
			if (response.ok && enrollData) {
				fetchedEnrollment = enrollData;
			} else {
				errorMessage = 'Terjadi kesalahan. Coba lagi.';
			}
		} catch {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
		}
	}

	async function handleConfirm(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		errorMessage = '';

		try {
			const client = createApiClient({ baseUrl: window.location.origin });
			const { data: confirmData, response } = await client.PUT('/api/v1/admin/auth/totp', {
				body: { code }
			});

			if (response.ok && confirmData) {
				confirmedCodes = confirmData.codes;
			} else if (response.status === 400) {
				errorMessage = 'Kode tidak valid.';
			} else {
				errorMessage = 'Terjadi kesalahan. Coba lagi.';
			}
		} catch {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			loading = false;
		}
	}

	async function handleComplete() {
		const target = sanitizeKembaliUrl(data.kembali ?? null);
		// eslint-disable-next-line svelte/no-navigation-without-resolve -- dynamic URL validated by sanitizeKembaliUrl
		await goto(target);
	}
</script>

<div class="enrollment-container">
	{#if recoveryCodes}
		<RecoveryCodesPanel codes={recoveryCodes} onconfirmed={handleComplete} />
	{:else}
		<h2>Aktifkan Autentikator</h2>
		<p>Pindai kode QR dengan aplikasi autentikator, lalu masukkan kode 6 digit.</p>

		{#if errorMessage}
			<div class="error-alert">{errorMessage}</div>
		{/if}

		{#if enrollment}
			<div class="qr-section">
				<img
					src={`data:image/png;base64,${enrollment.qrCodePng}`}
					alt="QR Code Autentikator"
					class="qr-code"
				/>
				<div class="manual-key">
					<span>Kunci manual</span>
					<code>{enrollment.secret}</code>
				</div>
			</div>
		{/if}

		<form onsubmit={handleConfirm}>
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

			<button type="submit" disabled={loading || code.length !== 6}>
				{loading ? 'Memuat...' : 'Verifikasi & Aktifkan'}
			</button>
		</form>
	{/if}
</div>
