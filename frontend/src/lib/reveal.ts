import { onMounted, onUnmounted } from 'vue';

// useReveal adds the `in` class to every `.reveal` element as it scrolls
// into view, using an IntersectionObserver. The observer is disconnected
// on unmount so it is not left observing a detached DOM. When
// IntersectionObserver is unavailable the elements are revealed eagerly.
export function useReveal() {
  let io: IntersectionObserver | undefined;

  onMounted(() => {
    const els = document.querySelectorAll('.reveal:not(.in)');
    if (!('IntersectionObserver' in window)) {
      els.forEach(e => e.classList.add('in'));
      return;
    }
    io = new IntersectionObserver(
      entries => {
        entries.forEach(en => {
          if (en.isIntersecting) {
            en.target.classList.add('in');
            io?.unobserve(en.target);
          }
        });
      },
      { threshold: 0.08, rootMargin: '0px 0px -8% 0px' },
    );
    els.forEach(e => io!.observe(e));
  });

  onUnmounted(() => io?.disconnect());
}
