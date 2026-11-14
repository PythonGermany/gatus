<template>
  <div class="dashboard-container bg-background">
    <div class="container mx-auto px-4 py-8 max-w-7xl">
      <div class="mb-6">
        <div class="flex items-center justify-between mb-6">
          <div>
            <h1 class="text-4xl font-bold tracking-tight">{{ dashboardHeading }}</h1>
            <p class="text-muted-foreground mt-2">{{ dashboardSubheading }}</p>
          </div>
          <div class="flex items-center gap-4">
            <Button v-if="hasCustomOrder" variant="ghost" size="icon" @click="resetCustomOrder" title="Reset custom order">
              <ListRestart class="h-5 w-5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              @click="toggleShowAverageResponseTime"
              :title="showAverageResponseTime ? 'Show min-max response time' : 'Show average response time'"
            >
              <Activity v-if="showAverageResponseTime" class="h-5 w-5" />
              <Timer v-else class="h-5 w-5" />
            </Button>
            <Button variant="ghost" size="icon" @click="refreshData" title="Refresh data">
              <RefreshCw class="h-5 w-5" />
            </Button>
          </div>
        </div>
        <!-- Announcement Banner (Active Announcements) -->
        <AnnouncementBanner :announcements="activeAnnouncements" />
        <!-- Search bar -->
        <SearchBar
          @search="handleSearch"
          @update:showOnlyFailing="showOnlyFailing = $event"
          @update:showRecentFailures="showRecentFailures = $event"
          @update:groupByGroup="groupByGroup = $event"
          @update:sortBy="sortBy = $event"
          @initializeCollapsedGroups="initializeCollapsedGroups"
        />
      </div>

      <div v-if="loading" class="flex items-center justify-center py-20">
        <Loading size="lg" />
      </div>

      <div v-else-if="filteredEndpoints.length === 0 && filteredSuites.length === 0" class="text-center py-20">
        <AlertCircle class="h-12 w-12 text-muted-foreground mx-auto mb-4" />
        <h3 class="text-lg font-semibold mb-2">No endpoints or suites found</h3>
        <p class="text-muted-foreground">
          {{ searchQuery || showOnlyFailing || showRecentFailures 
            ? 'Try adjusting your filters' 
            : 'No endpoints or suites are configured' }}
        </p>
      </div>

      <div v-else>
        <!-- Grouped view -->
        <TransitionGroup
          v-if="groupByGroup"
          tag="div"
          :name="reorderTransitionName"
          class="space-y-6"
          @dragover="onListDragOver('group', GROUPS_CONTAINER, $event)"
          @drop="onListDrop('group', GROUPS_CONTAINER, $event)"
        >
          <div
            v-for="group in combinedGroups"
            :key="group.name"
            :class="['endpoint-group border rounded-lg overflow-hidden transition-opacity', isDragged('group', group.name) && 'opacity-40']"
            @dragover="onItemDragOver('group', group.name, GROUPS_CONTAINER, $event)"
          >
            <!-- Group Header -->
            <div
              @click="toggleGroupCollapse(group.name)"
              class="endpoint-group-header group relative flex items-center justify-between p-4 bg-card border-b cursor-pointer hover:bg-accent/50 transition-colors"
            >
              <span
                v-if="canReorder"
                draggable="true"
                class="drag-handle card-drag-handle group-drag-handle opacity-0 group-hover:opacity-100"
                title="Drag to reorder"
                aria-label="Drag to reorder group"
                @click.stop
                @dragstart="onDragStart('group', group.name, GROUPS_CONTAINER, $event)"
                @dragend="onDragEnd"
              >
                <GripVertical class="h-4 w-4" />
              </span>
              <div class="flex items-center gap-3">
                <ChevronDown v-if="uncollapsedGroups.has(group.name)" class="h-5 w-5 text-muted-foreground" />
                <ChevronUp v-else class="h-5 w-5 text-muted-foreground" />
                <h2 class="text-xl font-semibold text-foreground">{{ group.name }}</h2>
              </div>
              <div class="flex items-center gap-2">
                <span v-if="calculateUnhealthyCount(group.endpoints) + calculateFailingSuitesCount(group.suites) > 0"
                      class="bg-red-600 text-white px-2 py-1 rounded-full text-sm font-medium">
                  {{ calculateUnhealthyCount(group.endpoints) + calculateFailingSuitesCount(group.suites) }}
                </span>
                <CheckCircle v-else class="h-6 w-6 text-green-600" />
              </div>
            </div>

            <!-- Group Content -->
            <div v-if="uncollapsedGroups.has(group.name)" class="endpoint-group-content p-4">
              <!-- Suites Section -->
              <div v-if="group.suites.length > 0" class="mb-4">
                <h3 class="text-sm font-semibold text-muted-foreground uppercase tracking-wider mb-3">Suites</h3>
                <TransitionGroup
                  tag="div"
                  :name="reorderTransitionName"
                  class="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3"
                  @dragover="onListDragOver('suite', group.name, $event)"
                  @drop="onListDrop('suite', group.name, $event)"
                >
                  <div
                    v-for="suite in group.suites"
                    :key="suite.key"
                    :class="['reorder-item group relative h-full transition-opacity', isDragged('suite', suite.key) && 'opacity-40']"
                    @dragover="onItemDragOver('suite', suite.key, group.name, $event)"
                  >
                    <SuiteCard
                      :suite="suite"
                      :maxResults="resultPageSize"
                      @showTooltip="showTooltip"
                    />
                    <span
                      v-if="canReorder"
                      draggable="true"
                      class="drag-handle card-drag-handle opacity-0 group-hover:opacity-100"
                      title="Drag to reorder"
                      :aria-label="`Drag to reorder suite ${suite.name}`"
                      @dragstart="onDragStart('suite', suite.key, group.name, $event)"
                      @dragend="onDragEnd"
                    >
                      <GripVertical class="h-4 w-4" />
                    </span>
                  </div>
                </TransitionGroup>
              </div>

              <!-- Endpoints Section -->
              <div v-if="group.endpoints.length > 0">
                <h3 v-if="group.suites.length > 0" class="text-sm font-semibold text-muted-foreground uppercase tracking-wider mb-3">Endpoints</h3>
                <TransitionGroup
                  tag="div"
                  :name="reorderTransitionName"
                  class="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3"
                  @dragover="onListDragOver('endpoint', group.name, $event)"
                  @drop="onListDrop('endpoint', group.name, $event)"
                >
                  <div
                    v-for="endpoint in group.endpoints"
                    :key="endpoint.key"
                    :class="['reorder-item group relative h-full transition-opacity', isDragged('endpoint', endpoint.key) && 'opacity-40']"
                    @dragover="onItemDragOver('endpoint', endpoint.key, group.name, $event)"
                  >
                    <EndpointCard
                      :endpoint="endpoint"
                      :maxResults="resultPageSize"
                      :showAverageResponseTime="showAverageResponseTime"
                      @showTooltip="showTooltip"
                    />
                    <span
                      v-if="canReorder"
                      draggable="true"
                      class="drag-handle card-drag-handle opacity-0 group-hover:opacity-100"
                      title="Drag to reorder"
                      :aria-label="`Drag to reorder endpoint ${endpoint.name}`"
                      @dragstart="onDragStart('endpoint', endpoint.key, group.name, $event)"
                      @dragend="onDragEnd"
                    >
                      <GripVertical class="h-4 w-4" />
                    </span>
                  </div>
                </TransitionGroup>
              </div>
            </div>
          </div>
        </TransitionGroup>

        <!-- Regular view -->
        <div v-else>
          <!-- Suites Section -->
          <div v-if="filteredSuites.length > 0" class="mb-6">
            <h2 class="text-lg font-semibold text-foreground mb-3">Suites</h2>
            <TransitionGroup
              tag="div"
              :name="reorderTransitionName"
              class="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3"
              @dragover="onListDragOver('suite', FLAT_CONTAINER, $event)"
              @drop="onListDrop('suite', FLAT_CONTAINER, $event)"
            >
              <div
                v-for="suite in paginatedSuites"
                :key="suite.key"
                :class="['reorder-item group relative h-full transition-opacity', isDragged('suite', suite.key) && 'opacity-40']"
                @dragover="onItemDragOver('suite', suite.key, FLAT_CONTAINER, $event)"
              >
                <SuiteCard
                  :suite="suite"
                  :maxResults="resultPageSize"
                  @showTooltip="showTooltip"
                />
                <span
                  v-if="canReorder"
                  draggable="true"
                  class="drag-handle card-drag-handle opacity-0 group-hover:opacity-100"
                  title="Drag to reorder"
                  :aria-label="`Drag to reorder suite ${suite.name}`"
                  @dragstart="onDragStart('suite', suite.key, FLAT_CONTAINER, $event)"
                  @dragend="onDragEnd"
                >
                  <GripVertical class="h-4 w-4" />
                </span>
              </div>
            </TransitionGroup>
          </div>

          <!-- Endpoints Section -->
          <div v-if="filteredEndpoints.length > 0">
            <h2 v-if="filteredSuites.length > 0" class="text-lg font-semibold text-foreground mb-3">Endpoints</h2>
            <TransitionGroup
              tag="div"
              :name="reorderTransitionName"
              class="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3"
              @dragover="onListDragOver('endpoint', FLAT_CONTAINER, $event)"
              @drop="onListDrop('endpoint', FLAT_CONTAINER, $event)"
            >
              <div
                v-for="endpoint in paginatedEndpoints"
                :key="endpoint.key"
                :class="['reorder-item group relative h-full transition-opacity', isDragged('endpoint', endpoint.key) && 'opacity-40']"
                @dragover="onItemDragOver('endpoint', endpoint.key, FLAT_CONTAINER, $event)"
              >
                <EndpointCard
                  :endpoint="endpoint"
                  :maxResults="resultPageSize"
                  :showAverageResponseTime="showAverageResponseTime"
                  @showTooltip="showTooltip"
                />
                <span
                  v-if="canReorder"
                  draggable="true"
                  class="drag-handle card-drag-handle opacity-0 group-hover:opacity-100"
                  title="Drag to reorder"
                  :aria-label="`Drag to reorder endpoint ${endpoint.name}`"
                  @dragstart="onDragStart('endpoint', endpoint.key, FLAT_CONTAINER, $event)"
                  @dragend="onDragEnd"
                >
                  <GripVertical class="h-4 w-4" />
                </span>
              </div>
            </TransitionGroup>
          </div>
        </div>

        <div v-if="!groupByGroup && totalPages > 1" class="mt-8 flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="icon"
            :disabled="currentPage === 1"
            @click="goToPage(currentPage - 1)"
          >
            <ChevronLeft class="h-4 w-4" />
          </Button>
          
          <div class="flex gap-1">
            <Button
              v-for="page in visiblePages"
              :key="page"
              :variant="page === currentPage ? 'default' : 'outline'"
              size="sm"
              @click="goToPage(page)"
            >
              {{ page }}
            </Button>
          </div>

          <Button
            variant="outline"
            size="icon"
            :disabled="currentPage === totalPages"
            @click="goToPage(currentPage + 1)"
          >
            <ChevronRight class="h-4 w-4" />
          </Button>
        </div>
      </div>

      <!-- Past Announcements Section -->
      <div v-if="archivedAnnouncements.length > 0" class="mt-12 pb-8">
        <PastAnnouncements :announcements="archivedAnnouncements" />
      </div>
    </div>

    <Settings @refreshData="fetchData" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Activity, Timer, RefreshCw, AlertCircle, ChevronLeft, ChevronRight, ChevronDown, ChevronUp, CheckCircle, GripVertical, ListRestart } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EndpointCard from '@/components/EndpointCard.vue'
