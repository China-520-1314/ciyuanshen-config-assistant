import { useEffect, useRef, useState } from 'react';
import { Play, Square, RefreshCw, Terminal, Monitor, Code2, Search, Check, X, Star, ShieldCheck, Layers } from 'lucide-react';
import { GetRouterModels, GetModelRouterStatuses, StartModelRouter, StopClientModelRouter, GetRouterAccountOptions, CreateToolKey } from '../wailsjs/go/main/App';

type Client = 'codex' | 'claude' | 'claude-desktop';
type Status = { client?: string; recoverable?: boolean; supported?: boolean; running: boolean; model: string; alias: string; models?: string[]; address: string; requests: number; failures: number; lastError: string; backupPath: string };
type Group = { name: string; description: string; ratio: string; models: { id: string }[] };
type Key = { provisionId: string; group: string; groupDescription?: string; name?: string; models: { id: string }[]; existing?: boolean };
type AccountOptions = { groups: Group[]; existingKeys?: Key[] };
type Request = { apiKey: string; provisionId: string; model: string; defaultModel?: string; models?: string[]; useExistingKey: boolean; client?: string; claudeOneM?: boolean };
type RouterBridge = {
  GetRouterModels(request: Request): Promise<{ models: { id: string }[] }>;
  GetModelRouterStatuses(): Promise<Record<Client, Status>>;
  StartModelRouter(request: Request): Promise<Status>;
  StopClientModelRouter(client: string): Promise<Status>;
  GetRouterAccountOptions(): Promise<AccountOptions>;
  CreateToolKey(request: { clientId: string; group: string }): Promise<Key>;
};
const bridge = (): RouterBridge | undefined => (window as unknown as { go?: { main?: { App?: unknown } } }).go?.main?.App
  ? { GetRouterModels, GetModelRouterStatuses, StartModelRouter, StopClientModelRouter, GetRouterAccountOptions, CreateToolKey } : undefined;
const initial: Status = { running: false, model: '', alias: '', models: [], address: '', requests: 0, failures: 0, lastError: '', backupPath: '' };

const clients: { id: Client; name: string; description: string; icon: typeof Terminal }[] = [
  { id: 'codex', name: 'Codex', description: '桌面端 / CLI', icon: Terminal },
  { id: 'claude', name: 'Claude Code', description: 'CLI / 编辑器插件', icon: Code2 },
  { id: 'claude-desktop', name: 'Claude Desktop', description: '桌面对话 / 模型菜单', icon: Monitor },
];

