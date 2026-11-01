import { useEffect, useRef, useState } from 'react';
import { Activity, ArrowRight, Play, Square, RefreshCw } from 'lucide-react';
import { GetRouterModels, GetModelRouterStatus, StartModelRouter, StopModelRouter, GetRouterAccountOptions, CreateToolKey } from '../wailsjs/go/main/App';

type Status = { running: boolean; model: string; alias: string; models?: string[]; address: string; requests: number; failures: number; lastError: string; backupPath: string };
type Group = { name: string; description: string; ratio: string; models: { id: string }[] };
type Key = { provisionId: string; group: string; groupDescription?: string; name?: string; models: { id: string }[]; existing?: boolean };
type AccountOptions = { groups: Group[]; existingKeys?: Key[] };
type Request = { apiKey: string; provisionId: string; model: string; defaultModel?: string; models?: string[]; useExistingKey: boolean; client?: string; claudeOneM?: boolean };
type RouterBridge = {
  GetRouterModels(request: Request): Promise<{ models: { id: string }[] }>;
  GetModelRouterStatus(): Promise<Status>;
  StartModelRouter(request: Request): Promise<Status>;
  StopModelRouter(): Promise<Status>;
  GetRouterAccountOptions(): Promise<AccountOptions>;
  CreateToolKey(request: { clientId: string; group: string }): Promise<Key>;
};
const bridge = (): RouterBridge | undefined => (window as unknown as { go?: { main?: { App?: unknown } } }).go?.main?.App
  ? { GetRouterModels, GetModelRouterStatus, StartModelRouter, StopModelRouter, GetRouterAccountOptions, CreateToolKey } : undefined;
const initial: Status = { running: false, model: '', alias: '', models: [], address: '', requests: 0, failures: 0, lastError: '', backupPath: '' };

