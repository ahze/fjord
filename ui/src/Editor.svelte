<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { basicSetup, EditorView } from 'codemirror';
  import { yaml } from '@codemirror/lang-yaml';
  import { oneDark } from '@codemirror/theme-one-dark';
  import { EditorState } from '@codemirror/state';
  import { keymap } from '@codemirror/view';

  export let content = '';
  export let language: 'yaml' | 'env' = 'yaml';
  export let readonly = false;

  const dispatch = createEventDispatcher();
  let editorContainer: HTMLDivElement;
  let view: EditorView;

  function initEditor() {
    if (view) view.destroy();
    if (!editorContainer) return;

    const saveKeymap = keymap.of([
      {
        key: 'Mod-s',
        run: () => {
          dispatch('save');
          return true;
        },
      },
    ]);

    const extensions = [
      basicSetup,
      oneDark,
      saveKeymap,
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          content = view.state.doc.toString();
          dispatch('change', content);
        }
      }),
    ];

    if (language === 'yaml') {
      extensions.push(yaml());
    }

    if (readonly) {
      // A generated spec (e.g. appjail-director.yml) is shown for reference,
      // not edited: fjord regenerates it, so hand-edits would be lost.
      extensions.push(EditorState.readOnly.of(true), EditorView.editable.of(false));
    }

    const state = EditorState.create({
      doc: content,
      extensions,
    });

    view = new EditorView({
      state,
      parent: editorContainer,
    });
  }

  $: if (editorContainer) {
    if (!view || view.state.doc.toString() !== content) {
      initEditor();
    }
  }

  onMount(() => {
    initEditor();
  });

  onDestroy(() => {
    if (view) view.destroy();
  });
</script>

<div bind:this={editorContainer} class="h-full w-full overflow-hidden text-left rounded-b-xl border-t-0 border-fjord-border shadow-inner"></div>

<style>
  :global(.cm-editor) {
    height: 100%;
    outline: none !important;
  }
  :global(.cm-scroller) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
    font-size: 14px;
    background: #161619;
  }
</style>
