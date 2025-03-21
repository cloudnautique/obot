<script lang="ts">
	import MessageIcon from '$lib/components/messages/MessageIcon.svelte';
	import { FileText, Pencil } from 'lucide-svelte/icons';
	import { Tween } from 'svelte/motion';
	import { ChatService, type Message, type Project } from '$lib/services';
	import highlight from 'highlight.js';
	import { toHTMLFromMarkdown } from '$lib/markdown.js';
	import { Paperclip, X } from 'lucide-svelte';
	import { formatTime } from '$lib/time';
	import { popover } from '$lib/actions';
	import { fly } from 'svelte/transition';
	import { waitingOnModelMessage } from '$lib/services/chat/messages';
	import Loading from '$lib/icons/Loading.svelte';
	import { fade } from 'svelte/transition';

	interface Props {
		msg: Message;
		project: Project;
		onLoadFile?: (filename: string) => void;
		onSendCredentials?: (id: string, credentials: Record<string, string>) => void;
		onSendCredentialsCancel?: (id: string) => void;
	}

	let {
		msg,
		project,
		onLoadFile = () => {},
		onSendCredentials = ChatService.sendCredentials,
		onSendCredentialsCancel
	}: Props = $props();

	let content = $derived(msg.message ? msg.message.join('') : '');
	let fullWidth = !msg.sent && !msg.oauthURL && !msg.tool;
	let showBubble = msg.sent;
	let isPrompt = msg.fields && msg.promptId;
	let renderMarkdown = !msg.sent && !msg.oauthURL && !msg.tool;
	let toolTT = popover({
		placement: 'bottom-start'
	});
	let shell = $state({
		input: '',
		output: ''
	});

	let promptCredentials = $state<Record<string, string>>({});
	let credentialsSubmitted = $state(false);

	let waiting = $derived(msg.message?.[0] === waitingOnModelMessage);
	let shouldAnimate = $derived(
		!msg.done && !msg.toolCall && !msg.promptId && !msg.sent && !waiting
	);
	let cursor = new Tween(0);
	let prevContent = $state('');
	let animatedText = $derived(shouldAnimate ? content.slice(0, cursor.current) : content);
	let animating = $state(false);

	$effect(() => {
		if (!shouldAnimate) return;

		if (!content.startsWith(prevContent)) {
			cursor.set(0, { duration: 0 });
		}
		prevContent = content;

		animating = true;
		cursor.set(content.length, { duration: 500 }).then(() => (animating = false));
	});

	$effect(() => {
		if (msg.toolCall && msg.sourceName === 'Shell') {
			try {
				shell.input = JSON.parse(msg.toolCall?.input ?? '').CMD ?? '';
				shell.output = msg.toolCall?.output ?? '';
			} catch {
				return;
			}
		}
	});

	$effect(() => {
		// this is a hack to ensure the effect is run each time animatedText updates
		void animatedText;

		const blocks = document.querySelectorAll('.message-content pre > code');
		blocks.forEach((block) => {
			if (block instanceof HTMLElement && block.dataset.highlighted !== 'yes') {
				highlight.highlightElement(block);
			}
		});

		const links = document.querySelectorAll('.message-content a');
		links.forEach((link) => {
			if (link instanceof HTMLAnchorElement && link.target == '') {
				link.target = '_blank';
			}
		});
	});

	function fileLoad() {
		if (msg.file?.filename) {
			onLoadFile(msg.file?.filename);
		}
	}

	// Citations

	// citation urls starting with knowledge:// mean it's a local file
	const citationKnowledgePrefix = 'knowledge://';

	function citationURL(url: string | undefined) {
		if (!url) return undefined;

		if (url.startsWith(citationKnowledgePrefix)) {
			return `/api/assistants/${project.assistantID}/projects/${project.id}/knowledge/${url.slice(citationKnowledgePrefix.length)}`;
		}

		return url;
	}

	function citationDisplayURL(url: string) {
		if (url.startsWith(citationKnowledgePrefix)) {
			// return only the last path element (file name)
			return decodeURIComponent(url.split('::').pop() ?? url);
		}

		// remove the protocol and www.
		const res = decodeURIComponent(url).replace(/^(.+:\/\/)?(www\.)?/, '');
		return res.length > 25 ? res.slice(0, 25) + '...' : res;
	}

	function citationFavicon(url: string) {
		if (url.startsWith(citationKnowledgePrefix)) {
			return '/favicon';
		}

		const _url = new URL(url);
		return _url.origin + '/favicon.ico';
	}

	function deduplicateCitations(citations: string[]) {
		const seen = new Set<string>();
		return citations.filter((url) => {
			if (seen.has(url)) {
				return false;
			}
			seen.add(url);
			return true;
		});
	}
