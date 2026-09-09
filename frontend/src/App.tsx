import { useEffect, useMemo, useRef, useState, type ChangeEvent, type CSSProperties, type MouseEvent as ReactMouseEvent, type ReactNode } from 'react';
import {
  Activity,
  AlertTriangle,
  ArrowUpRight,
  BarChart3,
  BookOpen,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  CircleDashed,
  ClipboardCheck,
  Crop,
  Download,
  Eye,
  EyeOff,
  FileCheck2,
  FileText,
  FolderArchive,
  Globe,
  HardDriveDownload,
  ImagePlus,
  KeyRound,
  Laptop,
  Layers3,
  LockKeyhole,
  LogIn,
  LogOut,
  Maximize2,
  Minus,
  PackageCheck,
  Palette,
  RefreshCw,
  RotateCcw,
  ScanSearch,
  Settings2,
  ShieldCheck,
  Trash2,
  UserRound,
  Users,
  WalletCards,
  X,
} from 'lucide-react';
import logo from './assets/images/logo-universal.png';
import claudeLogo from './assets/clients/claude.svg';
import codexLogo from './assets/clients/codex.svg';
import geminiLogo from './assets/clients/gemini.svg';
import grokLogo from './assets/clients/grok.svg';
import hermesLogo from './assets/clients/hermes.png';
import openclawLogo from './assets/clients/openclaw.svg';
import opencodeLogo from './assets/clients/opencode.svg';
import animeWallpaper from './assets/themes/anime-coffee-girl.jpg';
import cherryWallpaper from './assets/themes/cherry-blossoms-blue.jpg';
import mountainWallpaper from './assets/themes/mountain-place.jpg';
import cityWallpaper from './assets/themes/city-lights.jpg';
import ikunWallpaper from './assets/themes/ikun.png';
import chinaWallpaper from './assets/themes/China.png';
import animeBoyWallpaper from './assets/themes/二次元男.png';
import animeGirlWallpaper from './assets/themes/二次元女.png';
import './App.css';
import {
  CheckClientConnections,
  CheckForUpdates,
  ConfigureExistingTool,
  ConfigureProvisionedTool,
  ConfigureTool,
  CreateToolKey,
  DeleteBackup,
  DeleteSavedAccountLogin,
  GetAccountState,
  GetAccountToolOptions,
  GetAppInfo,
  GetBackupRoot,
  GetClientConfiguration,
  GetConfiguredToolModels,
  GetPublicGroupRatios,
  GetSavedAccountLogin,
  GetToolLifecycleInfo,
  InstallLatestUpdate,
  ListBackups,
  LoginAccount,
  LogoutAccount,
  OpenExternalURL,
  RestoreBackup,
  RefreshAccountState,
  RunToolLifecycleAction,
  SaveAccountLogin,
  ScanEnvironment,
  ValidateToolKey,
  VerifyAccountTwoFactor,
} from '../wailsjs/go/main/App';
import { main } from '../wailsjs/go/models';
import { ClipboardGetText, ClipboardSetText, EventsOn, Quit, WindowMinimise, WindowToggleMaximise } from '../wailsjs/runtime/runtime';

type ClientId = 'claude' | 'claude-desktop' | 'codex' | 'gemini' | 'grok' | 'opencode' | 'openclaw' | 'hermes';
type TabId = 'overview' | 'groups' | 'backups' | 'updates' | 'appearance';
type ThemeId = 'ciyuan' | 'anime' | 'sakura' | 'mountain' | 'city' | 'ikun' | 'china' | 'anime-boy' | 'anime-girl' | 'custom';
type ThemeMode = 'system' | 'light' | 'dark';
type ResolvedThemeMode = Exclude<ThemeMode, 'system'>;
type ThemeTransparency = { skin: number; content: number };
type ThemeTransparencyByTheme = Partial<Record<ThemeId, ThemeTransparency>>;
type NoticeTone = 'success' | 'error' | 'neutral';

type ClientStatus = {
  id: ClientId;
  name: string;
  supported?: boolean;
  installed: boolean;
  executablePath: string;
  configPath: string;
  configExists: boolean;
  configState: string;
  version: string;
  detail: string;
};

type EnvironmentReport = { os: string; home: string; scannedAt: string; clients: ClientStatus[] };
type Model = { id: string; object?: string; owned_by?: string };
type AppInfo = { name: string; version: string; platform?: string; updateManifestUrl: string; gatewayUrl: string };
type AccountState = { signedIn: boolean; username: string; balance?: string; quota?: number; balanceUpdatedAt?: string; expiresAt?: string };
type AccountLoginResult = { signedIn: boolean; requiresTwoFactor: boolean; flowToken: string; username: string; expiresAt?: string };
type SavedAccountLogin = { username: string; password: string };
type ToolGroupOption = { name: string; description: string; ratio: string; models: Model[] };
type ToolOptionsResponse = { clientId: ClientId; groups: ToolGroupOption[]; existingKeys?: ToolKeyResult[] };
type ToolKeyResult = { provisionId: string; clientId: ClientId; group: string; groupDescription?: string; name?: string; existing?: boolean; models: Model[]; status: number; endpoint: string };
type ToolKeyValidationResult = { clientId: ClientId; models: Model[]; selectedModel?: string; status: number; endpoint: string };
type ToolRestartResult = { clientId: ClientId; attempted: boolean; restarted: boolean; manualRestartRequired: boolean; message: string };
type ConfigureResult = { success: boolean; error?: string; configured: string[]; restarts?: ToolRestartResult[]; finishedAt: string };
type GroupRatio = { name: string; description: string; ratio: number };
type GroupRatioReport = { groups: GroupRatio[]; endpoint: string; fetchedAt: string };
type BackupFile = { clientId: string; originalPath: string; backupPath: string; exists: boolean };
type Backup = { id: string; createdAt: string; path: string; files: BackupFile[] };
type UpdateInfo = {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  downloadUrl?: string;
  releaseNotes?: string;
  publishedAt?: string;
  checkedAt: string;
  error?: string;
};
type InstallUpdateResult = { success: boolean; message?: string; error?: string; downloadUrl?: string };
type UpdateProgress = { stage: string; message: string; downloadedBytes: number; totalBytes: number; percent: number };
type ClientConnectionResult = {
  id: ClientId;
  name: string;
  success: boolean;
  configured: boolean;
  status: number;
  endpoint: string;
  message: string;
  checkedAt: string;
};
type ConnectionCheckReport = { results: ClientConnectionResult[]; checkedAt: string };
type SetupState = { clientId: ClientId; mode: 'account' | 'manual' };
type CodexExperimentalSettings = {
  contextManagementExperimentalMode: boolean;
  tokenBudgetEnabled: boolean;
  tokenBudgetUseHistoryNotesExtension: boolean;
};
type ClientConfigurationFile = { path: string; exists: boolean; content: string };
type ClientConfigurationView = { clientId: ClientId; clientName: string; files: ClientConfigurationFile[]; secretsRedacted: boolean };
type ToolLifecycleInfo = {
  clientId: ClientId;
  name: string;
  installed: boolean;
  currentVersion?: string;
  latestVersion?: string;
  updateAvailable: boolean;
  canInstall: boolean;
  canUpdate: boolean;
  downloadUrl?: string;
  installMethod?: string;
  checkedAt: string;
  message?: string;
  error?: string;
};
type ToolLifecycleResult = { success: boolean; manual: boolean; downloadUrl?: string; message?: string; error?: string; info: ToolLifecycleInfo };
type ConfirmationTone = 'default' | 'danger';
type ConfirmationRequest = {
  title: string;
  message: string;
  confirmLabel?: string;
  tone?: ConfirmationTone;
  resolve: (confirmed: boolean) => void;
};
type ConfirmationOptions = Omit<ConfirmationRequest, 'resolve'>;
type LoginInputKind = 'username' | 'password';
type LoginContextMenuState = { kind: LoginInputKind; left: number; top: number; start: number; end: number; hasSelection: boolean };

const documentationURL = 'https://ocn4dgkicvdh.feishu.cn/docx/Y88FdkLNPo6g17xWIfHcQfgknrc';
const officialWebsiteURL = 'https://ciyuanshen.top';
const walletURL = 'https://ciyuanshen.top/wallet';
const signUpURL = 'https://ciyuanshen.top/sign-up';
const forgotPasswordURL = 'https://ciyuanshen.top/forgot-password';
const qqGroupURL = 'https://qm.qq.com/q/rmwfirFNp8';
const desktopOnlyMessage = '浏览器预览无法读取本机配置，请下载并运行桌面安装版。';
const clientOrder: ClientId[] = ['codex', 'claude', 'claude-desktop', 'gemini', 'grok', 'opencode', 'openclaw', 'hermes'];
const clientCopy: Record<ClientId, { short: string; badge: string }> = {
  claude: { short: 'Claude Code CLI/插件', badge: 'Anthropic CLI' },
  'claude-desktop': { short: 'Claude Code客户端', badge: 'Anthropic Desktop' },
  codex: { short: 'ChatGPT/Codex CLI/Codex插件', badge: 'Responses' },
  gemini: { short: 'Gemini CLI', badge: 'Gemini API' },
  grok: { short: 'Grok Build', badge: 'Responses' },
  opencode: { short: 'OpenCode', badge: 'OpenAI compatible' },
  openclaw: { short: 'OpenClaw', badge: 'OpenAI compatible' },
  hermes: { short: 'Hermes Agent', badge: 'Chat Completions' },
};
const recommendedModels: Record<ClientId, string> = {
  claude: 'claude-sonnet-4-5',
  'claude-desktop': 'claude-sonnet-4-5',
  codex: 'gpt-5.6-terra',
  gemini: 'gemini-2.5-pro',
  grok: 'grok-4',
  opencode: 'gpt-5.6-sol',
  openclaw: 'gpt-5.6-sol',
  hermes: 'gpt-5.6-sol',
};
const clientLogos: Record<ClientId, string> = {
  claude: claudeLogo,
  'claude-desktop': claudeLogo,
  codex: codexLogo,
  gemini: geminiLogo,
  grok: grokLogo,
  opencode: opencodeLogo,
  openclaw: openclawLogo,
  hermes: hermesLogo,
};
type ThemeDefinition = {
  id: ThemeId;
  name: string;
  subtitle: string;
  source: string;
  sourceURL?: string;
  wallpaper?: string;
  swatches: string[];
};

const themeStorageKey = 'ciyuanshen-config-assistant.theme';
const themeModeStorageKey = 'ciyuanshen-config-assistant.theme-mode';
const customWallpaperStorageKey = 'ciyuanshen-config-assistant.custom-wallpaper';
const themeTransparencyStorageKey = 'ciyuanshen-config-assistant.theme-transparency';
const updateProgressEvent = 'ciyuanshen:update-progress';
const defaultThemeTransparency: ThemeTransparency = { skin: 100, content: 100 };
const defaultCodexExperimentalSettings = (): CodexExperimentalSettings => ({
  contextManagementExperimentalMode: true,
  tokenBudgetEnabled: true,
  tokenBudgetUseHistoryNotesExtension: true,
});
const themeDefinitions: ThemeDefinition[] = [
  { id: 'ciyuan', name: '词元神青', subtitle: '清爽工作台', source: '词元神', swatches: ['#173735', '#0c766d', '#f4f7f7', '#e4f3f0'] },
  { id: 'anime', name: '冬日人物', subtitle: '动漫人物 · 柔和青', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: animeWallpaper, swatches: ['#406b70', '#d97c9f', '#e9f2ef', '#b9e4de'] },
  { id: 'sakura', name: '蓝樱花', subtitle: '花枝风景 · 清透蓝', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: cherryWallpaper, swatches: ['#31566f', '#d8797c', '#edf5f5', '#bfe3e6'] },
  { id: 'mountain', name: '雪山远景', subtitle: '自然风景', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: mountainWallpaper, swatches: ['#122c35', '#76b6c9', '#203d46', '#d2e4d6'] },
  { id: 'city', name: '夜色城市', subtitle: '城市灯火', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: cityWallpaper, swatches: ['#171c29', '#e8873c', '#28303c', '#f5c96a'] },
  { id: 'ikun', name: '爱坤', subtitle: '用户提供皮肤', source: '词元神用户', wallpaper: ikunWallpaper, swatches: ['#20242a', '#cb7b38', '#f7f4ef', '#f0b864'] },
  { id: 'china', name: '中国风', subtitle: '水墨山水', source: '词元神用户', wallpaper: chinaWallpaper, swatches: ['#3d332b', '#a54d3d', '#f5ede0', '#d67155'] },
  { id: 'anime-boy', name: '二次元男', subtitle: '夜幕幻境', source: '词元神用户', wallpaper: animeBoyWallpaper, swatches: ['#20234f', '#7569ef', '#e8e8ff', '#a89dff'] },
  { id: 'anime-girl', name: '二次元女', subtitle: '樱夜和风', source: '词元神用户', wallpaper: animeGirlWallpaper, swatches: ['#3b2f58', '#a65caa', '#f6eefa', '#d391e1'] },
  { id: 'custom', name: '我的图片', subtitle: '自定义背景 · 本地保存', source: '本机图片', wallpaper: undefined, swatches: ['#173735', '#0c766d', '#f4f7f7', '#e4f3f0'] },
];

function readStoredTheme(): ThemeId {
  try {
    const stored = window.localStorage.getItem(themeStorageKey);
    if (stored === 'custom' && readStoredWallpaper()) return 'custom';
    if (stored && themeDefinitions.some((theme) => theme.id === stored)) return stored as ThemeId;
  } catch {
    // Private browsing or a restricted WebView may disable localStorage.
  }
  return 'ciyuan';
}

function readStoredWallpaper() {
  try {
    const stored = window.localStorage.getItem(customWallpaperStorageKey);
    return stored && stored.startsWith('data:image/') ? stored : '';
  } catch {
    return '';
  }
}

function readStoredThemeMode(): ThemeMode {
  try {
    const stored = window.localStorage.getItem(themeModeStorageKey);
    if (stored === 'system' || stored === 'dark' || stored === 'light') return stored;
  } catch {
    // Private browsing or a restricted WebView may disable localStorage.
  }
  return 'system';
}

function clampTransparency(value: unknown, fallback: number, minimum: number) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return fallback;
  return Math.round(Math.min(100, Math.max(minimum, value)));
}

function readStoredThemeTransparency(): ThemeTransparencyByTheme {
  try {
    const stored = window.localStorage.getItem(themeTransparencyStorageKey);
    if (!stored) return {};
    const parsed: unknown = JSON.parse(stored);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
    const saved = parsed as Record<string, unknown>;
    const settings: ThemeTransparencyByTheme = {};
    for (const item of themeDefinitions) {
      const value = saved[item.id];
      if (!value || typeof value !== 'object' || Array.isArray(value)) continue;
      const transparency = value as Record<string, unknown>;
      settings[item.id] = {
        skin: clampTransparency(transparency.skin, defaultThemeTransparency.skin, 0),
        content: clampTransparency(transparency.content, defaultThemeTransparency.content, 15),
      };
    }
    return settings;
  } catch {
    return {};
  }
}

function getThemeTransparency(settings: ThemeTransparencyByTheme, theme: ThemeId): ThemeTransparency {
  const saved = settings[theme];
  return {
    skin: clampTransparency(saved?.skin, defaultThemeTransparency.skin, 0),
    content: clampTransparency(saved?.content, defaultThemeTransparency.content, 15),
  };
}

function getSystemThemeMode(): ResolvedThemeMode {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function readUpdateProgress(value: unknown): UpdateProgress | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const progress = value as Record<string, unknown>;
  if (typeof progress.stage !== 'string' || typeof progress.message !== 'string') return null;
  return {
    stage: progress.stage,
    message: progress.message,
    downloadedBytes: typeof progress.downloadedBytes === 'number' && Number.isFinite(progress.downloadedBytes) ? Math.max(0, progress.downloadedBytes) : 0,
    totalBytes: typeof progress.totalBytes === 'number' && Number.isFinite(progress.totalBytes) ? Math.max(0, progress.totalBytes) : 0,
    percent: typeof progress.percent === 'number' && Number.isFinite(progress.percent) ? Math.min(100, Math.max(0, Math.round(progress.percent))) : 0,
  };
}

