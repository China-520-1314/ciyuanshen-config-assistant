import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import {
  Activity,
  AlertTriangle,
  ArrowUpRight,
  BarChart3,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  CircleDashed,
  ClipboardCheck,
  Cloud,
  Download,
  Eye,
  EyeOff,
  FileCheck2,
  FolderArchive,
  KeyRound,
  Laptop,
  Layers3,
  LockKeyhole,
  Maximize2,
  Minus,
  RefreshCw,
  RotateCcw,
  ScanSearch,
  Settings2,
  ShieldCheck,
  Trash2,
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
import './App.css';
import {
  CheckClientConnections,
  CheckForUpdates,
  Configure,
  DeleteBackup,
  FetchModels,
  GetAppInfo,
  GetBackupRoot,
  GetPublicGroupRatios,
  ListBackups,
  OpenExternalURL,
  PreviewConfiguration,
  RestoreBackup,
  ScanEnvironment,
} from '../wailsjs/go/main/App';
import { Quit, WindowMinimise, WindowToggleMaximise } from '../wailsjs/runtime/runtime';

type ClientId = 'claude' | 'codex' | 'gemini' | 'grok' | 'opencode' | 'openclaw' | 'hermes';
type TabId = 'overview' | 'groups' | 'backups' | 'updates';
type BusyState = 'scan' | 'preview' | 'configure' | 'update' | 'restore' | 'delete' | 'groups' | 'check' | '';

type ClientStatus = {
  id: ClientId;
  name: string;
  installed: boolean;
  executablePath: string;
  configPath: string;
  configExists: boolean;
  configState: string;
  version: string;
  detail: string;
};

type EnvironmentReport = {
  os: string;
  home: string;
  scannedAt: string;
  clients: ClientStatus[];
};

type Model = { id: string; object?: string; owned_by?: string };
type ModelResponse = { models: Model[]; status: number; message?: string; endpoint: string };
type Preview = {
  files: { clientId: string; path: string; action: string }[];
  warnings: string[];
  error?: string;
};
type BackupFile = { clientId: string; originalPath: string; backupPath: string; exists: boolean };
type Backup = { id: string; createdAt: string; path: string; files: BackupFile[] };
type ConfigureResult = {
  success: boolean;
  backup?: Backup;
  files: { clientId: string; path: string; action: string }[];
  warnings: string[];
  error?: string;
  configured: string[];
  finishedAt: string;
};
type UpdateInfo = {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  downloadUrl?: string;
  releaseNotes?: string;
  publishedAt?: string;
  sha256?: string;
  checkedAt: string;
  error?: string;
};
type AppInfo = { name: string; version: string; updateManifestUrl: string; gatewayUrl: string };
type GroupRatio = { name: string; description: string; ratio: number };
type GroupRatioReport = { groups: GroupRatio[]; endpoint: string; fetchedAt: string };
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

const clientOrder: ClientId[] = ['claude', 'codex', 'gemini', 'grok', 'opencode', 'openclaw', 'hermes'];
const clientCopy: Record<ClientId, { short: string; badge: string }> = {
  claude: { short: 'Claude Code', badge: 'Anthropic' },
  codex: { short: 'ChatGPT/Codex Cli/Codex插件', badge: 'Responses' },
  gemini: { short: 'Gemini CLI', badge: 'Gemini API' },
  grok: { short: 'Grok Build', badge: 'Responses' },
  opencode: { short: 'OpenCode', badge: 'OpenAI compatible' },
  openclaw: { short: 'OpenClaw', badge: 'OpenAI compatible' },
  hermes: { short: 'Hermes Agent', badge: 'Chat Completions' },
};
const recommendedModels: Record<ClientId, string> = {
  claude: 'claude-sonnet-4-5',
  codex: 'gpt-5.6-sol',
  gemini: 'gemini-2.5-pro',
  grok: 'grok-4',
  opencode: 'gpt-5.6-sol',
  openclaw: 'gpt-5.6-sol',
  hermes: 'gpt-5.6-sol',
};
const clientLogos: Record<ClientId, string> = {
  claude: claudeLogo,
  codex: codexLogo,
  gemini: geminiLogo,
  grok: grokLogo,
  opencode: opencodeLogo,
  openclaw: openclawLogo,
  hermes: hermesLogo,
};
const tabTitles: Record<TabId, string> = {
  overview: '环境概览',
  groups: '分组倍率',
  backups: '配置备份',
  updates: '版本更新',
};

const mockEnvironment: EnvironmentReport = {
  os: 'browser',
  home: '~',
  scannedAt: new Date().toISOString(),
  clients: clientOrder.map((id, index) => ({
    id,
    name: clientCopy[id].short,
    installed: index < 4,
    executablePath: '',
    configPath: `~/.${id}/config`,
    configExists: index < 2,
    configState: index < 2 ? 'valid' : 'missing',
    version: '',
    detail: index < 4 ? '已检测到客户端' : '未检测到',
  })),
};

const mockModels: Model[] = [
  { id: 'gpt-5.6-sol', owned_by: 'ciyuanshen' },
  { id: 'claude-sonnet-4-5', owned_by: 'ciyuanshen' },
  { id: 'gemini-2.5-pro', owned_by: 'ciyuanshen' },
  { id: 'grok-4', owned_by: 'ciyuanshen' },
];

function inWails() {
  return Boolean((window as unknown as { go?: unknown }).go);
}

function formatTime(value?: string) {
  if (!value) return '尚未检查';
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}

function formatRatio(value: number) {
  return `${value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}x`;
}

function App() {
  const [tab, setTab] = useState<TabId>('overview');
  const [environment, setEnvironment] = useState<EnvironmentReport>(mockEnvironment);
  const [appInfo, setAppInfo] = useState<AppInfo>({
    name: '词元神配置助手',
    version: '0.2.1',
    updateManifestUrl: '',
    gatewayUrl: 'https://ciyuanshen.top/v1',
  });
  const [apiKey, setApiKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [models, setModels] = useState<Model[]>([]);
  const [modelStatus, setModelStatus] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const [modelMessage, setModelMessage] = useState('输入 Key 后检查连接');
  const [selected, setSelected] = useState<ClientId[]>([]);
  const [modelByClient, setModelByClient] = useState<Record<ClientId, string>>(recommendedModels);
  const [preview, setPreview] = useState<Preview | null>(null);
  const [pendingTargets, setPendingTargets] = useState<ClientId[] | null>(null);
  const [busy, setBusy] = useState<BusyState>('');
  const [feedback, setFeedback] = useState<{ tone: 'success' | 'error' | 'neutral'; text: string } | null>(null);
  const feedbackTimer = useRef<number | undefined>(undefined);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [backupRoot, setBackupRoot] = useState('');
  const [update, setUpdate] = useState<UpdateInfo | null>(null);
  const [groupReport, setGroupReport] = useState<GroupRatioReport | null>(null);
  const [connectionResults, setConnectionResults] = useState<Record<string, ClientConnectionResult>>({});

  const modelOptions = useMemo(() => {
    const unique = new Map<string, Model>();
    (models.length > 0 ? models : mockModels).forEach((model) => unique.set(model.id, model));
    return [...unique.values()];
  }, [models]);

  const clientMap = useMemo(() => new Map(environment.clients.map((client) => [client.id, client])), [environment.clients]);

  useEffect(() => {
    void loadInitialState();
  }, []);

  useEffect(() => () => {
    if (feedbackTimer.current !== undefined) window.clearTimeout(feedbackTimer.current);
  }, []);

  function clearFeedback() {
    if (feedbackTimer.current !== undefined) {
      window.clearTimeout(feedbackTimer.current);
      feedbackTimer.current = undefined;
    }
    setFeedback(null);
  }

  function showFeedback(next: { tone: 'success' | 'error' | 'neutral'; text: string }, dismissAfter = 0) {
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

  async function loadInitialState() {
    try {
      const [info, report, root] = inWails()
        ? await Promise.all([GetAppInfo(), ScanEnvironment(), GetBackupRoot()])
        : [appInfo, mockEnvironment, '~/.config/CiyuanShen/Config Assistant/backups'];
      setAppInfo(info as AppInfo);
      setEnvironment(report as EnvironmentReport);
      setSelected([]);
      setBackupRoot(root as string);
    } catch {
      showFeedback({ tone: 'error', text: '环境检测失败，请重试' });
    }
    await refreshBackups();
  }

  async function refreshEnvironment(options: { announce?: boolean; trackBusy?: boolean } = {}) {
    const { announce = true, trackBusy = true } = options;
    if (trackBusy) setBusy('scan');
    clearFeedback();
    try {
      const report = inWails() ? await ScanEnvironment() : mockEnvironment;
      setEnvironment(report as EnvironmentReport);
      if (announce) showFeedback({ tone: 'success', text: '环境检测已完成' });
    } catch {
      if (announce) showFeedback({ tone: 'error', text: '环境检测失败，请检查权限后重试' });
    } finally {
      if (trackBusy) setBusy('');
    }
  }

  async function fetchModels() {
    if (!apiKey.trim()) {
      setModelStatus('error');
      setModelMessage('请输入 API Key');
      return;
    }
    setModelStatus('loading');
    setModelMessage('正在检查网关连接');
    try {
      const result = inWails() ? await FetchModels(apiKey.trim()) : ({ models: mockModels, status: 200 } as ModelResponse);
      const response = result as ModelResponse;
      if (!response.models?.length) throw new Error(response.message || '没有可用模型');
      setModels(response.models);
      setModelStatus('ready');
      setModelMessage(`连接正常，可用模型 ${response.models.length} 个`);
    } catch (error) {
      setModelStatus('error');
      setModelMessage(error instanceof Error ? error.message : '连接失败，请检查 Key');
    }
  }

  function toggleClient(id: ClientId) {
    setSelected((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  }

  function setClientModel(id: ClientId, value: string) {
    setModelByClient((current) => ({ ...current, [id]: value }));
  }

  function requestPayload(targets: ClientId[] = selected) {
    const modelsForRequest: Record<string, string> = { ...modelByClient };
    targets.forEach((id) => {
      if (id !== 'codex') modelsForRequest[id] = modelsForRequest[id]?.trim() || recommendedModels[id];
    });
    return { apiKey: apiKey.trim(), targets, models: modelsForRequest };
  }

  async function previewConfiguration(targets: ClientId[] = selected) {
    if (!apiKey.trim()) {
      showFeedback({ tone: 'error', text: '请先输入 API Key' });
      return;
    }
    if (targets.length === 0) {
      showFeedback({ tone: 'error', text: '至少选择一个工具' });
      return;
    }
    setBusy('preview');
    try {
      const result = inWails() ? await PreviewConfiguration(requestPayload(targets)) : mockPreview();
      const nextPreview = result as Preview;
      setPreview(nextPreview);
      if (nextPreview.error) {
        setPendingTargets(null);
        showFeedback({ tone: 'error', text: nextPreview.error });
      } else {
        setPendingTargets([...targets]);
      }
    } catch {
      setPendingTargets(null);
      showFeedback({ tone: 'error', text: '无法生成配置预览' });
    } finally {
      setBusy('');
    }
  }

  async function applyConfiguration() {
    const targets = pendingTargets ?? selected;
    if (!apiKey.trim() || targets.length === 0) {
      showFeedback({ tone: 'error', text: '请先输入 Key 并选择工具' });
      return;
    }
    let configuredTargets: ClientId[] = [];
    setBusy('configure');
    try {
      const result = inWails() ? await Configure(requestPayload(targets)) : mockConfigure();
      const configuration = result as ConfigureResult;
      if (!configuration.success) throw new Error(configuration.error || '配置失败');
      setPreview(null);
      setPendingTargets(null);
      setConnectionResults((current) => {
        const next = { ...current };
        configuration.configured.forEach((id) => delete next[id]);
        return next;
      });
      configuredTargets = configuration.configured.filter((id): id is ClientId => clientOrder.includes(id as ClientId));
      await Promise.all([refreshBackups(), refreshEnvironment({ announce: false, trackBusy: false })]);
      if (configuredTargets.length !== 1) {
        showFeedback({ tone: 'success', text: `已备份并配置 ${configuration.configured.length} 个工具` });
      }
    } catch (error) {
      showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '配置失败，原文件未被覆盖' });
    } finally {
      setBusy('');
    }
    if (configuredTargets.length === 1) await checkConnections(configuredTargets, true);
  }

  function configureOneClient(id: ClientId) {
    void previewConfiguration([id]);
  }

  async function refreshBackups() {
    try {
      const result = inWails() ? await ListBackups() : [];
      setBackups(result as Backup[]);
    } catch {
      setBackups([]);
    }
  }

  async function restore(id: string) {
    setBusy('restore');
    try {
      if (inWails()) await RestoreBackup(id);
      await refreshEnvironment({ announce: false, trackBusy: false });
      showFeedback({ tone: 'success', text: '备份已恢复，请重启对应工具' });
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
      showFeedback({ tone: 'success', text: '备份已删除' });
      await refreshBackups();
    } catch (error) {
      showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '删除备份失败' });
    } finally {
      setBusy('');
    }
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

  async function checkConnections(targets: ClientId[], autoDismiss = false) {
    if (targets.length === 0) {
      showFeedback({ tone: 'error', text: '至少选择一个要检测的工具' }, autoDismiss ? 3000 : 0);
      return;
    }
    setBusy('check');
    try {
      const result = inWails()
        ? await CheckClientConnections({ targets })
        : mockConnectionCheck(targets);
      const report = result as ConnectionCheckReport;
      const nextResults = Object.fromEntries(report.results.map((item) => [item.id, item]));
      setConnectionResults((current) => ({ ...current, ...nextResults }));
      const failed = report.results.filter((item) => !item.success);
      const notification: { tone: 'success' | 'error' | 'neutral'; text: string } = failed.length === 0
        ? { tone: 'success', text: `已通过 ${report.results.length} 个工具的配置与网关检测` }
        : report.results.length === 1
          ? { tone: 'error', text: `${failed[0].name} 未通过检测：${failed[0].message}` }
          : { tone: 'error', text: `${failed.length} 个工具未通过检测，请查看工具卡片提示` };
      showFeedback(notification, autoDismiss ? 3000 : 0);
    } catch (error) {
      showFeedback({ tone: 'error', text: error instanceof Error ? error.message : '检测失败' }, autoDismiss ? 3000 : 0);
    } finally {
      setBusy('');
    }
  }

  async function checkUpdate() {
    setBusy('update');
    try {
      const result = inWails()
        ? await CheckForUpdates()
        : ({ currentVersion: appInfo.version, latestVersion: appInfo.version, updateAvailable: false, checkedAt: new Date().toISOString() } as UpdateInfo);
      setUpdate(result as UpdateInfo);
    } catch {
      setUpdate({ currentVersion: appInfo.version, latestVersion: '', updateAvailable: false, checkedAt: new Date().toISOString(), error: '暂时无法检查更新' });
    } finally {
      setBusy('');
    }
  }

  async function openDownload() {
    if (update?.downloadUrl && inWails()) await OpenExternalURL(update.downloadUrl);
    else if (update?.downloadUrl) window.open(update.downloadUrl, '_blank', 'noopener,noreferrer');
  }

  function selectTab(nextTab: TabId) {
    setTab(nextTab);
    if (nextTab === 'groups') void fetchGroupRatios();
    if (nextTab === 'updates') void checkUpdate();
  }

  return (
    <div className="window-frame">
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
          </nav>
          <div className="sidebar-bottom">
            <div className="secure-note"><LockKeyhole size={16} /><span>Key 仅用于本次操作</span></div>
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
              <div className="gateway-pill"><span className="status-dot" />词元神网关 <code>/v1</code></div>
              <button className="icon-button" title="重新检测环境" onClick={() => void refreshEnvironment()} disabled={busy === 'scan'}>
                <RefreshCw size={17} className={busy === 'scan' ? 'spin' : ''} />
              </button>
            </div>
          </header>

          {feedback && <Feedback tone={feedback.tone} text={feedback.text} onClose={clearFeedback} />}

          {tab === 'overview' && (
            <Overview
              environment={environment}
              clientMap={clientMap}
              selected={selected}
              toggleClient={toggleClient}
              models={modelOptions}
              modelByClient={modelByClient}
              setClientModel={setClientModel}
              apiKey={apiKey}
              setApiKey={setApiKey}
              showKey={showKey}
              setShowKey={setShowKey}
              modelStatus={modelStatus}
              modelMessage={modelMessage}
              fetchModels={() => void fetchModels()}
              previewConfiguration={() => void previewConfiguration()}
              configureOneClient={configureOneClient}
              checkConnections={(targets) => void checkConnections(targets)}
              connectionResults={connectionResults}
              busy={busy}
            />
          )}
          {tab === 'groups' && <GroupRatios report={groupReport} busy={busy} refresh={() => void fetchGroupRatios()} />}
          {tab === 'backups' && <Backups backups={backups} backupRoot={backupRoot} busy={busy} restore={restore} remove={deleteBackup} refresh={() => void refreshBackups()} />}
          {tab === 'updates' && <Updates update={update} busy={busy} check={checkUpdate} openDownload={() => void openDownload()} />}
        </main>
      </div>

      {preview && <PreviewModal preview={preview} busy={busy} close={() => { setPreview(null); setPendingTargets(null); }} apply={() => void applyConfiguration()} />}
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

function NavButton({ active, icon, label, count, onClick }: { active: boolean; icon: ReactNode; label: string; count?: number; onClick: () => void }) {
  return <button className={`nav-button ${active ? 'active' : ''}`} onClick={onClick}>{icon}<span>{label}</span>{count ? <b>{count}</b> : <ChevronRight size={15} className="nav-arrow" />}</button>;
}

function Overview(props: {
  environment: EnvironmentReport;
  clientMap: Map<ClientId, ClientStatus>;
  selected: ClientId[];
  toggleClient: (id: ClientId) => void;
  models: Model[];
  modelByClient: Record<string, string>;
  setClientModel: (id: ClientId, value: string) => void;
  apiKey: string;
  setApiKey: (value: string) => void;
  showKey: boolean;
  setShowKey: (value: boolean) => void;
  modelStatus: 'idle' | 'loading' | 'ready' | 'error';
  modelMessage: string;
  fetchModels: () => void;
  previewConfiguration: () => void;
  configureOneClient: (id: ClientId) => void;
  checkConnections: (targets: ClientId[]) => void;
  connectionResults: Record<string, ClientConnectionResult>;
  busy: BusyState;
}) {
  const { environment, clientMap, selected, toggleClient, models, modelByClient, setClientModel, apiKey, setApiKey, showKey, setShowKey, modelStatus, modelMessage, fetchModels, previewConfiguration, configureOneClient, checkConnections, connectionResults, busy } = props;
  return (
    <div className="content-stack">
      <section className="summary-band">
        <div className="summary-copy"><div className="section-icon green"><ShieldCheck size={20} /></div><div><h2>配置状态</h2><p>检测到 {environment.clients.filter((client) => client.installed || client.configExists).length} / {environment.clients.length} 个客户端</p></div></div>
        <div className="summary-meta"><span className="meta-label">最近检测</span><strong>{formatTime(environment.scannedAt)}</strong><span className="platform-label"><Laptop size={14} /> {environment.os}</span></div>
      </section>

      <section className="key-panel">
        <div className="panel-heading"><div><p className="eyebrow">CREDENTIAL</p><h2>连接词元神网关</h2></div><div className="key-security"><LockKeyhole size={15} /> Key 不会上传到配置助手</div></div>
        <div className="key-row">
          <div className="key-input-wrap"><KeyRound size={17} /><input type={showKey ? 'text' : 'password'} placeholder="粘贴用户 API Key" value={apiKey} onChange={(event) => setApiKey(event.target.value)} autoComplete="off" /><button className="input-action" title={showKey ? '隐藏 Key' : '显示 Key'} onClick={() => setShowKey(!showKey)}>{showKey ? <EyeOff size={17} /> : <Eye size={17} />}</button></div>
          <button className="primary-button" onClick={fetchModels} disabled={modelStatus === 'loading'}><Cloud size={17} />{modelStatus === 'loading' ? '检查中' : '检查连接'}</button>
        </div>
        <div className={`connection-line ${modelStatus}`}><span className="connection-mark">{modelStatus === 'ready' ? <CheckCircle2 size={15} /> : modelStatus === 'error' ? <AlertTriangle size={15} /> : <CircleDashed size={15} />}</span><span>{modelMessage}</span>{modelStatus === 'ready' && <span className="connection-endpoint">GET /v1/models</span>}</div>
      </section>

      <section className="clients-section">
        <div className="section-heading"><div><p className="eyebrow">TARGETS</p><h2>选择要配置的工具</h2></div><span className="selected-count">{selected.length} 个已选择</span></div>
        <div className="client-grid">
          {clientOrder.map((id) => <ClientCard key={id} id={id} status={clientMap.get(id)} checked={selected.includes(id)} onToggle={() => toggleClient(id)} models={models} model={modelByClient[id] || recommendedModels[id]} onModelChange={(value) => setClientModel(id, value)} result={connectionResults[id]} onCheck={() => checkConnections([id])} onConfigure={() => configureOneClient(id)} busy={busy} />)}
        </div>
      </section>

      <div className="action-bar"><div><strong>{selected.length ? `将配置 ${selected.length} 个工具` : '请选择工具'}</strong><span>写入前会备份当前配置，再替换为词元神最新配置</span></div><div className="action-buttons"><button className="secondary-button" onClick={() => checkConnections(selected)} disabled={busy !== '' || selected.length === 0}><Activity size={17} />检测已选</button><button className="primary-button" onClick={previewConfiguration} disabled={busy !== '' || selected.length === 0}><ClipboardCheck size={17} />备份并配置</button></div></div>
    </div>
  );
}

function ClientCard({ id, status, checked, onToggle, models, model, onModelChange, result, onCheck, onConfigure, busy }: { id: ClientId; status?: ClientStatus; checked: boolean; onToggle: () => void; models: Model[]; model: string; onModelChange: (value: string) => void; result?: ClientConnectionResult; onCheck: () => void; onConfigure: () => void; busy: BusyState }) {
  const available = Boolean(status?.installed || status?.configExists);
  const state = status?.configState === 'invalid' || status?.configState === 'error' ? 'invalid' : available ? 'available' : 'not-found';
  return <article className={`client-card ${checked ? 'checked' : ''}`}>
    <div className="client-card-top"><button className={`check-control ${checked ? 'checked' : ''}`} aria-label={`${checked ? '取消选择' : '选择'} ${clientCopy[id].short}`} onClick={onToggle}>{checked && <Check size={14} strokeWidth={3} />}</button><div className="client-symbol"><img src={clientLogos[id]} alt="" /></div><div className="client-name"><strong>{clientCopy[id].short}</strong><span>{clientCopy[id].badge}</span></div><span className={`state-tag ${state}`}>{state === 'available' ? '可配置' : state === 'invalid' ? '需修复' : '未安装'}</span></div>
    <div className="client-card-path">{status?.configPath || '配置文件将自动创建'}</div>
    {id === 'codex'
      ? <div className="client-card-bottom fixed-model"><label>固定模板</label><span>review_model · gpt-5.6-sol</span></div>
      : <div className="client-card-bottom"><label>默认模型</label><input list={`models-${id}`} value={model} placeholder="输入或选择模型" onChange={(event) => onModelChange(event.target.value)} /><datalist id={`models-${id}`}>{models.map((option) => <option key={option.id} value={option.id} />)}</datalist></div>}
    <div className="client-card-actions"><span className={`check-result ${result ? (result.success ? 'success' : 'error') : ''}`}>{result ? (result.success ? <><CheckCircle2 size={14} />已通过</> : <><AlertTriangle size={14} />未通过</>) : <><CircleDashed size={14} />未检测</>}</span><button className="secondary-button compact" onClick={onCheck} disabled={busy !== ''}><Activity size={15} />检测</button><button className="primary-button compact" onClick={onConfigure} disabled={busy !== ''}><Settings2 size={15} />一键配置</button></div>
    {result && <p className={`client-check-message ${result.success ? 'success' : 'error'}`}>{result.message}</p>}
  </article>;
}

function GroupRatios({ report, busy, refresh }: { report: GroupRatioReport | null; busy: BusyState; refresh: () => void }) {
  return <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon blue"><Layers3 size={20} /></div><div><p className="eyebrow">PUBLIC ROUTING</p><h2>分组倍率</h2><p>当前词元神公开可见分组与基础倍率。</p></div><button className="primary-button" onClick={refresh} disabled={busy === 'groups'}><RefreshCw size={16} className={busy === 'groups' ? 'spin' : ''} />刷新倍率</button></section>{!report ? <section className="group-table"><EmptyState icon={<BarChart3 size={22} />} title="尚未读取" text="点击刷新倍率获取当前分组。" /></section> : <section className="group-table"><div className="group-table-header"><span>分组</span><span>基础</span><span>月卡</span><span>周卡</span></div>{report.groups.map((group) => <div className="group-row" key={group.name}><div><strong>{group.name}</strong><span>{group.description || '暂无分组说明'}</span></div><b className="ratio base">{formatRatio(group.ratio)}</b><b className="ratio month">{formatRatio(group.ratio * 0.85)}</b><b className="ratio week">{formatRatio(group.ratio * 0.9)}</b></div>)}<div className="group-table-footer">{report.groups.length} 个公开分组 · 更新于 {formatTime(report.fetchedAt)} · 月卡/周卡为参考，实际计费以词元神系统为准</div></section>}</div>;
}

function Backups({ backups, backupRoot, busy, restore, remove, refresh }: { backups: Backup[]; backupRoot: string; busy: BusyState; restore: (id: string) => void; remove: (id: string) => void; refresh: () => void }) {
  const [expanded, setExpanded] = useState<string | null>(null);
  return <div className="content-stack narrow-stack"><section className="page-intro backup-intro"><div className="section-icon amber"><RotateCcw size={20} /></div><div><p className="eyebrow">RECOVERY</p><h2>配置备份</h2><p className="backup-root"><FolderArchive size={13} /><code>{backupRoot || '正在读取备份目录'}</code></p></div><button className="icon-button inline" title="刷新备份" onClick={refresh}><RefreshCw size={17} /></button></section><section className="backup-list">{backups.length === 0 ? <EmptyState icon={<RotateCcw size={22} />} title="暂无备份" text="完成一次配置后，备份会显示在这里。" /> : backups.map((backup) => <article className="backup-entry" key={backup.id}><div className="backup-row"><div className="backup-icon"><FileCheck2 size={18} /></div><div className="backup-details"><strong>{formatTime(backup.createdAt)}</strong><span>{backup.files.length} 个文件 · {backup.path}</span></div><button className="icon-button compact-icon" title={expanded === backup.id ? '收起备份文件' : '查看备份文件'} onClick={() => setExpanded(expanded === backup.id ? null : backup.id)}>{expanded === backup.id ? <ChevronUp size={16} /> : <ChevronDown size={16} />}</button><button className="secondary-button compact" onClick={() => restore(backup.id)} disabled={busy === 'restore' || busy === 'delete'}><RotateCcw size={15} />恢复</button><button className="icon-button compact-icon danger-button" title="删除备份" onClick={() => remove(backup.id)} disabled={busy === 'restore' || busy === 'delete'}><Trash2 size={16} /></button></div>{expanded === backup.id && <div className="backup-files">{backup.files.map((file) => <div key={`${file.clientId}-${file.originalPath}`}><strong>{clientCopy[file.clientId as ClientId]?.short || file.clientId}</strong><span>{file.originalPath}</span><code>{file.exists ? file.backupPath : '原文件当时不存在'}</code></div>)}</div>}</article>)}</section></div>;
}

function Updates({ update, busy, check, openDownload }: { update: UpdateInfo | null; busy: BusyState; check: () => void; openDownload: () => void }) {
  return <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon blue"><Download size={20} /></div><div><p className="eyebrow">RELEASE CHANNEL</p><h2>版本更新</h2><p>通过 GitHub Release 获取最新安装版。</p></div><button className="primary-button" onClick={check} disabled={busy === 'update'}><RefreshCw size={16} className={busy === 'update' ? 'spin' : ''} />检查更新</button></section><section className="update-panel">{!update ? <EmptyState icon={<CircleDashed size={22} />} title="尚未检查" text="点击检查更新获取当前版本状态。" /> : update.error ? <div className="update-state error"><AlertTriangle size={22} /><div><strong>检查失败</strong><span>{update.error}</span></div></div> : update.updateAvailable ? <div className="update-state ready"><div className="update-state-icon"><Download size={20} /></div><div><strong>发现新版本 v{update.latestVersion}</strong><span>当前版本 v{update.currentVersion}{update.publishedAt ? ` · ${update.publishedAt}` : ''}</span></div><button className="primary-button" onClick={openDownload}><ArrowUpRight size={16} />打开下载</button></div> : <div className="update-state"><div className="update-state-icon"><CheckCircle2 size={20} /></div><div><strong>已经是最新版本</strong><span>v{update.currentVersion} · 检查于 {formatTime(update.checkedAt)}</span></div></div>}{update?.releaseNotes && <div className="release-notes">{update.releaseNotes}</div>}</section></div>;
}

function PreviewModal({ preview, busy, close, apply }: { preview: Preview; busy: BusyState; close: () => void; apply: () => void }) {
  return <div className="modal-backdrop" role="presentation"><section className="preview-modal" role="dialog" aria-modal="true" aria-labelledby="preview-title"><div className="modal-heading"><div><p className="eyebrow">SAFE REPLACEMENT</p><h2 id="preview-title">备份并替换配置？</h2></div><button className="icon-button" title="关闭" onClick={close}><X size={18} /></button></div>{preview.error ? <div className="modal-error"><AlertTriangle size={18} />{preview.error}</div> : <><p className="modal-copy">将先备份当前配置，再写入词元神最新配置。确认后可在“配置备份”中恢复或删除历史版本。</p><div className="preview-summary"><FileCheck2 size={19} /><span>{preview.files.length} 个文件将被创建或替换</span><span className="preview-safe"><ShieldCheck size={15} />自动备份</span></div><div className="file-list">{preview.files.map((file) => <div className="file-row" key={`${file.clientId}-${file.path}`}><span className={`file-action ${file.action}`}>{file.action === 'create' ? '+' : '~'}</span><div><strong>{clientCopy[file.clientId as ClientId]?.short || file.clientId}</strong><span>{file.path}</span></div></div>)}</div>{preview.warnings.length > 0 && <div className="warning-list">{preview.warnings.map((warning) => <div key={warning}><AlertTriangle size={15} />{warning}</div>)}</div>}<div className="modal-actions"><button className="secondary-button" onClick={close}>否，取消</button><button className="primary-button" onClick={apply} disabled={busy === 'configure'}><ClipboardCheck size={17} />{busy === 'configure' ? '备份并写入中' : '是，备份并替换'}</button></div></>}</section></div>;
}

function Feedback({ tone, text, onClose }: { tone: 'success' | 'error' | 'neutral'; text: string; onClose: () => void }) {
  const icon = tone === 'success' ? <CheckCircle2 size={17} /> : tone === 'error' ? <AlertTriangle size={17} /> : <Settings2 size={17} />;
  return <div className={`feedback ${tone}`}>{icon}<span>{text}</span><button className="feedback-close" title="关闭" onClick={onClose}><X size={15} /></button></div>;
}

function EmptyState({ icon, title, text }: { icon: ReactNode; title: string; text: string }) {
  return <div className="empty-state"><div className="empty-icon">{icon}</div><strong>{title}</strong><span>{text}</span></div>;
}

function mockPreview(): Preview {
  return { files: [{ clientId: 'claude', path: '~/.claude/settings.json', action: 'update' }, { clientId: 'codex', path: '~/.codex/config.toml', action: 'update' }, { clientId: 'codex', path: '~/.codex/auth.json', action: 'update' }], warnings: [] };
}

function mockConfigure(): ConfigureResult {
  return { success: true, configured: ['claude', 'codex', 'gemini'], files: [], warnings: [], finishedAt: new Date().toISOString() };
}

function mockGroupRatios(): GroupRatioReport {
  return { endpoint: 'https://ciyuanshen.top/api/user/groups', fetchedAt: new Date().toISOString(), groups: [{ name: 'GPT低价', description: '示例低价分组', ratio: 0.1 }, { name: 'Gemini', description: '示例 Gemini 分组', ratio: 0.4 }, { name: '默认', description: '示例默认分组', ratio: 1 }] };
}

function mockConnectionCheck(targets: ClientId[]): ConnectionCheckReport {
  const checkedAt = new Date().toISOString();
  return { checkedAt, results: targets.map((id) => ({ id, name: clientCopy[id].short, success: true, configured: true, status: 200, endpoint: 'https://ciyuanshen.top/v1/models', message: '配置文件与词元神网关均可用', checkedAt })) };
}

export default App;
