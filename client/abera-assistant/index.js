// Copyright 2026 Abera/Corteza contributors
// Licensed under the Apache License, Version 2.0.

const ROOT_ID = 'abera-assistant-root'
const STYLE_ID = 'abera-assistant-styles'
const API_BASE = '/api/system/assistant'
const MAX_FILE_SIZE = 10 * 1024 * 1024
const MAX_FILES_PER_TURN = 5
const ALLOWED_EXTENSIONS = new Set(['pdf', 'txt', 'docx', 'csv', 'xlsx', 'png', 'jpg', 'jpeg'])
const MIME_TYPES = {
  pdf: 'application/pdf',
  txt: 'text/plain',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  csv: 'text/csv',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
}

const icon = (path, label = '') => `
  <svg viewBox="0 0 24 24" aria-hidden="${label ? 'false' : 'true'}"${label ? ` aria-label="${label}"` : ''}>
    <path d="${path}"></path>
  </svg>
`

const icons = {
  assistant: icon('M12 2a2 2 0 0 1 2 2v1.08a7 7 0 0 1 4.92 4.92H20a2 2 0 1 1 0 4h-1.08A7 7 0 0 1 14 18.92V20a2 2 0 1 1-4 0v-1.08A7 7 0 0 1 5.08 14H4a2 2 0 1 1 0-4h1.08A7 7 0 0 1 10 5.08V4a2 2 0 0 1 2-2Zm0 6a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm-1.5 2.5h3v3h-3v-3Z'),
  close: icon('M6.7 5.3 12 10.6l5.3-5.3 1.4 1.4-5.3 5.3 5.3 5.3-1.4 1.4-5.3-5.3-5.3 5.3-1.4-1.4 5.3-5.3-5.3-5.3 1.4-1.4Z'),
  menu: icon('M4 6h16v2H4V6Zm0 5h16v2H4v-2Zm0 5h16v2H4v-2Z'),
  plus: icon('M11 4h2v7h7v2h-7v7h-2v-7H4v-2h7V4Z'),
  send: icon('m3.4 20.4 17.8-8a1 1 0 0 0 0-1.8l-17.8-8A1 1 0 0 0 2 3.7L3.6 10l9.4 2-9.4 2L2 20.3a1 1 0 0 0 1.4 1.1Z'),
  attach: icon('M16.5 6.5v9a4.5 4.5 0 0 1-9 0v-10a3.5 3.5 0 0 1 7 0V15a2.5 2.5 0 0 1-5 0V7h2v8a.5.5 0 0 0 1 0V5.5a1.5 1.5 0 0 0-3 0v10a2.5 2.5 0 0 0 5 0v-9h2Z'),
  trash: icon('M8 3h8l1 2h4v2H3V5h4l1-2Zm-2 6h12l-1 12H7L6 9Zm3 2v7h2v-7H9Zm4 0v7h2v-7h-2Z'),
  file: icon('M6 2h8l5 5v15H6V2Zm8 2.5V8h3.5L14 4.5ZM8 11v2h8v-2H8Zm0 4v2h8v-2H8Z'),
}