import SuiteCard from '@/components/SuiteCard.vue'
import SearchBar from '@/components/SearchBar.vue'
import Settings from '@/components/Settings.vue'
import Loading from '@/components/Loading.vue'
import AnnouncementBanner from '@/components/AnnouncementBanner.vue'
import PastAnnouncements from '@/components/PastAnnouncements.vue'
import { CUSTOM_ORDER_STORAGE_KEYS, loadOrder, saveOrder, applyOrder, moveKey } from '@/utils/customOrder'

const props = defineProps({
  announcements: {
    type: Array,
    default: () => []
  }
})

// Computed properties for active and archived announcements
const activeAnnouncements = computed(() => {
  return props.announcements ? props.announcements.filter(a => !a.archived) : []
})

const archivedAnnouncements = computed(() => {
  return props.announcements ? props.announcements.filter(a => a.archived) : []
})

const emit = defineEmits(['showTooltip'])

const endpointStatuses = ref([])
const suiteStatuses = ref([])
const loading = ref(false)
const currentPage = ref(1)
const itemsPerPage = 96
const searchQuery = ref('')
const showOnlyFailing = ref(false)
const showRecentFailures = ref(false)
const showAverageResponseTime = ref(localStorage.getItem('gatus:show-average-response-time') !== 'false')
const groupByGroup = ref(false)
const sortBy = ref(localStorage.getItem('gatus:sort-by') || 'name')
const uncollapsedGroups = ref(new Set())
const resultPageSize = 50

