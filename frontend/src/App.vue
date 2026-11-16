<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import {
  ChooseDirectory,
  CheckForUpdate,
  DeleteAccount,
  DeleteBucket,
  DeleteMapping,
  DownloadAndInstallUpdate,
  ListAccounts,
  ListAllMappings,
  ListBucketFolders,
  OpenLocalDirectory,
  SaveAccount,
  SaveBucketSelections,
  SaveMapping,
  SyncMapping,
  TestConnection,
} from '../wailsjs/go/main/App.js'
import { BrowserOpenURL, EventsOff, EventsOn } from '../wailsjs/runtime/runtime.js'

const accounts = ref([])
const mappings = ref([])
const availableBuckets = ref([])
const bucketFolders = ref([])
const loading = ref(false)
const notice = ref('')
const error = ref('')
const showAccountModal = ref(false)
const showAccountFormModal = ref(false)
const showAboutModal = ref(false)
const showUpdateModal = ref(false)
const showMappingModal = ref(false)
const showRemoteFolderModal = ref(false)
const showSyncModal = ref(false)
const showProgressModal = ref(false)
const showProgressDetails = ref(false)
const showDeleteMappingModal = ref(false)
const showDeleteBucketModal = ref(false)
const editingAccount = ref(null)
const editingMapping = ref(null)
const syncingMapping = ref(null)
const deletingMapping = ref(null)
const deletingBucket = ref(null)
const deleteBucketConfirm = ref('')
const mappingAccountID = ref(0)
const mappingBucketName = ref('')
const syncMode = ref('overwrite')
const enableHTMLCache = ref(true)
const htmlCacheMode = ref('no-cache')
const customCacheControl = ref('')
const refreshCDN = ref(false)
const cdnDomain = ref('')
const appVersion = 'v1.0.2'
const githubURL = 'https://github.com/chunbo007/cloudsync'
const updateInfo = ref(null)
const updateLoading = ref(false)
const updateMessage = ref('')
const updateError = ref('')
const accountFormNotice = ref('')
const accountFormError = ref('')
const syncProgress = reactive({
  mappingID: 0,
  uploaded: 0,
  total: 0,
  current: '',
  status: 'running',
})
const syncDetails = ref([])
const syncResultStatus = ref('running')
const syncResultError = ref('')
const refreshedURLs = ref(0)

function openGitHub() {
  BrowserOpenURL(githubURL)
}

const accountForm = reactive({
  id: 0,
  provider: 'qiniu',
  name: '',
  accessKey: '',
  secretKey: '',
})

const mappingForm = reactive({
  id: 0,
  name: '',
  localPath: '',
  remotePrefix: '',
})

const deletingBucketName = computed(() => deletingBucket.value?.bucket || '')
const selectedAccount = computed(() => accounts.value.find((account) => account.id === mappingAccountID.value) || null)
const selectedBucket = computed(() => availableBuckets.value.find((bucket) => bucket.name === mappingBucketName.value) || null)
const cacheControlValue = computed(() => {
  if (!enableHTMLCache.value) {
    return ''
  }
  if (htmlCacheMode.value === 'custom') {
    return customCacheControl.value.trim()
  }
  return htmlCacheMode.value
})
const syncProgressPercent = computed(() => {
  if (!syncProgress.total) {
    return 0
  }
  return Math.min(100, Math.round((syncProgress.uploaded / syncProgress.total) * 100))
})

function clearMessage() {
  notice.value = ''
  error.value = ''
}

async function refreshAccounts() {
  accounts.value = (await ListAccounts()) || []
}

async function refreshMappings() {
  mappings.value = (await ListAllMappings()) || []
}

async function checkForUpdate(silent = false) {
  updateLoading.value = true
  updateMessage.value = ''
  updateError.value = ''
  try {
    const result = await CheckForUpdate()
    updateInfo.value = result.available ? result : null
    if (result.available || !silent) {
      updateMessage.value = result.available ? '发现新版本。' : '当前已经是最新版本。'
      showUpdateModal.value = true
    }
  } catch (err) {
    if (!silent) {
      updateError.value = String(err)
      showUpdateModal.value = true
    }
  } finally {
    updateLoading.value = false
  }
}

