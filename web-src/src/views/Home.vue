<template>
  <main class="main">
    <div v-if="loading" class="state">
      <div class="spinner"></div>
      <div>加载中…</div>
    </div>
    <div v-else-if="error" class="state">
      <div class="big">加载失败</div>
      <div>{{ error }}</div>
      <div style="margin-top:14px"><button class="hbtn" @click="load">重试</button></div>
    </div>
    <div v-else>
      <!-- Hero：音乐律动风 -->
      <section class="hero hero-music">
        <div class="waveform" aria-hidden="true">
          <i v-for="n in 48" :key="n" :style="{ '--d': (n % 12) * 0.09 + 's', '--h': (18 + ((n * 37) % 82)) + '%' }"></i>
        </div>
        <div class="hero-inner">
          <div class="hero-logo"><img src="/logo.png?v=20261004" alt="R导航" /></div>
          <h1 class="hero-title"><span class="hero-r">R</span>导航</h1>
          <p class="hero-sub">一个干净、快速的网址导航</p>
          <div class="hero-search-tabs">
            <button v-for="e in engines" :key="e.id" :class="{ active: engine === e.id }" @click="engine = e.id">{{ e.name }}</button>
          </div>
          <div class="hero-search">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
            <input v-model="keyword" :placeholder="enginePlaceholder" @keydown.enter="onSearchEnter" @keydown.down.prevent="moveSel(1)" @keydown.up.prevent="moveSel(-1)" @focus="showSuggest = true" @blur="hideSuggest" />
            <button v-if="keyword" class="hero-clear" @click="keyword=''">✕</button>
            <button class="hero-go" @click="onSearchEnter">搜索</button>
            <div v-if="showSuggest && suggestions.length" class="suggest">
              <div v-for="(s, i) in suggestions" :key="s.id" class="suggest-item" :class="{ sel: i === selIdx }" @mousedown.prevent="openSuggestion(s)">
                <span class="suggest-icon"><img v-if="showImg(s)" :src="iconSrc(s)" @error="brokenIcons.add(s.id)"><span v-else class="letter">{{ (s.title || '?').slice(0,1) }}</span></span>
                <span class="suggest-name">{{ s.title }}</span>
                <span class="suggest-group">{{ s.groupTitle }}</span>
              </div>
              <div class="suggest-web" @mousedown.prevent="webSearch()">
                <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
                用{{ engineName }}搜索「{{ keyword }}」
              </div>
            </div>
          </div>
        </div>
      </section>
      <div v-if="!filteredGroups.length" class="state">
        <div class="big">没有匹配「{{ keyword }}」的书签</div>
      </div>
      <section v-else v-for="g in filteredGroups" :key="g.id" class="group">
      <div class="group-head">
        <span class="group-title">{{ g.title }}</span>
        <span v-if="g.publicVisitHide === 1" class="group-badge">仅管理员</span>
        <span v-if="g.description" class="group-desc">{{ g.description }}</span>
      </div>
      <div class="cards">
        <div v-for="c in g.cards" :key="c.id" class="card" :class="['sz' + sizeOf(c), { playing: playingId === c.id }]" @click="onCardClick(c, $event)" :title="c.description || c.title">
          <div class="card-icon">
            <span class="letter">{{ (c.title || '?').slice(0, 1) }}</span>
            <img v-if="showImg(c)" :src="iconSrc(c)" :alt="c.title" loading="lazy" @error="brokenIcons.add(c.id)">
            <span v-if="playingId === c.id" class="eq" aria-hidden="true"><i></i><i></i><i></i></span>
          </div>
          <div class="card-text">
            <div class="card-name">{{ c.title }}</div>
            <div v-if="loggedIn && c.description" class="card-desc">{{ c.description }}</div>
          </div>
        </div>
      </div>
    </section>
    <div class="footer">R导航</div>
    </div>
  </main>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { api, isLoggedIn, iconifyUrl } from '../api.js'

const groups = ref([])
const loading = ref(true)
const error = ref('')
const loggedIn = ref(isLoggedIn())
const brokenIcons = ref(new Set())
const keyword = ref('')
const playingId = ref(null)
let playingTimer = null
const showSuggest = ref(false)
const selIdx = ref(-1)

// 搜索引擎
const engines = [
  { id: 'bookmark', name: '书签' },
  { id: 'baidu', name: '百度' },
  { id: 'google', name: 'Google' },
  { id: 'bing', name: '必应' },
  { id: 'ai', name: 'AI搜索' },
]
const engine = ref('bookmark')
const engineName = computed(() => (engines.find(e => e.id === engine.value) || {}).name || '')
const enginePlaceholder = computed(() => {
  switch (engine.value) {
    case 'baidu': return '用百度搜索…'
    case 'google': return '用 Google 搜索…'
    case 'bing': return '用必应搜索…'
    case 'ai': return '问 AI…（回车用 DeepSeek）'
    default: return '搜索书签…'
  }
})
function webSearch() {
  const q = keyword.value.trim()
  if (!q) return
  const eq = encodeURIComponent(q)
  const urls = {
    baidu: `https://www.baidu.com/s?wd=${eq}`,
    google: `https://www.google.com/search?q=${eq}`,
    bing: `https://www.bing.com/search?q=${eq}`,
    ai: `https://chat.deepseek.com/?q=${eq}`,
  }
  const url = urls[engine.value]
  showSuggest.value = false
  if (url) window.open(url, '_blank', 'noopener')
  else if (engine.value === 'bookmark') onSearchEnter()
}
function onSearchEnter() {
  if (engine.value !== 'bookmark') { webSearch(); return }
  if (selIdx.value >= 0 && suggestions.value[selIdx.value]) {
    openSuggestion(suggestions.value[selIdx.value])
  } else if (suggestions.value.length) {
    openSuggestion(suggestions.value[0])
  } else {
    openFirst()
  }
}