// Custom (drag and drop) order, applied on top of the name and group sorting
const NO_GROUP = 'No Group'
const FLAT_CONTAINER = '__flat__'
const GROUPS_CONTAINER = '__groups__'
const REORDER_COOLDOWN_MS = 250
const REORDER_ANIMATION_MS = 300
const endpointOrder = ref(loadOrder(CUSTOM_ORDER_STORAGE_KEYS.endpoints))
const suiteOrder = ref(loadOrder(CUSTOM_ORDER_STORAGE_KEYS.suites))
const groupOrder = ref(loadOrder(CUSTOM_ORDER_STORAGE_KEYS.groups))
const drag = ref(null)
const animateReorder = ref(false)
let animateReorderTimeout = null

const canReorder = computed(() => sortBy.value !== 'health')
const hasCustomOrder = computed(() => {
  return canReorder.value && (endpointOrder.value.length > 0 || suiteOrder.value.length > 0 || groupOrder.value.length > 0)
})
// TransitionGroup only animates moves while its move class exists, so animations are limited to reordering
const reorderTransitionName = computed(() => animateReorder.value ? 'reorder' : 'reorder-disabled')

const sortGroupNames = (names) => {
  // Sort groups alphabetically, with 'No Group' at the end
  const sorted = [...names].sort((a, b) => {
    if (a === NO_GROUP) return 1
    if (b === NO_GROUP) return -1
    return a.localeCompare(b)
  })
  return applyOrder(sorted, groupOrder.value)
}

