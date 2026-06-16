<script setup lang="ts">
import { computed } from 'vue';
import { useHead } from '@unhead/vue';
import DefaultLayout from '../layouts/DefaultLayout.vue';
import { useContent } from '../content';
import { useReveal } from '../lib/reveal';
import SectionShell from '../components/landing/SectionShell.vue';
import HeroStage from '../components/landing/HeroStage.vue';
import ArchitectureSvg from '../components/landing/ArchitectureSvg.vue';
import CodeBlock from '../components/landing/CodeBlock.vue';

const content = useContent();
const c = content; // alias for template brevity

useHead({
  title: computed(() => content.value.meta.title),
  meta: [{ name: 'description', content: computed(() => content.value.meta.description) }],
});

useReveal();
</script>

<template>
  <DefaultLayout>
    <!-- Hero -->
    <section id="top" class="hero">
      <div class="hero-bg"></div>
      <div class="hero-grid"></div>
      <div class="container">
        <div class="hero-inner">
          <span class="hero-stamp">
            <span class="stamp-dot-p"></span>
            <span>{{ c.hero.stampProposer }}</span>
            <span>×</span>
            <span>{{ c.hero.stampCritic }}</span>
            <span class="stamp-dot-c"></span>
          </span>
          <h1 class="hero-title" v-html="c.hero.title"></h1>
          <p class="hero-sub" v-html="c.hero.sub"></p>
          <div class="hero-actions">
            <a class="btn btn-primary" href="#install">{{ c.hero.ctaPrimary }}</a>
            <a class="btn btn-ghost" href="#transcript">{{ c.hero.ctaSecondary }}</a>
          </div>

          <div class="harnesses">
            <span class="harnesses-label">{{ c.hero.worksWith }}</span>
            <div class="harnesses-row">
              <a class="harness" href="https://topos.latere.ai" title="Topos — agent platform by Latere">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                  <circle cx="6" cy="7" r="2.2" />
                  <circle cx="17" cy="6" r="2.2" />
                  <circle cx="18" cy="17" r="2.2" />
                  <circle cx="7" cy="18" r="2.2" />
                  <path d="M8.1 7.4c2.3 1.3 4.8 1.1 6.9-.5M16.5 8.1c1.4 2 1.8 4.3 1.5 6.7M15.9 17.4c-2.1.9-4.4 1.1-6.7.6M6.8 15.8c-.7-2.2-.8-4.4-.2-6.6M9 9.1l6 6" />
                </svg>
                <span><em>Topos</em></span>
              </a>
              <span class="harness" title="Claude Code by Anthropic">
                <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="m4.7144 15.9555 4.7174-2.6471.079-.2307-.079-.1275h-.2307l-.7893-.0486-2.6956-.0729-2.3375-.0971-2.2646-.1214-.5707-.1215-.5343-.7042.0546-.3522.4797-.3218.686.0608 1.5179.1032 2.2767.1578 1.6514.0972 2.4468.255h.3886l.0546-.1579-.1336-.0971-.1032-.0972L6.973 9.8356l-2.55-1.6879-1.3356-.9714-.7225-.4918-.3643-.4614-.1578-1.0078.6557-.7225.8803.0607.2246.0607.8925.686 1.9064 1.4754 2.4893 1.8336.3643.3035.1457-.1032.0182-.0728-.164-.2733-1.3539-2.4467-1.445-2.4893-.6435-1.032-.17-.6194c-.0607-.255-.1032-.4674-.1032-.7285L6.287.1335 6.6997 0l.9957.1336.419.3642.6192 1.4147 1.0018 2.2282 1.5543 3.0296.4553.8985.2429.8318.091.255h.1579v-.1457l.1275-1.706.2368-2.0947.2307-2.6957.0789-.7589.3764-.9107.7468-.4918.5828.2793.4797.686-.0668.4433-.2853 1.8517-.5586 2.9021-.3643 1.9429h.2125l.2429-.2429.9835-1.3053 1.6514-2.0643.7286-.8196.85-.9046.5464-.4311h1.0321l.759 1.1293-.34 1.1657-1.0625 1.3478-.8804 1.1414-1.2628 1.7-.7893 1.36.0729.1093.1882-.0183 2.8535-.607 1.5421-.2794 1.8396-.3157.8318.3886.091.3946-.3278.8075-1.967.4857-2.3072.4614-3.4364.8136-.0425.0304.0486.0607 1.5482.1457.6618.0364h1.621l3.0175.2247.7892.522.4736.6376-.079.4857-1.2142.6193-1.6393-.3886-3.825-.9107-1.3113-.3279h-.1822v.1093l1.0929 1.0686 2.0035 1.8092 2.5075 2.3314.1275.5768-.3218.4554-.34-.0486-2.2039-1.6575-.85-.7468-1.9246-1.621h-.1275v.17l.4432.6496 2.3436 3.5214.1214 1.0807-.17.3521-.6071.2125-.6679-.1214-1.3721-1.9246L14.38 17.959l-1.1414-1.9428-.1397.079-.674 7.2552-.3156.3703-.7286.2793-.6071-.4614-.3218-.7468.3218-1.4753.3886-1.9246.3157-1.53.2853-1.9004.17-.6314-.0121-.0425-.1397.0182-1.4328 1.9672-2.1796 2.9446-1.7243 1.8456-.4128.164-.7164-.3704.0667-.6618.4008-.5889 2.386-3.0357 1.4389-1.882.929-1.0868-.0062-.1579h-.0546l-6.3385 4.1164-1.1293.1457-.4857-.4554.0608-.7467.2307-.2429 1.9064-1.3114Z" />
                </svg>
                <span>Claude Code</span>
              </span>
              <span class="harness" title="Codex by OpenAI">
                <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z" />
                </svg>
                <span>Codex</span>
              </span>
              <span class="harness" title="GitHub Actions — automated PR review comments">
                <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
                </svg>
                <span>GitHub Action</span>
              </span>
            </div>
          </div>
        </div>

        <HeroStage />
      </div>
    </section>

    <!-- Transcript -->
    <SectionShell
      id="transcript"
      :eyebrow="c.transcript.eyebrow"
      :title="c.transcript.title"
      :lead="c.transcript.lead">
      <div class="tx">
        <div class="tx-head">
          <div class="lights"><span></span><span></span><span></span></div>
          <div class="tx-case">{{ c.transcript.case }}</div>
          <div class="tx-meta">{{ c.transcript.meta }}</div>
        </div>
        <div class="tx-body">
          <div
            v-for="(r, i) in c.transcript.rows"
            :key="i"
            class="tx-msg"
            :class="{ staked: r.isStake }">
            <div class="tx-n">{{ r.n }}</div>
            <div>
              <div class="tx-author" :class="r.actor">{{ r.actorLabel }}</div>
              <div class="tx-content" v-html="r.html"></div>
            </div>
            <div v-if="r.tag" class="tx-pill" :class="r.tag.kind">{{ r.tag.label }}</div>
            <div v-else></div>
          </div>
        </div>
        <div class="tx-foot">
          <span>{{ c.transcript.footer[0] }}</span>
          <span class="tx-v">{{ c.transcript.footer[1] }}</span>
        </div>
      </div>
    </SectionShell>

    <!-- Why -->
    <SectionShell id="why" :eyebrow="c.why.eyebrow" :title="c.why.title">
      <div class="pillars">
        <div v-for="(pl, i) in c.why.pillars" :key="i" class="pillar">
          <div class="pillar-k">{{ pl.k }} · property</div>
          <div class="pillar-t">{{ pl.t }}</div>
          <div class="pillar-d">{{ pl.d }}</div>
        </div>
      </div>
    </SectionShell>

    <!-- Compare -->
    <SectionShell id="compare" :eyebrow="c.compare.eyebrow" :title="c.compare.title">
      <div class="compare">
        <div class="compare-head">
          <div>{{ c.compare.headers[0] }}</div>
          <div class="ch-agon">{{ c.compare.headers[1] }}</div>
          <div>{{ c.compare.headers[2] }}</div>
          <div>{{ c.compare.headers[3] }}</div>
        </div>
        <div v-for="(r, i) in c.compare.rows" :key="i" class="compare-row">
          <div class="cr-prop" :data-h="c.compare.headers[0]">{{ r.p }}</div>
          <div
            v-for="(cell, j) in r.cols"
            :key="j"
            :data-h="c.compare.headers[j + 1]">
            <span v-if="cell === 'agon'"><span class="cmp-tick yes">✓</span><span class="cr-agon">yes</span></span>
            <span v-else-if="cell === 'partial'"><span class="cmp-tick partial">~</span>partial</span>
            <span v-else><span class="cmp-tick no">·</span>no</span>
          </div>
        </div>
      </div>
    </SectionShell>

    <!-- Use cases -->
    <SectionShell
      id="usecases"
      :eyebrow="c.usecases.eyebrow"
      :title="c.usecases.title"
      :lead="c.usecases.lead">
      <div class="usecases">
        <div v-for="(it, i) in c.usecases.items" :key="i" class="usecase">
          <div class="uc-head">
            <div class="uc-icon">{{ it.i }}</div>
            <div class="uc-t">{{ it.t }}</div>
          </div>
          <div class="uc-d">{{ it.d }}</div>
        </div>
      </div>
    </SectionShell>

    <!-- Architecture -->
    <SectionShell
      id="architecture"
      :eyebrow="c.arch.eyebrow"
      :title="c.arch.title"
      :lead="c.arch.lead">
      <div class="arch arch-dark">
        <ArchitectureSvg />
        <div class="arch-cap">{{ c.arch.cap }}</div>
      </div>
    </SectionShell>

    <!-- Signal -->
    <SectionShell
      id="signal"
      :eyebrow="c.signal.eyebrow"
      :title="c.signal.title"
      :lead="c.signal.lead">
      <div class="signal-grid">
        <div
          v-for="(cell, i) in c.signal.cells"
          :key="i"
          class="signal-cell"
          :class="cell.kind === 'r' ? 'resolved' : 'contested'">
          <span class="signal-tag" :class="cell.kind">{{ cell.tag }}</span>
          <div class="signal-num">{{ cell.num }} <small>{{ cell.label }}</small></div>
          <div class="signal-route"><strong>{{ cell.route }}</strong></div>
          <div class="signal-d">{{ cell.desc }}</div>
        </div>
      </div>
    </SectionShell>

    <!-- Foundations -->
    <SectionShell
      id="foundations"
      :eyebrow="c.found.eyebrow"
      :title="c.found.title"
      :lead="c.found.lead">
      <div class="refs">
        <div v-for="(r, i) in c.found.refs" :key="i" class="ref">
          <div class="ref-yr">{{ r.yr }}</div>
          <div class="ref-body">
            <span class="cite">{{ r.cite }}.</span> <em>{{ r.em }}</em>{{ r.tail }}
          </div>
          <a class="ref-link" :href="r.href" target="_blank" rel="noopener">{{ r.link }}</a>
        </div>
      </div>
      <div class="pullquote">
        <q>{{ c.found.pullquote.q }}</q>
        <div class="cite">— {{ c.found.pullquote.cite }}</div>
      </div>
      <div class="honest">
        <strong>{{ c.found.honestStrong }}</strong>{{ c.found.honestRest }}
      </div>
    </SectionShell>

    <!-- Stop-hook -->
    <SectionShell id="hook" :eyebrow="c.hook.eyebrow" :title="c.hook.title">
      <div class="hook">
        <div class="hook-body">
          <div class="hook-d">{{ c.hook.desc }}</div>
          <a class="btn btn-primary" href="#install" style="align-self: flex-start">{{ c.hook.cta }}</a>
        </div>
        <div class="hook-diagram">
          <div v-for="(l, i) in c.hook.lines" :key="i">
            <span class="hd-l">{{ l.p }}{{ l.p ? ' ' : '' }}</span>
            <span v-if="l.cmd" class="hd-cmd">{{ l.cmd }}</span>
            <span v-if="l.attn" class="hd-attn">{{ l.attn }}</span>
            <span v-if="l.l" class="hd-l">{{ l.l }}</span>
          </div>
        </div>
      </div>
    </SectionShell>

    <!-- FAQ -->
    <SectionShell id="faq" :eyebrow="c.faq.eyebrow" :title="c.faq.title">
      <div class="faq">
        <details
          v-for="(it, i) in c.faq.items"
          :key="i"
          class="faq-item"
          :open="i === 0">
          <summary>{{ it.q }}</summary>
          <div class="faq-body">{{ it.a }}</div>
        </details>
      </div>
    </SectionShell>

    <!-- Install -->
    <div id="install" class="install-wrap">
      <div class="install">
        <span class="eyebrow">{{ c.install.eyebrow }}</span>
        <h2 v-html="c.install.title"></h2>
        <p>{{ c.install.lead }}</p>
        <CodeBlock :comment="c.install.a.c" :command="c.install.a.cmd" />
        <CodeBlock :comment="c.install.b.c" :command="c.install.b.cmd" />
        <div class="install-cta">
          <a class="btn btn-primary" href="https://github.com/latere-ai/agon" target="_blank" rel="noopener">{{ c.install.ctaPrimary }}</a>
          <a class="btn btn-ghost" href="https://github.com/latere-ai/agon#readme" target="_blank" rel="noopener">{{ c.install.ctaSecondary }}</a>
        </div>
      </div>
    </div>
  </DefaultLayout>
</template>