function installStyles () {
  if (document.getElementById(STYLE_ID)) return
  const style = document.createElement('style')
  style.id = STYLE_ID
  style.textContent = `
    :root {
      --abera-ai-accent: #0b5fff;
      --abera-ai-accent-strong: #0648bd;
      --abera-ai-surface: #ffffff;
      --abera-ai-surface-muted: #f5f7fa;
      --abera-ai-border: #d9dee7;
      --abera-ai-text: #172033;
      --abera-ai-text-muted: #687386;
      --abera-ai-danger: #b42318;
      --abera-ai-success: #067647;
      --abera-ai-shadow: 0 18px 50px rgba(20, 35, 60, .22);
    }
    [data-color-mode="dark"] {
      --abera-ai-accent: #77a7ff;
      --abera-ai-accent-strong: #a8c7ff;
      --abera-ai-surface: #1d2532;
      --abera-ai-surface-muted: #263142;
      --abera-ai-border: #3b485c;
      --abera-ai-text: #f4f7fb;
      --abera-ai-text-muted: #b8c2d2;
      --abera-ai-danger: #ff8d86;
      --abera-ai-success: #75d6a8;
      --abera-ai-shadow: 0 18px 55px rgba(0, 0, 0, .48);
    }
    .abera-ai, .abera-ai * { box-sizing: border-box; }
    .abera-ai { position: fixed; right: 24px; bottom: 24px; z-index: 1100; color: var(--abera-ai-text); font-family: inherit; }
    .abera-ai button, .abera-ai textarea { font: inherit; }
    .abera-ai-launcher {
      display: flex; align-items: center; gap: 10px; min-height: 48px; padding: 0 18px;
      border: 1px solid rgba(255,255,255,.25); border-radius: 24px; color: #fff;
      background: #0b3a82; box-shadow: 0 8px 24px rgba(12, 41, 84, .3); cursor: pointer;
      font-weight: 650; letter-spacing: .01em;
    }
    .abera-ai-launcher:hover { background: #082f69; }
    .abera-ai-launcher:focus-visible, .abera-ai button:focus-visible, .abera-ai textarea:focus-visible {
      outline: 3px solid rgba(11,95,255,.32); outline-offset: 2px;
    }
    .abera-ai-launcher svg { width: 22px; height: 22px; fill: currentColor; }
    .abera-ai-panel {
      display: grid; grid-template-columns: 220px minmax(0, 1fr); width: min(760px, calc(100vw - 48px));
      height: min(720px, calc(100vh - 48px)); overflow: hidden; border: 1px solid var(--abera-ai-border);
      border-radius: 16px; background: var(--abera-ai-surface); box-shadow: var(--abera-ai-shadow);
    }
    .abera-ai-sidebar { display: flex; flex-direction: column; min-width: 0; padding: 14px; border-right: 1px solid var(--abera-ai-border); background: var(--abera-ai-surface-muted); }
    .abera-ai-brand { display: flex; align-items: center; gap: 10px; min-height: 42px; margin-bottom: 12px; }
    .abera-ai-brand-mark { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 9px; color: #fff; background: #0b3a82; }
    .abera-ai-brand-mark svg { width: 20px; height: 20px; fill: currentColor; }
    .abera-ai-brand strong { display: block; font-size: 14px; line-height: 1.2; }
    .abera-ai-brand small { color: var(--abera-ai-text-muted); font-size: 11px; }
    .abera-ai-new { display: flex; align-items: center; justify-content: center; gap: 7px; min-height: 38px; border: 1px solid var(--abera-ai-border); border-radius: 8px; color: var(--abera-ai-text); background: var(--abera-ai-surface); cursor: pointer; font-weight: 600; }
    .abera-ai-new:hover { border-color: var(--abera-ai-accent); color: var(--abera-ai-accent-strong); }
    .abera-ai-new svg, .abera-ai-icon-button svg, .abera-ai-send svg, .abera-ai-attach svg { width: 18px; height: 18px; fill: currentColor; }
    .abera-ai-conversations { flex: 1; min-height: 0; margin: 14px -5px 0; overflow-y: auto; }
    .abera-ai-conversation { position: relative; display: grid; grid-template-columns: minmax(0,1fr) 28px; gap: 4px; margin-bottom: 4px; border-radius: 8px; }
    .abera-ai-conversation.is-active { background: var(--abera-ai-surface); box-shadow: inset 3px 0 var(--abera-ai-accent); }
    .abera-ai-conversation-main { min-width: 0; padding: 9px 7px 9px 11px; border: 0; color: var(--abera-ai-text); background: transparent; cursor: pointer; text-align: left; }
    .abera-ai-conversation-title { display: block; overflow: hidden; font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
    .abera-ai-conversation-date { display: block; margin-top: 3px; color: var(--abera-ai-text-muted); font-size: 10px; }
    .abera-ai-conversation-delete { align-self: center; display: grid; place-items: center; width: 28px; height: 28px; border: 0; border-radius: 6px; color: var(--abera-ai-text-muted); background: transparent; cursor: pointer; opacity: 0; }
    .abera-ai-conversation:hover .abera-ai-conversation-delete, .abera-ai-conversation-delete:focus-visible { opacity: 1; }
    .abera-ai-conversation-delete:hover { color: var(--abera-ai-danger); background: rgba(180,35,24,.09); }
    .abera-ai-conversation-delete svg { width: 14px; height: 14px; fill: currentColor; }
    .abera-ai-main { display: grid; grid-template-rows: auto minmax(0,1fr) auto; min-width: 0; min-height: 0; }
    .abera-ai-header { display: flex; align-items: center; gap: 10px; min-height: 58px; padding: 8px 12px 8px 18px; border-bottom: 1px solid var(--abera-ai-border); }
    .abera-ai-header-copy { min-width: 0; flex: 1; }
    .abera-ai-header h2 { overflow: hidden; margin: 0; font-size: 15px; font-weight: 700; line-height: 1.25; text-overflow: ellipsis; white-space: nowrap; }
    .abera-ai-status { display: flex; align-items: center; gap: 6px; margin-top: 3px; color: var(--abera-ai-text-muted); font-size: 11px; }
    .abera-ai-status-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--abera-ai-success); }
    .abera-ai-icon-button { display: grid; place-items: center; width: 38px; height: 38px; padding: 0; border: 0; border-radius: 8px; color: var(--abera-ai-text-muted); background: transparent; cursor: pointer; }
    .abera-ai-icon-button:hover { color: var(--abera-ai-text); background: var(--abera-ai-surface-muted); }
    .abera-ai-menu { display: none; }
    .abera-ai-messages { min-height: 0; padding: 22px 24px; overflow-y: auto; scroll-behavior: smooth; }
    .abera-ai-welcome { max-width: 460px; margin: 10% auto 0; text-align: center; }
    .abera-ai-welcome-mark { display: grid; place-items: center; width: 48px; height: 48px; margin: 0 auto 14px; border: 1px solid var(--abera-ai-border); border-radius: 12px; color: var(--abera-ai-accent-strong); background: var(--abera-ai-surface-muted); }
    .abera-ai-welcome-mark svg { width: 26px; height: 26px; fill: currentColor; }
    .abera-ai-welcome h3 { margin: 0 0 7px; font-size: 18px; }
    .abera-ai-welcome p { margin: 0; color: var(--abera-ai-text-muted); font-size: 13px; line-height: 1.55; }
    .abera-ai-suggestions { display: grid; gap: 8px; margin-top: 20px; }
    .abera-ai-suggestion { min-height: 42px; padding: 9px 12px; border: 1px solid var(--abera-ai-border); border-radius: 8px; color: var(--abera-ai-text); background: var(--abera-ai-surface); cursor: pointer; font-size: 12px; text-align: left; }
    .abera-ai-suggestion:hover { border-color: var(--abera-ai-accent); background: var(--abera-ai-surface-muted); }
    .abera-ai-message { display: flex; margin-bottom: 18px; }
    .abera-ai-message.is-user { justify-content: flex-end; }
    .abera-ai-bubble { max-width: min(82%, 560px); padding: 11px 13px; border: 1px solid var(--abera-ai-border); border-radius: 12px 12px 12px 3px; background: var(--abera-ai-surface-muted); font-size: 13px; line-height: 1.55; overflow-wrap: anywhere; white-space: pre-wrap; }
    .abera-ai-message.is-user .abera-ai-bubble { border-color: #0b3a82; border-radius: 12px 12px 3px 12px; color: #fff; background: #0b3a82; }
    .abera-ai-message-files { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
    .abera-ai-message-file { display: inline-flex; align-items: center; gap: 4px; font-size: 10px; opacity: .88; }
    .abera-ai-message-file svg { width: 12px; height: 12px; fill: currentColor; }
    .abera-ai-typing { display: inline-flex; gap: 4px; padding: 4px 0; }
    .abera-ai-typing span { width: 6px; height: 6px; border-radius: 50%; background: var(--abera-ai-text-muted); animation: abera-ai-pulse 1.2s infinite ease-in-out; }
    .abera-ai-typing span:nth-child(2) { animation-delay: .15s; } .abera-ai-typing span:nth-child(3) { animation-delay: .3s; }
    @keyframes abera-ai-pulse { 0%, 70%, 100% { opacity: .3; transform: translateY(0); } 35% { opacity: 1; transform: translateY(-3px); } }
    .abera-ai-approval { margin: 6px 0 18px; padding: 14px; border: 1px solid #e2a93b; border-left-width: 4px; border-radius: 8px; background: rgba(226,169,59,.09); }
    .abera-ai-approval strong { display: block; margin-bottom: 5px; font-size: 13px; }
    .abera-ai-approval p { margin: 0; color: var(--abera-ai-text-muted); font-size: 12px; line-height: 1.5; }
    .abera-ai-approval-actions { display: flex; gap: 8px; margin-top: 12px; }
    .abera-ai-action { min-height: 36px; padding: 0 13px; border: 1px solid var(--abera-ai-border); border-radius: 7px; color: var(--abera-ai-text); background: var(--abera-ai-surface); cursor: pointer; font-size: 12px; font-weight: 650; }
    .abera-ai-action.is-primary { border-color: var(--abera-ai-accent); color: #fff; background: var(--abera-ai-accent); }
    .abera-ai-action:disabled { cursor: not-allowed; opacity: .55; }
    .abera-ai-tool-status { margin: -8px 0 14px; color: var(--abera-ai-text-muted); font-size: 11px; }
    .abera-ai-error { margin: 0 16px 10px; padding: 9px 11px; border: 1px solid rgba(180,35,24,.35); border-radius: 7px; color: var(--abera-ai-danger); background: rgba(180,35,24,.07); font-size: 12px; }
    .abera-ai-files { display: flex; gap: 7px; padding: 0 16px 8px; overflow-x: auto; }
    .abera-ai-file-chip { display: inline-flex; align-items: center; gap: 6px; max-width: 220px; min-height: 30px; padding: 0 8px; border: 1px solid var(--abera-ai-border); border-radius: 7px; background: var(--abera-ai-surface-muted); font-size: 11px; white-space: nowrap; }
    .abera-ai-file-chip span { overflow: hidden; text-overflow: ellipsis; }
    .abera-ai-file-chip svg { width: 14px; height: 14px; flex: 0 0 auto; fill: currentColor; }
    .abera-ai-file-remove { border: 0; color: var(--abera-ai-text-muted); background: transparent; cursor: pointer; font-size: 16px; line-height: 1; }
    .abera-ai-composer { padding: 10px 16px 15px; border-top: 1px solid var(--abera-ai-border); background: var(--abera-ai-surface); }
    .abera-ai-composer-box { display: grid; grid-template-columns: auto minmax(0,1fr) auto; align-items: end; gap: 6px; padding: 7px; border: 1px solid var(--abera-ai-border); border-radius: 11px; background: var(--abera-ai-surface); }
    .abera-ai-composer-box:focus-within { border-color: var(--abera-ai-accent); box-shadow: 0 0 0 3px rgba(11,95,255,.1); }
    .abera-ai-composer textarea { width: 100%; max-height: 130px; min-height: 32px; resize: none; border: 0; outline: 0; color: var(--abera-ai-text); background: transparent; font-size: 13px; line-height: 1.45; }
    .abera-ai-composer textarea::placeholder { color: var(--abera-ai-text-muted); }
    .abera-ai-attach, .abera-ai-send { display: grid; place-items: center; width: 34px; height: 34px; padding: 0; border: 0; border-radius: 8px; cursor: pointer; }
    .abera-ai-attach { color: var(--abera-ai-text-muted); background: transparent; }
    .abera-ai-attach:hover { color: var(--abera-ai-text); background: var(--abera-ai-surface-muted); }
    .abera-ai-send { color: #fff; background: #0b3a82; }
    .abera-ai-send:hover { background: #082f69; }
    .abera-ai-send:disabled, .abera-ai-attach:disabled { cursor: not-allowed; opacity: .45; }
    .abera-ai-composer-note { margin: 7px 2px 0; color: var(--abera-ai-text-muted); font-size: 10px; text-align: center; }
    .abera-ai-scrim { display: none; }
    @media (max-width: 760px) {
      .abera-ai { right: 14px; bottom: 14px; }
      .abera-ai-panel { position: fixed; inset: 0; grid-template-columns: 1fr; width: 100vw; height: 100dvh; border: 0; border-radius: 0; }
      .abera-ai-sidebar { position: absolute; inset: 0 auto 0 0; z-index: 3; width: min(82vw, 300px); border-right: 1px solid var(--abera-ai-border); transform: translateX(-105%); transition: transform .18s ease; }
      .abera-ai-sidebar.is-open { transform: translateX(0); }
      .abera-ai-scrim { position: absolute; inset: 0; z-index: 2; display: block; border: 0; background: rgba(12,20,33,.46); }
      .abera-ai-menu { display: grid; }
      .abera-ai-messages { padding: 18px 14px; }
      .abera-ai-bubble { max-width: 90%; }
      .abera-ai-launcher span { display: none; }
      .abera-ai-launcher { width: 52px; height: 52px; justify-content: center; padding: 0; border-radius: 50%; }
    }
    @media (prefers-reduced-motion: reduce) {
      .abera-ai-sidebar, .abera-ai-messages { transition: none; scroll-behavior: auto; }
      .abera-ai-typing span { animation: none; opacity: .65; }
    }
  `
  document.head.appendChild(style)
}