const filteredEndpoints = computed(() => {
  let filtered = [...endpointStatuses.value]
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    filtered = filtered.filter(endpoint => 
      endpoint.name.toLowerCase().includes(query) ||
      (endpoint.group && endpoint.group.toLowerCase().includes(query))
    )
  }
  
  if (showOnlyFailing.value) {
    filtered = filtered.filter(endpoint => {
      if (!endpoint.results || endpoint.results.length === 0) return false
      const latestResult = endpoint.results[endpoint.results.length - 1]
      return !latestResult.success
    })
  }
  
  if (showRecentFailures.value) {
    filtered = filtered.filter(endpoint => {
      if (!endpoint.results || endpoint.results.length === 0) return false
      return endpoint.results.some(result => !result.success)
    })
  }
  
  // Sort by health if selected
  if (sortBy.value === 'health') {
    filtered.sort((a, b) => {
      const aHealthy = a.results && a.results.length > 0 && a.results[a.results.length - 1].success
      const bHealthy = b.results && b.results.length > 0 && b.results[b.results.length - 1].success
      
      // Unhealthy first
      if (!aHealthy && bHealthy) return -1
      if (aHealthy && !bHealthy) return 1
      
      // Then sort by name
      return a.name.localeCompare(b.name)
    })
  }

  if (canReorder.value) {
    filtered = applyOrder(filtered, endpointOrder.value, endpoint => endpoint.key)
  }

  return filtered
})

