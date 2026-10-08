<template>
  <main class="main">
    <div class="admin-tabs">
      <button class="hbtn" :class="{ active: tab === 'group' }" @click="tab = 'group'">分组管理</button>
      <button class="hbtn" :class="{ active: tab === 'card' }" @click="tab = 'card'">卡片管理</button>
      <button class="hbtn" @click="$router.push('/admin/docker')">Docker 管理</button>
    </div>

    <div v-if="error" class="err">{{ error }}</div>

    <!-- 分组管理 -->
    <div v-if="tab === 'group'">
      <div class="toolbar">
        <button class="btn sm" @click="openGroupModal()">＋ 新建分组</button>
      </div>
      <div class="list">
        <div v-for="g in groups" :key="g.id" class="row">
          <div class="grow">
            <div class="t">{{ g.title }} <span class="tag hide" v-if="g.publicVisitHide === 1">访客隐藏</span><span class="tag" v-else>访客可见</span></div>
            <div class="d">{{ g.description || '—' }} · {{ (g.cards || []).length }} 张卡片</div>
          </div>
          <div class="acts">
            <button class="mini" @click="moveGroup(g, -1)">↑</button>
            <button class="mini" @click="moveGroup(g, 1)">↓</button>
            <button class="mini" @click="toggleHide(g)">{{ g.publicVisitHide === 1 ? '设为可见' : '访客隐藏' }}</button>
            <button class="mini" @click="openGroupModal(g)">编辑</button>
            <button class="mini danger" @click="delGroup(g)">删除</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 卡片管理 -->
    <div v-if="tab === 'card'">
      <div class="toolbar">
        <select class="select" v-model="curGroupId" style="width:auto;min-width:160px">
          <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.title }}</option>
        </select>
        <button class="btn sm" @click="openCardModal()">＋ 新建卡片</button>
      </div>
      <div class="list">
        <div v-for="c in curCards" :key="c.id" class="row" draggable="true" @dragstart="dragC = c" @dragend="dragC = null">
          <div class="grow">
            <div class="t">{{ c.title }} <span class="tag">{{ sizeLabel(c) }}</span></div>
            <div class="d">{{ cardUrl(c) }}</div>
          </div>
          <div class="acts">
            <button class="mini" @click="moveCard(c, -1)">↑</button>
            <button class="mini" @click="moveCard(c, 1)">↓</button>
            <button class="mini" @click="openCardModal(c)">编辑</button>
            <button class="mini danger" @click="delCard(c)">删除</button>
          </div>
        </div>
        <div v-if="!curCards.length" class="row"><div class="d">该分组暂无卡片</div></div>
      </div>
      <div v-if="groups.length > 1" class="drop-row" style="margin-top:12px">
        <span class="drop-hint">拖拽卡片到分组以跨分组移动：</span>
        <span v-for="g in groups" :key="g.id" class="drop-chip" :class="{ over: dropOver === g.id, cur: g.id === curGroupId }"
          @dragover.prevent="dropOver = g.id" @dragleave="dropOver = null" @drop.prevent="dropToGroup(g)">📁 {{ g.title }}</span>
      </div>
    </div>

    <!-- 分组弹窗 -->
    <div v-if="showGroupModal" class="modal-mask" @click.self="showGroupModal = false">
      <div class="modal">
        <h2>{{ groupForm.id ? '编辑分组' : '新建分组' }}</h2>
        <div class="field"><label>标题</label><input class="input" v-model="groupForm.title" placeholder="分组名称"></div>
        <div class="field"><label>描述</label><input class="input" v-model="groupForm.description" placeholder="可选"></div>
        <div class="switch-row">
          <span>访客隐藏（仅管理员可见）</span>
          <label class="switch"><input type="checkbox" v-model="groupForm.hide"><span class="tr"></span></label>
        </div>
        <div class="foot">
          <button class="btn ghost" @click="showGroupModal = false">取消</button>
          <button class="btn" :disabled="busy" @click="saveGroup">保存</button>
        </div>
      </div>
    </div>

    <!-- 卡片弹窗 -->
    <div v-if="showCardModal" class="modal-mask" @click.self="showCardModal = false">
      <div class="modal">
        <h2>{{ cardForm.id ? '编辑卡片' : '新建卡片' }}</h2>
        <div class="field"><label>标题</label><input class="input" v-model="cardForm.title" placeholder="卡片名称"></div>
        <div class="field"><label>链接</label><input class="input" v-model="cardForm.url" placeholder="https://"></div>
        <div class="field"><label>描述</label><input class="input" v-model="cardForm.description" placeholder="可选"></div>
        <div class="field"><label>所属分组</label>
          <select class="select" v-model="cardForm.groupId">
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.title }}</option>
          </select>
        </div>
        <div class="field"><label>打开方式</label>
          <select class="select" v-model.number="cardForm.openMethod">
            <option :value="2">新标签页</option>
            <option :value="1">当前页</option>
          </select>
        </div>
        <div class="field"><label>卡片尺寸</label>
          <div class="size-picker">
            <button type="button" v-for="s in cardSizes" :key="s.v" class="size-opt" :class="{ sel: cardForm.cardSize === s.v }" @click="cardForm.cardSize = s.v">
              <span class="size-grid" :class="'g' + s.v"><i v-for="n in s.cells" :key="n" :class="{ on: s.hot.includes(n) }"></i></span>
              <span class="size-lbl">{{ s.label }}</span>
            </button>
          </div>
        </div>
        <div class="field"><label>图标类型</label>
          <select class="select" v-model.number="cardForm.iconType">
            <option :value="3">Iconify 图标（如 devicon:google）</option>
            <option :value="2">图片地址</option>
            <option :value="1">首字母</option>
          </select>
        </div>
        <div class="field" v-if="cardForm.iconType === 3"><label>Iconify 名称</label><input class="input" v-model="cardForm.iconText" placeholder="devicon:google"></div>
        <div class="field" v-if="cardForm.iconType === 2"><label>图片地址</label><input class="input" v-model="cardForm.iconSrc" placeholder="https://"></div>
        <div class="foot">
          <button class="btn ghost" @click="showCardModal = false">取消</button>
          <button class="btn" :disabled="busy" @click="saveCard">保存</button>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api.js'