export default function ModelRouter({ onLogin }: { onLogin?: () => void }) {
  const [client, setClient] = useState<Client>('codex');
  const [visited, setVisited] = useState<Client[]>(['codex']);
  const [statuses, setStatuses] = useState<Record<Client, Status>>({ codex: initial, claude: initial, 'claude-desktop': initial });
  const [statusError, setStatusError] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  const accountCache = useRef<Promise<AccountOptions>>();
  const statusRevision = useRef(0);
  const loadOptions = (force = false) => {
    if (force || !accountCache.current) {
      accountCache.current = bridge()?.GetRouterAccountOptions().catch(e => { accountCache.current = undefined; throw e; });
    }
    return accountCache.current || Promise.reject(new Error('请在桌面安装版中读取账号模型'));
  };
  const updateStatus = (id: Client, status: Status) => {
    statusRevision.current++;
    setStatuses(previous => ({ ...previous, [id]: status }));
  };
  useEffect(() => {
    let active = true;
    let pending = false;
    const refresh = async () => {
      if (pending) return;
      pending = true;
      const revision = statusRevision.current;
      try {
        const value = await bridge()?.GetModelRouterStatuses();
        if (active && value && revision === statusRevision.current) { setStatuses(value); setStatusError(''); }
      } catch { if (active) setStatusError('路由状态读取失败，请刷新重试。'); }
      finally { pending = false; }
    };
    void refresh();
    const timer = window.setInterval(refresh, 2500);
    return () => { active = false; window.clearInterval(timer); };
  }, []);
  const running = clients.filter(item => statuses[item.id]?.running).length;
  const requests = clients.reduce((sum, item) => sum + (statuses[item.id]?.requests || 0), 0);
  return <div className="content-stack router-dashboard">
    <header className="router-dashboard-header"><div><p className="eyebrow">模型路由中心</p><h2>三个客户端，各自自由连接</h2><p>独立选择模型和分组，可同时运行；切换卡片不会中断其他客户端。</p></div><div className="router-overview"><span><strong>{running}<small> / 3</small></strong>运行中的客户端</span><span><strong>{requests}</strong>本次启动请求</span></div></header>
    <div className="router-client-grid" role="tablist" aria-label="路由客户端">
      {clients.map(item => {
        const state = statuses[item.id] || initial;
        const Icon = item.icon;
        return <button key={item.id} type="button" role="tab" aria-selected={client === item.id} aria-controls={item.id + '-panel'} id={item.id + '-tab'} className={'router-client-card' + (client === item.id ? ' selected' : '')} onClick={() => { setClient(item.id); setVisited(previous => previous.includes(item.id) ? previous : [...previous, item.id]); }}>
          <div className="router-client-top"><span className="router-client-icon"><Icon size={21} /></span><span className={'router-state-pill ' + (state.running ? 'running' : state.recoverable ? 'recovery' : '')}>{state.running ? '运行中' : state.recoverable ? '待恢复' : state.supported === false ? '当前系统不支持' : '未启动'}</span></div>
          <strong>{item.name}</strong><span>{item.description}</span><div className="router-client-model">{state.running || state.recoverable ? state.model || '已有恢复记录' : '选择模型，独立启动'}</div><div className="router-client-footer"><span>{state.models?.length || 0} 个模型</span><span>{state.requests || 0} 请求 · {state.failures || 0} 失败</span></div>
        </button>;
      })}
    </div>
    {statusError && <div className="router-error" role="alert">{statusError}<button className="secondary-button" disabled={refreshing} onClick={async () => { setRefreshing(true); try { const value = await bridge()?.GetModelRouterStatuses(); if (value) { setStatuses(value); setStatusError(''); } } catch { setStatusError('路由状态读取失败，请刷新重试。'); } finally { setRefreshing(false); } }}>刷新状态</button></div>}
    {visited.map(id => <div id={id + '-panel'} role="tabpanel" aria-labelledby={id + '-tab'} hidden={client !== id} key={id}><RouterPanel client={id} status={statuses[id] || initial} onStatus={value => updateStatus(id, value)} loadOptions={loadOptions} onLogin={onLogin} /></div>)}
  </div>;
}

