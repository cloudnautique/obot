<script lang="ts">
	import ProfileIcon from '$lib/components/profile/ProfileIcon.svelte';
	import type { Message } from '$lib/services';
	import { Pencil } from 'lucide-svelte/icons';
	import { AlertCircle } from 'lucide-svelte';

	interface Props {
		msg: Message;
	}

	let { msg }: Props = $props();
</script>

{#if !msg.icon}
	<!-- Nothing -->
{:else if msg.icon === 'Assistant'}
	<!-- Nothing -->
{:else}
	<div class="mr-3">
		{#if msg.icon === 'Pencil'}
			<Pencil class="h-8 w-8" />
		{:else if msg.icon === 'Assistant'}
			<!-- Nothing -->
		{:else if msg.icon === 'Profile'}
			<ProfileIcon />
		{:else if msg.icon === 'Error'}
			<AlertCircle class="h-8 w-8 text-red-500" />
		{:else}
			<img class="h-8 w-8 rounded-md bg-gray-100 p-1 text-blue" src={msg.icon} alt="message icon" />
		{/if}
	</div>
{/if}
