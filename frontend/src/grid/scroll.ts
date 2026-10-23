// Maps native scroll positions to content offsets.
// WebKit caps element sizes (LayoutUnit, ~33.5M px), so when the content
// is taller than MAX_SPACER the spacer is shrunk and scroll positions are
// scaled. With default row heights 1M rows (~21M px) stays unscaled.

export const MAX_SPACER = 30_000_000;

export class ScrollAxis {
  spacer = 0;
  private ratio = 1;

  // content: total scrollable content size including headers.
  // viewport: visible size of the scroller.
  update(content: number, viewport: number): void {
    this.spacer = Math.min(content, MAX_SPACER);
    const contentRange = Math.max(0, content - viewport);
    const spacerRange = Math.max(0, this.spacer - viewport);
    this.ratio = spacerRange > 0 ? contentRange / spacerRange : 1;
  }

  toContent(scrollPos: number): number {
    return scrollPos * this.ratio;
  }

  toScroll(contentPos: number): number {
    return contentPos / this.ratio;
  }
}