export default function ModelRouter({ onLogin }: { onLogin?: () => void }) {
  const [status, setStatus] = useState(initial);
  const [mode, setMode] = useState<'auto' | 'manual'>('auto');
  const [client, setClient] = useState<'codex' | 'claude' | 'claude-desktop'>('codex');
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
  useEffect(() => {
    let active = true;
    const refresh = () => { void bridge()?.GetModelRouterStatus().then(value => { if (active) setStatus(value); }).catch(() => { if (active) setError('读取路由状态失败，请重新打开此页面'); }); };
    refresh(); const timer = window.setInterval(refresh, 2500);
    return () => { active = false; window.clearInterval(timer); };
  }, []);
  const existing = mode === 'auto';
  const clientLabel = client === 'claude-desktop' ? 'Claude Desktop' : client === 'claude' ? 'Claude Code' : 'Codex';
  function resetModels() { setModels([]); setModel(''); setSelectedModels([]); setDefaultModel(''); setError(''); setNotice(''); setSelectedGroup(''); setProvisionId(''); }
  async function loadAutomaticOptions() {
    const api = bridge(); if (!api) return;
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoadingOptions(true); setError('');
    try {
      const account = await api.GetRouterAccountOptions();
      setGroups(account.groups || []); setKeys(account.existingKeys || []);
      const ids = [...new Set([...(account.existingKeys || []).flatMap(k => k.models), ...(account.groups || []).flatMap(g => g.models)].map(m => m.id))].sort();
      setModels(ids);
      setModel(current => ids.includes(current) ? current : '');
      setSelectedModels(current => current.filter(id => ids.includes(id)));
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
        setStatus(await api.StopModelRouter());
        setNotice(`已停止路由并恢复原配置。请重新打开 ${clientLabel}。`);
      }
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(''); }
  }
  const locked = Boolean(busy) || status.running;
  const visible = models.filter(id => id.toLowerCase().includes(modelSearch.trim().toLowerCase()));
  const providers = [...new Set(visible.map(id => id.split(/[-/:]/)[0].toLowerCase()))].sort();
  const ratio = (value: string) => Number.parseFloat(value.replace(',', '.').replace(/[^0-9.]/g, '')) || Number.POSITIVE_INFINITY;
  const primaryModel = selectedModels[0] || model;
  const effectiveDefaultModel = defaultModel.trim() || primaryModel;
  const modelGroups = groups.filter(group => selectedModels.every(id => group.models.some(item => item.id === id))).sort((a, b) => ratio(a.ratio) - ratio(b.ratio) || a.name.localeCompare(b.name));
  const modelKeys = keys.filter(item => selectedModels.every(id => item.models.some(candidate => candidate.id === id)));
  function toggleModel(id: string, checked: boolean) {
    const ids = checked ? [...selectedModels, id] : selectedModels.filter(value => value !== id);
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
  return <div className="content-stack narrow-stack router-page">
    <div className="page-intro"><div><p className="eyebrow">模型路由</p><h2>在客户端里使用更多模型</h2><p>选择词元神 Key 和实际模型，一键连接 Claude、Gemini、Grok、DeepSeek 等。</p></div></div>
    {existing && <div role="status" aria-live="polite"><p>{loadingOptions ? (models.length ? '正在刷新模型和分组，可继续选择已有模型…' : '正在自动获取账号模型和分组，请稍候…') : '切换页面会保留模型列表和选择；需要更新时请点击刷新。'}</p><button className="secondary-button" disabled={loadingOptions || Boolean(busy) || status.running} onClick={() => void loadAutomaticOptions()}><RefreshCw size={15} className={loadingOptions ? 'spin' : ''} />{loadingOptions ? '获取中…' : '刷新模型和分组'}</button></div>}
    <section className="router-card">
      <h3>选择要接入的客户端</h3><div className="field-block"><label htmlFor="router-client">路由客户端</label><select id="router-client" value={client} disabled={locked} onChange={e=>setClient(e.target.value as 'codex'|'claude'|'claude-desktop')}><option value="codex">Codex</option><option value="claude">Claude Code CLI / 插件</option><option value="claude-desktop">Claude Desktop</option></select><p className="field-note">{client === 'claude-desktop' ? 'Claude Desktop 菜单显示所选实际模型名称，最多支持 200 个映射模型。启动后请完全退出并重新打开 Claude Desktop，并保持助手运行。' : client === 'claude' ? 'Claude Code 会通过 Anthropic Messages API 连接本地路由；启动后请重新打开 Claude Code。' : 'Codex 会通过本地路由连接所选模型；启动后请重新打开 Codex。'}</p></div>
      {client === 'claude' && <div className="field-block"><label><input type="checkbox" checked={claudeOneM} disabled={locked} onChange={e => setClaudeOneM(e.target.checked)} /> 启用 Claude 1M 上下文</label><p className="field-note">开启后写入 Claude Code 的 1,000,000 token 上下文和自动压缩配置；是否真正可用仍取决于上游模型和账号权限。</p></div>}
      <h3>1 · 选择用于连接的 Key</h3>
      <fieldset className="router-choices" disabled={loadingOptions}>
        <label><input type="radio" name="router-key-source" checked={mode === 'auto'} disabled={locked} onChange={() => { setMode('auto'); setKey(''); resetModels(); }} />自动模式：从账号模型和分组中选择</label>
        <label><input type="radio" name="router-key-source" checked={mode === 'manual'} disabled={locked} onChange={() => { setMode('manual'); resetModels(); }} />手动模式：输入已有 Key</label>
      </fieldset>
      {mode === 'manual' && <div className="field-block"><label htmlFor="router-key">词元神 API Key</label><input id="router-key" type="password" autoComplete="off" value={key} disabled={locked} placeholder="粘贴 Key 后读取模型" onChange={e => { setKey(e.target.value); resetModels(); }} /></div>}

      {existing && (groups.length > 0 || keys.length > 0) && <div className="field-block"><label htmlFor="router-group">账号可用模型分组</label><select id="router-group" value={provisionId || selectedGroup} onChange={e => { const value = e.target.value; const found = modelKeys.find(k => k.provisionId === value); setProvisionId(found?.provisionId || ''); setSelectedGroup(found?.group || value); }} disabled={locked}><option value="">请选择模型后选择分组</option>{modelKeys.map(k => <option key={k.provisionId} value={k.provisionId}>已有 Key · {k.group} · {groups.find(g => g.name === k.group)?.ratio || '—'}x</option>)}{modelGroups.map(g => <option key={g.name} value={g.name}>自动创建 Key · {g.name} · {g.ratio || '—'}x</option>)}</select><p className="field-note">{groups.find(g => g.name === selectedGroup)?.description || keys.find(k => k.provisionId === provisionId)?.groupDescription || '请选择分组'}{!provisionId && selectedGroup ? '；启动时会自动创建此分组 Key。' : ''}</p></div>}
      {mode === 'manual' && <button className="secondary-button" disabled={locked || !key.trim()} onClick={() => void run('models')}><RefreshCw size={15} />{busy === 'models' ? '读取中…' : '读取这个 Key 的模型'}</button>}
      <p className="field-note">自动模式会读取账号所有可用分组；手动模式只使用你输入的 Key，Key 不会保存到助手。</p>
    </section>
    <section className="router-card">
      <h3>2 · 选择真正回答你的模型</h3>
      <div className="field-block">
        <label htmlFor="router-model-search">模型映射（勾选可多选）</label>
        <input id="router-model-search" type="search" value={modelSearch} onChange={e => setModelSearch(e.target.value)} placeholder="搜索模型名称" />
        <div className="router-model-checklist" role="group" aria-label="选择映射模型">
          {providers.map(provider => <fieldset key={provider} disabled={locked}><legend>{provider.toUpperCase()}</legend>{visible.filter(id => id.split(/[-/:]/)[0].toLowerCase() === provider).map(id => <label key={id} className="router-model-choice"><input type="checkbox" checked={selectedModels.includes(id)} disabled={client === 'claude-desktop' && selectedModels.length >= 200 && !selectedModels.includes(id)} onChange={e => toggleModel(id, e.target.checked)} /><span>{id}</span></label>)}</fieldset>)}
          {!visible.length && <p>{models.length ? '没有匹配的模型；搜索不会清除已勾选项。' : '请先读取可用模型。'}</p>}
        </div>
        <p className="router-model-count">已选择 {selectedModels.length} 个映射模型{client === 'claude-desktop' ? ' / 200' : ''}{selectedModels.length ? ` · 默认第一项 ${selectedModels[0]}` : ''}</p>
        <p className="field-note">直接勾选即可多选，按勾选顺序保存。{client === 'claude-desktop' ? 'Claude Desktop 菜单显示实际模型名称。' : client === 'codex' ? '所选模型会出现在 Codex 的 /model 菜单中。' : 'Claude Code 的 Sonnet、Opus、Haiku 对应前三项；其他模型可通过 /model 输入实际模型名切换。'}</p>
      </div>
      <div className="field-block"><label htmlFor="router-default-model">{client === 'codex' ? 'Codex 默认请求模型' : '默认请求模型'}</label><input id="router-default-model" value={defaultModel} disabled={locked || !selectedModels.length} placeholder={primaryModel || '留空时使用映射第一行'} onChange={e => setDefaultModel(e.target.value)} /><p className="field-note">留空时默认使用映射第一行。Claude Code 会同步到 Sonnet、Opus、Haiku；Claude Desktop 显示实际模型名称，默认模型排在菜单首位。</p></div>
      <div className="router-mapping"><span>菜单显示名<br /><strong>{status.running ? (client === 'claude-desktop' ? status.model : status.alias) : effectiveDefaultModel || '等待选择'}</strong></span><ArrowRight size={22} /><span>实际请求模型<br /><strong>{status.running ? status.model : effectiveDefaultModel || '等待选择'}</strong></span></div>
      <p className="field-note">模型列表汇总账号所有可用分组，并按供应商分类；当前选择的分组 Key 必须实际拥有映射模型权限。Claude Desktop 支持最多 200 个映射模型，不再限于四个角色。</p>
    </section>
    <section className="router-card">
      <h3>3 · 一键启动，回到客户端使用</h3>
      <p>启动时自动备份并切换客户端配置；停止或正常退出助手时恢复。如果配置被其他程序修改，会保留修改和恢复记录。{client === 'claude-desktop' ? '完全退出并重新打开 Claude Desktop 后生效。' : '重新打开客户端、新建对话后生效。'}</p>
      <div className="router-actions"><button className="primary-button" disabled={locked || !selectedModels.length || (client === 'claude-desktop' && selectedModels.length > 200) || (existing && !provisionId && !selectedGroup) || (!existing && !key.trim())} onClick={() => void run('start')}><Play size={16} />{busy === 'start' ? '启动中…' : '一键启动并配置客户端'}</button><button className="secondary-button" disabled={Boolean(busy) || (!status.running && !status.lastError)} onClick={() => void run('stop')}><Square size={15} />{busy === 'stop' ? '恢复中…' : '停止并恢复配置'}</button></div>
      <div className="router-live" role="status"><Activity size={17} /><strong>{status.running ? '运行中' : '未启动'}</strong>{status.running && <span>请求 {status.requests} 次 · 失败 {status.failures} 次</span>}</div>
      {status.running && <p className="field-note">本机地址：{status.address} · 使用期间请保持助手运行。</p>}
      {(error || status.lastError) && <div className="router-error" role="alert">{error || status.lastError}</div>}
      {notice && <p className="router-notice" role="status">{notice}</p>}
      <p className="field-note">支持文本流式回复、图片输入、函数与补丁工具。目标模型须支持相应能力。暂不支持内置联网搜索、服务端对话续接及远端压缩；遇到不支持的输入会明确报错。长对话请新建会话。</p>
    </section>
  </div>;
}