class AssistantAPI {
  constructor (accessToken) {
    this.accessToken = accessToken
  }

  async request (path, options = {}) {
    const headers = new Headers(options.headers || {})
    const token = this.accessToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
    if (options.json !== undefined) {
      headers.set('Content-Type', 'application/json')
      options.body = JSON.stringify(options.json)
    }
    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
      credentials: 'same-origin',
      cache: 'no-store',
    })
    if (!response.ok) throw await responseError(response)
    if (response.status === 204) return null
    return response.json()
  }

  async stream (path, body, onEvent) {
    const headers = new Headers({ Accept: 'text/event-stream', 'Content-Type': 'application/json' })
    const token = this.accessToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
    const response = await fetch(`${API_BASE}${path}`, {
      method: 'POST', headers, body: JSON.stringify(body), credentials: 'same-origin', cache: 'no-store',
    })
    if (!response.ok) throw await responseError(response)
    await readEventStream(response, onEvent)
  }
}

async function responseError (response) {
  let message = `Solicitud fallida (${response.status})`
  try {
    const payload = await response.json()
    message = payload.error || (typeof payload.detail === 'string' ? payload.detail : message)
  } catch (_) {}
  const error = new Error(message)
  error.status = response.status
  return error
}

export async function readEventStream (response, onEvent) {
  if (!response.body || !response.body.getReader) throw new Error('El navegador no permite recibir respuestas en tiempo real')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  const consume = block => {
    let event = 'message'
    const data = []
    block.split(/\r?\n/).forEach(line => {
      if (line.startsWith('event:')) event = line.slice(6).trim()
      if (line.startsWith('data:')) data.push(line.slice(5).trimStart())
    })
    if (!data.length) return
    let payload
    try { payload = JSON.parse(data.join('\n')) } catch (_) { payload = { message: data.join('\n') } }
    onEvent(event, payload)
  }
  while (true) {
    const { value, done } = await reader.read()
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done })
    const blocks = buffer.split(/\r?\n\r?\n/)
    buffer = blocks.pop() || ''
    blocks.forEach(consume)
    if (done) break
  }
  if (buffer.trim()) consume(buffer)
}

