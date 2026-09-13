<script lang="ts">
  export let size = 20;
  let cls = '';
  export { cls as class };
</script>

<svg viewBox="0 0 24 24" width={size} height={size} fill="none" class="animate-spin shrink-0 {cls}" aria-label="Loading">
  <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" class="opacity-25" />
  <path d="M12 2a10 10 0 0 1 10 10" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
</svg>