const filteredSuites = computed(() => {
  let filtered = [...(suiteStatuses.value || [])]
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    filtered = filtered.filter(suite => 
      suite.name.toLowerCase().includes(query) ||
      (suite.group && suite.group.toLowerCase().includes(query))
    )
  }
  
  if (showOnlyFailing.value) {
    filtered = filtered.filter(suite => {
      if (!suite.results || suite.results.length === 0) return false
      return !suite.results[suite.results.length - 1].success
    })
  }
  
  if (showRecentFailures.value) {
    filtered = filtered.filter(suite => {
      if (!suite.results || suite.results.length === 0) return false
      return suite.results.some(result => !result.success)
    })
  }
  
  // Sort by health if selected
  if (sortBy.value === 'health') {
    filtered.sort((a, b) => {
      const aHealthy = a.results && a.results.length > 0 && a.results[a.results.length - 1].success
      const bHealthy = b.results && b.results.length > 0 && b.results[b.results.length - 1].success
      
      // Unhealthy first
      if (!aHealthy && bHealthy) return -1
      if (aHealthy && !bHealthy) return 1
      
      // Then sort by name
      return a.name.localeCompare(b.name)
    })
  }

  if (canReorder.value) {
    filtered = applyOrder(filtered, suiteOrder.value, suite => suite.key)
  }

  return filtered
})

const totalPages = computed(() => {
  return Math.ceil((filteredEndpoints.value.length + filteredSuites.value.length) / itemsPerPage)
})

const groupedEndpoints = computed(() => {
  if (!groupByGroup.value) {
    return null
  }
  
  const grouped = {}
  filteredEndpoints.value.forEach(endpoint => {
    const group = endpoint.group || 'No Group'
    if (!grouped[group]) {
      grouped[group] = []
    }
    grouped[group].push(endpoint)
  })
  
  // Sort groups alphabetically, with 'No Group' at the end
  const sortedGroups = Object.keys(grouped).sort((a, b) => {
    if (a === 'No Group') return 1
    if (b === 'No Group') return -1
    return a.localeCompare(b)
  })
  
  const result = {}
  sortedGroups.forEach(group => {
    result[group] = grouped[group]
  })
  
  return result
})

const combinedGroups = computed(() => {
  if (!groupByGroup.value) {
    return null
  }
  
  // A Map keeps insertion order even for integer-like group names
  const combined = new Map()
  const getGroup = (name) => {
    if (!combined.has(name)) {
      combined.set(name, { name, endpoints: [], suites: [] })
    }
    return combined.get(name)
  }

  // Add endpoints
  filteredEndpoints.value.forEach(endpoint => {
    getGroup(endpoint.group || NO_GROUP).endpoints.push(endpoint)
  })

  // Add suites
  filteredSuites.value.forEach(suite => {
    getGroup(suite.group || NO_GROUP).suites.push(suite)
  })

  return sortGroupNames([...combined.keys()]).map(name => combined.get(name))
})

// All group names regardless of filters, used to keep hidden groups in place when reordering
const allGroupNames = computed(() => {
  const names = new Set()
  for (const item of [...endpointStatuses.value, ...(suiteStatuses.value || [])]) {
    names.add(item.group || NO_GROUP)
  }
  return sortGroupNames([...names])
})

const reorderLists = {
  endpoint: { order: endpointOrder, storageKey: CUSTOM_ORDER_STORAGE_KEYS.endpoints, allKeys: () => endpointStatuses.value.map(endpoint => endpoint.key) },
  suite: { order: suiteOrder, storageKey: CUSTOM_ORDER_STORAGE_KEYS.suites, allKeys: () => (suiteStatuses.value || []).map(suite => suite.key) },
  group: { order: groupOrder, storageKey: CUSTOM_ORDER_STORAGE_KEYS.groups, allKeys: () => allGroupNames.value }
}

const enableReorderAnimation = (keepEnabled = false) => {
  clearTimeout(animateReorderTimeout)
  animateReorder.value = true
  if (!keepEnabled) {
    animateReorderTimeout = setTimeout(() => { animateReorder.value = false }, REORDER_ANIMATION_MS)
  }
}

const matchesDrag = (type, container) => {
  return drag.value !== null && drag.value.type === type && drag.value.container === container
}

const isDragged = (type, key) => {
  return drag.value !== null && drag.value.active && drag.value.type === type && drag.value.key === key
}

const onDragStart = (type, key, container, event) => {
  const list = reorderLists[type]
  const item = event.currentTarget.closest(type === 'group' ? '.endpoint-group' : '.reorder-item')
  const dragImage = type === 'group' ? item?.querySelector('.endpoint-group-header') : item
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData('text/plain', key)
  if (dragImage) {
    const rect = dragImage.getBoundingClientRect()
    event.dataTransfer.setDragImage(dragImage, event.clientX - rect.left, event.clientY - rect.top)
  }
  drag.value = {
    type,
    key,
    container,
    snapshot: list.order.value,
    height: item ? item.getBoundingClientRect().height : 0,
    lastMoveAt: 0,
    dropped: false,
    active: false
  }
  enableReorderAnimation(true)
  // Dim the dragged item only after the browser has captured the drag image
  setTimeout(() => {
    if (drag.value && drag.value.key === key) {
      drag.value.active = true
    }
  })
}

