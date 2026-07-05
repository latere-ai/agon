import { ViteSSG } from 'vite-ssg';
import { createPinia } from 'pinia';
import App from './App.vue';
import { routes } from './router';
import './styles/tokens.css';
// Liquid Glass v2 material for shared latere-ui chrome (SiteFooter). Loaded
// after agon's own tokens so the crimson --accent and palette win; glass.css
// only supplies --glass-*/--canvas, never --accent.
import 'latere-ui/glass';
import './styles/app.css';
import './styles/dialectic.css';

export const createApp = ViteSSG(App, { routes }, ({ app }) => {
  app.use(createPinia());
});