</script>

{#snippet time()}
	{#if msg.time}
		<span class="mt-2 self-end text-sm text-gray">{formatTime(msg.time)}</span>
	{/if}
{/snippet}

{#snippet nameAndTime()}
	<div class="mb-1 flex items-center space-x-2">
		{#if msg.sourceName}
			<span class="text-sm font-semibold"
				>{msg.sourceName === 'Assistant' ? project?.name : msg.sourceName}</span
			>
		{/if}
		{#if msg.time}
			<span class="text-sm text-gray">{formatTime(msg.time)}</span>
		{/if}
	</div>
{/snippet}

{#snippet messageBody()}
	<div
		class:flex={showBubble}
		class:contents={!showBubble}
		class:message-content={renderMarkdown}
		class="flex w-full flex-col overflow-auto rounded-3xl bg-gray-70 px-6 py-4 text-black dark:bg-gray-950 dark:text-white"
	>
		{#if msg.oauthURL}
			{@render oauth()}
		{:else if content}
			{#if msg.sourceName !== 'Abort Current Task'}
				{@render messageContent()}
			{/if}
		{:else if msg.toolCall}
			{@render toolContent()}
		{/if}

		{@render files()}
		{@render citations()}
	</div>
{/snippet}

{#snippet files()}
	{#if msg.file?.filename}
		<button
			class="my-2 flex cursor-pointer flex-col divide-y
		 divide-gray-300 rounded-3xl
		 border border-gray-300
		 bg-white text-start
		 text-black shadow-lg
		   dark:bg-black
		    dark:text-gray-50"
			onclick={fileLoad}
		>
			<div class="flex gap-2 px-5 py-4">
				<div class="flex grow justify-start gap-2">
					<FileText />
					<span>{msg.file.filename}</span>
				</div>
				<div>
					<Pencil />
					<span class="sr-only">Open</span>
				</div>
			</div>
			<div class="relative">
				<div class="whitespace-pre-wrap p-5 font-body text-gray-700 dark:text-gray-300">
					{msg.file.content.split('\n').splice(0, 6).join('\n')}
				</div>
				<div
					class="absolute bottom-0 z-20 h-24 w-full rounded-3xl bg-gradient-to-b from-transparent to-white dark:to-black"
				></div>
			</div>
		</button>
	{/if}
{/snippet}

{#snippet explain()}
	{#if msg.explain}
		<div
			role="none"
			class="-m-6 -mb-4 mt-2 flex flex-col
		 divide-y divide-gray-300
		 rounded-3xl border
		 border-gray-300 bg-white
		 text-black shadow-lg
		   dark:bg-black
		    dark:text-gray-50"
		>
			<div class="flex gap-2 px-5 py-4">
				<Paperclip />
				<span>Selection from</span>
				<button
					class="font-medium"
					onclick={() => {
						if (msg.explain?.filename) {
							onLoadFile(msg.explain.filename);
						}
					}}>{msg.explain.filename}</button
				>
			</div>
			<div class="whitespace-pre-wrap p-5 font-body text-gray-700 dark:text-gray-300">
				{msg.explain.selection}
			</div>
		</div>
	{/if}
{/snippet}

{#snippet toolContent()}
	<button
		use:toolTT.ref
		class="text-left text-xs text-gray underline opacity-0 transition-opacity group-hover:opacity-100"
		onclick={() => {
			toolTT.toggle();
		}}
		>Details
	</button>
	<div
		use:toolTT.tooltip
		class="z-40 flex flex-col gap-2 rounded-3xl bg-gray-70 p-5 dark:bg-gray-900 dark:text-gray-50"
	>
		<div class="flex text-xl font-semibold">
			<span class="flex-1">Input</span>
			<button
				class="self-end rounded-lg p-2 hover:bg-white dark:hover:bg-black"
				onclick={() => {
					toolTT.toggle();
				}}
			>
				<X class="h-4 w-4" />
			</button>
		</div>
		<pre class="max-w-[500px] overflow-auto rounded-lg bg-white p-5 dark:bg-black">{msg.toolCall
				?.input ?? 'None'}</pre>
		<div class="text-xl font-semibold">Output</div>
		<pre class="max-w-[500px] overflow-auto rounded-lg bg-white p-5 dark:bg-black">{msg.toolCall
				?.output ?? 'None'}</pre>
	</div>
	{#if shell.input && shell.output}
		<div class="mt-1 rounded-3xl bg-gray-100 p-5 dark:bg-gray-900 dark:text-gray-50">
			<div class="pb-1 font-mono">
				> {shell.input}
			</div>
			<div class="font-mono">
				{shell.output}
			</div>
		</div>
	{/if}
{/snippet}

{#snippet messageContent()}
	{#if msg.sent}
		{#each content.split('\n') as line}
			<p>{line}</p>
		{/each}
		{@render explain()}
	{:else}
		<div transition:fade={{ duration: 1000 }}>
			{@html toHTMLFromMarkdown(animatedText)}

			{#if !msg.done || animating}
				<p class="flex items-center gap-2 text-sm text-gray-500">
					<Loading /> Loading...
				</p>
			{:else}
				<!-- spacer -->
				<div class="h-[28px]"></div>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet oauth()}
	<a
		href={msg.oauthURL}
		class="rounded-3xl bg-blue
						p-4
						text-white
					  hover:bg-blue-400"
		target="_blank"
		>Authentication is required
		<span class="underline">click here</span> to log-in using OAuth
	</a>
{/snippet}

{#snippet promptAuth()}
	{#if msg.fields && !credentialsSubmitted}
		<form
			class="flex flex-col gap-2"
			onsubmit={(e) => {
				e.preventDefault();
				if (msg.promptId) {
					onSendCredentials(msg.promptId, promptCredentials);
					credentialsSubmitted = true;
				}
			}}
		>
			{@html toHTMLFromMarkdown(msg.message.join('\n'))}

			{#each msg.fields as field}
				<div class="flex flex-col gap-1">
					<label for={field.name} class="mt-1 text-sm font-medium">{field.name}</label>
					<input
						class="rounded-lg bg-white p-2 outline-none dark:bg-gray-900"
						type={field.sensitive ? 'password' : 'text'}
						name={field.name}
						bind:value={promptCredentials[field.name]}
					/>
					{#if field.description}
						<p class="text-sm text-gray-500">{field.description}</p>
					{/if}
				</div>
			{/each}

			<div class="item-center flex gap-2 self-end">
				{#if onSendCredentialsCancel}
					<button
						class="button-secondary"
						onclick={() => onSendCredentialsCancel(msg.promptId ?? '')}
						>Cancel
					</button>
				{/if}
				<button class="button-primary" type="submit">Submit</button>
			</div>
			<span class="mt-1 flex grow items-end self-end text-sm text-gray"
				>*The submitted contents are not visible to AI.</span
			>
		</form>
	{/if}
{/snippet}

{#snippet citations()}
	{#if msg.citations && msg.citations.length > 0}
		<div class="mt-2 flex flex-wrap gap-2">
			{#each deduplicateCitations(msg.citations
					.map((c) => c.url)
					.filter((url) => url !== undefined)) as url, i}
				{#if msg.done}
					<a
						href={citationURL(url)}
						target="_blank"
						class="flex w-fit items-center gap-2 rounded-full bg-gray-100 p-2 text-sm dark:bg-gray-900"
						transition:fly={{ y: 100, delay: 50 * i, duration: 250 }}
					>
						<img
							src={citationFavicon(url)}
							alt="Favicon"
							class="size-4"
							onerror={(e) => ((e.currentTarget as HTMLImageElement).src = '/favicon.ico')}
						/>
						{citationDisplayURL(url)}
					</a>
				{/if}
			{/each}
		</div>
	{/if}
{/snippet}

{#if !msg.ignore}
	<div
		class="group relative flex items-start gap-3 {isPrompt
			? '-m-5 rounded-3xl bg-gray-100 p-5 dark:bg-gray-950'
			: ''}"
		class:justify-end={msg.sent}
	>
		{#if !msg.sent}
			<MessageIcon {msg} />
		{/if}

		<div class="flex w-full flex-col" class:w-full={fullWidth}>
			{#if isPrompt}
				{@render promptAuth()}
			{:else}
				{#if !msg.sent}
					{@render nameAndTime()}
				{/if}
				{@render messageBody()}
				{#if msg.sent}
					{@render time()}
				{/if}
			{/if}
		</div>
		{#if msg.aborted}
			<div
				class="pointer-events-none absolute bottom-0 z-10 flex h-full w-full flex-col items-center justify-center bg-white bg-opacity-60 text-xl font-semibold text-black text-opacity-30 dark:bg-black dark:bg-opacity-60 dark:text-white dark:text-opacity-30"
			>
				<p>Aborted</p>
				<p class="text-xs">This content will be ignored.</p>
			</div>
		{/if}
	</div>
{/if}

<style lang="postcss">
	/* The :global is to get rid of warnings about the selector not being found */
	:global {
		.message-content h1 {
			@apply my-4 text-4xl font-extrabold text-black dark:text-gray-100;
		}

		.message-content h2 {
			@apply my-4 text-3xl font-bold text-black dark:text-gray-100;
		}

		.message-content h3 {
			@apply my-4 text-2xl font-bold text-black dark:text-gray-100;
		}

		.message-content h4 {
			@apply my-4 text-xl font-bold text-black dark:text-gray-100;
		}

		.message-content h5 {
			@apply my-4 text-xl font-bold text-black dark:text-gray-100;
		}

		.message-content h6 {
			@apply my-4 text-lg font-bold text-black dark:text-gray-100;
		}

		.message-content p {
			@apply mb-2 text-gray-900 dark:text-gray-100;
		}

		.message-content a {
			@apply font-medium text-blue-600 hover:underline dark:text-gray-400;
		}

		.message-content ul {
			@apply relative mb-4 flex list-outside list-disc flex-col px-1 text-gray-900 marker:text-gray-900 dark:text-gray-100 dark:marker:text-gray-100;
		}

		.message-content ol {
			@apply relative mb-4 flex list-outside list-decimal flex-col px-1 text-gray-900 marker:text-gray-900 dark:text-gray-100 dark:marker:text-gray-100;
		}

		.message-content ul li {
			@apply ps-2;
		}

		.message-content code {
			@apply scrollbar-none;
		}

		span[data-end-indicator] {
			@apply invisible;
		}

		.loading-container span[data-end-indicator] {
			@apply visible relative -mt-[2px] ml-1 inline-block size-4 animate-pulse rounded-full bg-gray-400 align-middle text-transparent;
		}
	}
</style>
