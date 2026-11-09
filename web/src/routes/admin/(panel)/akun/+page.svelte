<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { goto } from '$app/navigation';
	import { panelClient } from '#lib/admin/panel-client.ts';
	import type { components } from '#lib/api/schema.d.ts';
	import StepUpDialog from '#lib/admin/StepUpDialog.svelte';
	import RecoveryCodesPanel from '#lib/admin/RecoveryCodesPanel.svelte';
	import { parseRetryAfterMinutes, problemDetail, isStepUpRequired } from '#lib/admin/problem.ts';
	import { hasActiveStepUp } from '#lib/admin/permissions.ts';

	type SessionState = components['schemas']['SessionState'];

	interface Props {
		data: {
			session?: SessionState;
		};
	}

	let { data }: Props = $props();

	const session = $derived(data.session);
	const totpEnrolled = $derived(session?.totpEnrolled ?? false);

	// Change password state
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let passwordLoading = $state(false);
	let passwordSuccess = $state('');
	let passwordError = $state('');

	// TOTP state
	let stepUpOpen = $state(false);
	let totpLoading = $state(false);
	let totpMessage = $state('');
	let totpError = $state('');
	let newRecoveryCodes = $state<string[] | null>(null);

	// TOTP Enrollment flow for non-enrolled user
	let enrolling = $state(false);
	let enrollment = $state<{
		secret: string;
		otpauthUri: string;
		qrCodePng: string;
	} | null>(null);
	let enrollCode = $state('');

	async function handlePasswordChange(e: SubmitEvent) {
		e.preventDefault();
		passwordSuccess = '';
		passwordError = '';

		if (newPassword.length < 12) {
			passwordError = 'Minimal 12 karakter.';
			return;
		}

		if (newPassword !== confirmPassword) {
			passwordError = 'Kata sandi baru tidak cocok.';
			return;
		}

		if (
			session?.user?.username &&
			newPassword.toLowerCase().includes(session.user.username.toLowerCase())
		) {
			passwordError = 'Kata sandi tidak boleh memuat nama pengguna.';
			return;
		}

		passwordLoading = true;
		try {
			const client = panelClient();
			const { response, error } = await client.PUT('/api/v1/admin/auth/password', {
				body: { currentPassword, newPassword }
			});

			if (response.ok) {
				passwordSuccess = 'Kata sandi berhasil diganti.';
				currentPassword = '';
				newPassword = '';
				confirmPassword = '';
			} else if (response.status === 400) {
				passwordError = problemDetail(error, 'Kata sandi saat ini salah.');
			} else if (response.status === 429) {
				const minutes = parseRetryAfterMinutes(response);
				passwordError = `Terlalu banyak percobaan gagal. Coba lagi dalam ${minutes} menit.`;
			} else if (response.status === 401) {
				const sessionRes = await client.GET('/api/v1/admin/auth/session');
				if (sessionRes.response.status === 401) {
					await goto('/admin/masuk?alasan=sesi-berakhir');
					return;
				}
				passwordError = problemDetail(error);
			} else {
				passwordError = problemDetail(error);
			}
		} catch {
			passwordError = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			passwordLoading = false;
		}
	}

	async function startEnrollment() {
		enrolling = true;
		totpError = '';
		totpLoading = true;

		try {
			const client = panelClient();
			const { data: enrollData, response } = await client.POST('/api/v1/admin/auth/totp');
			if (response.ok && enrollData) {
				enrollment = enrollData;
			} else {
				totpError = 'Terjadi kesalahan. Coba lagi.';
				enrolling = false;
			}
		} catch {
			totpError = 'Terjadi kesalahan. Coba lagi.';
			enrolling = false;
		} finally {
			totpLoading = false;
		}
	}

	async function confirmEnrollment(e: SubmitEvent) {
		e.preventDefault();
		totpLoading = true;
		totpError = '';

		try {
			const client = panelClient();
			const { data: confirmData, response } = await client.PUT('/api/v1/admin/auth/totp', {
				body: { code: enrollCode }
			});

			if (response.ok && confirmData) {
				newRecoveryCodes = confirmData.codes;
				totpMessage = 'Autentikator berhasil diaktifkan.';
				enrolling = false;
				enrollment = null;
			} else if (response.status === 400) {
				totpError = 'Kode tidak valid.';
			} else {
				totpError = 'Terjadi kesalahan. Coba lagi.';
			}
		} catch {
			totpError = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			totpLoading = false;
		}
	}

	function requestRegenerateRecoveryCodes() {
		if (hasActiveStepUp(session)) {
			executeRegenerateRecoveryCodes();
		} else {
			stepUpOpen = true;
		}
	}

	async function executeRegenerateRecoveryCodes() {
		stepUpOpen = false;
		totpLoading = true;
		totpError = '';

		try {
			const client = panelClient();
			const {
				data: codesRes,
				response,
				error
			} = await client.POST('/api/v1/admin/auth/totp/recovery-codes');

			if (response.ok && codesRes) {
				newRecoveryCodes = codesRes.codes;
			} else if (response.status === 403) {
				if (isStepUpRequired(error)) {
					stepUpOpen = true;
				} else {
					totpError = 'Akses ditolak.';
				}
			} else {
				totpError = problemDetail(error);
			}
		} catch {
			totpError = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			totpLoading = false;
		}
	}
