<script lang="ts">
	interface Props {
		codes: string[];
		onconfirmed?: () => void;
	}

	let { codes, onconfirmed }: Props = $props();
	let confirmed = $state(false);
</script>

<div class="recovery-codes-panel">
	<h3>Kode Pemulihan</h3>
	<p>
		Simpan kode ini di tempat aman. Setiap kode hanya dapat dipakai sekali dan tidak akan
		ditampilkan lagi.
	</p>

	<div class="codes-grid">
		{#each codes as code (code)}
			<code>{code}</code>
		{/each}
	</div>

	<label class="confirm-checkbox">
		<input type="checkbox" bind:checked={confirmed} />
		Saya sudah menyimpan kode ini
	</label>

	{#if onconfirmed}
		<button
			type="button"
			disabled={!confirmed}
			onclick={() => {
				if (confirmed) onconfirmed?.();
			}}
		>
			Lanjutkan
		</button>
	{/if}
</div>
