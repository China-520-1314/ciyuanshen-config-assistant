import { useEffect, useMemo, useRef, useState, type ChangeEvent, type CSSProperties, type ReactNode } from 'react';
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
import { Quit, WindowMinimise, WindowToggleMaximise } from '../wailsjs/runtime/runtime';

type ClientId = 'claude' | 'claude-desktop' | 'codex' | 'gemini' | 'grok' | 'opencode' | 'openclaw' | 'hermes';
type TabId = 'overview' | 'groups' | 'backups' | 'updates' | 'appearance';
type ThemeId = 'ciyuan' | 'anime' | 'sakura' | 'mountain' | 'city' | 'custom';
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
type AppInfo = { name: string; version: string; updateManifestUrl: string; gatewayUrl: string };
type AccountState = { signedIn: boolean; username: string; balance?: string; quota?: number; balanceUpdatedAt?: string; expiresAt?: string };
type AccountLoginResult = { signedIn: boolean; requiresTwoFactor: boolean; flowToken: string; username: string; expiresAt?: string };
type SavedAccountLogin = { username: string; password: string };
type ToolGroupOption = { name: string; description: string; ratio: string; models: Model[] };
type ToolOptionsResponse = { clientId: ClientId; groups: ToolGroupOption[] };
type ToolKeyResult = { provisionId: string; clientId: ClientId; group: string; models: Model[]; status: number; endpoint: string };
type ToolKeyValidationResult = { clientId: ClientId; models: Model[]; selectedModel?: string; status: number; endpoint: string };
type ConfigureResult = { success: boolean; error?: string; configured: string[]; finishedAt: string };
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

const documentationURL = 'https://ocn4dgkicvdh.feishu.cn/docx/Y88FdkLNPo6g17xWIfHcQfgknrc';
const officialWebsiteURL = 'https://ciyuanshen.top';
const walletURL = 'https://ciyuanshen.top/wallet';
const signUpURL = 'https://ciyuanshen.top/sign-up';
const forgotPasswordURL = 'https://ciyuanshen.top/forgot-password';
const qqGroupURL = 'https://qm.qq.com/q/rmwfirFNp8';
const desktopOnlyMessage = '浏览器预览无法读取本机配置，请下载并运行 Windows 安装版。';
const clientOrder: ClientId[] = ['claude', 'claude-desktop', 'codex', 'gemini', 'grok', 'opencode', 'openclaw', 'hermes'];
const clientCopy: Record<ClientId, { short: string; badge: string }> = {
  claude: { short: 'Claude Code终端', badge: 'Anthropic CLI' },
  'claude-desktop': { short: 'Claude Code客户端', badge: 'Anthropic Desktop' },
  codex: { short: 'ChatGPT/Codex Cli/Codex插件', badge: 'Responses' },
  gemini: { short: 'Gemini CLI', badge: 'Gemini API' },
  grok: { short: 'Grok Build', badge: 'Responses' },
  opencode: { short: 'OpenCode', badge: 'OpenAI compatible' },
  openclaw: { short: 'OpenClaw', badge: 'OpenAI compatible' },
  hermes: { short: 'Hermes Agent', badge: 'Chat Completions' },
};
const recommendedModels: Record<ClientId, string> = {
  claude: 'claude-sonnet-4-5',
  'claude-desktop': 'claude-sonnet-4-5',
  codex: 'gpt-5.6-sol',
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
  dark: boolean;
  swatches: string[];
};