// 自动补全：取所有匹配卡片的前 8 个
const suggestions = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return []
  const out = []
  for (const g of filteredGroups.value) {
    for (const c of (g.cards || [])) {
      out.push({ ...c, groupTitle: g.title })
      if (out.length >= 8) return out
    }
  }
  return out
})
function hideSuggest() { setTimeout(() => { showSuggest.value = false; selIdx.value = -1 }, 150) }
function moveSel(d) {
  if (!suggestions.value.length) return
  showSuggest.value = true
  selIdx.value = (selIdx.value + d + suggestions.value.length) % suggestions.value.length
}
function openSuggestion(s) {
  showSuggest.value = false
  keyword.value = ''
  openCard(s)
}

function onCardClick(c, ev) {
  // 涟漪：从点击位置扩散
  const el = ev.currentTarget
  if (el) {
    const r = el.getBoundingClientRect()
    const x = (ev.clientX || r.left + r.width / 2) - r.left
    const y = (ev.clientY || r.top + r.height / 2) - r.top
    const s = document.createElement('span')
    s.className = 'ripple'
    const d = Math.max(r.width, r.height) * 2.2
    s.style.width = s.style.height = d + 'px'
    s.style.left = (x - d / 2) + 'px'
    s.style.top = (y - d / 2) + 'px'
    el.appendChild(s)
    setTimeout(() => s.remove(), 650)
  }
  // 均衡器：图标上跳动 0.9 秒
  playingId.value = c.id
  clearTimeout(playingTimer)
  playingTimer = setTimeout(() => { playingId.value = null }, 900)
  // 稍延迟打开，让动效被看见
  setTimeout(() => openCard(c), 180)
}

const filteredGroups = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return groups.value
  // 模糊匹配：查询字符按顺序出现在目标中即可（如 "ggle" 命中 "Google"，"blbl" 命中 "Bilibili"）
  function fuzzyMatch(text, pattern) {
    text = (text || '').toLowerCase()
    let ti = 0
    for (let pi = 0; pi < pattern.length; pi++) {
      const pc = pattern[pi]
      ti = text.indexOf(pc, ti)
      if (ti === -1) return false
      ti++
    }
    return true
  }
  // 拼音首字母：常用字映射（如 "rgzn" 命中 "R智能"，"dh" 命中 "导航"）
  const PY = { 智:'z',能:'n',导:'d',航:'h',论:'l',坛:'t',视:'s',频:'p',资:'z',讯:'x',人:'r',工:'g',金:'j',融:'r',经:'j',济:'j',网:'w',络:'l',服:'f',务:'w',应:'y',用:'y',下:'x',载:'z',源:'y',中:'z',文:'w',平:'p',台:'t',控:'k',制:'z',算:'s',力:'l' }
  function pyInitials(text) {
    return (text || '').split('').map(ch => PY[ch] || (/[a-z]/i.test(ch) ? ch.toLowerCase() : '')).join('')
  }
  function matchCard(c) {
    const title = c.title || '', desc = c.description || '', url = (c.cardData?.url || '') + ''
    // 精确包含优先
    if (title.toLowerCase().includes(kw) || desc.toLowerCase().includes(kw) || url.toLowerCase().includes(kw)) return true
    // 模糊：标题/拼音首字母
    if (fuzzyMatch(title, kw) || fuzzyMatch(pyInitials(title), kw)) return true
    return false
  }
  return groups.value.map(g => ({
    ...g,
    cards: (g.cards || []).filter(matchCard)
  })).filter(g => g.cards.length)
})

function openFirst() {
  const g = filteredGroups.value[0]
  if (g && g.cards[0]) openCard(g.cards[0])
}

function showImg(c) {
  if (brokenIcons.value.has(c.id)) return false
  const ic = c.cardData?.icon
  if (!ic) return false
  return (ic.type === 3 && !!ic.text) || (ic.type === 2 && !!ic.src)
}
function iconSrc(c) {
  const ic = c.cardData?.icon || {}
  if (ic.type === 3 && ic.text) return iconifyUrl(ic.text)
  return ic.src || ''
}

// 卡片尺寸：1 紧凑 / 2 标准 / 3 宽卡 / 4 高卡 / 5 大卡 / 6 横幅；未知或缺失一律按 2（标准）处理
function sizeOf(c) {
  const s = Number(c?.cardSize)
  return s >= 1 && s <= 6 ? s : 2
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const d = await api('/panel/itemCard/getGroupList', {})
    groups.value = Array.isArray(d) ? d : []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function openCard(c) {
  const cd = c.cardData || {}
  const url = cd.url || cd.lanUrl || ''
  if (!url) return
  const u = /^https?:\/\//i.test(url) ? url : 'https://' + url
  if (cd.openMethod === 1) {
    location.href = u
  } else {
    window.open(u, '_blank', 'noopener')
  }
}

onMounted(load)
</script>