const onItemDragOver = (type, key, container, event) => {
  if (!matchesDrag(type, container)) return
  event.preventDefault()
  event.dataTransfer.dropEffect = 'move'
  if (key === drag.value.key) return
  const now = performance.now()
  // Wait for the previous move animation, otherwise items moving under the cursor would swap back
  if (now - drag.value.lastMoveAt < REORDER_COOLDOWN_MS) return
  if (type === 'group') {
    // Groups differ in height: only move once the dragged group would end up under the cursor
    const names = combinedGroups.value.map(group => group.name)
    const movingDown = names.indexOf(drag.value.key) < names.indexOf(key)
    const rect = event.currentTarget.getBoundingClientRect()
    const threshold = Math.min(drag.value.height, rect.height)
    if (movingDown ? event.clientY < rect.bottom - threshold : event.clientY > rect.top + threshold) return
  }
  const list = reorderLists[type]
  list.order.value = moveKey(list.order.value, list.allKeys(), drag.value.key, key)
  drag.value.lastMoveAt = now
}

const onListDragOver = (type, container, event) => {
  if (!matchesDrag(type, container)) return
  event.preventDefault()
  event.dataTransfer.dropEffect = 'move'
}

const onListDrop = (type, container, event) => {
  if (!matchesDrag(type, container)) return
  event.preventDefault()
  drag.value.dropped = true
}

const onDragEnd = () => {
  if (!drag.value) return
  const list = reorderLists[drag.value.type]
  if (drag.value.dropped) {
    saveOrder(list.storageKey, list.order.value)
  } else {
    // Drag was cancelled or released outside of its list
    list.order.value = drag.value.snapshot
  }
  drag.value = null
  enableReorderAnimation()
}

const resetCustomOrder = () => {
  Object.values(reorderLists).forEach(list => {
    list.order.value = []
    saveOrder(list.storageKey, [])
  })
  enableReorderAnimation()
}

const paginatedEndpoints = computed(() => {
  if (groupByGroup.value) {
    // When grouping, we don't paginate
    return groupedEndpoints.value
  }
  
  const start = (currentPage.value - 1) * itemsPerPage
  const end = start + itemsPerPage
  return filteredEndpoints.value.slice(start, end)
})

const paginatedSuites = computed(() => {
  if (groupByGroup.value) {
    // When grouping, we don't paginate
    return filteredSuites.value
  }
  
  const start = (currentPage.value - 1) * itemsPerPage
  const end = start + itemsPerPage
  return filteredSuites.value.slice(start, end)
})

const visiblePages = computed(() => {
  const pages = []
  const maxVisible = 5
  let start = Math.max(1, currentPage.value - Math.floor(maxVisible / 2))
  let end = Math.min(totalPages.value, start + maxVisible - 1)
  
  if (end - start < maxVisible - 1) {
    start = Math.max(1, end - maxVisible + 1)
  }
  
  for (let i = start; i <= end; i++) {
    pages.push(i)
  }
  
  return pages
})

const fetchData = async () => {
  // Don't show loading state on refresh to prevent UI flicker
  const isInitialLoad = endpointStatuses.value.length === 0 && suiteStatuses.value.length === 0
  if (isInitialLoad) {
    loading.value = true
  }
  try {
    // Fetch endpoints
    const endpointResponse = await fetch(`/api/v1/endpoints/statuses?page=1&pageSize=${resultPageSize}`, {
      credentials: 'include'
    })
    if (endpointResponse.status === 200) {
      const data = await endpointResponse.json()
      endpointStatuses.value = data
    } else {
      console.error('[Home][fetchData] Error fetching endpoints:', await endpointResponse.text())
    }
    
    // Fetch suites
    const suiteResponse = await fetch(`/api/v1/suites/statuses?page=1&pageSize=${resultPageSize}`, {
      credentials: 'include'
    })
    if (suiteResponse.status === 200) {
      const suiteData = await suiteResponse.json()
      suiteStatuses.value = suiteData || []
    } else {
      console.error('[Home][fetchData] Error fetching suites:', await suiteResponse.text())
      // Ensure suiteStatuses stays as empty array instead of becoming null/undefined
      if (!suiteStatuses.value) {
        suiteStatuses.value = []
      }
    }
  } catch (error) {
    console.error('[Home][fetchData] Error:', error)
  } finally {
    if (isInitialLoad) {
      loading.value = false
    }
  }
}