const tab = ref('group')
const groups = ref([])
const error = ref('')
const busy = ref(false)
const curGroupId = ref(null)

const showGroupModal = ref(false)
const groupForm = ref({ id: 0, title: '', description: '', hide: false })
const showCardModal = ref(false)
const cardForm = ref({ id: 0, title: '', url: '', description: '', groupId: null, openMethod: 2, cardSize: 2, iconType: 3, iconText: '', iconSrc: '' })
const dragC = ref(null)
const dropOver = ref(null)

// 卡片尺寸选项：1 紧凑 / 2 标准 / 3 宽卡 / 4 高卡 / 5 大卡 / 6 横幅
const cardSizes = [
  { v: 1, label: '紧凑', cells: [1, 2, 3, 4], hot: [1] },
  { v: 2, label: '标准', cells: [1, 2, 3, 4], hot: [1] },
  { v: 3, label: '宽卡', cells: [1, 2, 3, 4], hot: [1, 2] },
  { v: 4, label: '高卡', cells: [1, 2, 3, 4], hot: [1, 3] },
  { v: 5, label: '大卡', cells: [1, 2, 3, 4], hot: [1, 2, 3, 4] },
  { v: 6, label: '横幅', cells: [1, 2, 3], hot: [1, 2, 3] }
]
function sizeLabel(c) {
  const s = cardSizes.find(x => x.v === Number(c?.cardSize))
  return s ? s.label : '标准'
}

const curCards = computed(() => {
  const g = groups.value.find(x => x.id === curGroupId.value)
  return g ? (g.cards || []) : []
})

function cardUrl(c) {
  return c.cardData?.url || c.cardData?.lanUrl || '—'
}

async function load() {
  error.value = ''
  try {
    const d = await api('/panel/itemCard/getGroupList', {})
    groups.value = Array.isArray(d) ? d : []
    if (!curGroupId.value && groups.value.length) curGroupId.value = groups.value[0].id
    if (curGroupId.value && !groups.value.find(g => g.id === curGroupId.value)) {
      curGroupId.value = groups.value.length ? groups.value[0].id : null
    }
  } catch (e) {
    error.value = e.message
  }
}

// ---- 分组 ----
function openGroupModal(g) {
  groupForm.value = g
    ? { id: g.id, title: g.title, description: g.description || '', hide: g.publicVisitHide === 1 }
    : { id: 0, title: '', description: '', hide: false }
  showGroupModal.value = true
}