function makeID () {
  if (window.crypto && window.crypto.randomUUID) return window.crypto.randomUUID()
  const bytes = new Uint8Array(16)
  if (window.crypto && window.crypto.getRandomValues) {
    window.crypto.getRandomValues(bytes)
  } else {
    for (let index = 0; index < bytes.length; index++) bytes[index] = Math.floor(Math.random() * 256)
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const value = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
  return `${value.slice(0, 8)}-${value.slice(8, 12)}-${value.slice(12, 16)}-${value.slice(16, 20)}-${value.slice(20)}`
}

function fileExtension (name) {
  return String(name).split('.').pop().toLowerCase()
}

function makeComponent (api) {
  return {
    name: 'AberaAssistant',
    data: () => ({
      open: false,
      sidebarOpen: false,
      loading: true,
      streaming: false,
      deleting: '',
      context: null,
      conversations: [],
      conversation: null,
      messages: [],
      draft: '',
      files: [],
      approval: null,
      toolStatus: '',
      error: '',
      suggestions: [
        'Resume los leads que necesitan seguimiento hoy',
        'Muéstrame el estado del embudo comercial',
        'Ayúdame a crear un nuevo registro correctamente',
      ],
      icons,
    }),
    computed: {
      canSend () { return !this.streaming && (this.draft.trim().length > 0 || this.files.length > 0) },
      title () { return this.conversation ? this.conversation.title : 'Nueva conversación' },
    },
    async created () {
      document.addEventListener('keydown', this.onGlobalKeydown)
      try {
        this.context = await api.request('/auth/context')
        if (this.context.enabled && this.context.allowed) await this.loadConversations()
      } catch (error) {
        this.error = error.message
      } finally {
        this.loading = false
      }
    },
    beforeDestroy () {
      document.removeEventListener('keydown', this.onGlobalKeydown)
    },
    methods: {
      async loadConversations () {
        const result = await api.request('/conversations?limit=50')
        this.conversations = result.items || []
      },
      async toggle () {
        this.open = !this.open
        this.sidebarOpen = false
        if (this.open) {
          await this.$nextTick()
          this.$refs.composer && this.$refs.composer.focus()
        }
      },
      close () { this.open = false; this.sidebarOpen = false },
      onGlobalKeydown (event) {
        if (event.key === 'Escape' && this.open) this.close()
      },
      selectSuggestion (text) {
        this.draft = text
        this.$nextTick(() => this.$refs.composer && this.$refs.composer.focus())
      },
      async newConversation () {
        if (this.streaming) return
        this.conversation = null
        this.messages = []
        this.files = []
        this.approval = null
        this.error = ''
        this.sidebarOpen = false
        await this.$nextTick()
        this.$refs.composer && this.$refs.composer.focus()
      },
      async openConversation (conversation) {
        if (this.streaming) return
        this.error = ''
        const result = await api.request(`/conversations/${conversation.id}`)
        this.conversation = result.conversation
        this.messages = result.messages || []
        this.files = []
        this.approval = (result.approvals || [])[0] || null
        this.sidebarOpen = false
        this.scrollToBottom()
      },
      async deleteConversation (conversation) {
        if (this.streaming || !window.confirm(`¿Eliminar la conversación “${conversation.title}”?`)) return
        this.deleting = conversation.id
        try {
          await api.request(`/conversations/${conversation.id}`, { method: 'DELETE' })
          this.conversations = this.conversations.filter(item => item.id !== conversation.id)
          if (this.conversation && this.conversation.id === conversation.id) await this.newConversation()
        } catch (error) {
          this.error = error.message
        } finally {
          this.deleting = ''
        }
      },
      onComposerKeydown (event) {
        if (event.key === 'Enter' && !event.shiftKey && (event.ctrlKey || event.metaKey)) {
          event.preventDefault()
          this.send()
        }
      },
      chooseFiles () { if (!this.streaming) this.$refs.fileInput.click() },
      addFiles (event) {
        this.error = ''
        Array.from(event.target.files || []).forEach(file => {
          if (this.files.length >= MAX_FILES_PER_TURN) {
            this.error = 'Puedes adjuntar máximo 5 archivos por mensaje.'
          } else if (!ALLOWED_EXTENSIONS.has(fileExtension(file.name))) {
            this.error = `El archivo ${file.name} no tiene un formato permitido.`
          } else if (file.size <= 0 || file.size > MAX_FILE_SIZE) {
            this.error = `El archivo ${file.name} supera el límite de 10 MB.`
          } else if (!this.files.some(item => item.file.name === file.name && item.file.size === file.size)) {
            this.files.push({ key: makeID(), file, state: 'pending', id: null })
          }
        })
        event.target.value = ''
      },
      removeFile (key) { if (!this.streaming) this.files = this.files.filter(item => item.key !== key) },
      async ensureConversation (prompt) {
        if (this.conversation) return this.conversation
        const title = prompt.trim().replace(/\s+/g, ' ').slice(0, 72) || 'Conversación con archivos'
        this.conversation = await api.request('/conversations', { method: 'POST', json: { title } })
        this.conversations.unshift(this.conversation)
        return this.conversation
      },
      async uploadFiles () {
        const ids = []
        for (const item of this.files) {
          if (item.id) { ids.push(item.id); continue }
          item.state = 'uploading'
          const target = await api.request(`/conversations/${this.conversation.id}/files/prepare`, {
            method: 'POST',
            json: { name: item.file.name, contentType: item.file.type || MIME_TYPES[fileExtension(item.file.name)], size: item.file.size },
          })
          const uploadURL = new URL(target.uploadURL, window.location.origin)
          const headers = new Headers(target.headers || {})
          if (uploadURL.origin === window.location.origin) {
            const token = api.accessToken()
            if (token) headers.set('Authorization', `Bearer ${token}`)
          }
          const uploaded = await fetch(uploadURL.toString(), { method: target.method || 'PUT', headers, body: item.file })
          if (!uploaded.ok) throw await responseError(uploaded)
          const completed = await api.request(`/conversations/${this.conversation.id}/files/${target.file.id}/complete`, { method: 'POST' })
          item.id = completed.id
          item.state = 'ready'
          ids.push(item.id)
        }
        return ids
      },
      async send () {
        if (!this.canSend) return
        const prompt = this.draft.trim() || 'Analiza los archivos adjuntos y resume la información relevante.'
        const displayFiles = this.files.map(item => item.file.name)
        this.error = ''
        this.approval = null
        this.toolStatus = ''
        this.streaming = true
        this.draft = ''
        try {
          await this.ensureConversation(prompt)
          const fileIDs = await this.uploadFiles()
          this.messages.push({ role: 'user', content: prompt, localFiles: displayFiles, createdAt: new Date().toISOString() })
          this.files = []
          const assistant = { role: 'assistant', content: '', streaming: true, createdAt: new Date().toISOString() }
          this.messages.push(assistant)
          this.scrollToBottom()
          await api.stream(`/conversations/${this.conversation.id}/messages`, {
            content: prompt, fileIDs, clientRequestID: makeID(),
          }, (event, data) => this.onAgentEvent(event, data, assistant))
          assistant.streaming = false
          if (!assistant.content && this.approval) this.messages = this.messages.filter(message => message !== assistant)
          await this.loadConversations()
        } catch (error) {
          this.error = error.message
          this.messages = this.messages.filter(message => !(message.streaming && !message.content))
        } finally {
          this.streaming = false
          this.scrollToBottom()
        }
      },
      onAgentEvent (event, data, assistant) {
        if (event === 'delta') assistant.content += data.text || ''
        if (event === 'message') assistant.content = data.text || assistant.content
        if (event === 'tool') this.toolStatus = `Consultando ${data.name || 'el CRM'}…`
        if (event === 'approval_required') {
          this.approval = data
          this.toolStatus = ''
        }
        if (event === 'error') this.error = data.message || 'El agente no pudo completar la solicitud.'
        if (event === 'done') this.toolStatus = ''
        this.$forceUpdate()
        this.scrollToBottom()
      },
      async decideApproval (approved) {
        if (!this.approval || this.streaming) return
        const approval = this.approval
        this.approval = null
        this.error = ''
        this.streaming = true
        const assistant = { role: 'assistant', content: '', streaming: true, createdAt: new Date().toISOString() }
        this.messages.push(assistant)
        try {
          await api.stream(`/conversations/${this.conversation.id}/approvals/${approval.approvalID}`, { approved },
            (event, data) => this.onAgentEvent(event, data, assistant))
          assistant.streaming = false
        } catch (error) {
          this.error = error.message
          this.messages = this.messages.filter(message => message !== assistant)
        } finally {
          this.streaming = false
          this.scrollToBottom()
        }
      },
      scrollToBottom () {
        this.$nextTick(() => {
          const element = this.$refs.messages
          if (element) element.scrollTop = element.scrollHeight
        })
      },
      formatDate (value) {
        if (!value) return ''
        try { return new Intl.DateTimeFormat('es-CO', { day: 'numeric', month: 'short' }).format(new Date(value)) } catch (_) { return '' }
      },
    },
    template: `
      <div v-if="context && context.enabled && context.allowed" class="abera-ai">
        <button v-if="!open" class="abera-ai-launcher" type="button" aria-label="Abrir asistente de IA" @click="toggle">
          <span v-html="icons.assistant"></span><span>Asistente</span>
        </button>
        <section v-else class="abera-ai-panel" role="dialog" aria-label="Asistente de IA de Abera" aria-modal="false">
          <button v-if="sidebarOpen" class="abera-ai-scrim" type="button" aria-label="Cerrar historial" @click="sidebarOpen = false"></button>
          <aside class="abera-ai-sidebar" :class="{ 'is-open': sidebarOpen }" aria-label="Historial de conversaciones">
            <div class="abera-ai-brand">
              <span class="abera-ai-brand-mark" v-html="icons.assistant"></span>
              <span><strong>Asistente Abera</strong><small>Conectado al CRM</small></span>
            </div>
            <button class="abera-ai-new" type="button" :disabled="streaming" @click="newConversation">
              <span v-html="icons.plus"></span>Nueva conversación
            </button>
            <div class="abera-ai-conversations">
              <div v-for="item in conversations" :key="item.id" class="abera-ai-conversation" :class="{ 'is-active': conversation && item.id === conversation.id }">
                <button class="abera-ai-conversation-main" type="button" :disabled="streaming" @click="openConversation(item)">
                  <span class="abera-ai-conversation-title">{{ item.title }}</span>
                  <span class="abera-ai-conversation-date">{{ formatDate(item.updatedAt) }}</span>
                </button>
                <button class="abera-ai-conversation-delete" type="button" :disabled="deleting === item.id || streaming" aria-label="Eliminar conversación" @click="deleteConversation(item)" v-html="icons.trash"></button>
              </div>
            </div>
          </aside>
          <main class="abera-ai-main">
            <header class="abera-ai-header">
              <button class="abera-ai-icon-button abera-ai-menu" type="button" aria-label="Abrir historial" @click="sidebarOpen = true" v-html="icons.menu"></button>
              <div class="abera-ai-header-copy">
                <h2>{{ title }}</h2>
                <span class="abera-ai-status"><i class="abera-ai-status-dot"></i>{{ streaming ? 'Trabajando…' : 'Listo para ayudarte' }}</span>
              </div>
              <button class="abera-ai-icon-button" type="button" aria-label="Cerrar asistente" @click="close" v-html="icons.close"></button>
            </header>
            <div ref="messages" class="abera-ai-messages" aria-live="polite">
              <div v-if="!messages.length" class="abera-ai-welcome">
                <span class="abera-ai-welcome-mark" v-html="icons.assistant"></span>
                <h3>¿En qué trabajamos?</h3>
                <p>Puedo consultar el CRM, explicar tus datos y preparar cambios. Siempre pediré confirmación antes de modificar información.</p>
                <div class="abera-ai-suggestions">
                  <button v-for="suggestion in suggestions" :key="suggestion" class="abera-ai-suggestion" type="button" @click="selectSuggestion(suggestion)">{{ suggestion }}</button>
                </div>
              </div>
              <article v-for="(message, index) in messages" :key="message.id || index" class="abera-ai-message" :class="{ 'is-user': message.role === 'user' }">
                <div class="abera-ai-bubble">
                  <span v-if="message.content">{{ message.content }}</span>
                  <span v-else-if="message.streaming" class="abera-ai-typing" aria-label="El asistente está escribiendo"><span></span><span></span><span></span></span>
                  <span v-if="message.localFiles && message.localFiles.length" class="abera-ai-message-files">
                    <span v-for="name in message.localFiles" :key="name" class="abera-ai-message-file"><i v-html="icons.file"></i>{{ name }}</span>
                  </span>
                </div>
              </article>
              <div v-if="toolStatus" class="abera-ai-tool-status">{{ toolStatus }}</div>
              <section v-if="approval" class="abera-ai-approval" aria-label="Confirmación requerida">
                <strong>Confirma antes de modificar el CRM</strong>
                <p>{{ approval.description }}</p>
                <div class="abera-ai-approval-actions">
                  <button class="abera-ai-action is-primary" type="button" :disabled="streaming" @click="decideApproval(true)">Confirmar acción</button>
                  <button class="abera-ai-action" type="button" :disabled="streaming" @click="decideApproval(false)">Cancelar</button>
                </div>
              </section>
            </div>
            <div>
              <div v-if="error" class="abera-ai-error" role="alert">{{ error }}</div>
              <div v-if="files.length" class="abera-ai-files">
                <span v-for="item in files" :key="item.key" class="abera-ai-file-chip">
                  <i v-html="icons.file"></i><span>{{ item.file.name }}</span>
                  <button class="abera-ai-file-remove" type="button" aria-label="Quitar archivo" :disabled="streaming" @click="removeFile(item.key)">×</button>
                </span>
              </div>
              <footer class="abera-ai-composer">
                <div class="abera-ai-composer-box">
                  <input ref="fileInput" type="file" hidden multiple accept=".pdf,.txt,.docx,.csv,.xlsx,.png,.jpg,.jpeg" @change="addFiles">
                  <button class="abera-ai-attach" type="button" aria-label="Adjuntar archivo" :disabled="streaming" @click="chooseFiles" v-html="icons.attach"></button>
                  <textarea ref="composer" v-model="draft" rows="1" maxlength="50000" placeholder="Escribe una pregunta o pide una acción…" :disabled="streaming" @keydown="onComposerKeydown"></textarea>
                  <button class="abera-ai-send" type="button" aria-label="Enviar mensaje" :disabled="!canSend" @click="send" v-html="icons.send"></button>
                </div>
                <p class="abera-ai-composer-note">Ctrl + Enter para enviar · Las acciones requieren confirmación</p>
              </footer>
            </div>
          </main>
        </section>
      </div>
    `,
  }
}

export async function mountAssistant ({ Vue, auth }) {
  if (!Vue || !auth || document.getElementById(ROOT_ID)) return null
  installStyles()
  const root = document.createElement('div')
  root.id = ROOT_ID
  document.body.appendChild(root)
  const api = new AssistantAPI(() => auth.accessToken)
  const app = new Vue({ el: root, render: create => create(makeComponent(api)) })
  window.__aberaAssistant = app
  return app
}