function RouterPanel({ client, status, onStatus: setStatus, loadOptions, onLogin }: { client: Client; status: Status; onStatus: (status: Status) => void; loadOptions: (force?: boolean) => Promise<AccountOptions>; onLogin?: () => void }) {
  const [mode, setMode] = useState<'auto' | 'manual'>('auto');
  const [key, setKey] = useState('');
  const [models, setModels] = useState<string[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [keys, setKeys] = useState<Key[]>([]);
  const [selectedGroup, setSelectedGroup] = useState('');
  const [provisionId, setProvisionId] = useState('');
  const [model, setModel] = useState('');
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [defaultModel, setDefaultModel] = useState('');
  const [modelSearch, setModelSearch] = useState('');
  const [claudeOneM, setClaudeOneM] = useState(false);
  const [busy, setBusy] = useState('');
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [loadingOptions, setLoadingOptions] = useState(false);
  const loadingRef = useRef(false);
  const [onlySelected, setOnlySelected] = useState(false);
  const [providerFilter, setProviderFilter] = useState('');
  const hydrated = useRef(false);
  const currentStatus = useRef(status);
  currentStatus.current = status;
  useEffect(() => {
    if ((status.running || status.recoverable) && !hydrated.current) {
      hydrated.current = true;
      setSelectedModels(status.models || []);
      setModel(status.model);
      setDefaultModel(status.model);
    }
  }, [status.running, status.recoverable, status.model, status.models]);
  const existing = mode === 'auto';
  const clientLabel = client === 'claude-desktop' ? 'Claude Desktop' : client === 'claude' ? 'Claude Code' : 'Codex';
  function resetModels() { setModels([]); setModel(''); setSelectedModels([]); setDefaultModel(''); setError(''); setNotice(''); setSelectedGroup(''); setProvisionId(''); }
  async function loadAutomaticOptions(force = false) {
    const api = bridge(); if (!api) return;
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoadingOptions(true); setError('');
    try {
      const account = await loadOptions(force);
      setGroups(account.groups || []); setKeys(account.existingKeys || []);
      const ids = [...new Set([...(account.existingKeys || []).flatMap(k => k.models), ...(account.groups || []).flatMap(g => g.models)].map(m => m.id))].sort();
      setModels(ids);
      setModel(current => ids.includes(current) ? current : '');
      if (!currentStatus.current.running && !currentStatus.current.recoverable) setSelectedModels(current => current.filter(id => ids.includes(id)));
      setProvisionId(current => (account.existingKeys || []).some(k => k.provisionId === current) ? current : '');
      setSelectedGroup(current => (account.groups || []).some(g => g.name === current) ? current : '');
      setNotice(ids.length ? `已读取 ${ids.length} 个模型。` : '账号没有可用模型');
    } catch (e) { const message = e instanceof Error ? e.message : String(e); setError(message); if (/请先登录|登录词元神账号/.test(message)) onLogin?.(); } finally { loadingRef.current = false; setLoadingOptions(false); }
  }
  useEffect(() => { if (mode === 'auto') void loadAutomaticOptions(); }, [mode]);
  async function run(action: 'models' | 'start' | 'stop') {
    if (loadingOptions && action === 'start') { setError('请等待分组刷新完成后启动，模型仍可继续选择。'); return; }
    const api = bridge(); if (!api) { setError('请在桌面安装版中使用本地路由'); return; }
    if (action === 'start' && existing && provisionId && !keys.some(k => k.provisionId === provisionId && selectedModels.every(id => k.models.some(m => m.id === id)))) { setError('当前 Key 不支持全部已选模型，请重新选择分组。'); return; }
    if (action === 'start' && client === 'claude-desktop' && selectedModels.length > 200) { setError('Claude Desktop 最多支持 200 个映射模型，请减少选择。'); return; }
    setBusy(action); setError(''); setNotice('');
    const request = { apiKey: key, provisionId, model: defaultModel.trim() || selectedModels[0] || model, defaultModel: defaultModel.trim(), models: selectedModels, useExistingKey: existing, client, claudeOneM };
    try {
      if (action === 'models') {
        setModels([]); setModel('');
        if (existing) { await loadAutomaticOptions(); return; }
        const result = await api.GetRouterModels(request);
        const ids = result.models.map(item => item.id).sort(); setModels(ids); setModel(''); setSelectedModels([]); setDefaultModel('');
        setNotice(ids.length ? `已读取 ${ids.length} 个模型。请选择支持对话和工具调用的模型。` : '此 Key 没有可用模型');
      } else if (action === 'start') {
        let startRequest = request;
        if (existing && !provisionId) {
          if (!selectedGroup) throw new Error('请选择该模型对应的分组');
          setNotice('正在创建所选分组 Key…');
          const created = await api.CreateToolKey({ clientId: 'router', group: selectedGroup });
          setProvisionId(created.provisionId); setKeys(previous => [...previous, created]);
          startRequest = { ...request, provisionId: created.provisionId };
        }
        setStatus(await api.StartModelRouter(startRequest)); setKey('');
        setNotice(`路由已启动。请${client === 'claude-desktop' ? '完全退出' : '关闭'}并重新打开 ${clientLabel}，再新建对话后使用。期间请保持助手运行。`);
      } else {
        setStatus(await api.StopClientModelRouter(client));
        setNotice(`已停止路由并恢复原配置。请重新打开 ${clientLabel}。`);
      }
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(''); }
  }
  const locked = Boolean(busy) || status.running || Boolean(status.recoverable);
  const availableModels = [...new Set([...models, ...(status.running || status.recoverable ? status.models || [] : [])])];
  const providerOf = (id: string) => id.split(/[-/:]/)[0].toLowerCase();
  const visible = availableModels.filter(id => id.toLowerCase().includes(modelSearch.trim().toLowerCase()) && (!providerFilter || providerOf(id) === providerFilter) && (!onlySelected || selectedModels.includes(id)));
  const providers = [...new Set(availableModels.map(providerOf))].sort();
  const ratio = (value: string) => Number.parseFloat(value.replace(',', '.').replace(/[^0-9.]/g, '')) || Number.POSITIVE_INFINITY;
  const primaryModel = selectedModels[0] || model;
  const effectiveDefaultModel = defaultModel.trim() || primaryModel;
  const modelGroups = groups.filter(group => selectedModels.every(id => group.models.some(item => item.id === id))).sort((a, b) => ratio(a.ratio) - ratio(b.ratio) || a.name.localeCompare(b.name));
  const modelKeys = keys.filter(item => selectedModels.every(id => item.models.some(candidate => candidate.id === id)));
  function toggleModel(id: string, checked: boolean) {
    applySelection(checked ? [...selectedModels, id] : selectedModels.filter(value => value !== id));
  }
  function applySelection(ids: string[]) {
    if (client === 'claude-desktop' && ids.length > 200) { setError('Claude Desktop 最多支持 200 个映射模型。'); return; }
    setSelectedModels(ids); setModel(ids[0] || ''); setError('');
    if (defaultModel && selectedModels.includes(defaultModel) && !ids.includes(defaultModel)) setDefaultModel('');
    if (!existing) return;
    const supportsAll = (items: { id: string }[]) => ids.every(value => items.some(item => item.id === value));
    const match = keys.find(k => k.provisionId === provisionId && supportsAll(k.models)) || keys.find(k => supportsAll(k.models));
    const candidates = groups.filter(g => supportsAll(g.models)).sort((a, b) => ratio(a.ratio) - ratio(b.ratio) || a.name.localeCompare(b.name));
    if (match) { setProvisionId(match.provisionId); setSelectedGroup(match.group); setNotice(`已选择支持全部已选模型的 Key：${match.group || '未命名分组'}。`); }
    else if (candidates.length) { setProvisionId(''); setSelectedGroup(candidates[0].name); setNotice(`已选择支持全部已选模型的最低倍率分组：${candidates[0].name}。`); }
    else { setProvisionId(''); setSelectedGroup(''); setNotice('没有同时支持全部已选模型的分组，请调整模型选择或使用手动 Key。'); }
  }
  const selectedLimit = client === 'claude-desktop' ? 200 : Number.POSITIVE_INFINITY;
  const canStart = !locked && status.supported !== false && selectedModels.length > 0 && selectedModels.length <= selectedLimit && !loadingOptions && (existing ? Boolean(provisionId || selectedGroup) : Boolean(key.trim()));
  return <div className="router-workspace">
    <section className="router-card router-models-card">
      <div className="router-section-heading"><div><p className="eyebrow">01 / 模型列表</p><h3>选择 {clientLabel} 使用的模型</h3></div><span className="router-count-badge">{selectedModels.length}{client === 'claude-desktop' ? ' / 200' : ''} 已选</span></div>
      <div className="router-model-toolbar">
        <div className="router-search"><Search size={17} /><input aria-label="搜索模型名称" type="search" value={modelSearch} onChange={e => setModelSearch(e.target.value)} placeholder="搜索模型名称…" /></div>
        <select aria-label="供应商筛选" value={providerFilter} onChange={e => setProviderFilter(e.target.value)}><option value="">全部供应商</option>{providers.map(provider => <option key={provider} value={provider}>{provider.toUpperCase()}</option>)}</select>
      </div>
      <div className="router-selection-tools"><label><input type="checkbox" checked={onlySelected} onChange={e => setOnlySelected(e.target.checked)} />仅看已选</label><span>{visible.length} 个结果</span><button type="button" disabled={locked || !visible.length || selectedModels.length >= selectedLimit} onClick={() => applySelection([...new Set([...selectedModels, ...visible])].slice(0, selectedLimit))}>勾选筛选结果</button><button type="button" disabled={locked || !selectedModels.length} onClick={() => applySelection([])}>清空选择</button></div>
      <div className="router-model-checklist" role="group" aria-label="选择映射模型">
        {providers.filter(provider => visible.some(id => providerOf(id) === provider)).map(provider => <fieldset key={provider} disabled={locked}><legend>{provider.toUpperCase()}<span>{visible.filter(id => providerOf(id) === provider).length}</span></legend>{visible.filter(id => providerOf(id) === provider).map(id => <label key={id} className={'router-model-choice' + (selectedModels.includes(id) ? ' checked' : '')}><input type="checkbox" aria-label={id} checked={selectedModels.includes(id)} disabled={selectedModels.length >= selectedLimit && !selectedModels.includes(id)} onChange={e => toggleModel(id, e.target.checked)} /><span>{id}</span>{effectiveDefaultModel === id && <em>默认</em>}</label>)}</fieldset>)}
        {!visible.length && <div className="router-empty"><Layers size={30} /><strong>{loadingOptions ? '正在加载模型…' : availableModels.length ? '没有匹配的模型' : '先连接账号或读取 Key'}</strong><p>{availableModels.length ? '试试其他关键词，已勾选的模型会保留。' : '右侧选择账号分组，或输入已有 Key 读取模型。'}</p></div>}
      </div>
      <p className="field-note">{client === 'claude-desktop' ? '菜单显示实际模型名称，最多 200 个；选择顺序会保留。' : client === 'codex' ? '勾选的模型会出现在 Codex 的 /model 菜单。' : '前三项对应 Sonnet、Opus、Haiku；其他模型可通过 /model 输入模型名切换。'}</p>
    </section>
    <aside className="router-settings-column">
      <section className="router-card">
        <div className="router-section-heading"><div><p className="eyebrow">02 / 连接设置</p><h3>Key 与模型分组</h3></div><button type="button" className="router-icon-button" title="刷新模型和分组" aria-label="刷新模型和分组" disabled={loadingOptions || locked || !existing} onClick={() => void loadAutomaticOptions(true)}><RefreshCw size={17} className={loadingOptions ? 'spin' : ''} /></button></div>
        <div className="router-source-tabs" role="group" aria-label="Key 来源"><button type="button" className={existing ? 'active' : ''} disabled={locked || loadingOptions} onClick={() => { setMode('auto'); setKey(''); resetModels(); }}>账号分组</button><button type="button" className={!existing ? 'active' : ''} disabled={locked || loadingOptions} onClick={() => { setMode('manual'); resetModels(); }}>手动 Key</button></div>
        {existing ? <div className="field-block"><label htmlFor={client + '-group'}>支持全部已选模型的分组</label><select id={client + '-group'} value={provisionId || selectedGroup} onChange={e => { const value = e.target.value; const found = modelKeys.find(k => k.provisionId === value); setProvisionId(found?.provisionId || ''); setSelectedGroup(found?.group || value); }} disabled={locked || loadingOptions}><option value="">请选择分组</option>{modelKeys.map(k => <option key={k.provisionId} value={k.provisionId}>已有 Key · {k.group} · {groups.find(g => g.name === k.group)?.ratio || '—'}x</option>)}{modelGroups.map(g => <option key={g.name} value={g.name}>创建 Key · {g.name} · {g.ratio || '—'}x</option>)}</select><p className="field-note">{groups.find(g => g.name === selectedGroup)?.description || keys.find(k => k.provisionId === provisionId)?.groupDescription || '勾选模型后自动匹配可用 Key 和分组。'}{!provisionId && selectedGroup ? ' 启动时创建此分组 Key。' : ''}</p></div> : <><div className="field-block"><label htmlFor={client + '-key'}>词元神 API Key</label><input id={client + '-key'} type="password" autoComplete="off" value={key} disabled={locked} placeholder="粘贴 API Key" onChange={e => { setKey(e.target.value); resetModels(); }} /></div><button className="secondary-button" disabled={locked || !key.trim()} onClick={() => void run('models')}><RefreshCw size={15} />{busy === 'models' ? '读取中…' : '读取模型'}</button></>}
        <p className="router-security-note"><ShieldCheck size={15} />Key 仅用于本次连接，不写入页面缓存。</p>
      </section>
      <section className="router-card">
        <div className="router-section-heading"><div><p className="eyebrow">03 / 默认模型</p><h3>优先使用哪个模型</h3></div><Star size={18} /></div>
        <div className="field-block"><label htmlFor={client + '-default'}>默认请求模型</label><select id={client + '-default'} value={selectedModels.includes(defaultModel) ? defaultModel : ''} disabled={locked || !selectedModels.length} onChange={e => setDefaultModel(e.target.value)}><option value="">使用第一项{primaryModel ? ' · ' + primaryModel : ''}</option>{selectedModels.map(id => <option key={id} value={id}>{id}</option>)}</select></div>
        {client === 'claude' && <label className="router-inline-check"><input type="checkbox" checked={claudeOneM} disabled={locked} onChange={e => setClaudeOneM(e.target.checked)} />启用 1M 上下文（需模型支持）</label>}
        <div className="router-selected-summary"><span>已选顺序</span>{selectedModels.slice(0, 8).map((id, index) => <div key={id}><b>{index + 1}</b><span title={id}>{id}</span><button type="button" aria-label={'移除 ' + id} disabled={locked} onClick={() => toggleModel(id, false)}><X size={13} /></button></div>)}{selectedModels.length > 8 && <small>还有 {selectedModels.length - 8} 个，可使用「仅看已选」管理</small>}{!selectedModels.length && <small>从左侧勾选模型</small>}</div>
      </section>
    </aside>
    <section className="router-control-bar">
      <div className="router-control-summary"><span className={'router-status-dot' + (status.running ? ' running' : '')} /><div><strong>{status.running ? clientLabel + ' 路由运行中' : status.recoverable ? clientLabel + ' 待恢复配置' : clientLabel + ' 已就绪'}</strong><p>{status.running ? '请求 ' + status.requests + ' 次 · 失败 ' + status.failures + ' 次' : '仅启停当前客户端，其他路由保持运行'}</p></div></div>
      <div className="router-actions"><button className="secondary-button" disabled={Boolean(busy) || (!status.running && !status.recoverable)} onClick={() => void run('stop')}><Square size={15} />{busy === 'stop' ? '恢复中…' : '停止并恢复'}</button><button className="primary-button" disabled={!canStart} onClick={() => void run('start')}><Play size={16} />{busy === 'start' ? '启动中…' : '启动 ' + clientLabel}</button></div>
      {status.address && <code className="router-address">{status.address}</code>}
      {(error || status.lastError) && <div className="router-error" role="alert">{error || status.lastError}</div>}
      {notice && <p className="router-notice" role="status"><Check size={15} />{notice}</p>}
      {status.supported === false && <p className="router-notice">Claude Desktop 路由仅支持 Windows 和 macOS；仍可恢复已有配置。</p>}
    </section>
    <details className="router-help"><summary>生效方式与使用说明</summary><p>启动后重新打开对应客户端并新建对话；Claude Desktop 需完全退出后再打开。请保持助手运行，正常退出会分别恢复所有客户端配置。支持文本、图片、函数及补丁工具；上游模型需具备相应能力。暂不支持内置联网搜索和服务端对话续接。</p></details>
  </div>;
}