async function saveGroup() {
  if (!groupForm.value.title.trim()) { error.value = '请输入分组标题'; return }
  busy.value = true
  try {
    await api('/panel/itemCardGroup/edit', {
      id: groupForm.value.id || undefined,
      title: groupForm.value.title.trim(),
      description: groupForm.value.description.trim(),
      publicVisitHide: groupForm.value.hide ? 1 : 0
    })
    showGroupModal.value = false
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function delGroup(g) {
  if (!confirm(`删除分组「${g.title}」及其下所有卡片？`)) return
  try {
    await api('/panel/itemCardGroup/deletes', { ids: [g.id] })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function toggleHide(g) {
  try {
    await api('/panel/itemCardGroup/setPublicVisitHide', {
      id: g.id,
      publicVisitHide: g.publicVisitHide === 1 ? 0 : 1
    })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function moveGroup(g, dir) {
  const arr = [...groups.value].sort((a, b) => a.sort - b.sort)
  const i = arr.findIndex(x => x.id === g.id)
  const j = i + dir
  if (i < 0 || j < 0 || j >= arr.length) return
  const a = arr[i], b = arr[j]
  try {
    await api('/panel/itemCardGroup/saveSort', [
      { id: a.id, sort: b.sort },
      { id: b.id, sort: a.sort }
    ])
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// ---- 卡片 ----
function openCardModal(c) {
  const cd = c?.cardData || {}
  const ic = cd.icon || {}
  cardForm.value = c
    ? {
        id: c.id, title: c.title, url: cd.url || '', description: c.description || '',
        groupId: c.itemCardGroupId, openMethod: cd.openMethod || 2,
        cardSize: [1, 2, 3, 4, 5, 6].includes(Number(c.cardSize)) ? Number(c.cardSize) : 2,
        iconType: ic.type || 3, iconText: ic.text || '', iconSrc: ic.src || ''
      }
    : {
        id: 0, title: '', url: '', description: '',
        groupId: curGroupId.value, openMethod: 2, cardSize: 2,
        iconType: 3, iconText: '', iconSrc: ''
      }
  showCardModal.value = true
}

async function saveCard() {
  if (!cardForm.value.title.trim()) { error.value = '请输入卡片标题'; return }
  if (!cardForm.value.url.trim()) { error.value = '请输入链接'; return }
  if (!cardForm.value.groupId) { error.value = '请选择分组'; return }
  busy.value = true
  try {
    await api('/panel/itemCard/edit', {
      id: cardForm.value.id || undefined,
      title: cardForm.value.title.trim(),
      description: cardForm.value.description.trim(),
      itemCardGroupId: cardForm.value.groupId,
      cardSize: cardForm.value.cardSize,
      cardData: {
        url: cardForm.value.url.trim(),
        lanUrl: '',
        openMethod: cardForm.value.openMethod,
        icon: {
          type: cardForm.value.iconType,
          text: cardForm.value.iconType === 3 ? cardForm.value.iconText.trim() : '',
          src: cardForm.value.iconType === 2 ? cardForm.value.iconSrc.trim() : ''
        },
        customUrls: [],
        textColor: ''
      }
    })
    showCardModal.value = false
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function delCard(c) {
  if (!confirm(`删除卡片「${c.title}」？`)) return
  try {
    await api('/panel/itemCard/deletes', { ids: [c.id] })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function moveCard(c, dir) {
  const arr = [...curCards.value].sort((a, b) => a.sort - b.sort)
  const i = arr.findIndex(x => x.id === c.id)
  const j = i + dir
  if (i < 0 || j < 0 || j >= arr.length) return
  const a = arr[i], b = arr[j]
  try {
    await api('/panel/itemCard/saveSort', [
      { id: a.id, sort: b.sort },
      { id: b.id, sort: a.sort }
    ])
    await load()
  } catch (e) {
    error.value = e.message
  }
}

// 跨分组拖拽：把卡片拖到分组 chip 上即移动
async function dropToGroup(g) {
  dropOver.value = null
  const c = dragC.value
  dragC.value = null
  if (!c || c.itemCardGroupId === g.id) return
  try {
    await api('/panel/itemCard/edit', {
      id: c.id,
      title: c.title,
      description: c.description || '',
      background: c.background || '',
      onlyName: c.onlyName || '',
      cardSize: [1, 2, 3, 4, 5, 6].includes(Number(c.cardSize)) ? Number(c.cardSize) : 2,
      sort: c.sort || 0,
      showTitle: c.showTitle || 0,
      blur: c.blur || 0,
      microAppId: c.microAppId || '',
      itemCardGroupId: g.id,
      cardData: c.cardData || {}
    })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>
