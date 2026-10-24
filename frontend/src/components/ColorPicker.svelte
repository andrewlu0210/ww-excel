<script lang="ts">
  // Color palette popover in the style of Excel's: theme-like shades and
  // standard colors, plus "automatic"/"no fill".
  interface Props {
    // Label of the reset entry: 自動 (text) or 無填滿 (fill).
    resetLabel: string;
    onpick: (color: string) => void; // "RRGGBB" or "" for reset
    onclose: () => void;
  }

  let { resetLabel, onpick, onclose }: Props = $props();

  const theme = [
    ['FFFFFF', '000000', 'E7E6E6', '44546A', '4472C4', 'ED7D31', 'A5A5A5', 'FFC000', '5B9BD5', '70AD47'],
    ['F2F2F2', '7F7F7F', 'D0CECE', 'D6DCE4', 'D9E1F2', 'FCE4D6', 'EDEDED', 'FFF2CC', 'DDEBF7', 'E2EFDA'],
    ['D9D9D9', '595959', 'AEAAAA', 'ACB9CA', 'B4C6E7', 'F8CBAD', 'DBDBDB', 'FFE699', 'BDD7EE', 'C6E0B4'],
    ['BFBFBF', '404040', '757171', '8497B0', '8EA9DB', 'F4B084', 'C9C9C9', 'FFD966', '9BC2E6', 'A9D08E'],
    ['A6A6A6', '262626', '3A3838', '333F4F', '305496', 'C65911', '7B7B7B', 'BF8F00', '2F75B5', '548235'],
    ['808080', '0D0D0D', '161616', '222B35', '203764', '833C0C', '525252', '806000', '1F4E78', '375623'],
  ];
  const standard = ['C00000', 'FF0000', 'FFC000', 'FFFF00', '92D050', '00B050', '00B0F0', '0070C0', '002060', '7030A0'];

  function pick(c: string): void {
    onpick(c);
    onclose();
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="picker" role="dialog" aria-label="色彩">
  <button class="reset" tabindex="-1" onpointerdown={(e) => e.preventDefault()} onclick={() => pick('')}>{resetLabel}</button>
  <div class="label">佈景主題色彩</div>
  <div class="grid">
    {#each theme as row, i (i)}
      {#each row as c (c)}
        <button
          class="swatch"
          tabindex="-1"
          title={`#${c}`}
          style:background="#{c}"
          onpointerdown={(e) => e.preventDefault()}
          onclick={() => pick(c)}
          aria-label={`#${c}`}
        ></button>
      {/each}
    {/each}
  </div>
  <div class="label">標準色彩</div>
  <div class="grid">
    {#each standard as c (c)}
      <button
        class="swatch"
        tabindex="-1"
        title={`#${c}`}
        style:background="#{c}"
        onpointerdown={(e) => e.preventDefault()}
        onclick={() => pick(c)}
        aria-label={`#${c}`}
      ></button>
    {/each}
  </div>
</div>

<style>
  .picker {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 30;
    margin-top: 4px;
    padding: 8px;
    background: var(--popup-bg);
    border: 1px solid var(--popup-border);
    border-radius: 8px;
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.2);
  }
  .label {
    margin: 6px 0 4px;
    font-size: 11px;
    color: var(--chrome-muted);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(10, 18px);
    gap: 3px;
  }
  .swatch {
    width: 18px;
    height: 18px;
    padding: 0;
    border: 1px solid rgb(0 0 0 / 0.15);
    border-radius: 2px;
    cursor: pointer;
  }
  .swatch:hover {
    outline: 2px solid var(--accent);
  }
  .reset {
    width: 100%;
    padding: 4px;
    font: inherit;
    font-size: 12px;
    text-align: left;
    background: none;
    border: 1px solid var(--popup-border);
    border-radius: 4px;
    color: var(--chrome-text);
    cursor: pointer;
  }
</style>