async function installUpdate() {
  if (!updateInfo.value?.manifest) {
    return
  }
  updateLoading.value = true
  updateError.value = ''
  updateMessage.value = '正在下载更新，完成后程序会自动重启。'
  try {
    await DownloadAndInstallUpdate(updateInfo.value.manifest)
  } catch (err) {
    updateError.value = String(err)
    updateLoading.value = false
  }
}

function resetAccountForm() {
  Object.assign(accountForm, {
    id: 0,
    provider: 'qiniu',
    name: '',
    accessKey: '',
    secretKey: '',
  })
  editingAccount.value = null
  accountFormNotice.value = ''
  accountFormError.value = ''
}

function resetMappingForm() {
  Object.assign(mappingForm, {
    id: 0,
    name: '',
    localPath: '',
    remotePrefix: '',
  })
  editingMapping.value = null
  mappingAccountID.value = accounts.value[0]?.id || 0
  mappingBucketName.value = ''
  availableBuckets.value = []
  bucketFolders.value = []
}

async function openAccountManager() {
  clearMessage()
  await refreshAccounts()
  resetAccountForm()
  showAccountModal.value = true
}

function editAccount(account) {
  resetAccountForm()
  editingAccount.value = account
  Object.assign(accountForm, {
    id: account.id,
    provider: account.provider,
    name: account.name,
    accessKey: account.accessKey,
    secretKey: '',
  })
  showAccountFormModal.value = true
}

function openNewAccountForm() {
  resetAccountForm()
  showAccountFormModal.value = true
}