function readCodexBoolean(value: string) {
  const match = value.trim().match(/^(true|false)\b/i);
  return match ? match[1].toLowerCase() === 'true' : undefined;
}

function readInlineCodexBoolean(value: string, key: string) {
  const match = value.match(new RegExp(`(?:^|[,{])\\s*${key}\\s*=\\s*(true|false)\\b`, 'i'));
  return match ? match[1].toLowerCase() === 'true' : undefined;
}

// Missing values deliberately remain enabled so existing configs gain the
// new Codex defaults without changing an option the user explicitly disabled.
function readCodexExperimentalSettings(content: string): CodexExperimentalSettings {
  const settings = defaultCodexExperimentalSettings();
  let table = '';
  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    const tableMatch = line.match(/^\[\s*([^\]]+)\s*\]$/);
    if (tableMatch) {
      table = tableMatch[1].trim();
      continue;
    }
    const assignment = line.match(/^([A-Za-z0-9_.-]+)\s*=\s*(.+)$/);
    if (!assignment) continue;
    const [, key, value] = assignment;
    if (key === 'context_management.experimental_mode') {
      const parsed = readCodexBoolean(value);
      if (parsed !== undefined) settings.contextManagementExperimentalMode = parsed;
      continue;
    }
    if (key === 'token_budget.enabled') {
      const parsed = readCodexBoolean(value);
      if (parsed !== undefined) settings.tokenBudgetEnabled = parsed;
      continue;
    }
    if (key === 'token_budget.use_history_notes_extension') {
      const parsed = readCodexBoolean(value);
      if (parsed !== undefined) settings.tokenBudgetUseHistoryNotesExtension = parsed;
      continue;
    }
    if (key === 'context_management' && value.trim().startsWith('{')) {
      const parsed = readInlineCodexBoolean(value, 'experimental_mode');
      if (parsed !== undefined) settings.contextManagementExperimentalMode = parsed;
      continue;
    }
    if (key === 'token_budget' && value.trim().startsWith('{')) {
      const enabled = readInlineCodexBoolean(value, 'enabled');
      const history = readInlineCodexBoolean(value, 'use_history_notes_extension');
      if (enabled !== undefined) settings.tokenBudgetEnabled = enabled;
      if (history !== undefined) settings.tokenBudgetUseHistoryNotesExtension = history;
      continue;
    }
    if (table === 'context_management' && key === 'experimental_mode') {
      const parsed = readCodexBoolean(value);
      if (parsed !== undefined) settings.contextManagementExperimentalMode = parsed;
      continue;
    }
    if (table === 'token_budget' && key === 'enabled') {
      const parsed = readCodexBoolean(value);
      if (parsed !== undefined) settings.tokenBudgetEnabled = parsed;
      continue;
    }
    if (table === 'token_budget' && key === 'use_history_notes_extension') {
      const parsed = readCodexBoolean(value);
      if (parsed !== undefined) settings.tokenBudgetUseHistoryNotesExtension = parsed;
    }
  }
  return settings;
}

const tabTitles: Record<TabId, string> = {
  overview: '一键配置',
  groups: '分组倍率',
  backups: '配置备份',
  updates: '版本更新',
  appearance: '外观皮肤',
};

const mockEnvironment: EnvironmentReport = {
  os: 'browser',
  home: '~',
  scannedAt: new Date().toISOString(),
  clients: clientOrder.map((id) => ({
    id,
    name: clientCopy[id].short,
    supported: true,
    installed: false,
    executablePath: '',
    configPath: `~/.${id}/config`,
    configExists: false,
    configState: 'missing',
    version: '',
    detail: '未检测到',
  })),
};

function inWails() {
  const bridge = window as unknown as {
    go?: { main?: { App?: { GetAppInfo?: unknown; CheckClientConnections?: unknown } } };
  };
  return typeof bridge.go?.main?.App?.GetAppInfo === 'function'
    && typeof bridge.go?.main?.App?.CheckClientConnections === 'function';
}

function formatTime(value?: string) {
  if (!value) return '尚未检查';
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}

function formatRatio(value: number) {
  return `${value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}x`;
}

function formatFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`;
}

function configurationRestartNotice(result: ConfigureResult, clientId: ClientId): { tone: NoticeTone; text: string } {
  const restart = result.restarts?.find((item) => item.clientId === clientId);
  if (!restart?.message) return { tone: 'success', text: `${clientCopy[clientId].short} 一键配置完成` };
  return { tone: restart.manualRestartRequired ? 'neutral' : 'success', text: restart.message };
}

function defaultModel(clientId: ClientId, models: Model[], current?: string) {
  if (current && models.some((model) => model.id === current)) return current;
  if (models.some((model) => model.id === recommendedModels[clientId])) return recommendedModels[clientId];
  return models[0]?.id || '';
}

function App() {
  const [tab, setTab] = useState<TabId>('overview');
  const [theme, setTheme] = useState<ThemeId>(readStoredTheme);
  const [themeMode, setThemeMode] = useState<ThemeMode>(readStoredThemeMode);
  const [themeTransparency, setThemeTransparency] = useState<ThemeTransparencyByTheme>(readStoredThemeTransparency);
  const [systemThemeMode, setSystemThemeMode] = useState<ResolvedThemeMode>(getSystemThemeMode);
  const [customWallpaper, setCustomWallpaper] = useState(readStoredWallpaper);
  const [environment, setEnvironment] = useState<EnvironmentReport>(mockEnvironment);
  const [appInfo, setAppInfo] = useState<AppInfo>({ name: '词元神配置助手', version: '0.2.18', platform: '', updateManifestUrl: '', gatewayUrl: 'https://api.ciyuanshen.top/v1' });
  const [account, setAccount] = useState<AccountState>({ signedIn: false, username: '' });
  const [accountRefreshing, setAccountRefreshing] = useState(false);
  const [toolModels, setToolModels] = useState<Partial<Record<ClientId, Model[]>>>({});
  const [modelByClient, setModelByClient] = useState<Record<ClientId, string>>(recommendedModels);
  const [modelsLoading, setModelsLoading] = useState<Partial<Record<ClientId, boolean>>>({});
  const [modelErrors, setModelErrors] = useState<Partial<Record<ClientId, string>>>({});
  const [connectionResults, setConnectionResults] = useState<Partial<Record<ClientId, ClientConnectionResult>>>({});
  const [keyValidationResults, setKeyValidationResults] = useState<Partial<Record<ClientId, ToolKeyValidationResult>>>({});
  const [busy, setBusy] = useState('');
  const [checkingClient, setCheckingClient] = useState<ClientId | null>(null);
  const [applyingModelClient, setApplyingModelClient] = useState<ClientId | null>(null);
  const [lifecycleByClient, setLifecycleByClient] = useState<Partial<Record<ClientId, ToolLifecycleInfo>>>({});
  const [lifecycleBusyClient, setLifecycleBusyClient] = useState<ClientId | null>(null);
  const [feedback, setFeedback] = useState<{ tone: NoticeTone; text: string } | null>(null);
  const [confirmation, setConfirmation] = useState<ConfirmationRequest | null>(null);
  const [actionNotice, setActionNotice] = useState<{ tone: NoticeTone; text: string; left: number; top: number; below: boolean } | null>(null);
  const feedbackTimer = useRef<number | undefined>(undefined);
  const actionTimer = useRef<number | undefined>(undefined);
  const pendingConfirmation = useRef<ConfirmationRequest | null>(null);
  const configureAnchor = useRef<HTMLElement | null>(null);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [backupRoot, setBackupRoot] = useState('');
  const [update, setUpdate] = useState<UpdateInfo | null>(null);
  const [updateProgress, setUpdateProgress] = useState<UpdateProgress | null>(null);
  const [groupReport, setGroupReport] = useState<GroupRatioReport | null>(null);
  const [setup, setSetup] = useState<SetupState | null>(null);
  const [toolOptions, setToolOptions] = useState<ToolOptionsResponse | null>(null);
  const [setupGroup, setSetupGroup] = useState('');
  const [setupKey, setSetupKey] = useState('');
  const [showSetupKey, setShowSetupKey] = useState(false);
  const [setupValidation, setSetupValidation] = useState<ToolKeyValidationResult | null>(null);
  const [provision, setProvision] = useState<ToolKeyResult | null>(null);
  const [accountKeySource, setAccountKeySource] = useState<'existing' | 'new'>('existing');
  const [setupModel, setSetupModel] = useState('');
  const [codexExperimentalSettings, setCodexExperimentalSettings] = useState<CodexExperimentalSettings>(defaultCodexExperimentalSettings);
  const [setupBusy, setSetupBusy] = useState('');
  const [setupMessage, setSetupMessage] = useState<{ tone: NoticeTone; text: string } | null>(null);
  const [loginOpen, setLoginOpen] = useState(false);
  const [loginTarget, setLoginTarget] = useState<ClientId | null>(null);
  const [loginUsername, setLoginUsername] = useState('');
  const [loginPassword, setLoginPassword] = useState('');
  const [rememberLogin, setRememberLogin] = useState(false);
  const [showLoginPassword, setShowLoginPassword] = useState(false);
  const [twoFactorFlow, setTwoFactorFlow] = useState('');
  const [twoFactorCode, setTwoFactorCode] = useState('');
  const [loginBusy, setLoginBusy] = useState(false);
  const [loginMessage, setLoginMessage] = useState<string | null>(null);
  const [configurationClient, setConfigurationClient] = useState<ClientId | null>(null);
  const [configurationView, setConfigurationView] = useState<ClientConfigurationView | null>(null);
  const [configurationBusy, setConfigurationBusy] = useState(false);
  const [configurationError, setConfigurationError] = useState<string | null>(null);
  const [revealConfigurationSecrets, setRevealConfigurationSecrets] = useState(false);
  const codexSettingsRequest = useRef(0);
  const availableThemes = themeDefinitions;
  const activeTheme = availableThemes.find((item) => item.id === theme) || themeDefinitions[0];
  const wallpaper = theme === 'custom' ? customWallpaper : activeTheme.wallpaper || '';
  const activeThemeTransparency = getThemeTransparency(themeTransparency, theme);
  const resolvedThemeMode: ResolvedThemeMode = themeMode === 'system' ? systemThemeMode : themeMode;

  const clientMap = useMemo(() => new Map(environment.clients.map((client) => [client.id, client])), [environment.clients]);

  useEffect(() => {
    void loadInitialState();
  }, []);

  useEffect(() => () => {
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current);
    if (actionTimer.current !== undefined) window.clearTimeout(actionTimer.current);
    const pending = pendingConfirmation.current;
    pendingConfirmation.current = null;
    pending?.resolve(false);
  }, []);

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const syncThemeMode = () => setSystemThemeMode(mediaQuery.matches ? 'dark' : 'light');
    syncThemeMode();
    mediaQuery.addEventListener('change', syncThemeMode);
    return () => mediaQuery.removeEventListener('change', syncThemeMode);
  }, []);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.body.dataset.theme = theme;
    document.documentElement.dataset.themeMode = resolvedThemeMode;
    document.body.dataset.themeMode = resolvedThemeMode;
    document.documentElement.dataset.themeModePreference = themeMode;
    document.body.dataset.themeModePreference = themeMode;
    try {
      window.localStorage.setItem(themeStorageKey, theme);
      window.localStorage.setItem(themeModeStorageKey, themeMode);
      window.localStorage.setItem(themeTransparencyStorageKey, JSON.stringify(themeTransparency));
      if (customWallpaper) window.localStorage.setItem(customWallpaperStorageKey, customWallpaper);
      else window.localStorage.removeItem(customWallpaperStorageKey);
    } catch {
      // Theme remains active for this session when persistence is unavailable.
    }
    if (theme === 'custom' && !customWallpaper) setTheme('ciyuan');
  }, [theme, themeMode, resolvedThemeMode, customWallpaper, themeTransparency]);

  useEffect(() => {
    if (!inWails()) return;
    return EventsOn(updateProgressEvent, (payload: unknown) => {
      const next = readUpdateProgress(payload);
      if (next) setUpdateProgress(next);
    });
  }, []);

  function showFeedback(next: { tone: NoticeTone; text: string }, dismissAfter = 0) {
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current);
    feedbackTimer.current = undefined;
    setFeedback(next);
    if (dismissAfter > 0) {
      feedbackTimer.current = window.setTimeout(() => {
        feedbackTimer.current = undefined;
        setFeedback(null);
      }, dismissAfter);
    }
  }

  function showActionNotice(anchor: HTMLElement, next: { tone: NoticeTone; text: string }) {
    if (actionTimer.current !== undefined) window.clearTimeout(actionTimer.current);
    const rect = anchor.getBoundingClientRect();
    const below = rect.top < 112;
    const width = Math.min(330, window.innerWidth - 24);
    const left = Math.max(12, Math.min(window.innerWidth - width - 12, rect.right - width));
    setActionNotice({ tone: next.tone, text: next.text, left, top: below ? rect.bottom + 9 : rect.top - 9, below });
    actionTimer.current = window.setTimeout(() => {
      actionTimer.current = undefined;
      setActionNotice(null);
    }, 3200);
  }

  function confirmAction(options: ConfirmationOptions) {
    const previous = pendingConfirmation.current;
    if (previous) previous.resolve(false);
    return new Promise<boolean>((resolve) => {
      const request: ConfirmationRequest = { ...options, resolve };
      pendingConfirmation.current = request;
      setConfirmation(request);
    });
  }

  function settleConfirmation(confirmed: boolean) {
    const pending = pendingConfirmation.current;
    if (!pending) return;
    pendingConfirmation.current = null;
    setConfirmation(null);
    pending.resolve(confirmed);
  }

  async function loadInitialState() {
    try {
      if (inWails()) {
        const [info, report, root, accountState] = await Promise.all([GetAppInfo(), ScanEnvironment(), GetBackupRoot(), GetAccountState()]);
        const nextInfo = info as AppInfo;
        setAppInfo(nextInfo);
        setEnvironment(report as EnvironmentReport);
        setBackupRoot(root as string);
        const nextAccount = accountState as AccountState;
        setAccount(nextAccount);
        if (nextAccount.signedIn) void refreshAccountState(false);
        try {
          const saved = await GetSavedAccountLogin() as SavedAccountLogin;
          if (saved.username && saved.password) {
            setLoginUsername(saved.username);
            setLoginPassword(saved.password);
            setRememberLogin(true);
          }
        } catch {
          // Some Linux installations have no Secret Service provider. Login
          // remains available; remembering credentials is simply disabled.
        }
        void checkUpdate(true, nextInfo.version);
        await Promise.all([refreshConfiguredModels(report as EnvironmentReport), refreshToolLifecycles()]);
      } else {
        setEnvironment(mockEnvironment);
        setBackupRoot('~/.config/CiyuanShen/Config Assistant/backups');
      }
    } catch {
      showFeedback({ tone: 'error', text: '环境检测失败，请重试' });
    }
    await refreshBackups();
  }

  async function refreshAccountState(announce = true) {
    if (!inWails()) return;
    setAccountRefreshing(true);
    try {
      const nextState = await RefreshAccountState();
      setAccount(nextState as AccountState);
    } catch (error) {
      try {
        const currentState = await GetAccountState();
        setAccount(currentState as AccountState);
      } catch {
        // Keep the last rendered account summary when the bridge itself is unavailable.
      }
      if (announce) {
        showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '账户余额读取失败' }, 3200);
      }
    } finally {
      setAccountRefreshing(false);
    }
  }

  async function refreshEnvironment(announce = true) {
    setBusy('scan');
    try {
      const report = inWails() ? await ScanEnvironment() : mockEnvironment;
      setEnvironment(report as EnvironmentReport);
      await Promise.all([refreshConfiguredModels(report as EnvironmentReport), refreshToolLifecycles()]);
      if (announce) showFeedback({ tone: 'success', text: '环境检测已完成' }, 2200);
    } catch {
      if (announce) showFeedback({ tone: 'error', text: '环境检测失败，请检查权限后重试' });
    } finally {
      setBusy('');
    }
  }

  async function refreshConfiguredModels(report: EnvironmentReport) {
    const clients = report.clients.filter((client) => client.supported && client.configExists);
    setModelsLoading(Object.fromEntries(clients.map((client) => [client.id, true])) as Partial<Record<ClientId, boolean>>);
    setModelErrors({});
    setToolModels({});
    setConnectionResults({});
    setKeyValidationResults({});
    if (clients.length === 0) {
      return;
    }

    let checks: ClientConnectionResult[] = [];
    try {
      const response = inWails()
        ? await CheckClientConnections({ targets: clients.map((client) => client.id) })
        : browserPreviewConnectionCheck(clients.map((client) => client.id));
      checks = (response as ConnectionCheckReport).results || [];
      setConnectionResults((current) => ({
        ...current,
        ...Object.fromEntries(checks.map((check) => [check.id, check])) as Partial<Record<ClientId, ClientConnectionResult>>,
      }));
    } catch (error) {
      const message = error instanceof Error ? error.message : '检测已配置工具失败';
      const checkedAt = new Date().toISOString();
      setConnectionResults(Object.fromEntries(clients.map((client) => [client.id, {
        id: client.id,
        name: client.name,
        success: false,
        configured: false,
        status: 0,
        endpoint: 'https://api.ciyuanshen.top/v1/models',
        message,
        checkedAt,
      }])) as Partial<Record<ClientId, ClientConnectionResult>>);
      setModelErrors(Object.fromEntries(clients.map((client) => [client.id, message])) as Partial<Record<ClientId, string>>);
      setModelsLoading({});
      return;
    }

    const checkByClient = new Map(checks.map((check) => [check.id, check]));
    await Promise.all(clients.map(async (client) => {
      const check = checkByClient.get(client.id);
      if (!check?.success) {
        // A pre-existing client config can have harmless fields that differ
        // from the assistant's managed template. Still probe its stored Key
        // so users can see the models available to that Key while the config
        // check continues to report the exact mismatch separately.
        const models = await loadConfiguredClientModels(client.id);
        if (!models) {
          setModelErrors((current) => current[client.id]
            ? current
            : { ...current, [client.id]: check?.message || '配置检测未通过，暂不读取模型' });
        }
        return;
      }
      await loadConfiguredClientModels(client.id);
    }));
  }

  async function refreshToolLifecycles() {
    const entries = await Promise.all(clientOrder.map(async (clientId) => {
      try {
        const info = inWails()
          ? await GetToolLifecycleInfo(clientId)
          : await fetchBrowserPreviewToolLifecycle(clientId);
        return [clientId, info as ToolLifecycleInfo] as const;
      } catch (error) {
        return [clientId, {
          clientId,
          name: clientCopy[clientId].short,
          installed: false,
          updateAvailable: false,
          canInstall: false,
          canUpdate: false,
          checkedAt: new Date().toISOString(),
          error: error instanceof Error ? error.message : '工具检测失败',
        } as ToolLifecycleInfo] as const;
      }
    }));
    setLifecycleByClient(Object.fromEntries(entries) as Partial<Record<ClientId, ToolLifecycleInfo>>);
  }

  async function loadConfiguredClientModels(clientId: ClientId) {
    setModelsLoading((current) => ({ ...current, [clientId]: true }));
    setModelErrors((current) => {
      const next = { ...current };
      delete next[clientId];
      return next;
    });
    try {
      const response = inWails()
        ? await GetConfiguredToolModels(clientId)
        : mockToolValidation(clientId);
      const result = response as ToolKeyValidationResult;
      if (!result.models || result.models.length === 0) throw new Error('该 Key 没有返回可用模型');
      setToolModels((current) => ({ ...current, [clientId]: result.models }));
      setKeyValidationResults((current) => ({ ...current, [clientId]: result }));
      setModelByClient((current) => ({
        ...current,
        [clientId]: defaultModel(clientId, result.models, result.selectedModel || current[clientId]),
      }));
      return result;
    } catch (error) {
      setToolModels((current) => {
        const next = { ...current };
        delete next[clientId];
        return next;
      });
      setKeyValidationResults((current) => {
        const next = { ...current };
        delete next[clientId];
        return next;
      });
      setModelErrors((current) => ({ ...current, [clientId]: error instanceof Error ? error.message : '读取 Key 可用模型失败' }));
      return null;
    } finally {
      setModelsLoading((current) => ({ ...current, [clientId]: false }));
    }
  }

  async function refreshBackups() {
    try {
      const result = inWails() ? await ListBackups() : [];
      setBackups(result as Backup[]);
    } catch {
      setBackups([]);
    }
  }

  async function checkClient(clientId: ClientId, anchor?: HTMLElement | null) {
    setCheckingClient(clientId);
    try {
      const result = inWails()
        ? await CheckClientConnections({ targets: [clientId] })
        : browserPreviewConnectionCheck([clientId]);
      const report = result as ConnectionCheckReport;
      const check = report.results[0];
      if (!check) throw new Error('未获得检测结果');
      setConnectionResults((current) => ({ ...current, [clientId]: check }));
      let notice = check.success
        ? { tone: 'success' as const, text: `${check.name} 配置与连接正常` }
        : { tone: 'error' as const, text: check.message };
      if (check.success) {
        const models = await loadConfiguredClientModels(clientId);
        if (!models) {
          notice = { tone: 'error' as const, text: `${check.name} 连接正常，但读取可用模型失败` };
        }
      } else {
        const models = await loadConfiguredClientModels(clientId);
        if (!models) {
          setModelErrors((current) => current[clientId]
            ? current
            : { ...current, [clientId]: check.message || '检测未通过' });
        }
      }
      if (anchor) showActionNotice(anchor, notice);
      else showFeedback(notice, 3200);
    } catch (error) {
      const notice = { tone: 'error' as const, text: error instanceof Error ? error.message : '检测失败' };
      if (anchor) showActionNotice(anchor, notice);
      else showFeedback(notice, 3200);
    } finally {
      setCheckingClient(null);
    }
  }

  async function applyExistingModel(clientId: ClientId, model: string, anchor: HTMLElement) {
    if (!model) {
      showActionNotice(anchor, { tone: 'error', text: '请先读取该工具当前 Key 可用的模型' });
      return;
    }
    setApplyingModelClient(clientId);
    try {
      const result = inWails()
        ? await ConfigureExistingTool({ clientId, model })
        : mockConfigure(clientId);
      const configured = result as ConfigureResult;
      if (!configured.success) throw new Error(configured.error || '默认模型应用失败');
      setModelByClient((current) => ({ ...current, [clientId]: model }));
      await refreshEnvironment(false);
      showActionNotice(anchor, { tone: 'success', text: '默认模型修改成功' });
    } catch (error) {
      showActionNotice(anchor, { tone: 'error', text: error instanceof Error ? error.message : '默认模型应用失败' });
    } finally {
      setApplyingModelClient(null);
    }
  }

  async function loadClientConfiguration(clientId: ClientId, revealSecrets: boolean) {
    setConfigurationBusy(true);
    setConfigurationError(null);
    try {
      const view = inWails()
        ? await GetClientConfiguration(clientId, revealSecrets)
        : mockClientConfiguration(clientId, revealSecrets);
      setConfigurationView(view as ClientConfigurationView);
    } catch (error) {
      setConfigurationView(null);
      setConfigurationError(error instanceof Error ? error.message : '读取配置文件失败');
    } finally {
      setConfigurationBusy(false);
    }
  }

  function openClientConfiguration(clientId: ClientId) {
    setConfigurationClient(clientId);
    setConfigurationView(null);
    setConfigurationError(null);
    setRevealConfigurationSecrets(false);
    void loadClientConfiguration(clientId, false);
  }

  async function toggleConfigurationSecrets() {
    if (!configurationClient) return;
    const nextReveal = !revealConfigurationSecrets;
    if (nextReveal && !(await confirmAction({ title: '显示敏感信息', message: '配置文件可能包含 API Key 或访问令牌。确认在本机界面中显示明文吗？', confirmLabel: '显示明文' }))) return;
    setRevealConfigurationSecrets(nextReveal);
    void loadClientConfiguration(configurationClient, nextReveal);
  }

  async function checkToolLifecycle(clientId: ClientId, anchor?: HTMLElement) {
    setLifecycleBusyClient(clientId);
    try {
      const info = inWails()
        ? await GetToolLifecycleInfo(clientId)
        : await fetchBrowserPreviewToolLifecycle(clientId);
      const next = info as ToolLifecycleInfo;
      setLifecycleByClient((current) => ({ ...current, [clientId]: next }));
      if (anchor) {
        const message = lifecycleStatusMessage(next);
        showActionNotice(anchor, { tone: next.error ? 'error' : 'neutral', text: message });
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : '检查工具更新失败';
      if (anchor) showActionNotice(anchor, { tone: 'error', text: message });
    } finally {
      setLifecycleBusyClient(null);
    }
  }

  async function runToolLifecycleAction(clientId: ClientId, action: 'install' | 'update' | 'download', anchor: HTMLElement) {
    const lifecycle = lifecycleByClient[clientId];
    if (action === 'download') {
      const downloadURL = lifecycle?.downloadUrl;
      if (!downloadURL) {
        showActionNotice(anchor, { tone: 'error', text: '未找到官方下载地址，请先检查更新' });
        return;
      }
      await openExternal(downloadURL);
      showActionNotice(anchor, { tone: 'neutral', text: '已打开官方下载页面，安装完成后可返回一键配置' });
      return;
    }
    if (!inWails()) {
      showActionNotice(anchor, { tone: 'error', text: '浏览器预览无法安装或更新本机工具，请运行桌面安装版。' });
      return;
    }
    const verb = action === 'install' ? '安装' : '更新';
    if (!(await confirmAction({ title: `${verb}工具`, message: `将通过官方 npm 包${verb} ${clientCopy[clientId].short}。是否继续？`, confirmLabel: `确认${verb}` }))) return;
    setLifecycleBusyClient(clientId);
    try {
      const result = await RunToolLifecycleAction({ clientId, action });
      const lifecycleResult = result as ToolLifecycleResult;
      setLifecycleByClient((current) => ({ ...current, [clientId]: lifecycleResult.info }));
      if (lifecycleResult.manual) {
        const message = lifecycleResult.message || '该工具需要通过官方页面安装';
        if (lifecycleResult.downloadUrl && await confirmAction({ title: '打开官方下载页', message: `${message}\n\n现在打开官方下载页面吗？`, confirmLabel: '打开页面' })) {
          await openExternal(lifecycleResult.downloadUrl);
        }
        showActionNotice(anchor, { tone: 'neutral', text: message });
        return;
      }
      if (!lifecycleResult.success) throw new Error(lifecycleResult.error || `${verb}失败`);
      await refreshEnvironment(false);
      showActionNotice(anchor, { tone: 'success', text: `${lifecycleResult.message || `${verb}完成`}，现在可以一键配置` });
    } catch (error) {
      showActionNotice(anchor, { tone: 'error', text: error instanceof Error ? error.message : `${verb}失败` });
    } finally {
      setLifecycleBusyClient(null);
    }
  }

  function resetSetup() {
	  codexSettingsRequest.current += 1;
    setToolOptions(null);
    setSetupGroup('');
    setSetupKey('');
    setShowSetupKey(false);
    setSetupValidation(null);
    setProvision(null);
    setAccountKeySource('existing');
    setSetupModel('');
    setCodexExperimentalSettings(defaultCodexExperimentalSettings());
    setSetupBusy('');
    setSetupMessage(null);
  }

  function openToolSetup(clientId: ClientId, anchor: HTMLElement, mode: 'account' | 'manual' = 'account') {
    configureAnchor.current = anchor;
    resetSetup();
    setSetup({ clientId, mode });
    if (clientId === 'codex') void loadCodexExperimentalSettings();
    if (mode === 'account') void loadToolOptions(clientId);
  }

  async function loadCodexExperimentalSettings() {
    const request = ++codexSettingsRequest.current;
    if (!inWails()) return;
    try {
      const result = await GetClientConfiguration('codex', false);
      if (request !== codexSettingsRequest.current) return;
      const view = result as ClientConfigurationView;
      const config = view.files.find((file) => /(?:^|[\\/])config\.toml$/i.test(file.path));
      setCodexExperimentalSettings(readCodexExperimentalSettings(config?.content || ''));
    } catch {
      if (request === codexSettingsRequest.current) setCodexExperimentalSettings(defaultCodexExperimentalSettings());
    }
  }

  async function loadToolOptions(clientId: ClientId) {
    setSetupBusy('groups');
    setSetupMessage(null);
    try {
      const result = inWails() ? await GetAccountToolOptions(clientId) : mockToolOptions(clientId);
      const options = result as ToolOptionsResponse;
      setToolOptions(options);
      setSetupGroup(options.groups[0]?.name || '');
      const suggested = options.existingKeys?.[0];
      if (suggested) {
        setAccountKeySource('existing');
        setProvision(suggested);
        setSetupValidation({ clientId, models: suggested.models, status: suggested.status, endpoint: suggested.endpoint });
        setSetupModel(defaultModel(clientId, suggested.models, modelByClient[clientId]));
        setSetupMessage({ tone: 'success', text: `检测到已创建的 Key「${suggested.name || '已有 Key'}」，已为你选中` });
      } else {
        setAccountKeySource('new');
        setProvision(null);
        setSetupValidation(null);
        setSetupModel('');
      }
    } catch (error) {
      setSetupMessage({ tone: 'error', text: error instanceof Error ? error.message : '读取分组失败' });
    } finally {
      setSetupBusy('');
    }
  }

  function chooseExistingAccountKey(provisionId: string) {
    if (!setup || !toolOptions) return;
    const candidate = toolOptions.existingKeys?.find((item) => item.provisionId === provisionId);
    if (!candidate) return;
    setAccountKeySource('existing');
    setProvision(candidate);
    setSetupValidation({ clientId: setup.clientId, models: candidate.models, status: candidate.status, endpoint: candidate.endpoint });
    setSetupModel(defaultModel(setup.clientId, candidate.models, modelByClient[setup.clientId]));
    setSetupMessage({ tone: 'success', text: `已选择 Key「${candidate.name || '已有 Key'}」，请选择默认模型后配置` });
  }

  function chooseAccountKeySource(source: 'existing' | 'new') {
    if (source === 'existing') {
      const candidate = provision?.existing ? provision : toolOptions?.existingKeys?.[0];
      if (candidate) {
        chooseExistingAccountKey(candidate.provisionId);
        return;
      }
      return;
    }
    setAccountKeySource('new');
    setProvision(null);
    setSetupValidation(null);
    setSetupModel('');
    setSetupMessage(null);
  }

  function switchSetupMode(mode: 'account' | 'manual') {
    if (!setup) return;
    resetSetup();
    setSetup({ ...setup, mode });
    if (setup.clientId === 'codex') void loadCodexExperimentalSettings();
    if (mode === 'account') void loadToolOptions(setup.clientId);
  }

  async function createAccountKey() {
    if (!setup || !setupGroup) {
      setSetupMessage({ tone: 'error', text: '请选择分组' });
      return;
    }
    const selectedGroup = toolOptions?.groups.find((item) => item.name === setupGroup);
    const description = selectedGroup?.description ? `\n分组说明：${selectedGroup.description}` : '';
    const prompt = `将创建名为“自动配置创建”的新 Key，并限制为分组“${setupGroup}”支持的 ${clientCopy[setup.clientId].short} 模型。创建后可选择默认模型，再确认一键配置。${description}\n\n是否继续？`;
    if (!(await confirmAction({ title: '创建新的 Key', message: prompt, confirmLabel: '创建 Key' }))) return;
    setSetupBusy('key');
    setSetupMessage(null);
    setAccountKeySource('new');
    setProvision(null);
    setSetupValidation(null);
    try {
      const result = inWails()
        ? await CreateToolKey({ clientId: setup.clientId, group: setupGroup })
        : mockProvision(setup.clientId, setupGroup);
      const next = result as ToolKeyResult;
      setProvision(next);
      setSetupValidation({ clientId: setup.clientId, models: next.models, status: next.status, endpoint: next.endpoint });
      const selectedModel = defaultModel(setup.clientId, next.models, modelByClient[setup.clientId]);
      setSetupModel(selectedModel);
      setSetupMessage({ tone: 'success', text: '新的 Key 已创建，请选择默认模型后确认配置' });
    } catch (error) {
      setSetupMessage({ tone: 'error', text: error instanceof Error ? error.message : '创建 Key 失败' });
    } finally {
      setSetupBusy('');
    }
  }

  async function validateManualKey() {
    if (!setup || !setupKey.trim()) {
      setSetupMessage({ tone: 'error', text: '请输入 API Key' });
      return;
    }
    setSetupBusy('validate');
    setSetupMessage(null);
    try {
      const result = inWails()
        ? await ValidateToolKey({ clientId: setup.clientId, apiKey: setupKey.trim() })
        : mockToolValidation(setup.clientId);
      const next = result as ToolKeyValidationResult;
      setProvision(null);
      setSetupValidation(next);
      setSetupModel((current) => defaultModel(setup.clientId, next.models, current || modelByClient[setup.clientId]));
      setSetupMessage({ tone: 'success', text: 'Key 可用，请选择默认模型' });
    } catch (error) {
      setSetupValidation(null);
      setSetupMessage({ tone: 'error', text: error instanceof Error ? error.message : 'Key 检测失败' });
    } finally {
      setSetupBusy('');
    }
  }

  async function configureSelectedTool() {
    if (!setup || !setupValidation || !setupModel) {
      setSetupMessage({ tone: 'error', text: '请先检测 Key 并选择默认模型' });
      return;
    }
    if (!(await confirmAction({ title: '确认一键配置', message: `将备份 ${clientCopy[setup.clientId].short} 当前配置，并写入新配置。配置完成后会尝试自动重启该工具；无法重启时会提示你手动重启。是否继续？`, confirmLabel: '确认配置' }))) return;
    setSetupBusy('configure');
    setSetupMessage(null);
    try {
      const result = provision
        ? inWails()
          ? await ConfigureProvisionedTool(new main.ProvisionedToolConfigurationRequest({
            provisionId: provision.provisionId,
            clientId: setup.clientId,
            model: setupModel,
            ...(setup.clientId === 'codex' ? { codexExperimentalSettings } : {}),
          }))
          : mockConfigure(setup.clientId)
        : inWails()
          ? await ConfigureTool(new main.ToolConfigurationRequest({
            clientId: setup.clientId,
            apiKey: setupKey.trim(),
            model: setupModel,
            ...(setup.clientId === 'codex' ? { codexExperimentalSettings } : {}),
          }))
          : mockConfigure(setup.clientId);
      const configured = result as ConfigureResult;
      if (!configured.success) throw new Error(configured.error || '配置失败');
      const configuredClient = setup.clientId;
      const completionNotice = configurationRestartNotice(configured, configuredClient);
      setModelByClient((current) => ({ ...current, [configuredClient]: setupModel }));
      setSetup(null);
      resetSetup();
      showFeedback(completionNotice, completionNotice.tone === 'neutral' ? 6200 : 3600);
      await Promise.all([refreshBackups(), refreshEnvironment(false)]);
      await checkClient(configuredClient, configureAnchor.current);
    } catch (error) {
      setSetupMessage({ tone: 'error', text: error instanceof Error ? error.message : '配置失败，原文件未被覆盖' });
    } finally {
      setSetupBusy('');
    }
  }

  function openLogin(target?: ClientId) {
    setLoginTarget(target || null);
    setLoginMessage(null);
    setTwoFactorFlow('');
    setTwoFactorCode('');
    setLoginOpen(true);
  }

  async function finishAccountLogin(result: AccountLoginResult) {
    if (result.requiresTwoFactor) {
      setTwoFactorFlow(result.flowToken);
      setLoginMessage('请输入验证器代码或备用码');
      return;
    }
    const nextState = inWails()
      ? await GetAccountState()
      : { signedIn: true, username: result.username || loginUsername, expiresAt: result.expiresAt };
    setAccount(nextState as AccountState);
    if (inWails()) await refreshAccountState(false);
    if (inWails()) {
      try {
        if (rememberLogin) {
          await SaveAccountLogin({ username: loginUsername.trim(), password: loginPassword });
        } else {
          await DeleteSavedAccountLogin();
        }
      } catch (error) {
        showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '登录成功，但保存登录信息失败' }, 3600);
      }
    }
    if (!rememberLogin) setLoginPassword('');
    setLoginOpen(false);
    const target = loginTarget;
    setLoginTarget(null);
    if (target) {
      resetSetup();
      setSetup({ clientId: target, mode: 'account' });
      if (target === 'codex') void loadCodexExperimentalSettings();
      void loadToolOptions(target);
    }
  }

  async function submitLogin() {
    if (!loginUsername.trim() || !loginPassword) {
      setLoginMessage('请输入用户名或邮箱和密码');
      return;
    }
    setLoginBusy(true);
    setLoginMessage(null);
    try {
      const result = inWails()
        ? await LoginAccount({ username: loginUsername.trim(), password: loginPassword })
        : ({ signedIn: true, requiresTwoFactor: false, flowToken: '', username: loginUsername } as AccountLoginResult);
      await finishAccountLogin(result as AccountLoginResult);
    } catch (error) {
      setLoginMessage(error instanceof Error ? error.message : '登录失败');
    } finally {
      setLoginBusy(false);
    }
  }

  async function submitTwoFactor() {
    if (!twoFactorCode.trim()) {
      setLoginMessage('请输入两步验证代码');
      return;
    }
    setLoginBusy(true);
    setLoginMessage(null);
    try {
      const result = inWails()
        ? await VerifyAccountTwoFactor({ flowToken: twoFactorFlow, code: twoFactorCode.trim() })
        : ({ signedIn: true, requiresTwoFactor: false, flowToken: '', username: loginUsername } as AccountLoginResult);
      await finishAccountLogin(result as AccountLoginResult);
    } catch (error) {
      setLoginMessage(error instanceof Error ? error.message : '两步验证失败');
    } finally {
      setLoginBusy(false);
    }
  }

  async function logout() {
    if (inWails()) await LogoutAccount();
    setAccount({ signedIn: false, username: '', balance: '' });
    setToolOptions(null);
    showFeedback({ tone: 'neutral', text: '已退出本次应用会话' }, 2200);
  }

  async function fetchGroupRatios() {
    setBusy('groups');
    try {
      const result = inWails() ? await GetPublicGroupRatios() : mockGroupRatios();
      setGroupReport(result as GroupRatioReport);
    } catch (error) {
      showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '读取分组倍率失败' });
    } finally {
      setBusy('');
    }
  }

  async function restore(id: string) {
    setBusy('restore');
    try {
      if (inWails()) await RestoreBackup(id);
      await refreshEnvironment(false);
      showFeedback({ tone: 'success', text: '备份已恢复，请重启对应工具' }, 2800);
    } catch (error) {
      showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '恢复失败' });
    } finally {
      setBusy('');
    }
  }

  async function deleteBackup(id: string) {
    if (!(await confirmAction({ title: '删除配置备份', message: '确定永久删除此配置备份吗？此操作不能撤销。', confirmLabel: '确认删除', tone: 'danger' }))) return;
    setBusy('delete');
    try {
      if (inWails()) await DeleteBackup(id);
      await refreshBackups();
      showFeedback({ tone: 'success', text: '备份已删除' }, 2200);
    } catch (error) {
      showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '删除备份失败' });
    } finally {
      setBusy('');
    }
  }

  async function checkUpdate(promptToInstall = false, currentVersion = appInfo.version) {
    setUpdateProgress(null);
    setBusy('update');
    try {
      const result = inWails()
        ? await CheckForUpdates()
        : await fetchBrowserPreviewUpdate(currentVersion);
      const next = result as UpdateInfo;
      setUpdate(next);
      if (promptToInstall && next.updateAvailable && !next.error && await confirmAction({ title: '发现新版本', message: `发现新版本 v${next.latestVersion}。是否更新到最新版？`, confirmLabel: '更新到最新版' })) {
        await installUpdate(next, false);
      }
    } catch (error) {
      setUpdate({ currentVersion, latestVersion: '', updateAvailable: false, checkedAt: new Date().toISOString(), error: error instanceof Error ? error.message : '暂时无法检查更新' });
    } finally {
      setBusy('');
    }
  }

  async function installUpdate(currentUpdate = update, confirmInstall = true) {
    if (!currentUpdate?.downloadUrl) {
      showFeedback({ tone: 'error', text: '未找到可用更新包，请稍后重新检查' }, 3200);
      return;
    }
    if (confirmInstall && !(await confirmAction({ title: '安装更新', message: `将下载并安装 v${currentUpdate.latestVersion}，应用会自动关闭。是否继续？`, confirmLabel: '下载并安装' }))) return;
    setBusy('install-update');
    setUpdateProgress({ stage: 'checking', message: '正在检查可用更新', downloadedBytes: 0, totalBytes: 0, percent: 0 });
    try {
      if (!inWails()) {
        await openExternal(currentUpdate.downloadUrl);
        setUpdateProgress(null);
        return;
      }
      if (appInfo.platform === 'darwin') {
        await openExternal(currentUpdate.downloadUrl);
        setUpdateProgress(null);
        showFeedback({ tone: 'success', text: '已打开 macOS 安装包下载，请将应用拖入“应用程序”文件夹完成更新' }, 4200);
        return;
      }
      const result = await InstallLatestUpdate() as InstallUpdateResult;
      if (!result.success) throw new Error(result.error || '自动更新启动失败');
      showFeedback({ tone: 'success', text: result.message || '正在安装更新' });
    } catch (error) {
      const message = error instanceof Error ? error.message : '自动更新失败';
      setUpdateProgress({ stage: 'failed', message, downloadedBytes: 0, totalBytes: 0, percent: 0 });
      showFeedback({ tone: 'error', text: `${message}，可在版本更新页手动下载` }, 4600);
    } finally {
      setBusy('');
    }
  }

  async function openExternal(url: string) {
    if (inWails()) await OpenExternalURL(url);
    else window.open(url, '_blank', 'noopener,noreferrer');
  }

  function applyCustomWallpaper(wallpaper: string) {
    try {
      if (wallpaper) window.localStorage.setItem(customWallpaperStorageKey, wallpaper);
      else window.localStorage.removeItem(customWallpaperStorageKey);
    } catch {
      showFeedback({ tone: 'error', text: wallpaper ? '图片已应用，但本机存储空间不足，重启后可能需要重新选择' : '已恢复默认皮肤，但本机存储未能清理' }, 3600);
    }
    setCustomWallpaper(wallpaper);
    setTheme(wallpaper ? 'custom' : 'ciyuan');
  }

  function applyThemeTransparency(next: ThemeTransparency) {
    setThemeTransparency((current) => ({
      ...current,
      [theme]: {
        skin: clampTransparency(next.skin, defaultThemeTransparency.skin, 0),
        content: clampTransparency(next.content, defaultThemeTransparency.content, 15),
      },
    }));
  }

  function selectTab(nextTab: TabId) {
    setTab(nextTab);
    if (nextTab === 'groups') void fetchGroupRatios();
    if (nextTab === 'updates') void checkUpdate(false);
  }

  return (
    <div className="window-frame" data-theme={theme} data-theme-mode={resolvedThemeMode} data-theme-mode-preference={themeMode} data-has-wallpaper={wallpaper ? 'true' : 'false'} style={{ '--theme-wallpaper': wallpaper ? `url(${wallpaper})` : 'none', '--skin-opacity': `${activeThemeTransparency.skin}%`, '--content-opacity': `${activeThemeTransparency.content}%` } as CSSProperties}>
      {inWails() && <WindowTitlebar />}
      <div className="app-shell">
        <aside className="sidebar">
          <div className="brand-lockup">
            <div className="brand-mark"><img src={logo} alt="词元神" /></div>
            <div><strong>词元神</strong><span>配置助手</span></div>
          </div>
          <div className="sidebar-rule" />
          <nav className="side-nav" aria-label="主导航">
            <NavButton active={tab === 'overview'} icon={<ScanSearch size={17} />} label="一键配置" onClick={() => selectTab('overview')} />
            <NavButton active={false} icon={<BookOpen size={17} />} label="文档教程" onClick={() => void openExternal(documentationURL)} external />
            <NavButton active={tab === 'groups'} icon={<Layers3 size={17} />} label="分组倍率" onClick={() => selectTab('groups')} />
            <NavButton active={tab === 'backups'} icon={<RotateCcw size={17} />} label="配置备份" onClick={() => selectTab('backups')} count={backups.length || undefined} />
            <NavButton active={tab === 'appearance'} icon={<Palette size={17} />} label="外观皮肤" onClick={() => selectTab('appearance')} />
            <NavButton active={false} icon={<Globe size={17} />} label="进入官网" onClick={() => void openExternal(officialWebsiteURL)} external />
            <NavButton active={false} icon={<Users size={17} />} label="加入QQ群" onClick={() => void openExternal(qqGroupURL)} external />
            <NavButton active={tab === 'updates'} icon={<Download size={17} />} label="版本更新" onClick={() => selectTab('updates')} />
          </nav>
          <div className="sidebar-bottom">
            <div className="secure-note"><LockKeyhole size={16} /><span>账号会话与新 Key 仅保留本次运行；保存密码使用系统凭据管理器</span></div>
            <span className="version-label">v{appInfo.version}</span>
          </div>
        </aside>

        <main className="main-content">
          <header className="topbar">
            <div>
              <p className="eyebrow">WORKSPACE / LOCAL</p>
              <h1>{tabTitles[tab]}</h1>
            </div>
            <div className="topbar-actions">
              {account.signedIn ? <div className="account-summary"><UserRound size={16} /><div className="account-summary-copy"><span>{account.username || '已登录'}</span><b>余额 {account.balance || '读取中'}</b></div><button className="secondary-button compact account-recharge" onClick={() => void openExternal(walletURL)}><WalletCards size={15} />充值</button><button className="icon-button compact-icon" title="刷新余额" aria-label="刷新余额" onClick={() => void refreshAccountState()} disabled={accountRefreshing}><RefreshCw size={16} className={accountRefreshing ? 'spin' : ''} /></button><button className="icon-button compact-icon" title="退出登录" aria-label="退出登录" onClick={() => void logout()}><LogOut size={16} /></button></div> : <button className="secondary-button" onClick={() => openLogin()}><LogIn size={16} />登录词元神账号</button>}
              <button className="icon-button" title="重新检测环境" aria-label="重新检测环境" onClick={() => void refreshEnvironment()} disabled={busy === 'scan'}><RefreshCw size={17} className={busy === 'scan' ? 'spin' : ''} /></button>
            </div>
          </header>

          {feedback && <Feedback tone={feedback.tone} text={feedback.text} onClose={() => setFeedback(null)} />}
          {tab === 'overview' && <Overview environment={environment} clientMap={clientMap} toolModels={toolModels} modelByClient={modelByClient} modelsLoading={modelsLoading} modelErrors={modelErrors} keyValidationResults={keyValidationResults} setClientModel={(clientId, model, anchor) => void applyExistingModel(clientId, model, anchor)} connectionResults={connectionResults} checkingClient={checkingClient} applyingModelClient={applyingModelClient} lifecycleByClient={lifecycleByClient} lifecycleBusyClient={lifecycleBusyClient} onCheck={checkClient} onConfigure={openToolSetup} onViewConfiguration={openClientConfiguration} onLifecycleCheck={checkToolLifecycle} onLifecycleAction={runToolLifecycleAction} />}
          {tab === 'groups' && <GroupRatios report={groupReport} busy={busy} refresh={() => void fetchGroupRatios()} />}
          {tab === 'backups' && <Backups backups={backups} backupRoot={backupRoot} busy={busy} restore={restore} remove={deleteBackup} refresh={() => void refreshBackups()} />}
          {tab === 'updates' && <Updates update={update} progress={updateProgress} platform={appInfo.platform} busy={busy} check={() => void checkUpdate(false)} install={() => void installUpdate()} openDownload={() => update?.downloadUrl && void openExternal(update.downloadUrl)} />}
          {tab === 'appearance' && <ThemeGallery theme={theme} themeMode={themeMode} themes={availableThemes} customWallpaper={customWallpaper} transparency={activeThemeTransparency} onThemeChange={setTheme} onThemeModeChange={setThemeMode} onTransparencyChange={applyThemeTransparency} onCustomWallpaperChange={applyCustomWallpaper} onConfirm={confirmAction} onOpenSource={(url) => void openExternal(url)} />}
        </main>
      </div>

      {actionNotice && <ActionNotice {...actionNotice} />}
      {setup && <ToolSetupModal setup={setup} account={account} options={toolOptions} group={setupGroup} keyValue={setupKey} showKey={showSetupKey} validation={setupValidation} provision={provision} accountKeySource={accountKeySource} model={setupModel} codexExperimentalSettings={codexExperimentalSettings} busy={setupBusy} message={setupMessage} onClose={() => { setSetup(null); resetSetup(); }} onModeChange={switchSetupMode} onGroupChange={(value) => { setAccountKeySource('new'); setSetupGroup(value); setSetupValidation(null); setProvision(null); setSetupModel(''); setSetupMessage(null); }} onKeyChange={(value) => { setSetupKey(value); setSetupValidation(null); setProvision(null); setSetupModel(''); setSetupMessage(null); }} onExistingKeyChange={chooseExistingAccountKey} onAccountKeySourceChange={chooseAccountKeySource} onShowKey={() => setShowSetupKey((current) => !current)} onModelChange={setSetupModel} onCodexExperimentalSettingsChange={(patch) => setCodexExperimentalSettings((current) => ({ ...current, ...patch }))} onCreateKey={() => void createAccountKey()} onValidateKey={() => void validateManualKey()} onConfigure={() => void configureSelectedTool()} onLogin={() => openLogin(setup.clientId)} onReloadGroups={() => void loadToolOptions(setup.clientId)} />}
      {configurationClient && <ConfigurationViewerModal clientId={configurationClient} view={configurationView} busy={configurationBusy} error={configurationError} revealSecrets={revealConfigurationSecrets} onClose={() => { setConfigurationClient(null); setConfigurationView(null); setConfigurationError(null); setRevealConfigurationSecrets(false); }} onReload={() => void loadClientConfiguration(configurationClient, revealConfigurationSecrets)} onToggleSecrets={toggleConfigurationSecrets} />}
      {loginOpen && <AccountLoginModal username={loginUsername} password={loginPassword} rememberLogin={rememberLogin} code={twoFactorCode} requiresTwoFactor={Boolean(twoFactorFlow)} showPassword={showLoginPassword} busy={loginBusy} message={loginMessage} onUsername={setLoginUsername} onPassword={setLoginPassword} onRememberLogin={setRememberLogin} onCode={setTwoFactorCode} onTogglePassword={() => setShowLoginPassword((current) => !current)} onClose={() => { setLoginOpen(false); setLoginMessage(null); setTwoFactorFlow(''); }} onSubmit={() => void (twoFactorFlow ? submitTwoFactor() : submitLogin())} onRegister={() => void openExternal(signUpURL)} onForgotPassword={() => void openExternal(forgotPasswordURL)} />}
      {confirmation && <ConfirmModal title={confirmation.title} message={confirmation.message} confirmLabel={confirmation.confirmLabel} tone={confirmation.tone} onCancel={() => settleConfirmation(false)} onConfirm={() => settleConfirmation(true)} />}
    </div>
  );
}

function WindowTitlebar() {
  const dragStyle = { '--wails-draggable': 'drag' } as CSSProperties;
  return <header className="window-titlebar" style={dragStyle}>
    <div className="window-title"><img src={logo} alt="" /><span>词元神配置助手</span></div>
    <div className="window-controls" style={{ '--wails-draggable': 'no-drag' } as CSSProperties}>
      <button className="window-control" title="最小化" aria-label="最小化" onClick={WindowMinimise}><Minus size={16} /></button>
      <button className="window-control" title="最大化或还原" aria-label="最大化或还原" onClick={WindowToggleMaximise}><Maximize2 size={15} /></button>
      <button className="window-control close" title="关闭" aria-label="关闭" onClick={Quit}><X size={17} /></button>
    </div>
  </header>;
}

function ConfirmModal({ title, message, confirmLabel = '继续', tone = 'default', onCancel, onConfirm }: {
  title: string;
  message: string;
  confirmLabel?: string;
  tone?: ConfirmationTone;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const confirmButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCancel();
    };
    window.addEventListener('keydown', handleKeyDown);
    const frame = window.requestAnimationFrame(() => confirmButton.current?.focus());
    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [onCancel]);
  return <div className="modal-backdrop confirm-modal-backdrop" role="presentation" onMouseDown={onCancel}>
    <section className={`confirm-modal ${tone}`} role="dialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-message" onMouseDown={(event) => event.stopPropagation()}>
      <div className={`confirm-modal-icon ${tone}`}>{tone === 'danger' ? <AlertTriangle size={20} /> : <ClipboardCheck size={20} />}</div>
      <div className="confirm-modal-copy"><h2 id="confirm-title">{title}</h2><p id="confirm-message">{message}</p></div>
      <div className="modal-actions confirm-modal-actions"><button type="button" className="secondary-button" onClick={onCancel}>取消</button><button ref={confirmButton} type="button" className={tone === 'danger' ? 'danger-confirm-button' : 'primary-button'} onClick={onConfirm}>{confirmLabel}</button></div>
    </section>
  </div>;
}

function NavButton({ active, icon, label, count, onClick, external = false }: { active: boolean; icon: ReactNode; label: string; count?: number; onClick: () => void; external?: boolean }) {
  return <button className={`nav-button ${active ? 'active' : ''}`} onClick={onClick}>{icon}<span>{label}</span>{count ? <b>{count}</b> : external ? <ArrowUpRight size={15} className="nav-arrow" /> : <ChevronRight size={15} className="nav-arrow" />}</button>;
}

function Overview({ environment, clientMap, toolModels, modelByClient, modelsLoading, modelErrors, keyValidationResults, setClientModel, connectionResults, checkingClient, applyingModelClient, lifecycleByClient, lifecycleBusyClient, onCheck, onConfigure, onViewConfiguration, onLifecycleCheck, onLifecycleAction }: {
  environment: EnvironmentReport;
  clientMap: Map<ClientId, ClientStatus>;
  toolModels: Partial<Record<ClientId, Model[]>>;
  modelByClient: Record<ClientId, string>;
  modelsLoading: Partial<Record<ClientId, boolean>>;
  modelErrors: Partial<Record<ClientId, string>>;
  keyValidationResults: Partial<Record<ClientId, ToolKeyValidationResult>>;
  setClientModel: (clientId: ClientId, model: string, anchor: HTMLElement) => void;
  connectionResults: Partial<Record<ClientId, ClientConnectionResult>>;
  checkingClient: ClientId | null;
  applyingModelClient: ClientId | null;
  lifecycleByClient: Partial<Record<ClientId, ToolLifecycleInfo>>;
  lifecycleBusyClient: ClientId | null;
  onCheck: (clientId: ClientId, anchor: HTMLElement) => void;
  onConfigure: (clientId: ClientId, anchor: HTMLElement) => void;
  onViewConfiguration: (clientId: ClientId) => void;
  onLifecycleCheck: (clientId: ClientId, anchor?: HTMLElement) => void;
  onLifecycleAction: (clientId: ClientId, action: 'install' | 'update' | 'download', anchor: HTMLElement) => void;
}) {
  const configuredCount = environment.clients.filter((client) => {
    const result = connectionResults[client.id];
    return result ? result.success : client.configState === 'valid';
  }).length;
  return <div className="content-stack">
    <section className="summary-band">
      <div className="summary-copy"><div className="section-icon green"><ShieldCheck size={20} /></div><div><h2>配置状态</h2><p>已验证 {configuredCount} / {environment.clients.length} 个工具配置</p></div></div>
      <div className="summary-meta"><span className="meta-label">最近检测</span><strong>{formatTime(environment.scannedAt)}</strong><span className="platform-label"><Laptop size={14} /> {environment.os}</span></div>
    </section>
    <section className="clients-section">
      <div className="section-heading"><div><p className="eyebrow">TOOLS</p><h2>选择要配置的工具</h2></div></div>
      <div className="client-grid">
        {clientOrder.map((clientId) => <ClientCard key={clientId} clientId={clientId} status={clientMap.get(clientId)} models={toolModels[clientId] || []} model={modelByClient[clientId]} modelsLoading={Boolean(modelsLoading[clientId])} modelError={modelErrors[clientId]} keyValidated={Boolean(keyValidationResults[clientId])} onModelChange={(model, anchor) => setClientModel(clientId, model, anchor)} result={connectionResults[clientId]} checking={checkingClient === clientId} applying={applyingModelClient === clientId} lifecycle={lifecycleByClient[clientId]} lifecycleBusy={lifecycleBusyClient === clientId} onCheck={onCheck} onConfigure={onConfigure} onViewConfiguration={onViewConfiguration} onLifecycleCheck={onLifecycleCheck} onLifecycleAction={onLifecycleAction} />)}
      </div>
    </section>
  </div>;
}

function ClientCard({ clientId, status, models, model, modelsLoading, modelError, keyValidated, onModelChange, result, checking, applying, lifecycle, lifecycleBusy, onCheck, onConfigure, onViewConfiguration, onLifecycleCheck, onLifecycleAction }: {
  clientId: ClientId;
  status?: ClientStatus;
  models: Model[];
  model: string;
  modelsLoading: boolean;
  modelError?: string;
  keyValidated: boolean;
  onModelChange: (value: string, anchor: HTMLElement) => void;
  result?: ClientConnectionResult;
  checking: boolean;
  applying: boolean;
  lifecycle?: ToolLifecycleInfo;
  lifecycleBusy: boolean;
  onCheck: (clientId: ClientId, anchor: HTMLElement) => void;
  onConfigure: (clientId: ClientId, anchor: HTMLElement) => void;
  onViewConfiguration: (clientId: ClientId) => void;
  onLifecycleCheck: (clientId: ClientId, anchor?: HTMLElement) => void;
  onLifecycleAction: (clientId: ClientId, action: 'install' | 'update' | 'download', anchor: HTMLElement) => void;
}) {
  const unsupported = status?.supported === false;
  const installed = Boolean(status?.installed || lifecycle?.installed);
  const keyKnown = keyValidated || Boolean(modelError);
  const state = unsupported ? 'unsupported' : installed ? 'available' : 'not-found';
  const lifecycleAction = !lifecycle
    ? 'check'
    : !lifecycle.installed
      ? lifecycle.canInstall ? 'install' : lifecycle.downloadUrl ? 'download' : 'check'
      : lifecycle.updateAvailable
        ? lifecycle.canUpdate ? 'update' : lifecycle.downloadUrl ? 'download' : 'check'
        : 'check';
  const lifecycleLabel = lifecycleAction === 'install' ? '安装' : lifecycleAction === 'update' ? '更新' : lifecycleAction === 'download' ? '官方下载' : '检查更新';
  const lifecycleIcon = lifecycleAction === 'download' ? <ArrowUpRight size={14} /> : lifecycleAction === 'check' ? <RefreshCw size={14} /> : lifecycleAction === 'install' ? <HardDriveDownload size={14} /> : <PackageCheck size={14} />;
  const lifecycleText = lifecycle ? lifecycleVersionSummary(lifecycle) : '';
  return <article className="client-card">
    <div className="client-card-top"><div className="client-symbol"><img src={clientLogos[clientId]} alt="" /></div><div className="client-name"><strong>{clientCopy[clientId].short}</strong><span>{clientCopy[clientId].badge}</span></div><div className="client-status-tags"><span className={`state-tag ${state}`}>{state === 'available' ? '已安装' : state === 'unsupported' ? '不支持' : '未安装'}</span><span className={`key-state-tag ${keyValidated ? 'success' : keyKnown ? 'error' : ''}`}>{keyValidated ? 'Key正常' : keyKnown ? 'Key异常' : 'Key未检测'}</span></div></div>
    <div className="client-card-path">{status?.configPath || '配置文件将自动创建'}{status?.version && <span className="client-version"> · {status.version}</span>}</div>
    <div className="client-card-bottom">
      <label>当前 Key 可用模型</label>
      {modelsLoading ? <span className="model-empty"><RefreshCw size={13} className="spin" />正在读取 Key 可用模型</span> : models.length > 0 ? <select value={models.some((item) => item.id === model) ? model : ''} onChange={(event) => onModelChange(event.target.value, event.currentTarget)} disabled={!keyValidated || applying || unsupported}>{models.map((option) => <option key={option.id} value={option.id}>{option.id}</option>)}</select> : <span className={`model-empty ${modelError ? 'error' : ''}`} title={modelError}>{modelError ? `读取失败：${modelError}` : keyValidated ? 'Key 未返回可用模型' : '尚未检测到可用 Key'}</span>}
    </div>
    <div className="client-card-actions">
      <span className={`check-result ${result ? (result.success ? 'success' : 'error') : ''}`}>{result ? (result.success ? <><CheckCircle2 size={14} />已通过</> : <><AlertTriangle size={14} />未通过</>) : <><CircleDashed size={14} />未检测</>}</span>
      <div className="client-action-row"><button className="icon-button compact-icon" title="查看配置文件" aria-label="查看配置文件" onClick={() => onViewConfiguration(clientId)}><FileText size={15} /></button><button className="secondary-button compact" onClick={(event) => onCheck(clientId, event.currentTarget)} disabled={checking || unsupported}><Activity size={14} />{checking ? '检测中' : '检测'}</button><button className="secondary-button compact" onClick={(event) => lifecycleAction === 'check' ? onLifecycleCheck(clientId, event.currentTarget) : onLifecycleAction(clientId, lifecycleAction, event.currentTarget)} disabled={lifecycleBusy || unsupported}>{lifecycleBusy ? <RefreshCw size={14} className="spin" /> : lifecycleIcon}{lifecycleBusy ? '处理中' : lifecycleLabel}</button><button className="primary-button compact" onClick={(event) => onConfigure(clientId, event.currentTarget)} disabled={checking || unsupported}><Settings2 size={14} />一键配置</button></div>
    </div>
    {lifecycle && <div className={`lifecycle-note ${lifecycle.error ? 'error' : ''}`} title={lifecycleText}>{lifecycleText}</div>}
  </article>;
}

function ToolSetupModal({ setup, account, options, group, keyValue, showKey, validation, provision, accountKeySource, model, codexExperimentalSettings, busy, message, onClose, onModeChange, onGroupChange, onKeyChange, onExistingKeyChange, onAccountKeySourceChange, onShowKey, onModelChange, onCodexExperimentalSettingsChange, onCreateKey, onValidateKey, onConfigure, onLogin, onReloadGroups }: {
  setup: SetupState;
  account: AccountState;
  options: ToolOptionsResponse | null;
  group: string;
  keyValue: string;
  showKey: boolean;
  validation: ToolKeyValidationResult | null;
  provision: ToolKeyResult | null;
  accountKeySource: 'existing' | 'new';
  model: string;
  codexExperimentalSettings: CodexExperimentalSettings;
  busy: string;
  message: { tone: NoticeTone; text: string } | null;
  onClose: () => void;
  onModeChange: (mode: 'account' | 'manual') => void;
  onGroupChange: (value: string) => void;
  onKeyChange: (value: string) => void;
  onExistingKeyChange: (provisionId: string) => void;
  onAccountKeySourceChange: (source: 'existing' | 'new') => void;
  onShowKey: () => void;
  onModelChange: (value: string) => void;
  onCodexExperimentalSettingsChange: (patch: Partial<CodexExperimentalSettings>) => void;
  onCreateKey: () => void;
  onValidateKey: () => void;
  onConfigure: () => void;
  onLogin: () => void;
  onReloadGroups: () => void;
}) {
  const client = clientCopy[setup.clientId];
  const selectedGroup = options?.groups.find((item) => item.name === group);
  const hasExistingKeys = Boolean(options?.existingKeys?.length);
  const selectedExistingKey = accountKeySource === 'existing' ? provision?.existing ? provision : options?.existingKeys?.[0] : undefined;
  const keyReady = Boolean(validation && validation.models.length > 0);
  return <div className="modal-backdrop" role="presentation"><section className="setup-modal" role="dialog" aria-modal="true" aria-labelledby="setup-title">
    <div className="modal-heading"><div><p className="eyebrow">{client.badge}</p><h2 id="setup-title">配置 {client.short}</h2></div><button className="icon-button" title="关闭" aria-label="关闭" onClick={onClose}><X size={18} /></button></div>
    <div className="mode-switch" role="tablist" aria-label="Key 来源">
      <button className={setup.mode === 'account' ? 'active' : ''} role="tab" aria-selected={setup.mode === 'account'} onClick={() => onModeChange('account')}><UserRound size={15} />词元神账号</button>
      <button className={setup.mode === 'manual' ? 'active' : ''} role="tab" aria-selected={setup.mode === 'manual'} onClick={() => onModeChange('manual')}><KeyRound size={15} />手动输入 Key</button>
    </div>
    {setup.mode === 'account' ? <div className="setup-flow account-setup-flow">
      {!account.signedIn ? <div className="setup-empty"><UserRound size={21} /><strong>尚未登录词元神账号</strong><button className="secondary-button" onClick={onLogin}><LogIn size={16} />登录账号</button></div> : <>
        <section className={`account-key-path ${accountKeySource === 'existing' ? 'selected' : ''}`} aria-labelledby="existing-key-heading">
          <div className="account-key-path-heading"><div><h3 id="existing-key-heading">使用已创建的 Key</h3><p>选择已有 Key 后，直接在下方选择模型并确认配置。</p></div>{hasExistingKeys && <button className="path-select-button" type="button" onClick={() => onAccountKeySourceChange('existing')} disabled={busy === 'groups' || busy === 'key'}>{accountKeySource === 'existing' ? '当前选择' : '使用此方式'}</button>}</div>
          {hasExistingKeys ? <div className="field-block"><label htmlFor="existing-key-select">已创建的 Key</label><select id="existing-key-select" value={selectedExistingKey?.provisionId || ''} onChange={(event) => onExistingKeyChange(event.target.value)} disabled={busy === 'groups' || busy === 'key'}>{options?.existingKeys?.map((item) => <option key={item.provisionId} value={item.provisionId}>{item.name || '已有 Key'}{item.group ? ` · ${item.group}` : ''} · {item.models.length} 个可用模型</option>)}</select>{selectedExistingKey && <div className="existing-key-details"><span>分组：{selectedExistingKey.group || '未设置分组'}</span><span>分组说明：{selectedExistingKey.groupDescription || '暂无分组说明'}</span></div>}</div> : <p className="account-key-path-empty">当前账号没有适用于 {client.short} 的已创建 Key。</p>}
        </section>
        <section className={`account-key-path account-key-path-new ${accountKeySource === 'new' ? 'selected' : ''}`} aria-labelledby="new-key-heading">
          <div className="account-key-path-heading"><div><h3 id="new-key-heading">新建 Key</h3><p>按所选分组创建一个仅限该工具可用模型的新 Key。</p></div><button className="path-select-button" type="button" onClick={() => onAccountKeySourceChange('new')} disabled={busy === 'groups' || busy === 'key'}>{accountKeySource === 'new' ? '当前选择' : '使用此方式'}</button></div>
          <div className="field-block"><label htmlFor="group-select">新建 Key 分组</label><div className="select-row"><select id="group-select" value={group} onChange={(event) => onGroupChange(event.target.value)} disabled={busy === 'groups' || busy === 'key'}>{options?.groups.map((item) => <option key={item.name} value={item.name}>{item.name} · {item.ratio ? `${item.ratio}x` : '倍率待定'} · {item.models.length} 个模型</option>)}</select><button className="icon-button" title="刷新分组" aria-label="刷新分组" onClick={onReloadGroups} disabled={busy === 'groups' || busy === 'key'}><RefreshCw size={16} className={busy === 'groups' ? 'spin' : ''} /></button></div><span className="field-note">{selectedGroup?.description || '暂无分组说明'}</span></div>
          <button className="primary-button full-width" onClick={onCreateKey} disabled={busy === 'groups' || busy === 'key' || !group || accountKeySource !== 'new'}><ShieldCheck size={17} />{busy === 'key' ? '正在创建 Key' : '创建新的 Key'}</button>
        </section>
      </>}
    </div> : <div className="setup-flow">
      <div className="field-block"><label htmlFor="manual-key">API Key</label><div className="key-input-wrap"><KeyRound size={17} /><input id="manual-key" type={showKey ? 'text' : 'password'} value={keyValue} placeholder="粘贴 API Key" autoComplete="off" onChange={(event) => onKeyChange(event.target.value)} /><button className="input-action" title={showKey ? '隐藏 Key' : '显示 Key'} aria-label={showKey ? '隐藏 Key' : '显示 Key'} onClick={onShowKey}>{showKey ? <EyeOff size={17} /> : <Eye size={17} />}</button></div></div>
      {!keyReady && <button className="primary-button full-width" onClick={onValidateKey} disabled={busy === 'validate'}><Activity size={17} />{busy === 'validate' ? '检测中' : '检测 Key'}</button>}
    </div>}
    {setup.clientId === 'codex' && <CodexExperimentalSettingsPanel settings={codexExperimentalSettings} onChange={onCodexExperimentalSettingsChange} />}
    {message && <div className={`setup-status ${message.tone}`}><span>{message.tone === 'success' ? <CheckCircle2 size={16} /> : message.tone === 'error' ? <AlertTriangle size={16} /> : <CircleDashed size={16} />}</span>{message.text}</div>}
    {validation && validation.models.length > 0 && <div className="model-step"><div className="field-block"><label htmlFor="default-model">默认模型</label><select id="default-model" value={model} onChange={(event) => onModelChange(event.target.value)}>{validation.models.map((option) => <option key={option.id} value={option.id}>{option.id}</option>)}</select></div><div className="model-step-footer"><span>{provision?.existing ? `使用账号已有 Key${provision.name ? `「${provision.name}」` : ''}` : provision ? '新建 Key 已限制为该工具可用模型' : '仅使用当前输入的 Key 完成本次配置'}</span><button className="primary-button" onClick={onConfigure} disabled={busy === 'configure' || !model}><ClipboardCheck size={17} />{busy === 'configure' ? '备份并配置中' : '确认并一键配置'}</button></div></div>}
  </section></div>;
}

function CodexExperimentalSettingsPanel({ settings, onChange }: { settings: CodexExperimentalSettings; onChange: (patch: Partial<CodexExperimentalSettings>) => void }) {
  return <fieldset className="codex-settings">
    <legend>Codex 选项</legend>
    <label className="codex-settings-option"><input type="checkbox" checked={settings.contextManagementExperimentalMode} onChange={(event) => onChange({ contextManagementExperimentalMode: event.target.checked })} /><span>启用实验性上下文管理</span></label>
    <label className="codex-settings-option"><input type="checkbox" checked={settings.tokenBudgetEnabled} onChange={(event) => onChange({ tokenBudgetEnabled: event.target.checked })} /><span>启用 Token 预算</span></label>
    <label className="codex-settings-option"><input type="checkbox" checked={settings.tokenBudgetUseHistoryNotesExtension} onChange={(event) => onChange({ tokenBudgetUseHistoryNotesExtension: event.target.checked })} /><span>使用历史笔记扩展</span></label>
  </fieldset>;
}

function ConfigurationViewerModal({ clientId, view, busy, error, revealSecrets, onClose, onReload, onToggleSecrets }: {
  clientId: ClientId;
  view: ClientConfigurationView | null;
  busy: boolean;
  error: string | null;
  revealSecrets: boolean;
  onClose: () => void;
  onReload: () => void;
  onToggleSecrets: () => void;
}) {
  const [selectedPath, setSelectedPath] = useState('');
  const files = view?.files || [];
  const selected = files.find((file) => file.path === selectedPath) || files[0];
  return <div className="modal-backdrop" role="presentation"><section className="configuration-modal" role="dialog" aria-modal="true" aria-labelledby="configuration-title">
    <div className="modal-heading"><div><p className="eyebrow">LOCAL CONFIGURATION</p><h2 id="configuration-title">{view?.clientName || clientCopy[clientId].short} 配置文件</h2></div><div className="modal-heading-actions"><button className="icon-button" title="刷新配置文件" aria-label="刷新配置文件" onClick={onReload} disabled={busy}><RefreshCw size={17} className={busy ? 'spin' : ''} /></button><button className="icon-button" title="关闭" aria-label="关闭" onClick={onClose}><X size={18} /></button></div></div>
    <div className="configuration-toolbar"><span>{revealSecrets ? '敏感信息正在以明文显示' : '敏感信息已隐藏'}</span><button className="secondary-button compact" onClick={onToggleSecrets}>{revealSecrets ? <EyeOff size={15} /> : <Eye size={15} />}{revealSecrets ? '隐藏敏感信息' : '显示敏感信息'}</button></div>
    {busy ? <div className="configuration-loading"><CircleDashed size={20} className="spin" /><span>正在读取本机配置文件</span></div> : error ? <div className="configuration-error"><AlertTriangle size={18} /><span>{error}</span></div> : files.length === 0 ? <div className="configuration-loading"><FileText size={20} /><span>未找到可查看的配置文件</span></div> : <div className="configuration-body"><div className="configuration-tabs" role="tablist" aria-label="配置文件列表">{files.map((file) => <button key={file.path} className={(selected?.path === file.path) ? 'active' : ''} role="tab" aria-selected={selected?.path === file.path} title={file.path} onClick={() => setSelectedPath(file.path)}>{configurationFileName(file.path)}{file.exists ? null : <em>未创建</em>}</button>)}</div><div className="configuration-content"><code>{selected?.path}</code><pre>{selected?.exists ? selected.content : '配置文件尚未创建。完成一键配置后可在这里查看。'}</pre></div></div>}
  </section></div>;
}

function configurationFileName(path: string) {
  const segments = path.split(/[\\/]/).filter(Boolean);
  return segments[segments.length - 1] || path;
}

async function writeClipboardText(value: string) {
  if (inWails()) {
    const copied = await ClipboardSetText(value);
    if (!copied) throw new Error('无法写入系统剪贴板');
    return;
  }
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = value;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  const copied = document.execCommand('copy');
  document.body.removeChild(textarea);
  if (!copied) throw new Error('无法写入系统剪贴板');
}

async function readClipboardText() {
  if (inWails()) return ClipboardGetText();
  if (navigator.clipboard?.readText) return navigator.clipboard.readText();
  throw new Error('当前环境无法读取系统剪贴板');
}

function AccountLoginModal({ username, password, rememberLogin, code, requiresTwoFactor, showPassword, busy, message, onUsername, onPassword, onRememberLogin, onCode, onTogglePassword, onClose, onSubmit, onRegister, onForgotPassword }: {
  username: string;
  password: string;
  rememberLogin: boolean;
  code: string;
  requiresTwoFactor: boolean;
  showPassword: boolean;
  busy: boolean;
  message: string | null;
  onUsername: (value: string) => void;
  onPassword: (value: string) => void;
  onRememberLogin: (value: boolean) => void;
  onCode: (value: string) => void;
  onTogglePassword: () => void;
  onClose: () => void;
  onSubmit: () => void;
  onRegister: () => void;
  onForgotPassword: () => void;
}) {
  const usernameInput = useRef<HTMLInputElement>(null);
  const passwordInput = useRef<HTMLInputElement>(null);
  const contextMenuRef = useRef<HTMLDivElement>(null);
  const [contextMenu, setContextMenu] = useState<LoginContextMenuState | null>(null);

  useEffect(() => {
    if (!contextMenu) return;
    const dismissMenu = (event: MouseEvent) => {
      if (!contextMenuRef.current?.contains(event.target as Node)) setContextMenu(null);
    };
    const dismissOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setContextMenu(null);
    };
    window.addEventListener('mousedown', dismissMenu);
    window.addEventListener('keydown', dismissOnEscape);
    return () => {
      window.removeEventListener('mousedown', dismissMenu);
      window.removeEventListener('keydown', dismissOnEscape);
    };
  }, [contextMenu]);

  function inputValue(kind: LoginInputKind) {
    return kind === 'username' ? username : password;
  }

  function inputRef(kind: LoginInputKind) {
    return kind === 'username' ? usernameInput.current : passwordInput.current;
  }

  function replaceSelection(kind: LoginInputKind, start: number, end: number, replacement: string) {
    const current = inputValue(kind);
    const next = current.slice(0, start) + replacement + current.slice(end);
    if (kind === 'username') onUsername(next);
    else onPassword(next);
    const caret = start + replacement.length;
    window.requestAnimationFrame(() => {
      const input = inputRef(kind);
      input?.focus();
      input?.setSelectionRange(caret, caret);
    });
  }

  function openContextMenu(kind: LoginInputKind, event: ReactMouseEvent<HTMLInputElement>) {
    event.preventDefault();
    const input = event.currentTarget;
    const start = input.selectionStart ?? input.value.length;
    const end = input.selectionEnd ?? start;
    const menuWidth = 148;
    const menuHeight = 156;
    setContextMenu({
      kind,
      left: Math.max(8, Math.min(window.innerWidth - menuWidth - 8, event.clientX)),
      top: Math.max(8, Math.min(window.innerHeight - menuHeight - 8, event.clientY)),
      start,
      end,
      hasSelection: start !== end,
    });
  }

  async function copySelection() {
    if (!contextMenu || !contextMenu.hasSelection) return;
    try {
      await writeClipboardText(inputValue(contextMenu.kind).slice(contextMenu.start, contextMenu.end));
    } catch {
      // Keep the login form usable when the host denies clipboard access.
    } finally {
      setContextMenu(null);
    }
  }

  async function cutSelection() {
    if (!contextMenu || !contextMenu.hasSelection) return;
    try {
      await writeClipboardText(inputValue(contextMenu.kind).slice(contextMenu.start, contextMenu.end));
      replaceSelection(contextMenu.kind, contextMenu.start, contextMenu.end, '');
    } catch {
      // Do not alter the input unless copying the selected text succeeded.
    } finally {
      setContextMenu(null);
    }
  }

  async function pasteClipboard() {
    if (!contextMenu) return;
    try {
      const pasted = await readClipboardText();
      replaceSelection(contextMenu.kind, contextMenu.start, contextMenu.end, pasted);
    } catch {
      // Keep the current input unchanged when clipboard access is unavailable.
    } finally {
      setContextMenu(null);
    }
  }

  function selectAll() {
    if (!contextMenu) return;
    const input = inputRef(contextMenu.kind);
    input?.focus();
    input?.select();
    setContextMenu(null);
  }

  return <div className="modal-backdrop" role="presentation"><section className="login-modal" role="dialog" aria-modal="true" aria-labelledby="login-title">
    <div className="modal-heading"><div><p className="eyebrow">CIYUANSHEN ACCOUNT</p><h2 id="login-title">登录词元神</h2></div><button className="icon-button" title="关闭" aria-label="关闭" onClick={onClose}><X size={18} /></button></div>
    {requiresTwoFactor ? <div className="login-fields"><div className="field-block"><label htmlFor="two-factor-code">两步验证代码</label><input id="two-factor-code" inputMode="numeric" autoComplete="one-time-code" value={code} onChange={(event) => onCode(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') onSubmit(); }} /></div></div> : <div className="login-fields"><div className="field-block"><label htmlFor="account-username">用户名或邮箱</label><input ref={usernameInput} id="account-username" type="text" inputMode="email" autoComplete="username" placeholder="输入用户名或邮箱" value={username} onChange={(event) => onUsername(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') onSubmit(); }} onContextMenu={(event) => openContextMenu('username', event)} /></div><div className="field-block"><label htmlFor="account-password">密码</label><div className="key-input-wrap"><LockKeyhole size={17} /><input ref={passwordInput} id="account-password" type={showPassword ? 'text' : 'password'} autoComplete="current-password" placeholder="输入密码" value={password} onChange={(event) => onPassword(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') onSubmit(); }} onContextMenu={(event) => openContextMenu('password', event)} /><button className="input-action" title={showPassword ? '隐藏密码' : '显示密码'} aria-label={showPassword ? '隐藏密码' : '显示密码'} onClick={onTogglePassword}>{showPassword ? <EyeOff size={17} /> : <Eye size={17} />}</button></div></div></div>}
    {!requiresTwoFactor && <><label className="remember-login"><input type="checkbox" checked={rememberLogin} onChange={(event) => onRememberLogin(event.target.checked)} />保存密码（仅保存在系统凭据管理器）</label><div className="login-links"><div className="login-register"><span>还没有注册？</span><button type="button" onClick={onRegister}>去注册</button></div><button className="login-link" type="button" onClick={onForgotPassword}>忘记密码</button></div></>}
    {message && <div className="setup-status neutral"><CircleDashed size={16} />{message}</div>}
    <div className="modal-actions"><button className="secondary-button" onClick={onClose}>取消</button><button className="primary-button" onClick={onSubmit} disabled={busy}><LogIn size={17} />{busy ? '登录中' : requiresTwoFactor ? '验证并登录' : '登录'}</button></div>
  </section>{contextMenu && <div ref={contextMenuRef} className="login-context-menu" role="menu" aria-label="登录输入操作" style={{ left: contextMenu.left, top: contextMenu.top }} onContextMenu={(event) => event.preventDefault()}><button type="button" role="menuitem" disabled={!contextMenu.hasSelection} onClick={() => void copySelection()}>复制</button><button type="button" role="menuitem" disabled={!contextMenu.hasSelection} onClick={() => void cutSelection()}>剪切</button><button type="button" role="menuitem" onClick={() => void pasteClipboard()}>粘贴</button><button type="button" role="menuitem" onClick={selectAll}>全选</button></div>}</div>;
}

function ActionNotice({ tone, text, left, top, below }: { tone: NoticeTone; text: string; left: number; top: number; below: boolean }) {
  const icon = tone === 'success' ? <CheckCircle2 size={16} /> : tone === 'error' ? <AlertTriangle size={16} /> : <CircleDashed size={16} />;
  return <div className={`action-notice ${tone} ${below ? 'below' : ''}`} role="status" style={{ left, top }}>{icon}<span>{text}</span></div>;
}

type CropSource = { src: string; width: number; height: number };

function cropSourceRect(width: number, height: number, zoom: number, focusX: number, focusY: number) {
  const targetRatio = 16 / 9;
  let cropWidth = width;
  let cropHeight = height;
  if (width / height > targetRatio) cropWidth = height * targetRatio;
  else cropHeight = width / targetRatio;
  cropWidth = Math.max(1, cropWidth / zoom);
  cropHeight = Math.max(1, cropHeight / zoom);
  const maxX = Math.max(0, width - cropWidth);
  const maxY = Math.max(0, height - cropHeight);
  return { sx: maxX * (focusX / 100), sy: maxY * (focusY / 100), sw: cropWidth, sh: cropHeight };
}

function drawCrop(canvas: HTMLCanvasElement, image: HTMLImageElement, zoom: number, focusX: number, focusY: number, width = 720, height = 405) {
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d');
  if (!context) return;
  const rect = cropSourceRect(image.naturalWidth, image.naturalHeight, zoom, focusX, focusY);
  context.clearRect(0, 0, width, height);
  context.drawImage(image, rect.sx, rect.sy, rect.sw, rect.sh, 0, 0, width, height);
}

function decodeImage(source: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error('图片无法读取，请换一张图片'));
    image.src = source;
  });
}

function readCropSource(file: File): Promise<CropSource> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error('读取图片失败'));
    reader.onload = async () => {
      if (typeof reader.result !== 'string') {
        reject(new Error('图片格式无法识别'));
        return;
      }
      try {
        const image = await decodeImage(reader.result);
        resolve({ src: reader.result, width: image.naturalWidth, height: image.naturalHeight });
      } catch (error) {
        reject(error);
      }
    };
    reader.readAsDataURL(file);
  });
}

function ThemeGallery({ theme, themeMode, themes, customWallpaper, transparency, onThemeChange, onThemeModeChange, onTransparencyChange, onCustomWallpaperChange, onConfirm, onOpenSource }: {
  theme: ThemeId;
  themeMode: ThemeMode;
  themes: ThemeDefinition[];
  customWallpaper: string;
  transparency: ThemeTransparency;
  onThemeChange: (theme: ThemeId) => void;
  onThemeModeChange: (mode: ThemeMode) => void;
  onTransparencyChange: (transparency: ThemeTransparency) => void;
  onCustomWallpaperChange: (wallpaper: string) => void;
  onConfirm: (options: ConfirmationOptions) => Promise<boolean>;
  onOpenSource: (url: string) => void;
}) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewCanvasRef = useRef<HTMLCanvasElement>(null);
  const [cropSource, setCropSource] = useState<CropSource | null>(null);
  const [cropZoom, setCropZoom] = useState(1);
  const [cropFocusX, setCropFocusX] = useState(50);
  const [cropFocusY, setCropFocusY] = useState(50);
  const [cropBusy, setCropBusy] = useState(false);
  const [cropError, setCropError] = useState('');
  const cards = themes;

  useEffect(() => {
    if (!cropSource || !previewCanvasRef.current) return;
    let cancelled = false;
    void decodeImage(cropSource.src).then((image) => {
      if (!cancelled && previewCanvasRef.current) drawCrop(previewCanvasRef.current, image, cropZoom, cropFocusX, cropFocusY);
    }).catch(() => {
      if (!cancelled) setCropError('图片预览失败，请重新选择');
    });
    return () => { cancelled = true; };
  }, [cropSource, cropZoom, cropFocusX, cropFocusY]);

  async function selectFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      setCropError('请选择 JPG、PNG 或 WebP 图片');
      return;
    }
    if (file.size > 15 * 1024 * 1024) {
      setCropError('图片不能超过 15 MB');
      return;
    }
    try {
      const source = await readCropSource(file);
      if (source.width < 320 || source.height < 180) throw new Error('图片尺寸太小，至少需要 320×180');
      setCropError('');
      setCropZoom(1);
      setCropFocusX(50);
      setCropFocusY(50);
      setCropSource(source);
    } catch (error) {
      setCropError(error instanceof Error ? error.message : '读取图片失败');
    }
  }

  function openPicker() {
    fileInputRef.current?.click();
  }

  function closeCrop() {
    if (cropBusy) return;
    setCropSource(null);
    setCropError('');
  }

  async function applyCrop() {
    if (!cropSource) return;
    setCropBusy(true);
    setCropError('');
    try {
      const image = await decodeImage(cropSource.src);
      const encode = (width: number, height: number, quality: number) => {
        const canvas = document.createElement('canvas');
        drawCrop(canvas, image, cropZoom, cropFocusX, cropFocusY, width, height);
        return canvas.toDataURL('image/jpeg', quality);
      };
      let wallpaper = encode(1600, 900, 0.84);
      if (wallpaper.length > 3800000) wallpaper = encode(1280, 720, 0.78);
      if (wallpaper.length > 4800000) throw new Error('压缩后的图片仍然过大，请选择尺寸更小的图片');
      onCustomWallpaperChange(wallpaper);
      setCropSource(null);
    } catch (error) {
      setCropError(error instanceof Error ? error.message : '应用自定义皮肤失败');
    } finally {
      setCropBusy(false);
    }
  }

  async function removeCustom() {
    if (!(await onConfirm({ title: '删除自定义图片', message: '删除已保存的自定义皮肤并恢复默认吗？', confirmLabel: '确认删除', tone: 'danger' }))) return;
    onCustomWallpaperChange('');
  }

  function changeTransparency(setting: keyof ThemeTransparency, value: number) {
    onTransparencyChange({ ...transparency, [setting]: value });
  }

  return <>
    <div className="content-stack narrow-stack">
      <section className="page-intro">
        <div className="section-icon blue"><Palette size={20} /></div>
        <div><p className="eyebrow">LOCAL APPEARANCE</p><h2>外观皮肤</h2><p>选择人物、动漫和风景背景，偏好会自动保存在这台设备上。</p></div>
      </section>
      <section className="theme-panel">
        <div className="theme-panel-controls">
          <div className="theme-mode-switch" role="group" aria-label="界面主题">
            <button type="button" className={themeMode === 'system' ? 'active' : ''} aria-pressed={themeMode === 'system'} onClick={() => onThemeModeChange('system')}>系统</button>
            <button type="button" className={themeMode === 'light' ? 'active' : ''} aria-pressed={themeMode === 'light'} onClick={() => onThemeModeChange('light')}>浅色</button>
            <button type="button" className={themeMode === 'dark' ? 'active' : ''} aria-pressed={themeMode === 'dark'} onClick={() => onThemeModeChange('dark')}>深色</button>
          </div>
          <p>如果你有好看的皮肤想让他人看到，可以进群联系管理，审核成功后下个版本会看到你的界面皮肤</p>
        </div>
        <div className="theme-transparency-controls">
          <label className="range-field theme-transparency-field"><span>皮肤透明度 <b>{transparency.skin}%</b></span><input type="range" min="0" max="100" step="1" value={transparency.skin} aria-label="皮肤透明度" onChange={(event) => changeTransparency('skin', Number(event.target.value))} /></label>
          <label className="range-field theme-transparency-field"><span>工具框透明度 <b>{transparency.content}%</b></span><input type="range" min="15" max="100" step="1" value={transparency.content} aria-label="工具框透明度" onChange={(event) => changeTransparency('content', Number(event.target.value))} /></label>
        </div>
        <div className="theme-grid">
          {cards.map((item) => {
            const isCustom = item.id === 'custom';
            const hasCustomWallpaper = isCustom && Boolean(customWallpaper);
            const wallpaper = isCustom ? customWallpaper : item.wallpaper;
            return <button type="button" key={item.id} className={`theme-card ${theme === item.id ? 'active' : ''}`} onClick={() => { if (!isCustom || hasCustomWallpaper) onThemeChange(item.id); }} disabled={isCustom && !hasCustomWallpaper} aria-pressed={theme === item.id}>
              <span className="theme-preview" style={{ '--theme-preview-bg': item.swatches[2], '--theme-preview-sidebar': item.swatches[0], '--theme-preview-accent': item.swatches[1], '--theme-preview-positive': item.swatches[3], '--theme-preview-wallpaper': wallpaper ? `url(${wallpaper})` : 'none' } as CSSProperties}><span className="theme-preview-sidebar" /><span className="theme-preview-main"><i /><i /><b /></span></span>
              <span className="theme-card-copy"><strong>{item.name}</strong><span>{isCustom && !hasCustomWallpaper ? '尚未上传图片' : item.subtitle}</span><em>{item.sourceURL ? <span className="theme-source" onClick={(event) => { event.stopPropagation(); onOpenSource(item.sourceURL as string); }}>{item.source} <ArrowUpRight size={11} /></span> : item.source}</em></span>
              {theme === item.id && <span className="theme-selected" aria-label="当前使用"><Check size={14} /></span>}
            </button>;
          })}
          <button type="button" className="theme-upload-card" onClick={openPicker}><ImagePlus size={21} /><span><strong>上传新图片</strong><small>选择图片后可调整裁剪区域</small></span></button>
        </div>
        {cropError && !cropSource && <div className="theme-inline-error"><AlertTriangle size={15} />{cropError}</div>}
        <div className="theme-panel-footer"><span>当前皮肤：<strong>{cards.find((item) => item.id === theme)?.name || '词元神青'}</strong></span><span>{customWallpaper ? <button type="button" className="theme-delete" onClick={() => void removeCustom()}><Trash2 size={13} />删除自定义图片</button> : '图片随安装包提供，离线也能使用'}</span></div>
      </section>
    </div>
    <input ref={fileInputRef} className="theme-file-input" type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => void selectFile(event)} />
    {cropSource && <div className="modal-backdrop" role="presentation"><section className="crop-modal" role="dialog" aria-modal="true" aria-labelledby="crop-title"><div className="modal-heading"><div><p className="eyebrow">CUSTOM WALLPAPER</p><h2 id="crop-title">裁剪自定义皮肤</h2></div><button type="button" className="icon-button" title="关闭" aria-label="关闭" onClick={closeCrop} disabled={cropBusy}><X size={18} /></button></div><div className="crop-body"><div className="crop-preview"><canvas ref={previewCanvasRef} aria-label="裁剪预览" /></div><div className="crop-controls"><label className="range-field"><span>缩放 <b>{cropZoom.toFixed(2)}x</b></span><input type="range" min="1" max="2.5" step="0.05" value={cropZoom} onChange={(event) => setCropZoom(Number(event.target.value))} /></label><label className="range-field"><span>水平位置 <b>{Math.round(cropFocusX)}%</b></span><input type="range" min="0" max="100" step="1" value={cropFocusX} onChange={(event) => setCropFocusX(Number(event.target.value))} /></label><label className="range-field"><span>垂直位置 <b>{Math.round(cropFocusY)}%</b></span><input type="range" min="0" max="100" step="1" value={cropFocusY} onChange={(event) => setCropFocusY(Number(event.target.value))} /></label>{cropError && <div className="crop-error"><AlertTriangle size={16} />{cropError}</div>}</div></div><div className="modal-actions"><button type="button" className="secondary-button" onClick={closeCrop} disabled={cropBusy}>取消</button><button type="button" className="primary-button" onClick={() => void applyCrop()} disabled={cropBusy}><Crop size={16} />{cropBusy ? '处理中' : '应用皮肤'}</button></div></section></div>}
  </>;
}

function GroupRatios({ report, busy, refresh }: { report: GroupRatioReport | null; busy: string; refresh: () => void }) {
  return <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon blue"><Layers3 size={20} /></div><div><p className="eyebrow">PUBLIC ROUTING</p><h2>分组倍率</h2><p>当前词元神公开可见分组与基础倍率。</p></div><button className="primary-button" onClick={refresh} disabled={busy === 'groups'}><RefreshCw size={16} className={busy === 'groups' ? 'spin' : ''} />刷新倍率</button></section>{!report ? <section className="group-table"><EmptyState icon={<BarChart3 size={22} />} title="尚未读取" text="点击刷新倍率获取当前分组。" /></section> : <section className="group-table"><div className="group-table-header"><span>分组</span><span>基础</span><span>月卡</span><span>周卡</span></div>{report.groups.map((item) => <div className="group-row" key={item.name}><div><strong>{item.name}</strong><span>{item.description || '暂无分组说明'}</span></div><b className="ratio base">{formatRatio(item.ratio)}</b><b className="ratio month">{formatRatio(item.ratio * 0.85)}</b><b className="ratio week">{formatRatio(item.ratio * 0.9)}</b></div>)}<div className="group-table-footer">{report.groups.length} 个公开分组 · 更新于 {formatTime(report.fetchedAt)}</div></section>}</div>;
}

function Backups({ backups, backupRoot, busy, restore, remove, refresh }: { backups: Backup[]; backupRoot: string; busy: string; restore: (id: string) => void; remove: (id: string) => void; refresh: () => void }) {
  const [expanded, setExpanded] = useState<string | null>(null);
  return <div className="content-stack narrow-stack"><section className="page-intro backup-intro"><div className="section-icon amber"><RotateCcw size={20} /></div><div><p className="eyebrow">RECOVERY</p><h2>配置备份</h2><p className="backup-root"><FolderArchive size={13} /><code>{backupRoot || '正在读取备份目录'}</code></p></div><button className="icon-button inline" title="刷新备份" aria-label="刷新备份" onClick={refresh}><RefreshCw size={17} /></button></section><section className="backup-list">{backups.length === 0 ? <EmptyState icon={<RotateCcw size={22} />} title="暂无备份" text="完成一次配置后，备份会显示在这里。" /> : backups.map((backup) => <article className="backup-entry" key={backup.id}><div className="backup-row"><div className="backup-icon"><FileCheck2 size={18} /></div><div className="backup-details"><strong>{formatTime(backup.createdAt)}</strong><span>{backup.files.length} 个文件 · {backup.path}</span></div><button className="icon-button compact-icon" title={expanded === backup.id ? '收起备份文件' : '查看备份文件'} aria-label={expanded === backup.id ? '收起备份文件' : '查看备份文件'} onClick={() => setExpanded(expanded === backup.id ? null : backup.id)}>{expanded === backup.id ? <ChevronUp size={16} /> : <ChevronDown size={16} />}</button><button className="secondary-button compact" onClick={() => restore(backup.id)} disabled={busy === 'restore' || busy === 'delete'}><RotateCcw size={15} />恢复</button><button className="icon-button compact-icon danger-button" title="删除备份" aria-label="删除备份" onClick={() => remove(backup.id)} disabled={busy === 'restore' || busy === 'delete'}><Trash2 size={16} /></button></div>{expanded === backup.id && <div className="backup-files">{backup.files.map((file) => <div key={`${file.clientId}-${file.originalPath}`}><strong>{clientCopy[file.clientId as ClientId]?.short || file.clientId}</strong><span>{file.originalPath}</span><code>{file.exists ? file.backupPath : '原文件当时不存在'}</code></div>)}</div>}</article>)}</section></div>;
}

function Updates({ update, progress, platform, busy, check, install, openDownload }: { update: UpdateInfo | null; progress: UpdateProgress | null; platform?: string; busy: string; check: () => void; install: () => void; openDownload: () => void }) {
  const installing = busy === 'install-update';
  const completedStage = progress?.stage === 'verifying' || progress?.stage === 'installing';
  const determinate = (progress?.totalBytes ?? 0) > 0 || completedStage;
  const percent = completedStage ? 100 : progress?.percent || 0;
  const indeterminate = Boolean(progress) && !determinate && progress?.stage !== 'failed';
  const showProgress = Boolean(progress) && (installing || progress?.stage === 'failed');
  const progressDetail = !progress
    ? ''
    : progress.totalBytes > 0
      ? `${formatFileSize(progress.downloadedBytes)} / ${formatFileSize(progress.totalBytes)} · ${percent}%`
      : progress.downloadedBytes > 0
        ? `已下载 ${formatFileSize(progress.downloadedBytes)}`
        : completedStage
          ? '更新包已准备完成'
          : '正在连接更新服务';

  return <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon blue"><Download size={20} /></div><div><p className="eyebrow">RELEASE CHANNEL</p><h2>版本更新</h2><p>检测到新版本后可自动下载、校验并安装。</p></div><button className="primary-button" onClick={check} disabled={busy === 'update' || installing}><RefreshCw size={16} className={busy === 'update' ? 'spin' : ''} />检查更新</button></section><section className="update-panel">{!update ? <EmptyState icon={<CircleDashed size={22} />} title="尚未检查" text="正在启动检测，或点击检查更新获取当前版本状态。" /> : update.error ? <div className="update-state error"><AlertTriangle size={22} /><div><strong>检查失败</strong><span>{update.error}</span></div></div> : update.updateAvailable ? <div className="update-state ready"><div className="update-state-icon"><Download size={20} /></div><div><strong>发现新版本 v{update.latestVersion}</strong><span>当前版本 v{update.currentVersion}{update.publishedAt ? ` · ${update.publishedAt}` : ''}</span></div><button className="primary-button" onClick={install} disabled={installing}>{installing ? <RefreshCw size={16} className="spin" /> : <HardDriveDownload size={16} />}{installing ? progress?.stage === 'downloading' ? '下载中' : '准备更新' : platform === 'darwin' ? '打开安装包' : '立即更新'}</button><button className="secondary-button compact" onClick={openDownload} disabled={installing}><ArrowUpRight size={15} />手动下载</button></div> : <div className="update-state"><div className="update-state-icon"><CheckCircle2 size={20} /></div><div><strong>已经是最新版本</strong><span>当前版本 v{update.currentVersion} · 最新版本 v{update.latestVersion || update.currentVersion} · 检查于 {formatTime(update.checkedAt)}</span></div></div>}{showProgress && progress && <div className={`update-progress ${progress.stage === 'failed' ? 'error' : ''}`} aria-live="polite"><div className="update-progress-copy"><strong>{progress.message}</strong><span>{progressDetail}</span></div><div className={`update-progress-track ${indeterminate ? 'indeterminate' : ''}`} role="progressbar" aria-label="更新进度" aria-valuemin={0} aria-valuemax={determinate ? 100 : undefined} aria-valuenow={determinate ? percent : undefined} aria-valuetext={progressDetail}><span className="update-progress-fill" style={determinate ? { width: `${percent}%` } : undefined} /></div></div>}{update?.releaseNotes && <div className="release-notes">{update.releaseNotes}</div>}</section></div>;
}

function Feedback({ tone, text, onClose }: { tone: NoticeTone; text: string; onClose: () => void }) {
  const icon = tone === 'success' ? <CheckCircle2 size={17} /> : tone === 'error' ? <AlertTriangle size={17} /> : <Settings2 size={17} />;
  return <div className={`feedback ${tone}`}>{icon}<span>{text}</span><button className="feedback-close" title="关闭" aria-label="关闭" onClick={onClose}><X size={15} /></button></div>;
}

function EmptyState({ icon, title, text }: { icon: ReactNode; title: string; text: string }) {
  return <div className="empty-state"><div className="empty-icon">{icon}</div><strong>{title}</strong><span>{text}</span></div>;
}

function mockToolValidation(clientId: ClientId): ToolKeyValidationResult {
  return { clientId, models: mockToolModels(clientId), status: 200, endpoint: 'https://api.ciyuanshen.top/v1/models' };
}

function mockToolModels(clientId: ClientId): Model[] {
  const all = [{ id: 'gpt-5.6-terra' }, { id: 'gpt-5.6-sol' }, { id: 'claude-sonnet-4-5' }, { id: 'gemini-2.5-pro' }, { id: 'grok-4' }];
  if (clientId === 'claude' || clientId === 'claude-desktop') return all.filter((model) => model.id.startsWith('claude'));
  if (clientId === 'codex') return all.filter((model) => model.id.startsWith('gpt'));
  if (clientId === 'gemini') return all.filter((model) => model.id.startsWith('gemini'));
  if (clientId === 'grok') return all.filter((model) => model.id.startsWith('grok'));
  return all;
}

function mockToolOptions(clientId: ClientId): ToolOptionsResponse {
  return { clientId, groups: [{ name: `${clientCopy[clientId].short} 分组`, description: '示例可用分组', ratio: '0.2', models: mockToolModels(clientId) }] };
}

function mockProvision(clientId: ClientId, group: string): ToolKeyResult {
  return { provisionId: 'preview-provision', clientId, group, models: mockToolModels(clientId), status: 200, endpoint: 'https://api.ciyuanshen.top/v1/models' };
}

function mockConfigure(clientId: ClientId): ConfigureResult {
  return { success: true, configured: [clientId], finishedAt: new Date().toISOString() };
}

function mockGroupRatios(): GroupRatioReport {
  return { endpoint: 'https://api.ciyuanshen.top/api/user/groups', fetchedAt: new Date().toISOString(), groups: [{ name: 'GPT低价', description: '示例低价分组', ratio: 0.1 }, { name: 'Gemini', description: '示例 Gemini 分组', ratio: 0.4 }, { name: '默认', description: '示例默认分组', ratio: 1 }] };
}

function browserPreviewConnectionCheck(targets: ClientId[]): ConnectionCheckReport {
  const checkedAt = new Date().toISOString();
  return { checkedAt, results: targets.map((id) => ({ id, name: clientCopy[id].short, success: false, configured: false, status: 0, endpoint: 'https://api.ciyuanshen.top/v1/models', message: desktopOnlyMessage, checkedAt })) };
}

function mockClientConfiguration(clientId: ClientId, revealSecrets: boolean): ClientConfigurationView {
  const key = revealSecrets ? 'demo-key' : '********';
  return { clientId, clientName: clientCopy[clientId].short, secretsRedacted: !revealSecrets, files: [{ path: `~/.${clientId}/config`, exists: true, content: `model = "${recommendedModels[clientId]}"\napi_key = "${key}"\n` }] };
}

function lifecycleVersionSummary(info: ToolLifecycleInfo) {
  if (info.error) return `版本检查失败：${info.error}`;
  const parts: string[] = [];
  if (info.currentVersion) parts.push(`当前 ${info.currentVersion}`);
  if (info.latestVersion) parts.push(`最新 ${info.latestVersion}`);
  if (info.latestVersion && info.updateAvailable) parts.push('有更新');
  if (parts.length > 0) return parts.join(' · ');
  return info.message || '尚未获取版本信息';
}

function lifecycleStatusMessage(info: ToolLifecycleInfo) {
  if (info.error) return info.error;
  if (info.latestVersion) {
    if (info.updateAvailable) return `发现新版本 ${info.latestVersion}`;
    if (info.currentVersion) return `当前 ${info.currentVersion}，最新 ${info.latestVersion}`;
    return `最新版本 ${info.latestVersion}${info.message ? `。${info.message}` : ''}`;
  }
  if (info.message) return info.message;
  return info.installed ? '已检测到工具，但暂未获取到最新版本' : '尚未安装该工具';
}

const browserPreviewNPMPackages: Partial<Record<ClientId, string>> = {
  claude: '@anthropic-ai/claude-code',
  codex: '@openai/codex',
  gemini: '@google/gemini-cli',
  grok: '@xai-official/grok',
  opencode: 'opencode-ai',
  openclaw: 'openclaw',
};

function browserPreviewLifecycleBase(clientId: ClientId): ToolLifecycleInfo {
  return {
    clientId,
    name: clientCopy[clientId].short,
    installed: false,
    updateAvailable: false,
    canInstall: false,
    canUpdate: false,
    checkedAt: new Date().toISOString(),
    message: '浏览器预览仅查询官方最新版本；本机版本和安装状态请在桌面安装版中查看',
  };
}

async function fetchBrowserPreviewToolLifecycle(clientId: ClientId): Promise<ToolLifecycleInfo> {
  const base = browserPreviewLifecycleBase(clientId);
  const packageName = browserPreviewNPMPackages[clientId];
  try {
    if (packageName) {
      const response = await fetch(`https://registry.npmjs.org/${encodeURIComponent(packageName)}/latest`, { headers: { Accept: 'application/json' } });
      if (!response.ok) throw new Error(`npm 返回 HTTP ${response.status}`);
      const payload = await response.json() as { version?: unknown };
      const latestVersion = typeof payload.version === 'string' ? payload.version.trim().replace(/^v/, '') : '';
      if (!latestVersion) throw new Error('npm 未返回有效版本号');
      return { ...base, latestVersion, installMethod: 'npm' };
    }
    if (clientId === 'hermes') {
      const response = await fetch('https://api.github.com/repos/NousResearch/hermes-agent/releases/latest', { headers: { Accept: 'application/vnd.github+json' } });
      if (!response.ok) throw new Error(`GitHub 返回 HTTP ${response.status}`);
      const payload = await response.json() as { tag_name?: unknown };
      const latestVersion = typeof payload.tag_name === 'string' ? payload.tag_name.trim().replace(/^v/, '') : '';
      if (!latestVersion) throw new Error('GitHub 未返回有效版本号');
      return { ...base, latestVersion };
    }
    return { ...base, downloadUrl: 'https://claude.com/download', message: 'Claude Code客户端由应用内更新；本机版本请在桌面安装版中查看' };
  } catch (error) {
    return { ...base, error: error instanceof Error ? `暂时无法查询最新版本：${error.message}` : '暂时无法查询最新版本' };
  }
}