const refreshData = () => {
  endpointStatuses.value = [];
  suiteStatuses.value = [];
  fetchData()
}

const handleSearch = (query) => {
  searchQuery.value = query
  currentPage.value = 1
}

const goToPage = (page) => {
  currentPage.value = page
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const toggleShowAverageResponseTime = () => {
  showAverageResponseTime.value = !showAverageResponseTime.value
  localStorage.setItem('gatus:show-average-response-time', showAverageResponseTime.value ? 'true' : 'false')
}

const showTooltip = (result, event, action = 'hover') => {
  emit('showTooltip', result, event, action)
}

const calculateUnhealthyCount = (endpoints) => {
  return endpoints.filter(endpoint => {
    if (!endpoint.results || endpoint.results.length === 0) return false
    const latestResult = endpoint.results[endpoint.results.length - 1]
    return !latestResult.success
  }).length
}

const calculateFailingSuitesCount = (suites) => {
  return suites.filter(suite => {
    if (!suite.results || suite.results.length === 0) return false
    return !suite.results[suite.results.length - 1].success
  }).length
}

const toggleGroupCollapse = (groupName) => {
  if (uncollapsedGroups.value.has(groupName)) {
    uncollapsedGroups.value.delete(groupName)
  } else {
    uncollapsedGroups.value.add(groupName)
  }
  // Save to localStorage
  const uncollapsed = Array.from(uncollapsedGroups.value)
  localStorage.setItem('gatus:uncollapsed-groups', JSON.stringify(uncollapsed))
  localStorage.removeItem('gatus:collapsed-groups') // Remove old key if it exists
}

const initializeCollapsedGroups = () => {
  // Get saved uncollapsed groups from localStorage
  try {
    const saved = localStorage.getItem('gatus:uncollapsed-groups')
    if (saved) {
      uncollapsedGroups.value = new Set(JSON.parse(saved))
    }
    // If no saved state, uncollapsedGroups stays empty (all collapsed by default)
  } catch (e) {
    console.warn('Failed to parse saved uncollapsed groups:', e)
    localStorage.removeItem('gatus:uncollapsed-groups')
    // On error, uncollapsedGroups stays empty (all collapsed by default)
  }
}

const dashboardHeading = computed(() => {
  return window.config && window.config.dashboardHeading && window.config.dashboardHeading !== '{{ .UI.DashboardHeading }}' ? window.config.dashboardHeading : "Health Dashboard"
})

const dashboardSubheading = computed(() => {
  return window.config && window.config.dashboardSubheading && window.config.dashboardSubheading !== '{{ .UI.DashboardSubheading }}' ? window.config.dashboardSubheading : "Monitor the health of your endpoints in real-time"
})

onMounted(() => {
  fetchData()
})
</script>

<style>
.reorder-move {
  transition: transform 200ms ease;
}

.card-drag-handle {
  position: absolute;
  top: 50%;
  left: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 0.75rem;
  height: 2.5rem;
  transform: translateY(-50%);
  color: hsl(var(--muted-foreground));
  cursor: grab;
  transition: opacity 150ms ease, color 150ms ease;
}

.card-drag-handle:hover {
  color: hsl(var(--foreground));
}

.card-drag-handle:active {
  cursor: grabbing;
}

@media (min-width: 640px) {
  .card-drag-handle {
    width: 1.5rem;
  }
}

/* Group headers have a uniform p-4 padding */
.card-drag-handle.group-drag-handle {
  width: 1rem;
}

/* Devices without hover (touch) would never reveal the handles */
@media (hover: none) {
  .drag-handle.drag-handle {
    opacity: 1;
  }
}
</style>