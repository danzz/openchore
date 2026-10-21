/**
 * iOS Safari ignores `overscroll-behavior-x: none` for its native edge-swipe
 * back/forward navigation gestures.
 *
 * Intercepting single-finger `touchstart` near the viewport edges (< 32px)
 * with `{ passive: false }` and `preventDefault()` prevents WebKit from
 * starting the system-level history swipe-back / swipe-forward navigation.
 *
 * If the touch is simply a tap (moved < 10px) rather than a swipe, we dispatch
 * a click to the target so interactive elements near the edges continue to work.
 */
export function initEdgeSwipePrevention(edgeThreshold = 32) {
  if (typeof window === 'undefined') return;

  let touchStartX = 0;
  let touchStartY = 0;
  let touchPrevented = false;
  let touchTarget: EventTarget | null = null;

  window.addEventListener(
    'touchstart',
    (e: TouchEvent) => {
      if (e.touches.length !== 1) return;
      const x = e.touches[0].clientX;
      const winW = window.innerWidth;

      if (x < edgeThreshold || x > winW - edgeThreshold) {
        touchPrevented = true;
        touchStartX = x;
        touchStartY = e.touches[0].clientY;
        touchTarget = e.target;
        e.preventDefault();
      } else {
        touchPrevented = false;
      }
    },
    { passive: false }
  );

  window.addEventListener(
    'touchmove',
    (e: TouchEvent) => {
      if (touchPrevented) {
        e.preventDefault();
      }
    },
    { passive: false }
  );

  window.addEventListener(
    'touchend',
    (e: TouchEvent) => {
      if (touchPrevented && e.changedTouches.length === 1) {
        const endX = e.changedTouches[0].clientX;
        const endY = e.changedTouches[0].clientY;
        const dist = Math.hypot(endX - touchStartX, endY - touchStartY);

        // If it was a tap rather than a swipe, trigger click on the interactive target
        if (dist < 10) {
          const clickable = touchTarget instanceof Element
            ? touchTarget.closest('button, a, input, select, textarea, [role="button"]')
            : null;
          if (clickable instanceof HTMLElement) {
            clickable.click();
          } else if (touchTarget instanceof HTMLElement) {
            touchTarget.click();
          }
        }
      }
      touchPrevented = false;
      touchTarget = null;
    },
    { passive: false }
  );
}
