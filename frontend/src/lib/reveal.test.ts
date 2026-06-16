import { describe, it, expect, vi, afterEach } from 'vitest';
import { createApp, defineComponent, h } from 'vue';
import { useReveal } from './reveal';

const Harness = defineComponent({
  setup() {
    useReveal();
    return () => h('div');
  },
});

describe('useReveal', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.innerHTML = '';
  });

  it('disconnects the IntersectionObserver on unmount', () => {
    const disconnect = vi.fn();
    const observe = vi.fn();
    const unobserve = vi.fn();
    class FakeIO {
      disconnect = disconnect;
      observe = observe;
      unobserve = unobserve;
      constructor(_cb: IntersectionObserverCallback) {}
    }
    vi.stubGlobal('IntersectionObserver', FakeIO);

    const el = document.createElement('div');
    el.className = 'reveal';
    document.body.appendChild(el);

    const host = document.createElement('div');
    const app = createApp(Harness);
    app.mount(host); // fires onMounted -> creates observer, observes .reveal
    expect(observe).toHaveBeenCalledTimes(1);

    app.unmount(); // fires onUnmounted -> disconnect
    expect(disconnect).toHaveBeenCalledTimes(1);
  });
});
