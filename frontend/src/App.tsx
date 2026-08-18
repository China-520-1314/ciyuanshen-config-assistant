import { useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  AlertTriangle,
  ArrowUpRight,
  Check,
  CheckCircle2,
  ChevronRight,
  CircleDashed,
  ClipboardCheck,
  Cloud,
  Download,
  Eye,
  EyeOff,
  FileCheck2,
  KeyRound,
  Laptop,
  LockKeyhole,
  RefreshCw,
  RotateCcw,
  ScanSearch,
  Settings2,
  ShieldCheck,
  TerminalSquare,
  X,
} from 'lucide-react';
import logo from './assets/images/logo-universal.png';
import './App.css';
import {
  CheckForUpdates,
  Configure,
  FetchModels,
  GetAppInfo,
  ListBackups,
  OpenExternalURL,
  PreviewConfiguration,
  RestoreBackup,
  ScanEnvironment,
} from '../wailsjs/go/main/App';

type ClientId = 'claude' | 'codex' | 'gemini' | 'grok' | 'opencode' | 'openclaw' | 'hermes';
type TabId = 'overview' | 'backups' | 'updates';

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
type ConfigureResult = {
  success: boolean;
  backup?: Backup;
  files: { clientId: string; path: string; action: string }[];
  warnings: string[];
  error?: string;
  configured: string[];
  finishedAt: string;
};
type Backup = {
  id: string;
  createdAt: string;
  path: string;
  files: { clientId: string; originalPath: string; exists: boolean }[];
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

const clientOrder: ClientId[] = ['claude', 'codex', 'gemini', 'grok', 'opencode', 'openclaw', 'hermes'];
const clientCopy: Record<ClientId, { short: string; badge: string }> = {
  claude: { short: 'Claude Code', badge: 'Anthropic' },
  codex: { short: 'Codex', badge: 'Responses' },
  gemini: { short: 'Gemini CLI', badge: 'Gemini API' },
  grok: { short: 'Grok Build', badge: 'Responses' },
  opencode: { short: 'OpenCode', badge: 'OpenAI compatible' },
  openclaw: { short: 'OpenClaw', badge: 'OpenAI compatible' },
  hermes: { short: 'Hermes Agent', badge: 'Chat Completions' },
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
  { id: 'claude-sonnet-5', owned_by: 'ciyuanshen' },
  { id: 'gemini-3.6-flash', owned_by: 'ciyuanshen' },
  { id: 'grok-4.5', owned_by: 'ciyuanshen' },
];

function inWails() {
  return Boolean((window as unknown as { go?: unknown }).go);
}

function initialSelection(report: EnvironmentReport) {
  const detected = report.clients.filter((client) => client.installed || client.configExists).map((client) => client.id);
  return detected.length > 0 ? detected : clientOrder.slice(0, 4);
}

function formatTime(value?: string) {
  if (!value) return '尚未检查';
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}

function App() {
  const [tab, setTab] = useState<TabId>('overview');
  const [environment, setEnvironment] = useState<EnvironmentReport>(mockEnvironment);
  const [appInfo, setAppInfo] = useState<AppInfo>({
    name: '词元神配置助手',
    version: '0.1.1',
    updateManifestUrl: '',
    gatewayUrl: 'https://ciyuanshen.top/v1',
  });
  const [apiKey, setApiKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [models, setModels] = useState<Model[]>([]);
  const [modelStatus, setModelStatus] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const [modelMessage, setModelMessage] = useState('输入 Key 后检查连接');
  const [selected, setSelected] = useState<ClientId[]>(initialSelection(mockEnvironment));
  const [modelByClient, setModelByClient] = useState<Record<string, string>>({ default: mockModels[0].id });
  const [preview, setPreview] = useState<Preview | null>(null);
  const [busy, setBusy] = useState<'scan' | 'preview' | 'configure' | 'update' | 'restore' | ''>('');
  const [feedback, setFeedback] = useState<{ tone: 'success' | 'error' | 'neutral'; text: string } | null>(null);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [update, setUpdate] = useState<UpdateInfo | null>(null);

  const modelOptions = useMemo(() => {
    const unique = new Map<string, Model>();
    [...models, ...mockModels].forEach((model) => unique.set(model.id, model));
    return [...unique.values()];
  }, [models]);

  const clientMap = useMemo(() => {
    return new Map(environment.clients.map((client) => [client.id, client]));
  }, [environment.clients]);

  useEffect(() => {
    void loadInitialState();
  }, []);

  async function loadInitialState() {
    try {
      const [info, report] = inWails()
        ? await Promise.all([GetAppInfo(), ScanEnvironment()])
        : [appInfo, mockEnvironment];
      setAppInfo(info as AppInfo);
      setEnvironment(report as EnvironmentReport);
      setSelected(initialSelection(report as EnvironmentReport));
    } catch {
      setFeedback({ tone: 'error', text: '环境检测失败，请重试' });
    }
    await refreshBackups();
  }

  async function refreshEnvironment() {
    setBusy('scan');
    setFeedback(null);
    try {
      const report = inWails() ? await ScanEnvironment() : mockEnvironment;
      setEnvironment(report as EnvironmentReport);
      setSelected(initialSelection(report as EnvironmentReport));
      setFeedback({ tone: 'success', text: '环境检测已完成' });
    } catch {
      setFeedback({ tone: 'error', text: '环境检测失败，请检查权限后重试' });
    } finally {
      setBusy('');
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
      setModelByClient((current) => ({ default: current.default || response.models[0].id, ...current }));
    } catch (error) {
      setModelStatus('error');
      setModelMessage(error instanceof Error ? error.message : '连接失败，请检查 Key');
    }
  }

  function toggleClient(id: ClientId) {
    setSelected((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  }

  function setClientModel(id: ClientId, value: string) {
    setModelByClient((current) => ({ ...current, [id]: value, default: current.default || value }));
  }

  function requestPayload() {
    const modelsForRequest: Record<string, string> = { ...modelByClient };
    selected.forEach((id) => {
      modelsForRequest[id] = modelsForRequest[id] || modelsForRequest.default || modelOptions[0]?.id || '';
    });
    return { apiKey, targets: selected, models: modelsForRequest };
  }

  async function previewConfiguration() {
    if (!apiKey.trim()) {
      setFeedback({ tone: 'error', text: '请先输入 API Key' });
      return;
    }
    if (selected.length === 0) {
      setFeedback({ tone: 'error', text: '至少选择一个工具' });
      return;
    }
    setBusy('preview');
    try {
      const result = inWails() ? await PreviewConfiguration(requestPayload()) : mockPreview();
      const nextPreview = result as Preview;
      setPreview(nextPreview);
      if (nextPreview.error) setFeedback({ tone: 'error', text: nextPreview.error });
    } catch {
      setFeedback({ tone: 'error', text: '无法生成配置预览' });
    } finally {
      setBusy('');
    }
  }

  async function applyConfiguration() {
    if (!apiKey.trim() || selected.length === 0) {
      setFeedback({ tone: 'error', text: '请先输入 Key 并选择工具' });
      return;
    }
    setBusy('configure');
    try {
      const result = inWails() ? await Configure(requestPayload()) : mockConfigure();
      const configuration = result as ConfigureResult;
      if (!configuration.success) throw new Error(configuration.error || '配置失败');
      setPreview(null);
      setFeedback({ tone: 'success', text: `已配置 ${configuration.configured.length} 个工具，备份已保存` });
      await refreshBackups();
      await refreshEnvironment();
    } catch (error) {
      setFeedback({ tone: 'error', text: error instanceof Error ? error.message : '配置失败，原文件未被覆盖' });
    } finally {
      setBusy('');
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

  async function restore(id: string) {
    setBusy('restore');
    try {
      if (inWails()) await RestoreBackup(id);
      setFeedback({ tone: 'success', text: '备份已恢复，请重启对应工具' });
      await refreshEnvironment();
    } catch (error) {
      setFeedback({ tone: 'error', text: error instanceof Error ? error.message : '恢复失败' });
    } finally {
      setBusy('');
    }
  }

  async function checkUpdate() {
    setBusy('update');
    try {
      const result = inWails() ? await CheckForUpdates() : ({ currentVersion: appInfo.version, latestVersion: appInfo.version, updateAvailable: false, checkedAt: new Date().toISOString() } as UpdateInfo);
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

  const selectedCount = selected.length;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand-lockup">
          <div className="brand-mark"><img src={logo} alt="" /></div>
          <div><strong>词元神</strong><span>配置助手</span></div>
        </div>
        <div className="sidebar-rule" />
        <nav className="side-nav" aria-label="主导航">
          <NavButton active={tab === 'overview'} icon={<ScanSearch size={17} />} label="环境概览" onClick={() => setTab('overview')} />
          <NavButton active={tab === 'backups'} icon={<RotateCcw size={17} />} label="配置备份" onClick={() => setTab('backups')} count={backups.length || undefined} />
          <NavButton active={tab === 'updates'} icon={<Download size={17} />} label="版本更新" onClick={() => { setTab('updates'); void checkUpdate(); }} />
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
            <h1>{tab === 'overview' ? '环境概览' : tab === 'backups' ? '配置备份' : '版本更新'}</h1>
          </div>
          <div className="topbar-actions">
            <div className="gateway-pill"><span className="status-dot" />词元神网关 <code>/v1</code></div>
            <button className="icon-button" title="重新检测环境" onClick={() => void refreshEnvironment()} disabled={busy === 'scan'}>
              <RefreshCw size={17} className={busy === 'scan' ? 'spin' : ''} />
            </button>
          </div>
        </header>

        {feedback && <Feedback tone={feedback.tone} text={feedback.text} onClose={() => setFeedback(null)} />}

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
            selectedCount={selectedCount}
            previewConfiguration={() => void previewConfiguration()}
            applyConfiguration={() => void applyConfiguration()}
            busy={busy}
          />
        )}
        {tab === 'backups' && <Backups backups={backups} busy={busy} restore={restore} refresh={() => void refreshBackups()} />}
        {tab === 'updates' && <Updates update={update} busy={busy} check={checkUpdate} openDownload={() => void openDownload()} />}
      </main>

      {preview && <PreviewModal preview={preview} busy={busy} close={() => setPreview(null)} apply={() => void applyConfiguration()} />}
    </div>
  );
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
  selectedCount: number;
  previewConfiguration: () => void;
  applyConfiguration: () => void;
  busy: string;
}) {
  const { environment, clientMap, selected, toggleClient, models, modelByClient, setClientModel, apiKey, setApiKey, showKey, setShowKey, modelStatus, modelMessage, fetchModels, selectedCount, previewConfiguration, applyConfiguration, busy } = props;
  return (
    <div className="content-stack">
      <section className="summary-band">
        <div className="summary-copy"><div className="section-icon green"><ShieldCheck size={20} /></div><div><h2>配置状态</h2><p>检测到 {environment.clients.filter((client) => client.installed || client.configExists).length} / {environment.clients.length} 个客户端</p></div></div>
        <div className="summary-meta"><span className="meta-label">最近检测</span><strong>{formatTime(environment.scannedAt)}</strong><span className="platform-label"><Laptop size={14} /> {environment.os}</span></div>
      </section>

      <section className="key-panel">
        <div className="panel-heading"><div><p className="eyebrow">CREDENTIAL</p><h2>连接词元神网关</h2></div><div className="key-security"><LockKeyhole size={15} /> 不会上传到配置助手</div></div>
        <div className="key-row">
          <div className="key-input-wrap"><KeyRound size={17} /><input type={showKey ? 'text' : 'password'} placeholder="粘贴用户 API Key" value={apiKey} onChange={(event) => setApiKey(event.target.value)} autoComplete="off" /><button className="input-action" title={showKey ? '隐藏 Key' : '显示 Key'} onClick={() => setShowKey(!showKey)}>{showKey ? <EyeOff size={17} /> : <Eye size={17} />}</button></div>
          <button className="primary-button" onClick={fetchModels} disabled={modelStatus === 'loading'}><Cloud size={17} />{modelStatus === 'loading' ? '检查中' : '检查连接'}</button>
        </div>
        <div className={`connection-line ${modelStatus}`}><span className="connection-mark">{modelStatus === 'ready' ? <CheckCircle2 size={15} /> : modelStatus === 'error' ? <AlertTriangle size={15} /> : <CircleDashed size={15} />}</span><span>{modelMessage}</span>{modelStatus === 'ready' && <span className="connection-endpoint">GET {props.models.length} models</span>}</div>
      </section>

      <section className="clients-section">
        <div className="section-heading"><div><p className="eyebrow">TARGETS</p><h2>选择要配置的工具</h2></div><span className="selected-count">{selectedCount} 个已选择</span></div>
        <div className="client-grid">
          {clientOrder.map((id) => <ClientCard key={id} id={id} status={clientMap.get(id)} checked={selected.includes(id)} onToggle={() => toggleClient(id)} models={models} model={modelByClient[id] || modelByClient.default || ''} onModelChange={(value) => setClientModel(id, value)} />)}
        </div>
      </section>

      <div className="action-bar"><div><strong>{selectedCount ? `将配置 ${selectedCount} 个工具` : '请选择工具'}</strong><span>配置前会自动创建可恢复备份</span></div><div className="action-buttons"><button className="secondary-button" onClick={previewConfiguration} disabled={busy !== '' || selectedCount === 0}><FileCheck2 size={17} />预览变更</button><button className="primary-button" onClick={applyConfiguration} disabled={busy !== '' || selectedCount === 0}><ClipboardCheck size={17} />{busy === 'configure' ? '写入中' : '一键配置'}</button></div></div>
    </div>
  );
}

function ClientCard({ id, status, checked, onToggle, models, model, onModelChange }: { id: ClientId; status?: ClientStatus; checked: boolean; onToggle: () => void; models: Model[]; model: string; onModelChange: (value: string) => void }) {
  const available = Boolean(status?.installed || status?.configExists);
  const state = status?.configState === 'invalid' ? 'invalid' : available ? 'available' : 'not-found';
  return <article className={`client-card ${checked ? 'checked' : ''}`}>
    <div className="client-card-top"><button className={`check-control ${checked ? 'checked' : ''}`} aria-label={`${checked ? '取消选择' : '选择'} ${clientCopy[id].short}`} onClick={onToggle}>{checked && <Check size={14} strokeWidth={3} />}</button><div className="client-symbol"><TerminalSquare size={19} /></div><div className="client-name"><strong>{clientCopy[id].short}</strong><span>{clientCopy[id].badge}</span></div><span className={`state-tag ${state}`}>{state === 'available' ? '可配置' : state === 'invalid' ? '需修复' : '未安装'}</span></div>
    <div className="client-card-path">{status?.configPath || '配置文件将自动创建'}</div>
    <div className="client-card-bottom"><label>默认模型</label><input list={`models-${id}`} value={model} placeholder="输入或选择模型" onChange={(event) => onModelChange(event.target.value)} disabled={!checked} /><datalist id={`models-${id}`}>{models.map((option) => <option key={option.id} value={option.id} />)}</datalist></div>
  </article>;
}

function Backups({ backups, busy, restore, refresh }: { backups: Backup[]; busy: string; restore: (id: string) => void; refresh: () => void }) {
  return <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon amber"><RotateCcw size={20} /></div><div><p className="eyebrow">RECOVERY</p><h2>配置备份</h2><p>每次写入都会保留原始文件，恢复不会覆盖其他备份。</p></div><button className="icon-button inline" title="刷新备份" onClick={refresh}><RefreshCw size={17} /></button></section><section className="backup-list">{backups.length === 0 ? <EmptyState icon={<RotateCcw size={22} />} title="暂无备份" text="完成一次配置后，备份会显示在这里。" /> : backups.map((backup) => <div className="backup-row" key={backup.id}><div className="backup-icon"><FileCheck2 size={18} /></div><div className="backup-details"><strong>{formatTime(backup.createdAt)}</strong><span>{backup.files.length} 个文件 · {backup.id}</span></div><button className="secondary-button compact" onClick={() => restore(backup.id)} disabled={busy === 'restore'}><RotateCcw size={15} />恢复</button></div>)}</section></div>;
}

function Updates({ update, busy, check, openDownload }: { update: UpdateInfo | null; busy: string; check: () => void; openDownload: () => void }) {
  return <div className="content-stack narrow-stack"><section className="page-intro"><div className="section-icon blue"><Download size={20} /></div><div><p className="eyebrow">RELEASE CHANNEL</p><h2>版本更新</h2><p>通过 HTTPS 清单检查最新安装包。</p></div><button className="primary-button" onClick={check} disabled={busy === 'update'}><RefreshCw size={16} className={busy === 'update' ? 'spin' : ''} />检查更新</button></section><section className="update-panel">{!update ? <EmptyState icon={<CircleDashed size={22} />} title="尚未检查" text="点击右上角检查当前版本。" /> : update.error ? <div className="update-state error"><AlertTriangle size={22} /><div><strong>检查失败</strong><span>{update.error}</span></div></div> : update.updateAvailable ? <div className="update-state ready"><div className="update-state-icon"><Download size={20} /></div><div><strong>发现新版本 v{update.latestVersion}</strong><span>当前版本 v{update.currentVersion}{update.publishedAt ? ` · ${update.publishedAt}` : ''}</span></div><button className="primary-button" onClick={openDownload}><ArrowUpRight size={16} />打开下载</button></div> : <div className="update-state"><div className="update-state-icon"><CheckCircle2 size={20} /></div><div><strong>已经是最新版本</strong><span>v{update.currentVersion} · 检查于 {formatTime(update.checkedAt)}</span></div></div>}{update?.releaseNotes && <div className="release-notes">{update.releaseNotes}</div>}</section></div>;
}

function PreviewModal({ preview, busy, close, apply }: { preview: Preview; busy: string; close: () => void; apply: () => void }) {
  return <div className="modal-backdrop" role="presentation"><section className="preview-modal" role="dialog" aria-modal="true" aria-labelledby="preview-title"><div className="modal-heading"><div><p className="eyebrow">WRITE PLAN</p><h2 id="preview-title">确认配置变更</h2></div><button className="icon-button" title="关闭" onClick={close}><X size={18} /></button></div>{preview.error ? <div className="modal-error"><AlertTriangle size={18} />{preview.error}</div> : <><div className="preview-summary"><FileCheck2 size={19} /><span>{preview.files.length} 个文件将被创建或更新</span><span className="preview-safe"><ShieldCheck size={15} />先备份</span></div><div className="file-list">{preview.files.map((file) => <div className="file-row" key={`${file.clientId}-${file.path}`}><span className={`file-action ${file.action}`}>{file.action === 'create' ? '+' : '~'}</span><div><strong>{clientCopy[file.clientId as ClientId]?.short || file.clientId}</strong><span>{file.path}</span></div></div>)}</div>{preview.warnings.length > 0 && <div className="warning-list">{preview.warnings.map((warning) => <div key={warning}><AlertTriangle size={15} />{warning}</div>)}</div>}<div className="modal-actions"><button className="secondary-button" onClick={close}>取消</button><button className="primary-button" onClick={apply} disabled={busy === 'configure'}><ClipboardCheck size={17} />{busy === 'configure' ? '写入中' : '确认并写入'}</button></div></>}</section></div>;
}

function Feedback({ tone, text, onClose }: { tone: 'success' | 'error' | 'neutral'; text: string; onClose: () => void }) {
  const icon = tone === 'success' ? <CheckCircle2 size={17} /> : tone === 'error' ? <AlertTriangle size={17} /> : <Settings2 size={17} />;
  return <div className={`feedback ${tone}`}>{icon}<span>{text}</span><button className="feedback-close" title="关闭" onClick={onClose}><X size={15} /></button></div>;
}

function EmptyState({ icon, title, text }: { icon: ReactNode; title: string; text: string }) {
  return <div className="empty-state"><div className="empty-icon">{icon}</div><strong>{title}</strong><span>{text}</span></div>;
}

function mockPreview(): Preview {
  return { files: [{ clientId: 'claude', path: '~/.claude/settings.json', action: 'update' }, { clientId: 'codex', path: '~/.codex/config.toml', action: 'create' }, { clientId: 'gemini', path: '~/.gemini/.env', action: 'update' }], warnings: [] };
}

function mockConfigure(): ConfigureResult {
  return { success: true, configured: ['claude', 'codex', 'gemini'], files: [], warnings: [], finishedAt: new Date().toISOString() };
}

export default App;