async function fetchBrowserPreviewUpdate(currentVersion: string): Promise<UpdateInfo> {
  const response = await fetch('https://api.github.com/repos/China-520-1314/ciyuanshen-config-assistant/releases/latest', { headers: { Accept: 'application/vnd.github+json' } });
  if (!response.ok) throw new Error(`GitHub 更新服务返回 HTTP ${response.status}`);
  const payload = await response.json() as { tag_name?: unknown; body?: unknown; published_at?: unknown; assets?: unknown };
  const latestVersion = typeof payload.tag_name === 'string' ? payload.tag_name.trim().replace(/^v/, '') : '';
  if (!latestVersion) throw new Error('GitHub Release 缺少版本标签');
  const assets = Array.isArray(payload.assets) ? payload.assets as { name?: unknown; browser_download_url?: unknown }[] : [];
  const installer = assets.find((asset) => typeof asset.name === 'string' && /installer.*\.exe$/i.test(asset.name) && typeof asset.browser_download_url === 'string');
  const updateAvailable = compareBrowserVersions(latestVersion, currentVersion) > 0;
  if (updateAvailable && !installer) throw new Error('GitHub Release 未包含 Windows 安装包');
  return {
    currentVersion,
    latestVersion,
    updateAvailable,
    downloadUrl: typeof installer?.browser_download_url === 'string' ? installer.browser_download_url : undefined,
    releaseNotes: typeof payload.body === 'string' ? payload.body.trim() : undefined,
    publishedAt: typeof payload.published_at === 'string' ? payload.published_at : undefined,
    checkedAt: new Date().toISOString(),
  };
}

function compareBrowserVersions(left: string, right: string) {
  const normalise = (value: string) => value.trim().replace(/^v/, '');
  const [leftMain, leftPre = ''] = normalise(left).split('-', 2);
  const [rightMain, rightPre = ''] = normalise(right).split('-', 2);
  const leftParts = leftMain.split('.').map((part) => Number.parseInt(part, 10) || 0);
  const rightParts = rightMain.split('.').map((part) => Number.parseInt(part, 10) || 0);
  for (let index = 0; index < Math.max(leftParts.length, rightParts.length, 3); index += 1) {
    const difference = (leftParts[index] || 0) - (rightParts[index] || 0);
    if (difference !== 0) return difference > 0 ? 1 : -1;
  }
  if (leftPre === rightPre) return 0;
  if (!leftPre) return 1;
  if (!rightPre) return -1;
  return leftPre > rightPre ? 1 : -1;
}

export default App;