async function removeAccount(account) {
  if (!window.confirm(`确定删除“${account.name}”吗？本地映射配置会一并删除，远程文件不会删除。`)) {
    return
  }
  loading.value = true
  clearMessage()
  try {
    await DeleteAccount(account.id)
    await refreshAccounts()
    await refreshMappings()
    notice.value = '账号已删除'
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}

async function saveAccount() {
  loading.value = true
  accountFormNotice.value = ''
  accountFormError.value = ''
  try {
    await SaveAccount({ ...accountForm })
    await refreshAccounts()
    await refreshMappings()
    resetAccountForm()
    showAccountFormModal.value = false
  } catch (err) {
    accountFormError.value = String(err)
  } finally {
    loading.value = false
  }
}

async function testAccount() {
  loading.value = true
  accountFormNotice.value = ''
  accountFormError.value = ''
  try {
    const buckets = await TestConnection({ ...accountForm })
    accountFormNotice.value = `连接成功，读取到 ${buckets.length} 个 Bucket`
  } catch (err) {
    accountFormError.value = String(err)
  } finally {
    loading.value = false
  }
}

async function loadMappingBuckets() {
  mappingBucketName.value = ''
  availableBuckets.value = []
  if (!mappingAccountID.value) {
    return
  }
  loading.value = true
  clearMessage()
  try {
    availableBuckets.value = await TestConnection({
      id: mappingAccountID.value,
      provider: selectedAccount.value?.provider || 'qiniu',
      name: selectedAccount.value?.name || '',
      accessKey: selectedAccount.value?.accessKey || '',
      secretKey: '',
    })
    await SaveBucketSelections(mappingAccountID.value, availableBuckets.value)
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}

async function addMapping() {
  clearMessage()
  await refreshAccounts()
  if (!accounts.value.length) {
    await openAccountManager()
    return
  }
  resetMappingForm()
  if (mappingAccountID.value) {
    await loadMappingBuckets()
  }
  showMappingModal.value = true
}

async function editMapping(mapping) {
  clearMessage()
  editingMapping.value = mapping
  Object.assign(mappingForm, {
    id: mapping.id,
    name: mapping.name,
    localPath: mapping.localPath,
    remotePrefix: mapping.remotePrefix,
  })
  mappingAccountID.value = mapping.accountId
  mappingBucketName.value = mapping.bucketName
  await loadMappingBuckets()
  mappingBucketName.value = mapping.bucketName
  showMappingModal.value = true
}

async function chooseLocalPath() {
  const path = await ChooseDirectory()
  if (path) {
    mappingForm.localPath = path
  }
}

async function chooseRemotePath() {
  if (!mappingAccountID.value || !mappingBucketName.value) {
    error.value = '请先选择账号和 Bucket'
    return
  }
  loading.value = true
  clearMessage()
  try {
    bucketFolders.value = await ListBucketFolders(mappingAccountID.value, mappingBucketName.value)
    showRemoteFolderModal.value = true
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}

function selectRemotePath(prefix) {
  mappingForm.remotePrefix = prefix
  showRemoteFolderModal.value = false
}

async function saveMappingForm() {
  if (!mappingAccountID.value || !mappingBucketName.value || !mappingForm.name || !mappingForm.localPath) {
    error.value = '请完整填写账号、Bucket、映射名称和本地目录'
    return
  }
  loading.value = true
  clearMessage()
  try {
    await SaveMapping(mappingAccountID.value, mappingBucketName.value, { ...mappingForm })
    await refreshMappings()
    showMappingModal.value = false
    notice.value = '映射已保存'
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}

function removeMapping(mapping) {
  deletingMapping.value = mapping
  showDeleteMappingModal.value = true
}

async function confirmDeleteMapping() {
  if (!deletingMapping.value) {
    return
  }
  loading.value = true
  clearMessage()
  try {
    await DeleteMapping(deletingMapping.value.id)
    await refreshMappings()
    showDeleteMappingModal.value = false
    notice.value = '映射已删除，远程文件未受影响'
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
    deletingMapping.value = null
  }
}

function openDeleteBucket(accountID, bucketName) {
  deletingBucket.value = { accountId: accountID, bucket: bucketName }
  deleteBucketConfirm.value = ''
  showDeleteBucketModal.value = true
}

async function confirmDeleteBucket() {
  if (!deletingBucket.value || deleteBucketConfirm.value !== deletingBucket.value.bucket) {
    return
  }
  loading.value = true
  clearMessage()
  try {
    const result = await DeleteBucket({
      accountId: deletingBucket.value.accountId,
      bucket: deletingBucket.value.bucket,
      confirm: deleteBucketConfirm.value,
    })
    await refreshAccounts()
    await refreshMappings()
    showDeleteBucketModal.value = false
    showMappingModal.value = false
    notice.value = `Bucket 已删除，共清理 ${result.deleted + result.alreadyAbsent} 个文件`
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
    deletingBucket.value = null
    deleteBucketConfirm.value = ''
  }
}

function syncMapping(mapping) {
  syncingMapping.value = mapping
  syncMode.value = 'overwrite'
  enableHTMLCache.value = true
  htmlCacheMode.value = 'no-cache'
  customCacheControl.value = ''
  cdnDomain.value = mapping.cdnDomain || ''
  refreshCDN.value = Boolean(cdnDomain.value)
  showSyncModal.value = true
}

function handleSyncProgress(progress) {
  if (!progress || progress.mappingId !== syncingMapping.value?.id) {
    return
  }
  Object.assign(syncProgress, progress)
  if (progress.status === 'uploaded') {
    const existing = syncDetails.value.find((item) => item.path === progress.current)
    if (existing) {
      existing.status = '已完成'
    } else {
      syncDetails.value.push({ path: progress.current, status: '已完成' })
    }
  } else if (progress.status === 'uploading') {
    const existing = syncDetails.value.find((item) => item.path === progress.current)
    if (!existing) {
      syncDetails.value.push({ path: progress.current, status: '上传中' })
    }
  }
}

function closeProgressModal() {
  showProgressModal.value = false
  syncingMapping.value = null
}

async function confirmSync() {
  if (!syncingMapping.value) {
    return
  }
  if (enableHTMLCache.value && !cacheControlValue.value) {
    error.value = '请输入 Cache-Control 值'
    return
  }
  const mapping = syncingMapping.value
  showSyncModal.value = false
  Object.assign(syncProgress, {
    mappingID: mapping.id,
    uploaded: 0,
    total: 0,
    current: '',
    status: 'running',
  })
  syncDetails.value = []
  showProgressDetails.value = false
  syncResultStatus.value = 'running'
  syncResultError.value = ''
  refreshedURLs.value = 0
  showProgressModal.value = true
  loading.value = true
  clearMessage()
  try {
    const result = await SyncMapping(mapping.id, {
      clearRemote: syncMode.value === 'clear',
      overwrite: true,
      htmlCacheControl: cacheControlValue.value,
      refreshCDN: refreshCDN.value && Boolean(cdnDomain.value.trim()),
      cdnDomain: cdnDomain.value.trim(),
    })
    syncResultStatus.value = 'success'
    syncProgress.status = 'success'
    refreshedURLs.value = result.refreshedUrls || 0
    mapping.cdnDomain = cdnDomain.value.trim()
  } catch (err) {
    syncResultStatus.value = 'error'
    syncProgress.status = 'error'
    syncResultError.value = String(err)
  } finally {
    loading.value = false
  }
}

async function openLocalDirectory(path) {
  clearMessage()
  try {
    await OpenLocalDirectory(path)
  } catch (err) {
    error.value = String(err)
  }
}

onMounted(async () => {
  EventsOn('sync-progress', handleSyncProgress)
  try {
    await refreshAccounts()
    await refreshMappings()
    await checkForUpdate(true)
  } catch (err) {
    error.value = String(err)
  }
})

onUnmounted(() => {
  EventsOff('sync-progress')
})
</script>

<template>
  <main class="page-shell">
    <nav class="menu-bar">
      <button class="menu-item" type="button" @click="addMapping">添加映射</button>
      <button class="menu-item" type="button" @click="openAccountManager">账号管理</button>
      <button class="menu-item" type="button" :disabled="updateLoading" @click="checkForUpdate()">检查更新</button>
      <button class="menu-item" type="button" @click="showAboutModal = true">关于</button>
    </nav>

    <p v-if="notice" class="notice success">{{ notice }}</p>
    <p v-if="error" class="notice error">{{ error }}</p>

    <section class="workspace panel">
      <div v-if="!mappings.length" class="empty-state">
        <div class="empty-state-mark">＋</div>
        <strong>还没有同步映射</strong>
        <span>请点击左上角“添加映射”开始配置。</span>
      </div>
      <div v-for="mapping in mappings" :key="mapping.id" class="mapping-row">
        <div class="mapping-name">
          <span class="mapping-label">映射名称</span>
          <div class="mapping-name-line">
            <strong :title="mapping.name">{{ mapping.name }}</strong>
            <small :title="mapping.accountName">{{ mapping.accountName }}</small>
          </div>
        </div>
        <div class="mapping-local">
          <span class="mapping-label">本地路径</span>
          <button class="path-link" type="button" :title="mapping.localPath" @click="openLocalDirectory(mapping.localPath)">
            {{ mapping.localPath }}
          </button>
        </div>
        <span class="mapping-arrow">→</span>
        <div class="mapping-remote" :title="`${mapping.bucketName}/${mapping.remotePrefix || ''}`">
          <span class="mapping-label">远程路径</span>
          <div class="remote-path-line">
            <span class="bucket-tag" :class="{ private: mapping.private }">{{ mapping.private ? '私有' : '公开' }}</span>
            <strong>{{ mapping.bucketName }}/{{ mapping.remotePrefix }}</strong>
          </div>
        </div>
        <div class="row-actions">
          <button class="link-button" type="button" :disabled="loading" @click="syncMapping(mapping)">同步</button>
          <button class="link-button" type="button" :disabled="loading" @click="editMapping(mapping)">编辑</button>
          <button class="danger-button" type="button" :disabled="loading" @click="removeMapping(mapping)">删除</button>
        </div>
      </div>
    </section>

    <div v-if="showAccountModal" class="modal-backdrop" @click.self="showAccountModal = false">
      <section class="modal panel account-modal">
        <div class="modal-heading">
          <div>
            <h2>账号管理</h2>
            <p>SecretKey 只保存在本机加密凭据中</p>
          </div>
          <button class="link-button" type="button" @click="showAccountModal = false">关闭</button>
        </div>
        <div class="account-manager-list">
          <div v-if="!accounts.length" class="manager-empty">
            <div class="manager-empty-mark">＋</div>
            <strong>还没有账号配置</strong>
            <p>先新建一个云平台账号，再添加 Bucket 和同步映射。</p>
          </div>
          <div v-for="account in accounts" :key="account.id" class="manager-row">
            <div>
              <strong>{{ account.name }}</strong>
              <p>{{ account.bucketCount }} 个 Bucket</p>
            </div>
            <div class="row-actions">
              <button class="link-button" type="button" @click="editAccount(account)">编辑</button>
              <button class="danger-button" type="button" @click="removeAccount(account)">删除</button>
            </div>
          </div>
        </div>
        <div class="form-actions account-actions">
          <button class="primary-button" type="button" :disabled="loading" @click="openNewAccountForm">新建配置</button>
        </div>
      </section>
    </div>

    <div v-if="showAccountFormModal" class="modal-backdrop" @click.self="showAccountFormModal = false">
      <section class="modal panel account-form-modal">
        <div class="modal-heading">
          <div>
            <h2>{{ editingAccount ? '编辑配置' : '新建配置' }}</h2>
            <p>填写七牛云账号信息。</p>
          </div>
          <button class="link-button" type="button" @click="showAccountFormModal = false">关闭</button>
        </div>
        <div class="form-grid single-column">
          <label>
            云厂商
            <select v-model="accountForm.provider">
              <option value="qiniu">七牛云</option>
            </select>
          </label>
          <label>
            配置名称
            <input v-model="accountForm.name" placeholder="例如：公司生产环境" />
          </label>
          <label>
            AccessKey
            <input v-model="accountForm.accessKey" autocomplete="off" />
          </label>
          <label>
            SecretKey
            <input v-model="accountForm.secretKey" type="password" autocomplete="new-password" placeholder="编辑时留空表示不修改" />
          </label>
        </div>
        <p v-if="accountFormNotice" class="notice success form-notice">{{ accountFormNotice }}</p>
        <p v-if="accountFormError" class="notice error form-notice">{{ accountFormError }}</p>
        <div class="form-actions">
          <button class="secondary-button" type="button" :disabled="loading" @click="testAccount">测试</button>
          <button class="primary-button" type="button" :disabled="loading" @click="saveAccount">保存</button>
        </div>
      </section>
    </div>

    <div v-if="showMappingModal" class="modal-backdrop">
      <section class="modal panel mapping-modal">
        <div class="modal-heading">
          <div>
            <h2>{{ editingMapping ? '编辑映射' : '添加映射' }}</h2>
            <p>先选择平台账号，再选择 Bucket。</p>
          </div>
          <button class="link-button" type="button" @click="showMappingModal = false">关闭</button>
        </div>
        <div class="form-grid single-column">
          <label>
            平台账号
            <select v-model.number="mappingAccountID" @change="loadMappingBuckets">
              <option :value="0">请选择账号</option>
              <option v-for="account in accounts" :key="account.id" :value="account.id">
                {{ account.name }}
              </option>
            </select>
          </label>
          <label>
            Bucket
            <select v-model="mappingBucketName">
              <option value="">请选择 Bucket</option>
              <option v-for="bucket in availableBuckets" :key="bucket.name" :value="bucket.name">
                {{ bucket.name }}（{{ bucket.private ? '私有' : '公开' }}）
              </option>
            </select>
          </label>
          <div v-if="mappingBucketName" class="inline-actions">
            <span class="field-hint">当前 Bucket：{{ mappingBucketName }}</span>
            <button class="danger-button" type="button" @click="openDeleteBucket(mappingAccountID, mappingBucketName)">删除 Bucket</button>
          </div>
          <label>映射名称<input v-model="mappingForm.name" placeholder="例如：设计器前端" /></label>
          <label>
            本地目录
            <div class="path-input">
              <input v-model="mappingForm.localPath" readonly placeholder="点击右侧按钮选择本地目录" />
              <button class="secondary-button" type="button" @click="chooseLocalPath">选择</button>
            </div>
          </label>
          <label>
            远程路径
            <div class="path-input">
              <input :value="mappingForm.remotePrefix || 'Bucket 根目录'" readonly placeholder="点击右侧按钮选择远程目录" />
              <button class="secondary-button" type="button" :disabled="!mappingBucketName" @click="chooseRemotePath">选择</button>
            </div>
          </label>
        </div>
        <div class="form-actions">
          <button class="secondary-button" type="button" @click="showMappingModal = false">取消</button>
          <button class="primary-button" type="button" :disabled="loading" @click="saveMappingForm">保存映射</button>
        </div>
      </section>
    </div>

    <div v-if="showRemoteFolderModal" class="modal-backdrop nested-modal" @click.self="showRemoteFolderModal = false">
      <section class="modal panel remote-folder-modal">
        <div class="modal-heading">
          <div>
            <h2>选择远程目录</h2>
            <p>{{ mappingBucketName }}</p>
          </div>
          <button class="link-button" type="button" @click="showRemoteFolderModal = false">关闭</button>
        </div>
        <div class="remote-folder-list">
          <button class="remote-folder-option" type="button" @click="selectRemotePath('')">
            <span>Bucket 根目录</span>
            <small>/</small>
          </button>
          <button v-for="folder in bucketFolders" :key="folder.prefix" class="remote-folder-option" type="button" @click="selectRemotePath(folder.prefix)">
            <span>{{ folder.prefix }}</span>
            <small>目录</small>
          </button>
          <p v-if="!bucketFolders.length" class="empty-state">当前 Bucket 暂无已发现的远程目录，可以选择根目录。</p>
        </div>
        <div class="form-actions">
          <button class="secondary-button" type="button" @click="showRemoteFolderModal = false">取消</button>
        </div>
      </section>
    </div>

    <div v-if="showSyncModal" class="modal-backdrop" @click.self="showSyncModal = false">
      <section class="modal panel sync-modal">
        <div class="modal-heading">
          <div>
            <h2>选择同步方式</h2>
            <p>{{ syncingMapping?.name }} · {{ syncingMapping?.bucketName }}</p>
          </div>
          <button class="link-button" type="button" @click="showSyncModal = false">关闭</button>
        </div>
        <div class="sync-options">
          <label class="sync-option" :class="{ selected: syncMode === 'overwrite' }">
            <input v-model="syncMode" type="radio" value="overwrite" />
            <span>
              <strong>覆盖上传</strong>
              <small>覆盖同名文件，保留远程其他文件。</small>
            </span>
          </label>
          <label class="sync-option" :class="{ selected: syncMode === 'clear' }">
            <input v-model="syncMode" type="radio" value="clear" />
            <span>
              <strong>清空后上传</strong>
              <small>先清空当前远程目录，再上传本地文件。</small>
            </span>
          </label>
        </div>
        <label class="check-row"><input v-model="enableHTMLCache" type="checkbox" />为 HTML、HTM 文件设置 Cache-Control</label>
        <div v-if="enableHTMLCache" class="cache-control-box">
          <label>
            Cache-Control 值
            <select v-model="htmlCacheMode">
              <option value="no-cache">no-cache（推荐）</option>
              <option value="no-store">no-store</option>
              <option value="max-age=0, must-revalidate">max-age=0, must-revalidate</option>
              <option value="max-age=300">max-age=300（5 分钟）</option>
              <option value="max-age=3600">max-age=3600（1 小时）</option>
              <option value="custom">自定义</option>
            </select>
          </label>
          <label v-if="htmlCacheMode === 'custom'">
            自定义值
            <input v-model="customCacheControl" placeholder="例如：public, max-age=600" />
          </label>
        </div>
        <label class="check-row">
          <input v-model="refreshCDN" type="checkbox" />
          上传 HTML、HTM 后刷新 CDN 缓存
        </label>
        <div v-if="refreshCDN" class="cdn-refresh-box">
          <label>
            CDN 域名
            <input v-model="cdnDomain" placeholder="例如：https://cdn.example.com" />
          </label>
          <p>域名会按 Bucket 保存，后续同步可直接使用。</p>
        </div>
        <div class="form-actions">
          <button class="secondary-button" type="button" @click="showSyncModal = false">取消</button>
          <button class="primary-button" type="button" :disabled="loading" @click="confirmSync">开始同步</button>
        </div>
      </section>
    </div>

    <div v-if="showProgressModal" class="modal-backdrop progress-backdrop">
      <section class="modal panel progress-modal">
        <div class="modal-heading">
          <div>
            <h2>{{ syncResultStatus === 'running' ? '正在同步' : syncResultStatus === 'success' ? '同步完成' : '同步失败' }}</h2>
            <p>{{ syncingMapping?.name }} · {{ syncingMapping?.bucketName }}</p>
          </div>
          <span v-if="syncResultStatus === 'running'" class="progress-percent">{{ syncProgressPercent }}%</span>
        </div>
        <div class="progress-track">
          <div class="progress-value" :style="{ width: `${syncProgressPercent}%` }"></div>
        </div>
        <div class="progress-summary">
          <span>{{ syncProgress.uploaded }} / {{ syncProgress.total }} 个文件</span>
          <span>{{ syncProgress.current || '准备上传…' }}</span>
        </div>
        <p v-if="syncResultStatus === 'success'" class="progress-result success-text">
          已成功上传 {{ syncProgress.uploaded }} 个文件。<span v-if="refreshedURLs">已刷新 CDN {{ refreshedURLs }} 个地址。</span>
        </p>
        <p v-if="syncResultStatus === 'error'" class="progress-result error-text">{{ syncResultError }}</p>
        <div v-if="showProgressDetails" class="progress-details">
          <div v-for="detail in syncDetails" :key="detail.path" class="progress-detail-row">
            <span :title="detail.path">{{ detail.path }}</span>
            <small :class="{ done: detail.status === '已完成' }">{{ detail.status }}</small>
          </div>
          <p v-if="!syncDetails.length" class="empty-state">还没有上传明细。</p>
        </div>
        <p v-if="syncResultStatus === 'running'" class="progress-tip">同步过程中请不要关闭软件。</p>
        <div class="progress-actions">
          <button class="secondary-button" type="button" @click="showProgressDetails = !showProgressDetails">
            {{ showProgressDetails ? '收起详情' : '展开详情' }}
          </button>
          <button v-if="syncResultStatus !== 'running'" class="primary-button" type="button" @click="closeProgressModal">关闭</button>
        </div>
      </section>
    </div>

    <div v-if="showDeleteMappingModal" class="modal-backdrop" @click.self="showDeleteMappingModal = false">
      <section class="modal panel confirm-modal">
        <div class="modal-heading">
          <div>
            <h2>删除映射</h2>
            <p>确定删除“{{ deletingMapping?.name }}”吗？</p>
          </div>
          <button class="link-button" type="button" @click="showDeleteMappingModal = false">关闭</button>
        </div>
        <p class="confirm-hint">只会删除本地映射配置，远程文件不会删除。</p>
        <div class="form-actions">
          <button class="secondary-button" type="button" @click="showDeleteMappingModal = false">取消</button>
          <button class="danger-solid-button" type="button" :disabled="loading" @click="confirmDeleteMapping">确认删除</button>
        </div>
      </section>
    </div>

    <div v-if="showDeleteBucketModal" class="modal-backdrop" @click.self="showDeleteBucketModal = false">
      <section class="modal panel confirm-modal">
        <div class="modal-heading">
          <div>
            <h2>删除 Bucket</h2>
            <p>此操作会删除远程 Bucket 及其中的全部文件。</p>
          </div>
          <button class="link-button" type="button" @click="showDeleteBucketModal = false">关闭</button>
        </div>
        <label>
          请输入 Bucket 名称确认
          <input v-model="deleteBucketConfirm" :placeholder="deletingBucketName" />
        </label>
        <div class="form-actions">
          <button class="secondary-button" type="button" @click="showDeleteBucketModal = false">取消</button>
          <button class="danger-solid-button" type="button" :disabled="loading || deleteBucketConfirm !== deletingBucketName" @click="confirmDeleteBucket">确认删除</button>
        </div>
      </section>
    </div>

    <div v-if="showAboutModal" class="modal-backdrop" @click.self="showAboutModal = false">
      <section class="modal panel about-modal">
        <div class="modal-heading">
          <div>
            <h2>关于云存储同步工具</h2>
            <p>用于管理本地目录与对象存储映射。</p>
          </div>
          <button class="link-button" type="button" @click="showAboutModal = false">关闭</button>
        </div>
        <div class="about-info">
          <div><span>当前版本</span><strong>{{ appVersion }}</strong></div>
          <div><span>GitHub</span><button class="external-link" type="button" @click="openGitHub">{{ githubURL }}</button></div>
        </div>
      </section>
    </div>

    <div v-if="showUpdateModal" class="modal-backdrop" @click.self="showUpdateModal = false">
      <section class="modal panel update-modal">
        <div class="modal-heading">
          <div>
            <h2>检查更新</h2>
            <p>当前版本 {{ appVersion }}</p>
          </div>
          <button class="link-button" type="button" @click="showUpdateModal = false">关闭</button>
        </div>
        <p v-if="updateMessage" class="form-notice">{{ updateMessage }}</p>
        <p v-if="updateError" class="notice error form-notice">{{ updateError }}</p>
        <div v-if="updateInfo" class="update-info">
          <div><span>最新版本</span><strong>{{ updateInfo.manifest.version }}</strong></div>
          <div v-if="updateInfo.manifest.notes"><span>更新内容</span><p>{{ updateInfo.manifest.notes }}</p></div>
        </div>
        <div class="form-actions">
          <button v-if="updateInfo" class="primary-button" type="button" :disabled="updateLoading" @click="installUpdate">
            {{ updateLoading ? '正在下载…' : '立即更新' }}
          </button>
        </div>
      </section>
    </div>
  </main>
</template>
