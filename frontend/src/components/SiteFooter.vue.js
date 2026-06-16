import { storeToRefs } from 'pinia';
import { SiteFooter as PlatformFooter } from 'latere-ui';
import 'latere-ui/styles';
import { usePrefsStore } from '../stores/prefs';
const prefs = usePrefsStore();
const { theme, locale } = storeToRefs(prefs);
const localeOptions = [
    { code: 'en', label: 'EN', name: 'English' },
    { code: 'zh', label: '中', name: '中文' },
];
function onLocale(code) {
    if (code === 'en' || code === 'zh')
        prefs.setLocale(code);
}
debugger; /* PartiallyEnd: #3632/scriptSetup.vue */
const __VLS_ctx = {};
let __VLS_components;
let __VLS_directives;
const __VLS_0 = {}.PlatformFooter;
/** @type {[typeof __VLS_components.PlatformFooter, ]} */ ;
// @ts-ignore
const __VLS_1 = __VLS_asFunctionalComponent(__VLS_0, new __VLS_0({
    ...{ 'onUpdate:locale': {} },
    theme: (__VLS_ctx.theme),
    locale: (__VLS_ctx.locale),
    locales: (__VLS_ctx.localeOptions),
}));
const __VLS_2 = __VLS_1({
    ...{ 'onUpdate:locale': {} },
    theme: (__VLS_ctx.theme),
    locale: (__VLS_ctx.locale),
    locales: (__VLS_ctx.localeOptions),
}, ...__VLS_functionalComponentArgsRest(__VLS_1));
let __VLS_4;
let __VLS_5;
let __VLS_6;
const __VLS_7 = {
    'onUpdate:locale': (__VLS_ctx.onLocale)
};
var __VLS_3;
var __VLS_dollars;
const __VLS_self = (await import('vue')).defineComponent({
    setup() {
        return {
            PlatformFooter: PlatformFooter,
            theme: theme,
            locale: locale,
            localeOptions: localeOptions,
            onLocale: onLocale,
        };
    },
});
export default (await import('vue')).defineComponent({
    setup() {
        return {};
    },
});
; /* PartiallyEnd: #4569/main.vue */
