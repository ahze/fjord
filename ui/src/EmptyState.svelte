<script lang="ts">
  // HIG placeholder page: large dim icon + title + description + optional action.
  import { createEventDispatcher } from 'svelte';
  import Icon from './Icon.svelte';

  export let icon = '';
  export let title: string;
  export let description = '';
  export let actionLabel = '';
  export let secondaryLabel = '';

  const dispatch = createEventDispatcher();
</script>

<div class="h-full flex flex-col items-center justify-center text-center gap-1 py-12 px-6">
  {#if icon}
    <Icon name={icon} size={48} strokeWidth={1.5} class="text-slate-600 mb-3" />
  {/if}
  <div class="text-lg font-semibold text-slate-300">{title}</div>
  {#if description}
    <p class="text-sm text-slate-500 max-w-sm leading-relaxed">{description}</p>
  {/if}
  {#if actionLabel || secondaryLabel}
    <div class="mt-4 flex items-center gap-2">
      {#if actionLabel}
        <button
          on:click={() => dispatch('action')}
          class="px-4 py-2 rounded-lg text-sm font-medium bg-fjord-accent hover:bg-fjord-accent-hover text-white transition-colors"
          >{actionLabel}</button
        >
      {/if}
      {#if secondaryLabel}
        <button
          on:click={() => dispatch('secondary')}
          class="px-4 py-2 rounded-lg text-sm font-medium bg-fjord-border hover:bg-fjord-accent hover:text-white text-slate-300 transition-colors"
          >{secondaryLabel}</button
        >
      {/if}
    </div>
  {/if}
</div>