const themeStorageKey = 'ciyuanshen-config-assistant.theme';
const customWallpaperStorageKey = 'ciyuanshen-config-assistant.custom-wallpaper';
const themeDefinitions: ThemeDefinition[] = [
  { id: 'ciyuan', name: '词元神青', subtitle: '清爽工作台', source: '词元神', dark: false, swatches: ['#173735', '#0c766d', '#f4f7f7', '#e4f3f0'] },
  { id: 'anime', name: '冬日人物', subtitle: '动漫人物 · 柔和青', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: animeWallpaper, dark: false, swatches: ['#406b70', '#d97c9f', '#e9f2ef', '#b9e4de'] },
  { id: 'sakura', name: '蓝樱花', subtitle: '花枝风景 · 清透蓝', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: cherryWallpaper, dark: false, swatches: ['#31566f', '#d8797c', '#edf5f5', '#bfe3e6'] },
  { id: 'mountain', name: '雪山远景', subtitle: '自然风景 · 深色', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: mountainWallpaper, dark: true, swatches: ['#122c35', '#76b6c9', '#203d46', '#d2e4d6'] },
  { id: 'city', name: '夜色城市', subtitle: '城市灯火 · 暗色', source: 'FrenzyExists/wallpapers', sourceURL: 'https://github.com/FrenzyExists/wallpapers', wallpaper: cityWallpaper, dark: true, swatches: ['#171c29', '#e8873c', '#28303c', '#f5c96a'] },
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

function customThemeDefinition(wallpaper: string): ThemeDefinition {
  return { id: 'custom', name: '我的图片', subtitle: '自定义背景 · 本地保存', source: '本机图片', wallpaper, dark: true, swatches: ['#101923', '#82c7cf', '#1d2a35', '#d9b56d'] };
}

const tabTitles: Record<TabId, string> = {
  overview: '环境概览',
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

function defaultModel(clientId: ClientId, models: Model[], current?: string) {
  if (current && models.some((model) => model.id === current)) return current;
  if (models.some((model) => model.id === recommendedModels[clientId])) return recommendedModels[clientId];
  return models[0]?.id || '';
}

function App() {
  const [tab, setTab] = useState<TabId>('overview');
  const [theme, setTheme] = useState<ThemeId>(readStoredTheme);
  const [customWallpaper, setCustomWallpaper] = useState(readStoredWallpaper);
  const [environment, setEnvironment] = useState<EnvironmentReport>(mockEnvironment);
  const [appInfo, setAppInfo] = useState<AppInfo>({ name: '词元神配置助手', version: '0.2.5', updateManifestUrl: '', gatewayUrl: 'https://api.ciyuanshen.top/v1' });
  const [account, setAccount] = useState<AccountState>({ signedIn: false, username: '' });
  const [accountRefreshing, setAccountRefreshing] = useState(false);
  const [toolModels, setToolModels] = useState<Partial<Record<ClientId, Model[]>>>({});
  const [modelByClient, setModelByClient] = useState<Record<ClientId, string>>(recommendedModels);
  const [modelsLoading, setModelsLoading] = useState<Partial<Record<ClientId, boolean>>>({});
  const [modelErrors, setModelErrors] = useState<Partial<Record<ClientId, string>>>({});
  const [connectionResults, setConnectionResults] = useState<Partial<Record<ClientId, ClientConnectionResult>>>({});
  const [busy, setBusy] = useState('');
  const [checkingClient, setCheckingClient] = useState<ClientId | null>(null);
  const [applyingModelClient, setApplyingModelClient] = useState<ClientId | null>(null);
  const [lifecycleByClient, setLifecycleByClient] = useState<Partial<Record<ClientId, ToolLifecycleInfo>>>({});
  const [lifecycleBusyClient, setLifecycleBusyClient] = useState<ClientId | null>(null);
  const [feedback, setFeedback] = useState<{ tone: NoticeTone; text: string } | null>(null);
  const [actionNotice, setActionNotice] = useState<{ tone: NoticeTone; text: string; left: number; top: number; below: boolean } | null>(null);
  const feedbackTimer = useRef<number | undefined>(undefined);
  const actionTimer = useRef<number | undefined>(undefined);
  const configureAnchor = useRef<HTMLElement | null>(null);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [backupRoot, setBackupRoot] = useState('');
  const [update, setUpdate] = useState<UpdateInfo | null>(null);
  const [groupReport, setGroupReport] = useState<GroupRatioReport | null>(null);
  const [setup, setSetup] = useState<SetupState | null>(null);
  const [toolOptions, setToolOptions] = useState<ToolOptionsResponse | null>(null);
  const [setupGroup, setSetupGroup] = useState('');
  const [setupKey, setSetupKey] = useState('');
  const [showSetupKey, setShowSetupKey] = useState(false);
  const [setupValidation, setSetupValidation] = useState<ToolKeyValidationResult | null>(null);
  const [provision, setProvision] = useState<ToolKeyResult | null>(null);
  const [setupModel, setSetupModel] = useState('');
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
  const availableThemes = useMemo(() => customWallpaper ? [...themeDefinitions, customThemeDefinition(customWallpaper)] : themeDefinitions, [customWallpaper]);
  const activeTheme = availableThemes.find((item) => item.id === theme) || themeDefinitions[0];

  const clientMap = useMemo(() => new Map(environment.clients.map((client) => [client.id, client])), [environment.clients]);

  useEffect(() => {
    void loadInitialState();
  }, []);

  useEffect(() => () => {
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current);
    if (actionTimer.current !== undefined) window.clearTimeout(actionTimer.current);
  }, []);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.body.dataset.theme = theme;
    try {
      window.localStorage.setItem(themeStorageKey, theme);
      if (customWallpaper) window.localStorage.setItem(customWallpaperStorageKey, customWallpaper);
      else window.localStorage.removeItem(customWallpaperStorageKey);
    } catch {
      // Theme remains active for this session when persistence is unavailable.
    }
    if (theme === 'custom' && !customWallpaper) setTheme('ciyuan');
  }, [theme, customWallpaper]);

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

  async function loadInitialState() {
    try {
      if (inWails()) {
        const [info, report, root, accountState] = await Promise.all([GetAppInfo(), ScanEnvironment(), GetBackupRoot(), GetAccountState()]);
        setAppInfo(info as AppInfo);
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
        await refreshConfiguredModels(report as EnvironmentReport);
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
      await refreshConfiguredModels(report as EnvironmentReport);
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

  async function applyExistingModel(clientId: ClientId, anchor: HTMLElement) {
    const model = modelByClient[clientId];
    if (!model) {
      showActionNotice(anchor, { tone: 'error', text: '请先读取该工具当前 Key 可用的模型' });
      return;
    }
    if (!window.confirm(`将备份 ${clientCopy[clientId].short} 当前配置，并将默认模型改为 ${model}。是否继续？`)) return;
    setApplyingModelClient(clientId);
    try {
      const result = inWails()
        ? await ConfigureExistingTool({ clientId, model })
        : mockConfigure(clientId);
      const configured = result as ConfigureResult;
      if (!configured.success) throw new Error(configured.error || '默认模型应用失败');
      await Promise.all([refreshBackups(), refreshEnvironment(false)]);
      await checkClient(clientId, anchor);
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

  function toggleConfigurationSecrets() {
    if (!configurationClient) return;
    const nextReveal = !revealConfigurationSecrets;
    if (nextReveal && !window.confirm('配置文件可能包含 API Key 或访问令牌。确认在本机界面中显示明文吗？')) return;
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
      showActionNotice(anchor, { tone: 'error', text: '浏览器预览无法安装或更新本机工具，请运行 Windows 安装版。' });
      return;
    }
    const verb = action === 'install' ? '安装' : '更新';
    if (!window.confirm(`将通过官方 npm 包${verb} ${clientCopy[clientId].short}。是否继续？`)) return;
    setLifecycleBusyClient(clientId);
    try {
      const result = await RunToolLifecycleAction({ clientId, action });
      const lifecycleResult = result as ToolLifecycleResult;
      setLifecycleByClient((current) => ({ ...current, [clientId]: lifecycleResult.info }));
      if (lifecycleResult.manual) {
        const message = lifecycleResult.message || '该工具需要通过官方页面安装';
        if (lifecycleResult.downloadUrl && window.confirm(`${message}\n\n现在打开官方下载页面吗？`)) {
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
    setToolOptions(null);
    setSetupGroup('');
    setSetupKey('');
    setShowSetupKey(false);
    setSetupValidation(null);
    setProvision(null);
    setSetupModel('');
    setSetupBusy('');
    setSetupMessage(null);
  }

  function openToolSetup(clientId: ClientId, anchor: HTMLElement, mode: 'account' | 'manual' = 'account') {
    configureAnchor.current = anchor;
    resetSetup();
    setSetup({ clientId, mode });
    if (mode === 'account') void loadToolOptions(clientId);
  }

  async function loadToolOptions(clientId: ClientId) {
    setSetupBusy('groups');
    setSetupMessage(null);
    try {
      const result = inWails() ? await GetAccountToolOptions(clientId) : mockToolOptions(clientId);
      const options = result as ToolOptionsResponse;
      setToolOptions(options);
      setSetupGroup(options.groups[0]?.name || '');
    } catch (error) {
      setSetupMessage({ tone: 'error', text: error instanceof Error ? error.message : '读取分组失败' });
    } finally {
      setSetupBusy('');
    }
  }

  function switchSetupMode(mode: 'account' | 'manual') {
    if (!setup) return;
    resetSetup();
    setSetup({ ...setup, mode });
    if (mode === 'account') void loadToolOptions(setup.clientId);
  }

  async function createAccountKey() {
    if (!setup || !setupGroup) {
      setSetupMessage({ tone: 'error', text: '请选择分组' });
      return;
    }
    setSetupBusy('key');
    setSetupMessage(null);
    try {
      const result = inWails()
        ? await CreateToolKey({ clientId: setup.clientId, group: setupGroup })
        : mockProvision(setup.clientId, setupGroup);
      const next = result as ToolKeyResult;
      setProvision(next);
      setSetupValidation({ clientId: setup.clientId, models: next.models, status: next.status, endpoint: next.endpoint });
      setSetupModel((current) => defaultModel(setup.clientId, next.models, current || modelByClient[setup.clientId]));
      setSetupMessage({ tone: 'success', text: '已创建并检测 Key，请选择默认模型' });
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
    if (!window.confirm(`将备份 ${clientCopy[setup.clientId].short} 当前配置，并写入新配置。是否继续？`)) return;
    setSetupBusy('configure');
    setSetupMessage(null);
    try {
      const result = provision
        ? inWails()
          ? await ConfigureProvisionedTool({ provisionId: provision.provisionId, clientId: setup.clientId, model: setupModel })
          : mockConfigure(setup.clientId)
        : inWails()
          ? await ConfigureTool({ clientId: setup.clientId, apiKey: setupKey.trim(), model: setupModel })
          : mockConfigure(setup.clientId);
      const configured = result as ConfigureResult;
      if (!configured.success) throw new Error(configured.error || '配置失败');
      const configuredClient = setup.clientId;
      setModelByClient((current) => ({ ...current, [configuredClient]: setupModel }));
      setSetup(null);
      resetSetup();
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
      void loadToolOptions(target);
    }
  }

  async function submitLogin() {
    if (!loginUsername.trim() || !loginPassword) {
      setLoginMessage('请输入账号和密码');
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
    if (!window.confirm('确定永久删除此配置备份吗？此操作不能撤销。')) return;
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

  async function checkUpdate() {
    setBusy('update');
    try {
      const result = inWails()
        ? await CheckForUpdates()
        : await fetchBrowserPreviewUpdate(appInfo.version);
      setUpdate(result as UpdateInfo);
    } catch (error) {
      setUpdate({ currentVersion: appInfo.version, latestVersion: '', updateAvailable: false, checkedAt: new Date().toISOString(), error: error instanceof Error ? error.message : '暂时无法检查更新' });
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

  function selectTab(nextTab: TabId) {
    setTab(nextTab);
    if (nextTab === 'groups') void fetchGroupRatios();
    if (nextTab === 'updates') void checkUpdate();
  }

  return (
    <div className="window-frame" data-theme={theme} style={{ '--theme-wallpaper': activeTheme.wallpaper ? `url(${activeTheme.wallpaper})` : 'none' } as CSSProperties}>
      {inWails() && <WindowTitlebar />}
      <div className="app-shell">
        <aside className="sidebar">
          <div className="brand-lockup">
            <div className="brand-mark"><img src={logo} alt="词元神" /></div>
            <div><strong>词元神</strong><span>配置助手</span></div>
          </div>
          <div className="sidebar-rule" />
          <nav className="side-nav" aria-label="主导航">
            <NavButton active={tab === 'overview'} icon={<ScanSearch size={17} />} label="环境概览" onClick={() => selectTab('overview')} />
            <NavButton active={tab === 'groups'} icon={<Layers3 size={17} />} label="分组倍率" onClick={() => selectTab('groups')} />
            <NavButton active={tab === 'backups'} icon={<RotateCcw size={17} />} label="配置备份" onClick={() => selectTab('backups')} count={backups.length || undefined} />
            <NavButton active={tab === 'updates'} icon={<Download size={17} />} label="版本更新" onClick={() => selectTab('updates')} />
            <NavButton active={tab === 'appearance'} icon={<Palette size={17} />} label="外观皮肤" onClick={() => selectTab('appearance')} />
            <NavButton active={false} icon={<BookOpen size={17} />} label="文档教程" onClick={() => void openExternal(documentationURL)} external />
            <NavButton active={false} icon={<Globe size={17} />} label="进入官网" onClick={() => void openExternal(officialWebsiteURL)} external />
            <NavButton active={false} icon={<Users size={17} />} label="加入QQ群" onClick={() => void openExternal(qqGroupURL)} external />
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
          {tab === 'overview' && <Overview environment={environment} clientMap={clientMap} toolModels={toolModels} modelByClient={modelByClient} modelsLoading={modelsLoading} modelErrors={modelErrors} setClientModel={(clientId, model) => setModelByClient((current) => ({ ...current, [clientId]: model }))} connectionResults={connectionResults} checkingClient={checkingClient} applyingModelClient={applyingModelClient} lifecycleByClient={lifecycleByClient} lifecycleBusyClient={lifecycleBusyClient} onCheck={checkClient} onConfigure={openToolSetup} onApplyModel={applyExistingModel} onViewConfiguration={openClientConfiguration} onLifecycleCheck={checkToolLifecycle} onLifecycleAction={runToolLifecycleAction} />}
          {tab === 'groups' && <GroupRatios report={groupReport} busy={busy} refresh={() => void fetchGroupRatios()} />}
          {tab === 'backups' && <Backups backups={backups} backupRoot={backupRoot} busy={busy} restore={restore} remove={deleteBackup} refresh={() => void refreshBackups()} />}
          {tab === 'updates' && <Updates update={update} busy={busy} check={checkUpdate} openDownload={() => update?.downloadUrl && void openExternal(update.downloadUrl)} />}
          {tab === 'appearance' && <ThemeGallery theme={theme} themes={availableThemes} customWallpaper={customWallpaper} onThemeChange={setTheme} onCustomWallpaperChange={applyCustomWallpaper} onOpenSource={(url) => void openExternal(url)} />}
        </main>
      </div>

      {actionNotice && <ActionNotice {...actionNotice} />}
      {setup && <ToolSetupModal setup={setup} account={account} options={toolOptions} group={setupGroup} keyValue={setupKey} showKey={showSetupKey} validation={setupValidation} provision={provision} model={setupModel} busy={setupBusy} message={setupMessage} onClose={() => { setSetup(null); resetSetup(); }} onModeChange={switchSetupMode} onGroupChange={(value) => { setSetupGroup(value); setSetupValidation(null); setProvision(null); setSetupModel(''); setSetupMessage(null); }} onKeyChange={(value) => { setSetupKey(value); setSetupValidation(null); setProvision(null); setSetupModel(''); setSetupMessage(null); }} onShowKey={() => setShowSetupKey((current) => !current)} onModelChange={setSetupModel} onCreateKey={() => void createAccountKey()} onValidateKey={() => void validateManualKey()} onConfigure={() => void configureSelectedTool()} onLogin={() => openLogin(setup.clientId)} onReloadGroups={() => void loadToolOptions(setup.clientId)} />}
      {configurationClient && <ConfigurationViewerModal clientId={configurationClient} view={configurationView} busy={configurationBusy} error={configurationError} revealSecrets={revealConfigurationSecrets} onClose={() => { setConfigurationClient(null); setConfigurationView(null); setConfigurationError(null); setRevealConfigurationSecrets(false); }} onReload={() => void loadClientConfiguration(configurationClient, revealConfigurationSecrets)} onToggleSecrets={toggleConfigurationSecrets} />}
      {loginOpen && <AccountLoginModal username={loginUsername} password={loginPassword} rememberLogin={rememberLogin} code={twoFactorCode} requiresTwoFactor={Boolean(twoFactorFlow)} showPassword={showLoginPassword} busy={loginBusy} message={loginMessage} onUsername={setLoginUsername} onPassword={setLoginPassword} onRememberLogin={setRememberLogin} onCode={setTwoFactorCode} onTogglePassword={() => setShowLoginPassword((current) => !current)} onClose={() => { setLoginOpen(false); setLoginMessage(null); setTwoFactorFlow(''); }} onSubmit={() => void (twoFactorFlow ? submitTwoFactor() : submitLogin())} onRegister={() => void openExternal(signUpURL)} onForgotPassword={() => void openExternal(forgotPasswordURL)} />}
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

function NavButton({ active, icon, label, count, onClick, external = false }: { active: boolean; icon: ReactNode; label: string; count?: number; onClick: () => void; external?: boolean }) {
  return <button className={`nav-button ${active ? 'active' : ''}`} onClick={onClick}>{icon}<span>{label}</span>{count ? <b>{count}</b> : external ? <ArrowUpRight size={15} className="nav-arrow" /> : <ChevronRight size={15} className="nav-arrow" />}</button>;
}

function Overview({ environment, clientMap, toolModels, modelByClient, modelsLoading, modelErrors, setClientModel, connectionResults, checkingClient, applyingModelClient, lifecycleByClient, lifecycleBusyClient, onCheck, onConfigure, onApplyModel, onViewConfiguration, onLifecycleCheck, onLifecycleAction }: {
  environment: EnvironmentReport;
  clientMap: Map<ClientId, ClientStatus>;
  toolModels: Partial<Record<ClientId, Model[]>>;
  modelByClient: Record<ClientId, string>;
  modelsLoading: Partial<Record<ClientId, boolean>>;
  modelErrors: Partial<Record<ClientId, string>>;
  setClientModel: (clientId: ClientId, model: string) => void;
  connectionResults: Partial<Record<ClientId, ClientConnectionResult>>;
  checkingClient: ClientId | null;
  applyingModelClient: ClientId | null;
  lifecycleByClient: Partial<Record<ClientId, ToolLifecycleInfo>>;
  lifecycleBusyClient: ClientId | null;
  onCheck: (clientId: ClientId, anchor: HTMLElement) => void;
  onConfigure: (clientId: ClientId, anchor: HTMLElement) => void;
  onApplyModel: (clientId: ClientId, anchor: HTMLElement) => void;
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
        {clientOrder.map((clientId) => <ClientCard key={clientId} clientId={clientId} status={clientMap.get(clientId)} models={toolModels[clientId] || []} model={modelByClient[clientId]} modelsLoading={Boolean(modelsLoading[clientId])} modelError={modelErrors[clientId]} onModelChange={(model) => setClientModel(clientId, model)} result={connectionResults[clientId]} checking={checkingClient === clientId} applying={applyingModelClient === clientId} lifecycle={lifecycleByClient[clientId]} lifecycleBusy={lifecycleBusyClient === clientId} onCheck={onCheck} onConfigure={onConfigure} onApplyModel={onApplyModel} onViewConfiguration={onViewConfiguration} onLifecycleCheck={onLifecycleCheck} onLifecycleAction={onLifecycleAction} />)}
      </div>
    </section>
  </div>;
}

function ClientCard({ clientId, status, models, model, modelsLoading, modelError, onModelChange, result, checking, applying, lifecycle, lifecycleBusy, onCheck, onConfigure, onApplyModel, onViewConfiguration, onLifecycleCheck, onLifecycleAction }: {
  clientId: ClientId;
  status?: ClientStatus;
  models: Model[];
  model: string;
  modelsLoading: boolean;
  modelError?: string;
  onModelChange: (value: string) => void;
  result?: ClientConnectionResult;
  checking: boolean;
  applying: boolean;
  lifecycle?: ToolLifecycleInfo;
  lifecycleBusy: boolean;
  onCheck: (clientId: ClientId, anchor: HTMLElement) => void;
  onConfigure: (clientId: ClientId, anchor: HTMLElement) => void;
  onApplyModel: (clientId: ClientId, anchor: HTMLElement) => void;
  onViewConfiguration: (clientId: ClientId) => void;
  onLifecycleCheck: (clientId: ClientId, anchor?: HTMLElement) => void;
  onLifecycleAction: (clientId: ClientId, action: 'install' | 'update' | 'download', anchor: HTMLElement) => void;
}) {
  const unsupported = status?.supported === false;
  const available = Boolean(status?.installed || status?.configExists);
  const state = unsupported ? 'unsupported' : status?.configState === 'invalid' || status?.configState === 'error' ? 'invalid' : available ? 'available' : 'not-found';
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
    <div className="client-card-top"><div className="client-symbol"><img src={clientLogos[clientId]} alt="" /></div><div className="client-name"><strong>{clientCopy[clientId].short}</strong><span>{clientCopy[clientId].badge}</span></div><span className={`state-tag ${state}`}>{state === 'available' ? '可配置' : state === 'invalid' ? '需修复' : state === 'unsupported' ? '不支持' : '未安装'}</span></div>
    <div className="client-card-path">{status?.configPath || '配置文件将自动创建'}{status?.version && <span className="client-version"> · {status.version}</span>}</div>
    <div className="client-card-bottom">
      <label>当前 Key 可用模型</label>
      {modelsLoading ? <span className="model-empty"><RefreshCw size={13} className="spin" />正在读取 Key 可用模型</span> : models.length > 0 ? <><select value={models.some((item) => item.id === model) ? model : ''} onChange={(event) => onModelChange(event.target.value)}>{models.map((option) => <option key={option.id} value={option.id}>{option.id}</option>)}</select><button className="icon-button compact-icon" title="应用默认模型" aria-label="应用默认模型" onClick={(event) => onApplyModel(clientId, event.currentTarget)} disabled={applying || unsupported}><ClipboardCheck size={15} /></button></> : <span className={`model-empty ${modelError ? 'error' : ''}`} title={modelError}>{modelError ? `读取失败：${modelError}` : result?.success ? 'Key 未返回可用模型' : '尚未检测到可用 Key'}</span>}
    </div>
    <div className="client-card-actions">
      <span className={`check-result ${result ? (result.success ? 'success' : 'error') : ''}`}>{result ? (result.success ? <><CheckCircle2 size={14} />已通过</> : <><AlertTriangle size={14} />未通过</>) : <><CircleDashed size={14} />未检测</>}</span>
      <div className="client-action-row"><button className="icon-button compact-icon" title="查看配置文件" aria-label="查看配置文件" onClick={() => onViewConfiguration(clientId)}><FileText size={15} /></button><button className="secondary-button compact" onClick={(event) => onCheck(clientId, event.currentTarget)} disabled={checking || unsupported}><Activity size={14} />{checking ? '检测中' : '检测'}</button><button className="secondary-button compact" onClick={(event) => lifecycleAction === 'check' ? onLifecycleCheck(clientId, event.currentTarget) : onLifecycleAction(clientId, lifecycleAction, event.currentTarget)} disabled={lifecycleBusy || unsupported}>{lifecycleBusy ? <RefreshCw size={14} className="spin" /> : lifecycleIcon}{lifecycleBusy ? '处理中' : lifecycleLabel}</button><button className="primary-button compact" onClick={(event) => onConfigure(clientId, event.currentTarget)} disabled={checking || unsupported}><Settings2 size={14} />一键配置</button></div>
    </div>
    {lifecycle && <div className={`lifecycle-note ${lifecycle.error ? 'error' : ''}`} title={lifecycleText}>{lifecycleText}</div>}
  </article>;
}

function ToolSetupModal({ setup, account, options, group, keyValue, showKey, validation, provision, model, busy, message, onClose, onModeChange, onGroupChange, onKeyChange, onShowKey, onModelChange, onCreateKey, onValidateKey, onConfigure, onLogin, onReloadGroups }: {
  setup: SetupState;
  account: AccountState;
  options: ToolOptionsResponse | null;
  group: string;
  keyValue: string;
  showKey: boolean;
  validation: ToolKeyValidationResult | null;
  provision: ToolKeyResult | null;
  model: string;
  busy: string;
  message: { tone: NoticeTone; text: string } | null;
  onClose: () => void;
  onModeChange: (mode: 'account' | 'manual') => void;
  onGroupChange: (value: string) => void;
  onKeyChange: (value: string) => void;
  onShowKey: () => void;
  onModelChange: (value: string) => void;
  onCreateKey: () => void;
  onValidateKey: () => void;
  onConfigure: () => void;
  onLogin: () => void;
  onReloadGroups: () => void;
}) {
  const client = clientCopy[setup.clientId];
  const selectedGroup = options?.groups.find((item) => item.name === group);
  const keyReady = Boolean(validation && validation.models.length > 0);
  return <div className="modal-backdrop" role="presentation"><section className="setup-modal" role="dialog" aria-modal="true" aria-labelledby="setup-title">
    <div className="modal-heading"><div><p className="eyebrow">{client.badge}</p><h2 id="setup-title">配置 {client.short}</h2></div><button className="icon-button" title="关闭" aria-label="关闭" onClick={onClose}><X size={18} /></button></div>
    <div className="mode-switch" role="tablist" aria-label="Key 来源">
      <button className={setup.mode === 'account' ? 'active' : ''} role="tab" aria-selected={setup.mode === 'account'} onClick={() => onModeChange('account')}><UserRound size={15} />词元神账号</button>
      <button className={setup.mode === 'manual' ? 'active' : ''} role="tab" aria-selected={setup.mode === 'manual'} onClick={() => onModeChange('manual')}><KeyRound size={15} />手动输入 Key</button>
    </div>
    {setup.mode === 'account' ? <div className="setup-flow">
      {!account.signedIn ? <div className="setup-empty"><UserRound size={21} /><strong>尚未登录词元神账号</strong><button className="secondary-button" onClick={onLogin}><LogIn size={16} />登录账号</button></div> : <>
        <div className="field-block"><label htmlFor="group-select">可用分组</label><div className="select-row"><select id="group-select" value={group} onChange={(event) => onGroupChange(event.target.value)} disabled={busy === 'groups' || Boolean(provision)}>{options?.groups.map((item) => <option key={item.name} value={item.name}>{item.name} · {item.ratio ? `${item.ratio}x` : '倍率待定'} · {item.models.length} 个模型</option>)}</select><button className="icon-button" title="刷新分组" aria-label="刷新分组" onClick={onReloadGroups} disabled={busy === 'groups' || Boolean(provision)}><RefreshCw size={16} className={busy === 'groups' ? 'spin' : ''} /></button></div>{selectedGroup?.description && <span className="field-note">{selectedGroup.description}</span>}</div>
        {!keyReady && <button className="primary-button full-width" onClick={onCreateKey} disabled={busy === 'groups' || busy === 'key' || !group}><ShieldCheck size={17} />{busy === 'key' ? '创建并检测中' : '创建并检测 Key'}</button>}
      </>}
    </div> : <div className="setup-flow">
      <div className="field-block"><label htmlFor="manual-key">API Key</label><div className="key-input-wrap"><KeyRound size={17} /><input id="manual-key" type={showKey ? 'text' : 'password'} value={keyValue} placeholder="粘贴 API Key" autoComplete="off" onChange={(event) => onKeyChange(event.target.value)} /><button className="input-action" title={showKey ? '隐藏 Key' : '显示 Key'} aria-label={showKey ? '隐藏 Key' : '显示 Key'} onClick={onShowKey}>{showKey ? <EyeOff size={17} /> : <Eye size={17} />}</button></div></div>
      {!keyReady && <button className="primary-button full-width" onClick={onValidateKey} disabled={busy === 'validate'}><Activity size={17} />{busy === 'validate' ? '检测中' : '检测 Key'}</button>}
    </div>}
    {message && <div className={`setup-status ${message.tone}`}><span>{message.tone === 'success' ? <CheckCircle2 size={16} /> : message.tone === 'error' ? <AlertTriangle size={16} /> : <CircleDashed size={16} />}</span>{message.text}</div>}
    {validation && validation.models.length > 0 && <div className="model-step"><div className="field-block"><label htmlFor="default-model">默认模型</label><select id="default-model" value={model} onChange={(event) => onModelChange(event.target.value)}>{validation.models.map((option) => <option key={option.id} value={option.id}>{option.id}</option>)}</select></div><div className="model-step-footer"><span>{provision ? '新建 Key 已限制为该工具可用模型' : '仅使用当前输入的 Key 完成本次配置'}</span><button className="primary-button" onClick={onConfigure} disabled={busy === 'configure' || !model}><ClipboardCheck size={17} />{busy === 'configure' ? '备份并配置中' : '备份并一键配置'}</button></div></div>}
  </section></div>;
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
  return <div className="modal-backdrop" role="presentation"><section className="login-modal" role="dialog" aria-modal="true" aria-labelledby="login-title">
    <div className="modal-heading"><div><p className="eyebrow">CIYUANSHEN ACCOUNT</p><h2 id="login-title">登录词元神</h2></div><button className="icon-button" title="关闭" aria-label="关闭" onClick={onClose}><X size={18} /></button></div>
    {requiresTwoFactor ? <div className="login-fields"><div className="field-block"><label htmlFor="two-factor-code">两步验证代码</label><input id="two-factor-code" inputMode="numeric" autoComplete="one-time-code" value={code} onChange={(event) => onCode(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') onSubmit(); }} /></div></div> : <div className="login-fields"><div className="field-block"><label htmlFor="account-username">账号</label><input id="account-username" autoComplete="username" value={username} onChange={(event) => onUsername(event.target.value)} /></div><div className="field-block"><label htmlFor="account-password">密码</label><div className="key-input-wrap"><LockKeyhole size={17} /><input id="account-password" type={showPassword ? 'text' : 'password'} autoComplete="current-password" value={password} onChange={(event) => onPassword(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') onSubmit(); }} /><button className="input-action" title={showPassword ? '隐藏密码' : '显示密码'} aria-label={showPassword ? '隐藏密码' : '显示密码'} onClick={onTogglePassword}>{showPassword ? <EyeOff size={17} /> : <Eye size={17} />}</button></div></div></div>}
    {!requiresTwoFactor && <><label className="remember-login"><input type="checkbox" checked={rememberLogin} onChange={(event) => onRememberLogin(event.target.checked)} />保存密码（仅保存在系统凭据管理器）</label><div className="login-links"><div className="login-register"><span>还没有注册？</span><button type="button" onClick={onRegister}>去注册</button></div><button className="login-link" type="button" onClick={onForgotPassword}>忘记密码</button></div></>}
    {message && <div className="setup-status neutral"><CircleDashed size={16} />{message}</div>}
    <div className="modal-actions"><button className="secondary-button" onClick={onClose}>取消</button><button className="primary-button" onClick={onSubmit} disabled={busy}><LogIn size={17} />{busy ? '登录中' : requiresTwoFactor ? '验证并登录' : '登录'}</button></div>
  </section></div>;
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

function ThemeGallery({ theme, themes, customWallpaper, onThemeChange, onCustomWallpaperChange, onOpenSource }: { theme: ThemeId; themes: ThemeDefinition[]; customWallpaper: string; onThemeChange: (theme: ThemeId) => void; onCustomWallpaperChange: (wallpaper: string) => void; onOpenSource: (url: string) => void }) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const previewCanvasRef = useRef<HTMLCanvasElement>(null);
  const [cropSource, setCropSource] = useState<CropSource | null>(null);
  const [cropZoom, setCropZoom] = useState(1);
  const [cropFocusX, setCropFocusX] = useState(50);
  const [cropFocusY, setCropFocusY] = useState(50);
  const [cropBusy, setCropBusy] = useState(false);
  const [cropError, setCropError] = useState('');
  const cards = customWallpaper ? themes : [...themes, customThemeDefinition('')];

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

  function removeCustom() {
    if (!window.confirm('删除已保存的自定义皮肤并恢复默认吗？')) return;
    onCustomWallpaperChange('');
  }

  return <>
    <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon blue"><Palette size={20} /></div><div><p className="eyebrow">LOCAL APPEARANCE</p><h2>外观皮肤</h2><p>选择人物、动漫和风景背景，偏好会自动保存在这台设备上。</p></div></section><section className="theme-panel"><div className="theme-grid">{cards.map((item) => <button type="button" key={item.id} className={`theme-card ${theme === item.id ? 'active' : ''} ${item.dark ? 'dark-preview' : ''}`} onClick={() => item.id === 'custom' && !customWallpaper ? openPicker() : onThemeChange(item.id)} aria-pressed={theme === item.id}><span className="theme-preview" style={{ '--theme-preview-bg': item.swatches[2], '--theme-preview-sidebar': item.swatches[0], '--theme-preview-accent': item.swatches[1], '--theme-preview-positive': item.swatches[3], '--theme-preview-wallpaper': item.wallpaper ? `url(${item.wallpaper})` : 'none' } as CSSProperties}><span className="theme-preview-sidebar" /><span className="theme-preview-main"><i /><i /><b /></span></span><span className="theme-card-copy"><strong>{item.id === 'custom' && !customWallpaper ? '自定义皮肤' : item.name}</strong><span>{item.id === 'custom' && !customWallpaper ? '上传图片并裁剪应用' : item.subtitle}</span><em>{item.sourceURL ? <span className="theme-source" onClick={(event) => { event.stopPropagation(); onOpenSource(item.sourceURL as string); }}>{item.source} <ArrowUpRight size={11} /></span> : item.source}</em></span>{theme === item.id && <span className="theme-selected" aria-label="当前使用"><Check size={14} /></span>}</button>)}<button type="button" className="theme-upload-card" onClick={openPicker}><ImagePlus size={21} /><span><strong>上传新图片</strong><small>选择图片后可调整裁剪区域</small></span></button></div>{cropError && !cropSource && <div className="theme-inline-error"><AlertTriangle size={15} />{cropError}</div>}<div className="theme-panel-footer"><span>当前皮肤：<strong>{cards.find((item) => item.id === theme)?.name || '词元神青'}</strong></span><span>{customWallpaper ? <button type="button" className="theme-delete" onClick={removeCustom}><Trash2 size={13} />删除自定义图片</button> : '图片随安装包提供，离线也能使用'}</span></div></section></div>
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

function Updates({ update, busy, check, openDownload }: { update: UpdateInfo | null; busy: string; check: () => void; openDownload: () => void }) {
  return <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon blue"><Download size={20} /></div><div><p className="eyebrow">RELEASE CHANNEL</p><h2>版本更新</h2><p>通过 GitHub Release 获取最新安装版。</p></div><button className="primary-button" onClick={check} disabled={busy === 'update'}><RefreshCw size={16} className={busy === 'update' ? 'spin' : ''} />检查更新</button></section><section className="update-panel">{!update ? <EmptyState icon={<CircleDashed size={22} />} title="尚未检查" text="点击检查更新获取当前版本状态。" /> : update.error ? <div className="update-state error"><AlertTriangle size={22} /><div><strong>检查失败</strong><span>{update.error}</span></div></div> : update.updateAvailable ? <div className="update-state ready"><div className="update-state-icon"><Download size={20} /></div><div><strong>发现新版本 v{update.latestVersion}</strong><span>当前版本 v{update.currentVersion}{update.publishedAt ? ` · ${update.publishedAt}` : ''}</span></div><button className="primary-button" onClick={openDownload}><ArrowUpRight size={16} />打开下载</button></div> : <div className="update-state"><div className="update-state-icon"><CheckCircle2 size={20} /></div><div><strong>已经是最新版本</strong><span>当前版本 v{update.currentVersion} · 最新版本 v{update.latestVersion || update.currentVersion} · 检查于 {formatTime(update.checkedAt)}</span></div></div>}{update?.releaseNotes && <div className="release-notes">{update.releaseNotes}</div>}</section></div>;
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
  const all = [{ id: 'gpt-5.6-sol' }, { id: 'claude-sonnet-4-5' }, { id: 'gemini-2.5-pro' }, { id: 'grok-4' }];
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
    message: '浏览器预览仅查询官方最新版本；本机版本和安装状态请在 Windows 安装版中查看',
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
    return { ...base, downloadUrl: 'https://claude.com/download', message: 'Claude Code客户端由应用内更新；本机版本请在 Windows 安装版中查看' };
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