</script>

<div class="account-page">
	<h2>Akun Saya</h2>

	<section class="password-section">
		<h3>Ganti Kata Sandi</h3>

		{#if passwordSuccess}
			<div class="success-alert">{passwordSuccess}</div>
		{/if}

		{#if passwordError}
			<div class="error-alert">{passwordError}</div>
		{/if}

		<form onsubmit={handlePasswordChange}>
			<label>
				Kata sandi saat ini
				<input type="password" required bind:value={currentPassword} disabled={passwordLoading} />
			</label>

			<label>
				Kata sandi baru
				<input
					type="password"
					required
					minlength="12"
					maxlength="128"
					bind:value={newPassword}
					disabled={passwordLoading}
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
					disabled={passwordLoading}
				/>
			</label>

			<button type="submit" disabled={passwordLoading}>
				{passwordLoading ? 'Memuat...' : 'Simpan'}
			</button>
		</form>
	</section>

	<hr class="section-divider" />

	<section class="totp-section">
		<h3>Verifikasi Dua Langkah (Autentikator)</h3>
		{#if totpMessage}
			<div class="success-alert">{totpMessage}</div>
		{/if}

		{#if totpError}
			<div class="error-alert">{totpError}</div>
		{/if}

		{#if newRecoveryCodes}
			<RecoveryCodesPanel
				codes={newRecoveryCodes}
				onconfirmed={() => {
					newRecoveryCodes = null;
				}}
			/>
		{:else if enrolling && enrollment}
			<div class="enrollment-box">
				<h4>Aktifkan Autentikator</h4>
				<p>Pindai kode QR dengan aplikasi autentikator, lalu masukkan kode 6 digit.</p>
				<img src={`data:image/png;base64,${enrollment.qrCodePng}`} alt="QR Code Autentikator" />
				<div class="manual-key">
					<span>Kunci manual</span>
					<code>{enrollment.secret}</code>
				</div>

				<form onsubmit={confirmEnrollment}>
					<label>
						Kode autentikator (6 digit)
						<input
							type="text"
							inputmode="numeric"
							pattern={'[0-9]{6}'}
							maxlength="6"
							required
							bind:value={enrollCode}
							disabled={totpLoading}
						/>
					</label>
					<div class="form-actions">
						<button
							type="button"
							onclick={() => {
								enrolling = false;
								enrollment = null;
							}}
							disabled={totpLoading}
						>
							Batal
						</button>
						<button type="submit" disabled={totpLoading || enrollCode.length !== 6}>
							{totpLoading ? 'Memuat...' : 'Verifikasi & Aktifkan'}
						</button>
					</div>
				</form>
			</div>
		{:else if totpEnrolled}
			<p>Autentikator aktif pada akun Anda.</p>
			<button
				type="button"
				class="btn-secondary"
				onclick={requestRegenerateRecoveryCodes}
				disabled={totpLoading}
			>
				Buat Ulang Kode Pemulihan
			</button>
		{:else}
			<p>Tingkatkan keamanan akun Anda dengan mengaktifkan autentikator dua langkah.</p>
			<button type="button" class="btn-primary" onclick={startEnrollment} disabled={totpLoading}>
				Aktifkan Autentikator
			</button>
		{/if}
	</section>

	<StepUpDialog
		open={stepUpOpen}
		client={panelClient()}
		onverified={executeRegenerateRecoveryCodes}
		oncancel={() => {
			stepUpOpen = false;
		}}
	/>
</div>
